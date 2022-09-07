// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************

// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"errors"
	"fmt"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/utils/hashing"
	"github.com/chain4travel/caminogo/utils/math"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

var (
	errUnknownSpendMode  = errors.New("unknown spend mode")
	errUnknownOwnersType = errors.New("unknown owners")
	errUnknownOwners     = errors.New("owner of produced utxo isn't presented in consumed utxo owners")
	errCantSign          = errors.New("can't sign")
)

type spendMode uint8

const (
	spendModeBond spendMode = iota
	spendModeDeposit
)

var spendModeStrings = map[spendMode]string{
	spendModeBond:    "bond",
	spendModeDeposit: "deposit",
}

func (mode spendMode) String() string {
	return spendModeStrings[mode]
}

func (mode spendMode) Verify() error {
	if mode != spendModeBond && mode != spendModeDeposit {
		return errUnknownSpendMode
	}
	return nil
}

// spend the provided amount while deducting the provided fee.
// Arguments:
// - [keys] are the owners of the funds
// - [totalAmountToSpend] is the amount of funds that are trying to be spended (changed their state)
// - [totalAmountToBurn] is the amount of AVAX that should be burned
// - [changeAddr] is the address that change, if there is any, is sent to
// - [spendMode] in what way tokens will be spended (bonded / deposited)
// Returns:
// - [inputs] the inputs that should be consumed to fund the outputs
// - [returnedOutputs] the outputs that should be immediately returned to the
//                     UTXO set
// - [notLockedOuts] the outputs produced as result of spending and should be counted as not locked
// - [lockedOuts] the outputs produced as result of spending and should be counted as locked
// - [inputIndexes] input indexes that produced outputs (output[i] produced by inputs[inputIndexes[i]]). First for not locked outs, then for locked
// - [signers] the proof of ownership of the funds being moved
func (vm *VM) spend(
	keys []*crypto.PrivateKeySECP256K1R,
	totalAmountToSpend uint64,
	totalAmountToBurn uint64,
	changeAddr ids.ShortID,
	spendMode spendMode,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // nonTransitionedOutputs
	[]*avax.TransferableOutput, // transitionedOutputs
	[]uint32, // lockedOutInIndexes
	[][]*crypto.PrivateKeySECP256K1R, // signers
	error,
) {
	if err := spendMode.Verify(); err != nil {
		return nil, nil, nil, nil, nil, err
	}

	addrs := ids.NewShortSet(len(keys)) // The addresses controlled by [keys]
	for _, key := range keys {
		addrs.Add(key.PublicKey().Address())
	}
	utxos, err := avax.GetAllUTXOs(vm.internalState, addrs) // The UTXOs controlled by [keys]
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("couldn't get UTXOs: %w", err)
	}

	lockedUTXOState := vm.internalState.LockedUTXOsChainState()

	kc := secp256k1fx.NewKeychain(keys...) // Keychain consumes UTXOs and creates new ones

	// Minimum time this transaction will be issued at
	now := uint64(vm.clock.Time().Unix())

	ins := []*avax.TransferableInput{}
	notLockedOuts := []*avax.TransferableOutput{}
	lockedOuts := []*avax.TransferableOutput{}
	signers := [][]*crypto.PrivateKeySECP256K1R{}
	notLockedOutInputIDs := []ids.ID{}
	lockedOutInputIDs := []ids.ID{}

	// Amount of AVAX that has been spended
	amountSpended := uint64(0)

	// Consume locked UTXOs
	for _, utxo := range utxos {
		// If we have consumed more AVAX than we are trying to spend, then we
		// have no need to consume more locked AVAX
		if amountSpended >= totalAmountToSpend {
			break
		}

		if assetID := utxo.AssetID(); assetID != vm.ctx.AVAXAssetID {
			continue // We only care about staking AVAX, so ignore other assets
		}

		if utxoLockState := lockedUTXOState.GetUTXOLockState(utxo.InputID()); !utxoLockState.isLocked() {
			// This output isn't locked, so it will be handled
			// during the next iteration of the UTXO set
			continue
		} else if spendMode == spendModeBond && utxoLockState.isBonded() ||
			spendMode == spendModeDeposit && utxoLockState.isDeposited() {
			// This output can't be spended with that spendMode
			continue
		}

		innerOut, ok := utxo.Out.(*secp256k1fx.TransferOutput)
		if !ok {
			// We only know how to clone secp256k1 outputs for now
			continue
		}

		inIntf, inSigners, err := kc.Spend(utxo.Out, now)
		if err != nil {
			// We couldn't spend the output, so move on to the next one
			continue
		}
		in, ok := inIntf.(avax.TransferableIn)
		if !ok { // should never happen
			vm.ctx.Log.Warn("expected input to be avax.TransferableIn but is %T", inIntf)
			continue
		}

		// The remaining value is initially the full value of the input
		remainingValue := in.Amount()

		// Spend any value that should be spended
		amountToSpend := math.Min64(
			totalAmountToSpend-amountSpended, // Amount we still need to spend
			remainingValue,                   // Amount available to spend
		)
		amountSpended += amountToSpend
		remainingValue -= amountToSpend

		// Add the input to the consumed inputs
		ins = append(ins, &avax.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
			In:     in,
		})
		inputID := utxo.InputID()

		// Add the output to the transitioned outputs
		lockedOuts = append(lockedOuts, &avax.TransferableOutput{
			Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
			Out: &secp256k1fx.TransferOutput{
				Amt:          amountToSpend,
				OutputOwners: innerOut.OutputOwners,
			},
		})
		lockedOutInputIDs = append(lockedOutInputIDs, inputID)

		if remainingValue > 0 {
			// This input provided more value than was needed to be spended.
			// Some of it must be returned
			notLockedOuts = append(notLockedOuts, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt:          remainingValue,
					OutputOwners: innerOut.OutputOwners,
				},
			})
			notLockedOutInputIDs = append(notLockedOutInputIDs, inputID)
		}

		// Add the signers needed for this input to the set of signers
		signers = append(signers, inSigners)
	}

	// Amount of AVAX that has been burned
	amountBurned := uint64(0)

	// consume or/and burn unlocked utxos
	for _, utxo := range utxos {
		// If we have burned more AVAX then we need to, then we have no need to
		// consume more AVAX
		if amountSpended >= totalAmountToSpend && amountBurned >= totalAmountToBurn {
			break
		}

		if assetID := utxo.AssetID(); assetID != vm.ctx.AVAXAssetID {
			continue // We only care about burning AVAX, so ignore other assets
		}

		if utxoLockState := lockedUTXOState.GetUTXOLockState(utxo.InputID()); utxoLockState.isLocked() {
			// This output is currently locked, so this output can't be
			// burned. Additionally, it may have already been consumed
			// above. Regardless, we skip to the next UTXO
			continue
		}

		inIntf, inSigners, err := kc.Spend(utxo.Out, now)
		if err != nil {
			// We couldn't spend this UTXO, so we skip to the next one
			continue
		}
		in, ok := inIntf.(avax.TransferableIn)
		if !ok {
			// Because we only use the secp Fx right now, this should never
			// happen
			continue
		}

		// The remaining value is initially the full value of the input
		remainingValue := in.Amount()

		// Burn any value that should be burned
		amountToBurn := math.Min64(
			totalAmountToBurn-amountBurned, // Amount we still need to burn
			remainingValue,                 // Amount available to burn
		)
		amountBurned += amountToBurn
		remainingValue -= amountToBurn

		// Spend any value that should be spended
		amountToSpend := math.Min64(
			totalAmountToSpend-amountSpended, // Amount we still need to spend
			remainingValue,                   // Amount available to spend
		)
		amountSpended += amountToSpend
		remainingValue -= amountToSpend

		// Add the input to the consumed inputs
		ins = append(ins, &avax.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
			In:     in,
		})
		inputID := utxo.InputID()

		if amountToSpend > 0 {
			// Some of this input was put for spending
			lockedOuts = append(lockedOuts, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: amountToSpend,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						// ! @evlekht this violates no-transfer rule of p-chain
						// ! cause it basically transfer part of the funds to change address
						// ! And what about initial owners? We have asset owned by 5 keys,
						// ! 10% of it spended for bonding (staking) or depositng
						// ! and then it suddenly owned by another key ??
						Addrs: []ids.ShortID{changeAddr},
					},
				},
			})
			lockedOutInputIDs = append(lockedOutInputIDs, inputID)
		}

		if remainingValue > 0 {
			// This input had extra value, so some of it must be returned
			notLockedOuts = append(notLockedOuts, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: remainingValue,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						// ! @evlekht this violates no-transfer rule of p-chain
						// ! cause it basically transfer part of the funds to change address
						// ! And what about initial owners? We have asset owned by 5 people,
						// ! 10% of it spended and the rest is returned as change
						// ! to just one address ??
						Addrs: []ids.ShortID{changeAddr},
					},
				},
			})
			notLockedOutInputIDs = append(notLockedOutInputIDs, inputID)
		}

		// Add the signers needed for this input to the set of signers
		signers = append(signers, inSigners)
	}

	if amountBurned < totalAmountToBurn || amountSpended < totalAmountToSpend {
		return nil, nil, nil, nil, nil, fmt.Errorf(
			"provided keys have balance (unlocked, locked) (%d, %d) but need (%d, %d)",
			amountBurned, amountSpended, totalAmountToBurn, totalAmountToSpend)
	}

	avax.SortTransferableInputsWithSigners(ins, signers) // sort inputs and keys
	avax.SortTransferableOutputs(notLockedOuts, Codec)   // sort outputs
	avax.SortTransferableOutputs(lockedOuts, Codec)      // sort outputs

	inputIndexesMap := make(map[ids.ID]uint32, len(ins))
	for inputIndex, in := range ins {
		inputIndexesMap[in.InputID()] = uint32(inputIndex)
	}

	inputIndexes := make([]uint32, len(notLockedOutInputIDs)+len(lockedOutInputIDs))
	for i, inputID := range notLockedOutInputIDs {
		inputIndexes[i] = uint32(inputIndexesMap[inputID])
	}
	offset := len(notLockedOutInputIDs)
	for i, inputID := range lockedOutInputIDs {
		inputIndexes[offset+i] = uint32(inputIndexesMap[inputID])
	}

	return ins, notLockedOuts, lockedOuts, inputIndexes, signers, nil
}

