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
	errWrongLocktime    = errors.New("wrong locktime reported")
	errWrongInputState  = errors.New("wrong input state")
	errUnknownSpendMode = errors.New("unknown spend mode")
	errUnknownOwners    = errors.New("unknown owners")
	errCantSign         = errors.New("can't sign")
)

type spendMode uint8

const (
	spendModeBond spendMode = iota
	spendModeDeposite
	spendModeUnbond
	spendModeUndeposit
)

func (mode spendMode) isValid() bool {
	switch mode {
	case spendModeDeposite, spendModeBond, spendModeUndeposit, spendModeUnbond:
		return true
	}
	return false
}

// spend the provided amount while deducting the provided fee.
// Arguments:
// - [keys] are the owners of the funds
// - [amount] is the amount of funds that are trying to be spended (changed their state)
// - [fee] is the amount of AVAX that should be burned
// - [changeAddr] is the address that change, if there is any, is sent to
// Returns:
// - [inputs] the inputs that should be consumed to fund the outputs
// - [returnedOutputs] the outputs that should be immediately returned to the
//                     UTXO set
// - [createdOutputs] the outputs that was created as result of spending
// - [signers] the proof of ownership of the funds being moved
func (vm *VM) spend(
	keys []*crypto.PrivateKeySECP256K1R,
	totalAmountToSpend uint64,
	fee uint64,
	changeAddr ids.ShortID,
	spendMode spendMode,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // returnedOutputs
	[]*avax.TransferableOutput, // createdOutputs
	[][]*crypto.PrivateKeySECP256K1R, // signers
	error,
) {
	if !spendMode.isValid() {
		return nil, nil, nil, nil, errUnknownSpendMode
	}

	addrs := ids.NewShortSet(len(keys)) // The addresses controlled by [keys]
	for _, key := range keys {
		addrs.Add(key.PublicKey().Address())
	}
	utxos, err := avax.GetAllUTXOs(vm.internalState, addrs) // The UTXOs controlled by [keys]
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("couldn't get UTXOs: %w", err)
	}

	kc := secp256k1fx.NewKeychain(keys...) // Keychain consumes UTXOs and creates new ones

	// Minimum time this transaction will be issued at
	now := uint64(vm.clock.Time().Unix())

	ins := []*avax.TransferableInput{}
	returnedOuts := []*avax.TransferableOutput{}
	createdOuts := []*avax.TransferableOutput{}
	signers := [][]*crypto.PrivateKeySECP256K1R{}

	// Amount of AVAX that has been spended
	amountSpended := uint64(0)
	//TODO@clean loops
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

		// out.IsLocked() for prioritizing spending locked utxos over unlocked ones
		out, ok := utxo.Out.(*PChainOut)
		if !ok || !out.IsLocked() || !canBeSpended(out.State, spendMode) {
			// This output isn't locked or can't be spended with that spendMode,
			// so it will be handled during the next iteration of the UTXO set
			continue
		}

		inner, ok := out.TransferableOut.(*secp256k1fx.TransferOutput)
		if !ok {
			// We only know how to clone secp256k1 outputs for now
			continue
		}

		inIntf, inSigners, err := kc.Spend(out.TransferableOut, now)
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
			In: &PChainIn{
				State:          out.State,
				TransferableIn: in,
			},
		})

		// Add the output to the created outputs
		createdOuts = append(createdOuts, &avax.TransferableOutput{
			Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
			Out: &PChainOut{
				State: stateAfterSpending(out.State, spendMode),
				TransferableOut: &secp256k1fx.TransferOutput{
					Amt:          amountToSpend,
					OutputOwners: inner.OutputOwners,
				},
			},
		})

		if remainingValue > 0 {
			// This input provided more value than was needed to be spended.
			// Some of it must be returned
			returnedOuts = append(returnedOuts, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &PChainOut{
					State: out.State,
					TransferableOut: &secp256k1fx.TransferOutput{
						Amt:          remainingValue,
						OutputOwners: inner.OutputOwners,
					},
				},
			})
		}

		// Add the signers needed for this input to the set of signers
		signers = append(signers, inSigners)
	}

	// Amount of AVAX that has been burned
	amountBurned := uint64(0)

	// consume or/and burn unlocked utxos
	for _, utxo := range utxos {
		shouldBeBurned := amountBurned < fee
		shouldBeSpended := amountSpended < totalAmountToSpend
		// If we have burned more AVAX then we need to, then we have no need to
		// consume more AVAX
		if !shouldBeSpended && !shouldBeBurned {
			break
		}

		if assetID := utxo.AssetID(); assetID != vm.ctx.AVAXAssetID {
			continue // We only care about burning AVAX, so ignore other assets
		}

		out := utxo.Out
		spendedOutState := PUTXOStateTransferable
		if inner, ok := out.(*PChainOut); ok {
			out = inner.TransferableOut
			spendedOutState = inner.State
		}

		shouldBeBurned = shouldBeBurned && canBeBurned(spendedOutState, spendMode)
		shouldBeSpended = shouldBeSpended && canBeSpended(spendedOutState, spendMode)

		if !shouldBeBurned && !shouldBeSpended {
			// This output can't be burned or spended with this spendMode
			continue
		}

		inIntf, inSigners, err := kc.Spend(out, now)
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

		if shouldBeBurned {
			// Burn any value that should be burned
			amountToBurn := math.Min64(
				fee-amountBurned, // Amount we still need to burn
				remainingValue,   // Amount available to burn
			)
			amountBurned += amountToBurn
			remainingValue -= amountToBurn
		}

		if shouldBeSpended {
			// Spend any value that should be spended
			amountToSpend := math.Min64(
				totalAmountToSpend-amountSpended, // Amount we still need to spend
				remainingValue,                   // Amount available to spend
			)
			amountSpended += amountToSpend
			remainingValue -= amountToSpend

			if amountToSpend > 0 {
				// Some of this input was put for spending
				createdOuts = append(createdOuts, &avax.TransferableOutput{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					Out: &PChainOut{
						State: stateAfterSpending(spendedOutState, spendMode),
						TransferableOut: &secp256k1fx.TransferOutput{
							Amt: amountToSpend,
							OutputOwners: secp256k1fx.OutputOwners{
								Locktime:  0,
								Threshold: 1,
								Addrs:     []ids.ShortID{changeAddr},
							},
						},
					},
				})
			}
		}

		// Add the input to the consumed inputs
		ins = append(ins, &avax.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
			In:     in,
		})

		if remainingValue > 0 {
			// This input had extra value, so some of it must be returned
			returnedOuts = append(returnedOuts, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: remainingValue,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						Addrs:     []ids.ShortID{changeAddr},
					},
				},
			})
		}

		// Add the signers needed for this input to the set of signers
		signers = append(signers, inSigners)
	}

	if amountBurned < fee || amountSpended < totalAmountToSpend {
		return nil, nil, nil, nil, fmt.Errorf(
			"provided keys have balance (unlocked, locked) (%d, %d) but need (%d, %d)",
			amountBurned, amountSpended, fee, totalAmountToSpend)
	}

	avax.SortTransferableInputsWithSigners(ins, signers) // sort inputs and keys
	avax.SortTransferableOutputs(returnedOuts, Codec)    // sort outputs
	avax.SortTransferableOutputs(createdOuts, Codec)     // sort outputs

	return ins, returnedOuts, createdOuts, signers, nil
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
		return nil, nil, errUnknownOwners
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
	spendMode spendMode,
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

	return vm.semanticVerifySpendUTXOs(tx, utxos, ins, outs, creds, feeAmount, feeAssetID, spendMode)
}

// Verify that [tx] is semantically valid.
// [db] should not be committed if an error is returned
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
	spendMode spendMode,
) error {
	if !spendMode.isValid() {
		return errUnknownSpendMode
	}

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

	// ownerID -> PUTXOState -> amount
	consumed := make(map[ids.ID]map[PUTXOState]uint64)
	produced := make(map[ids.ID]map[PUTXOState]uint64)

	for index, input := range ins {
		utxo := utxos[index] // The UTXO consumed by [input]

		if assetID := utxo.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}
		if assetID := input.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}

		spendedOut := utxo.Out
		spendedOutState := PUTXOStateTransferable
		// Set [locktime] to this UTXO's locktime, if applicable
		if inner, ok := spendedOut.(*PChainOut); ok {
			spendedOut = inner.TransferableOut
			spendedOutState = inner.State
		}

		in := input.In

		// Unwrapping input if it's needed. Checking that input state == consumed output state.
		// ok == false means that input isn't PChainIn, so its PUTXOStateTransferable by default
		if inner, ok := in.(*PChainIn); !ok && spendedOutState != PUTXOStateTransferable {
			return errWrongInputState
		} else if ok {
			if inner.State != spendedOutState {
				return errWrongInputState
			}
			in = inner.TransferableIn
		}

		// Verify that this tx's credentials allow [in] to be spent
		if err := vm.fx.VerifyTransfer(tx, in, creds[index], spendedOut); err != nil {
			return fmt.Errorf("failed to verify transfer: %w", err)
		}

		amount := in.Amount()

		// getting spended out owners
		owned, ok := spendedOut.(Owned)
		if !ok {
			return errUnknownOwners
		}
		spendedOutOwner := owned.Owners()

		// getting spended out ownerID
		ownerBytes, err := Codec.Marshal(CodecVersion, spendedOutOwner)
		if err != nil {
			return fmt.Errorf("couldn't marshal owner: %w", err)
		}
		ownerID := hashing.ComputeHash256Array(ownerBytes)

		// TODO@ comment
		consumedFromOwner, ok := consumed[ownerID]
		if !ok {
			consumedFromOwner = make(map[PUTXOState]uint64)
			consumed[ownerID] = consumedFromOwner
		}
		newAmount, err := math.Add64(consumedFromOwner[spendedOutState], amount)
		if err != nil {
			return err
		}
		consumedFromOwner[spendedOutState] = newAmount
	}

	for _, out := range outs {
		if assetID := out.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}

		producedOut := out.Output()
		producedOutState := PUTXOStateTransferable
		// Set [locktime] to this output's locktime, if applicable
		if inner, ok := producedOut.(*PChainOut); ok {
			producedOut = inner.TransferableOut
			producedOutState = inner.State
		}

		amount := producedOut.Amount()

		// getting prdoduced out owners
		owned, ok := producedOut.(Owned)
		if !ok {
			return errUnknownOwners
		}
		owner := owned.Owners()

		// getting spended out ownerID
		ownerBytes, err := Codec.Marshal(CodecVersion, owner)
		if err != nil {
			return fmt.Errorf("couldn't marshal owner: %w", err)
		}
		ownerID := hashing.ComputeHash256Array(ownerBytes)

		// TODO@ comment
		producedForOwner, ok := produced[ownerID]
		if !ok {
			producedForOwner = make(map[PUTXOState]uint64)
			produced[ownerID] = producedForOwner
		}
		newAmount, err := math.Add64(producedForOwner[producedOutState], amount)
		if err != nil {
			return err
		}
		producedForOwner[producedOutState] = newAmount
	}

	// deposite:   PUTXOStateTransferable       reduced => PUTXOStateDeposited          increased
	// deposite:   PUTXOStateBonded             reduced => PUTXOStateDepositedAndBonded increased
	// bond:       PUTXOStateTransferable       reduced => PUTXOStateBonded             increased
	// bond:       PUTXOStateDeposited          reduced => PUTXOStateDepositedAndBonded increased
	// undeposite: PUTXOStateDeposited          reduced => PUTXOStateTransferable       increased
	// undeposite: PUTXOStateDepositedAndBonded reduced => PUTXOStateBonded             increased
	// unbond:     PUTXOStateBonded             reduced => PUTXOStateTransferable       increased
	// unbond:     PUTXOStateDepositedAndBonded reduced => PUTXOStateDeposited          increased

	// deposite:   PUTXOStateTransferable       abs diff >= PUTXOStateDeposited          abs diff
	// deposite:   PUTXOStateBonded             abs diff >= PUTXOStateDepositedAndBonded abs diff
	// bond:       PUTXOStateTransferable       abs diff >= PUTXOStateBonded             abs diff
	// bond:       PUTXOStateDeposited          abs diff >= PUTXOStateDepositedAndBonded abs diff
	// undeposite: PUTXOStateDeposited          abs diff >= PUTXOStateTransferable       abs diff
	// undeposite: PUTXOStateDepositedAndBonded abs diff >= PUTXOStateBonded             abs diff
	// unbond:     PUTXOStateBonded             abs diff >= PUTXOStateTransferable       abs diff
	// unbond:     PUTXOStateDepositedAndBonded abs diff >= PUTXOStateDeposited          abs diff

	for ownerID, consumedFromOwner := range consumed {
		producedForOwner := produced[ownerID] // TODO@ can it be nil?

		switch spendMode {
		case spendModeDeposite:
			if !(consumedFromOwner[PUTXOStateTransferable] >= producedForOwner[PUTXOStateTransferable] ||
				consumedFromOwner[PUTXOStateBonded] >= producedForOwner[PUTXOStateBonded] ||
				consumedFromOwner[PUTXOStateDeposited] <= producedForOwner[PUTXOStateDeposited] ||
				consumedFromOwner[PUTXOStateDepositedAndBonded] <= producedForOwner[PUTXOStateDepositedAndBonded]) {
				return fmt.Errorf("") // TODO@
			}

			transferableDiff := consumedFromOwner[PUTXOStateTransferable] - producedForOwner[PUTXOStateTransferable]
			bondedDiff := consumedFromOwner[PUTXOStateBonded] - producedForOwner[PUTXOStateBonded]
			depositedDiff := producedForOwner[PUTXOStateDeposited] - consumedFromOwner[PUTXOStateDeposited]
			depositedAndBondedDiff := producedForOwner[PUTXOStateDepositedAndBonded] - consumedFromOwner[PUTXOStateDepositedAndBonded]

			if !(transferableDiff >= depositedDiff || bondedDiff >= depositedAndBondedDiff) {
				return fmt.Errorf("") // TODO@
			}
		case spendModeBond:
			if !(consumedFromOwner[PUTXOStateTransferable] >= producedForOwner[PUTXOStateTransferable] ||
				consumedFromOwner[PUTXOStateDeposited] >= producedForOwner[PUTXOStateDeposited] ||
				consumedFromOwner[PUTXOStateBonded] <= producedForOwner[PUTXOStateBonded] ||
				consumedFromOwner[PUTXOStateDepositedAndBonded] <= producedForOwner[PUTXOStateDepositedAndBonded]) {
				return fmt.Errorf("") // TODO@
			}

			transferableDiff := consumedFromOwner[PUTXOStateTransferable] - producedForOwner[PUTXOStateTransferable]
			depositedDiff := consumedFromOwner[PUTXOStateDeposited] - producedForOwner[PUTXOStateDeposited]
			bondedDiff := producedForOwner[PUTXOStateBonded] - consumedFromOwner[PUTXOStateBonded]
			depositedAndBondedDiff := producedForOwner[PUTXOStateDepositedAndBonded] - consumedFromOwner[PUTXOStateDepositedAndBonded]

			if !(transferableDiff >= bondedDiff || depositedDiff >= depositedAndBondedDiff) {
				return fmt.Errorf("") // TODO@
			}
		case spendModeUndeposit:
			if !(consumedFromOwner[PUTXOStateDeposited] >= producedForOwner[PUTXOStateDeposited] ||
				consumedFromOwner[PUTXOStateDepositedAndBonded] >= producedForOwner[PUTXOStateDepositedAndBonded] ||
				consumedFromOwner[PUTXOStateTransferable] <= producedForOwner[PUTXOStateTransferable] ||
				consumedFromOwner[PUTXOStateBonded] <= producedForOwner[PUTXOStateBonded]) {
				return fmt.Errorf("") // TODO@
			}

			depositedDiff := consumedFromOwner[PUTXOStateDeposited] - producedForOwner[PUTXOStateDeposited]
			depositedAndBondedDiff := consumedFromOwner[PUTXOStateDepositedAndBonded] - producedForOwner[PUTXOStateDepositedAndBonded]
			transferableDiff := producedForOwner[PUTXOStateTransferable] - consumedFromOwner[PUTXOStateTransferable]
			bondedDiff := producedForOwner[PUTXOStateBonded] - consumedFromOwner[PUTXOStateBonded]

			if !(depositedDiff >= transferableDiff || depositedAndBondedDiff >= bondedDiff) {
				return fmt.Errorf("") // TODO@
			}
		case spendModeUnbond:
			if !(consumedFromOwner[PUTXOStateBonded] >= producedForOwner[PUTXOStateBonded] ||
				consumedFromOwner[PUTXOStateDepositedAndBonded] >= producedForOwner[PUTXOStateDepositedAndBonded] ||
				consumedFromOwner[PUTXOStateTransferable] <= producedForOwner[PUTXOStateTransferable] ||
				consumedFromOwner[PUTXOStateDeposited] <= producedForOwner[PUTXOStateDeposited]) {
				return fmt.Errorf("") // TODO@
			}

			bondedDiff := consumedFromOwner[PUTXOStateBonded] - producedForOwner[PUTXOStateBonded]
			depositedAndBondedDiff := consumedFromOwner[PUTXOStateDepositedAndBonded] - producedForOwner[PUTXOStateDepositedAndBonded]
			transferableDiff := producedForOwner[PUTXOStateTransferable] - consumedFromOwner[PUTXOStateTransferable]
			depositedDiff := producedForOwner[PUTXOStateDeposited] - consumedFromOwner[PUTXOStateDeposited]

			if !(bondedDiff >= transferableDiff || depositedAndBondedDiff >= depositedDiff) {
				return fmt.Errorf("") // TODO@
			}
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

func canBeSpended(utxoState PUTXOState, spendMode spendMode) bool {
	switch spendMode {
	case spendModeDeposite:
		return utxoState&PUTXOStateDeposited == 0
	case spendModeBond:
		return utxoState&PUTXOStateBonded == 0
	case spendModeUndeposit:
		return utxoState&PUTXOStateDeposited == 1
	case spendModeUnbond:
		return utxoState&PUTXOStateBonded == 1
	}
	return false
}

func canBeBurned(utxoState PUTXOState, spendMode spendMode) bool {
	switch spendMode {
	case spendModeDeposite, spendModeBond:
		return utxoState == PUTXOStateTransferable
	case spendModeUndeposit:
		return utxoState&PUTXOStateBonded == 0
	case spendModeUnbond:
		return utxoState&PUTXOStateDeposited == 0
	}
	return false
}

// stateAfterSpending will only work for correct utxoState that can be spended with correct spendMode
func stateAfterSpending(utxoState PUTXOState, spendMode spendMode) PUTXOState {
	switch spendMode {
	case spendModeDeposite:
		return utxoState | PUTXOStateDeposited
	case spendModeBond:
		return utxoState | PUTXOStateBonded
	case spendModeUndeposit:
		return utxoState ^ PUTXOStateDeposited
	case spendModeUnbond:
		return utxoState ^ PUTXOStateBonded
	}
	return 0
}