// authorize an operation on behalf of the named subnet with the provided keys.
func (vm *VM) authorize(
	vs MutableState,
	subnetID ids.ID,
	keys []*crypto.PrivateKeySECP256K1R,
) (
	verify.Verifiable, // Input that names owners
	[]*crypto.PrivateKeySECP256K1R, // Keys that prove ownership
	error,
) {
	subnetTx, _, err := vs.GetTx(subnetID)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to fetch subnet %s: %w",
			subnetID,
			err,
		)
	}
	subnet, ok := subnetTx.UnsignedTx.(*UnsignedCreateSubnetTx)
	if !ok {
		return nil, nil, errWrongTxType
	}

	// Make sure the owners of the subnet match the provided keys
	owner, ok := subnet.Owner.(*secp256k1fx.OutputOwners)
	if !ok {
		return nil, nil, errUnknownOwnersType
	}

	// Add the keys to a keychain
	kc := secp256k1fx.NewKeychain(keys...)

	// Make sure that the operation is valid after a minimum time
	now := uint64(vm.clock.Time().Unix())

	// Attempt to prove ownership of the subnet
	indices, signers, matches := kc.Match(owner, now)
	if !matches {
		return nil, nil, errCantSign
	}

	return &secp256k1fx.Input{SigIndices: indices}, signers, nil
}

// Verify that [tx] is semantically valid.
// [db] should not be committed if an error is returned
// [ins] and [outs] are the inputs and outputs of [tx].
// [creds] are the credentials of [tx], which allow [ins] to be spent.
// Precondition: [tx] has already been syntactically verified
func (vm *VM) semanticVerifySpend(
	utxoDB UTXOGetter,
	tx UnsignedTx,
	ins []*avax.TransferableInput,
	outs []*avax.TransferableOutput,
	creds []verify.Verifiable,
	feeAmount uint64,
	feeAssetID ids.ID,
) error {
	utxos := make([]*avax.UTXO, len(ins))
	for index, input := range ins {
		utxo, err := utxoDB.GetUTXO(input.InputID())
		if err != nil {
			return fmt.Errorf(
				"failed to read consumed UTXO %s due to: %w",
				&input.UTXOID,
				err,
			)
		}
		utxos[index] = utxo
	}

	return vm.semanticVerifySpendUTXOs(tx, utxos, ins, outs, creds, feeAmount, feeAssetID)
}

// Verify that [tx] is semantically valid.
// [ins] and [outs] are the inputs and outputs of [tx].
// [creds] are the credentials of [tx], which allow [ins] to be spent.
// [utxos[i]] is the UTXO being consumed by [ins[i]]
// Precondition: [tx] has already been syntactically verified
func (vm *VM) semanticVerifySpendUTXOs(
	tx UnsignedTx,
	utxos []*avax.UTXO,
	ins []*avax.TransferableInput,
	outs []*avax.TransferableOutput,
	creds []verify.Verifiable,
	feeAmount uint64,
	feeAssetID ids.ID,
) error {
	if len(ins) != len(creds) {
		return fmt.Errorf(
			"there are %d inputs but %d credentials. Should be same number",
			len(ins),
			len(creds),
		)
	}
	if len(ins) != len(utxos) {
		return fmt.Errorf(
			"there are %d inputs but %d utxos. Should be same number",
			len(ins),
			len(utxos),
		)
	}
	for _, cred := range creds { // Verify credentials are well-formed.
		if err := cred.Verify(); err != nil {
			return err
		}
	}

	// Track the amount of unlocked transfers
	unlockedProduced := feeAmount
	unlockedConsumed := uint64(0)

	// Track the amount of locked transfers and their owners
	// locktime -> ownerID -> amount
	lockedProduced := make(map[uint64]map[ids.ID]uint64)
	lockedConsumed := make(map[uint64]map[ids.ID]uint64)

	for index, input := range ins {
		utxo := utxos[index] // The UTXO consumed by [input]

		if assetID := utxo.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}
		if assetID := input.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}

		// Verify that this tx's credentials allow [in] to be spent
		if err := vm.fx.VerifyTransfer(tx, input.In, creds[index], utxo.Out); err != nil {
			return fmt.Errorf("failed to verify transfer: %w", err)
		}

		amount := input.In.Amount()

		if true {
			newUnlockedConsumed, err := math.Add64(unlockedConsumed, amount)
			if err != nil {
				return err
			}
			unlockedConsumed = newUnlockedConsumed
			continue
		}

		owned, ok := utxo.Out.(Owned)
		if !ok {
			return errUnknownOwners
		}
		owner := owned.Owners()
		ownerBytes, err := Codec.Marshal(CodecVersion, owner)
		if err != nil {
			return fmt.Errorf("couldn't marshal owner: %w", err)
		}
		ownerID := hashing.ComputeHash256Array(ownerBytes)
		owners, ok := lockedConsumed[0]
		if !ok {
			owners = make(map[ids.ID]uint64)
			lockedConsumed[0] = owners
		}
		newAmount, err := math.Add64(owners[ownerID], amount)
		if err != nil {
			return err
		}
		owners[ownerID] = newAmount
	}

	for _, out := range outs {
		if assetID := out.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}

		output := out.Output()
		amount := output.Amount()

		if true {
			newUnlockedProduced, err := math.Add64(unlockedProduced, amount)
			if err != nil {
				return err
			}
			unlockedProduced = newUnlockedProduced
			continue
		}

		owned, ok := output.(Owned)
		if !ok {
			return errUnknownOwners
		}
		owner := owned.Owners()
		ownerBytes, err := Codec.Marshal(CodecVersion, owner)
		if err != nil {
			return fmt.Errorf("couldn't marshal owner: %w", err)
		}
		ownerID := hashing.ComputeHash256Array(ownerBytes)
		owners, ok := lockedProduced[0]
		if !ok {
			owners = make(map[ids.ID]uint64)
			lockedProduced[0] = owners
		}
		newAmount, err := math.Add64(owners[ownerID], amount)
		if err != nil {
			return err
		}
		owners[ownerID] = newAmount
	}

	// Make sure that for each locktime, tokens produced <= tokens consumed
	for locktime, producedAmounts := range lockedProduced {
		consumedAmounts := lockedConsumed[locktime]
		for ownerID, producedAmount := range producedAmounts {
			consumedAmount := consumedAmounts[ownerID]

			if producedAmount > consumedAmount {
				increase := producedAmount - consumedAmount
				if increase > unlockedConsumed {
					return fmt.Errorf(
						"address %s produces %d unlocked and consumes %d unlocked for locktime %d",
						ownerID,
						increase,
						unlockedConsumed,
						locktime,
					)
				}
				unlockedConsumed -= increase
			}
		}
	}

	// More unlocked tokens produced than consumed. Invalid.
	if unlockedProduced > unlockedConsumed {
		return fmt.Errorf(
			"tx produces more unlocked (%d) than it consumes (%d)",
			unlockedProduced,
			unlockedConsumed,
		)
	}
	return nil
}

// Verify that [inputs], their [inputIndexes] and [outputs] are syntacticly valid in conjunction.
// Arguments:
// - [inputs] Inputs produces this outputs
// - [inputIndexes] inputs[inputIndexes[i]] produce output[i]
// - [outputs] outputs produced by [inputs]
// Precondition: [tx] has already been syntactically verified
func syntacticVerifyInputIndexes(
	inputs []*avax.TransferableInput,
	inputIndexes []uint32,
	outputs []*avax.TransferableOutput,
) error {
	if len(inputs) > 4_294_967_295 { // max uint32
		return fmt.Errorf("inputs len is to big")
	}

	producedAmount := make(map[uint32]uint64, len(inputs))

	for outputIndex, out := range outputs {
		inputIndex := inputIndexes[outputIndex]

		if inputs[inputIndex].AssetID() != out.AssetID() {
			return fmt.Errorf("input[%d] assetID isn't equal to produced output[%d] assetID",
				inputIndex, outputIndex)
		}

		newProducedAmount, err := math.Add64(producedAmount[inputIndex], out.Out.Amount())
		if err != nil {
			return err
		}
		producedAmount[inputIndex] = newProducedAmount
	}

	for inputIndex, in := range inputs {
		if in.In.Amount() < producedAmount[uint32(inputIndex)] {
			return fmt.Errorf("input[%d] produces more tokens than it consumes", inputIndex)
		}
	}

	return nil
}

// Removes the UTXOs consumed by [ins] from the UTXO set
func consumeInputs(
	utxoDB UTXODeleter,
	ins []*avax.TransferableInput,
) {
	for _, input := range ins {
		utxoDB.DeleteUTXO(input.InputID())
	}
}

// Adds the UTXOs created by [outs] to the UTXO set.
// [txID] is the ID of the tx that created [outs].
func produceOutputs(
	utxoDB UTXOAdder,
	txID ids.ID,
	assetID ids.ID,
	outs []*avax.TransferableOutput,
) {
	for index, out := range outs {
		utxoDB.AddUTXO(&avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(index),
			},
			Asset: avax.Asset{ID: assetID},
			Out:   out.Output(),
		})
	}
}
