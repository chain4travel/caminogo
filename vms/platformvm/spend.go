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
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/chain4travel/caminogo/codec"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/utils/math"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

var (
	errLockedFundsNotMarkedAsLocked = errors.New("locked funds not marked as locked")
	errWrongLockState               = errors.New("wrong lock state reported")
	errInvalidTargetLockState       = errors.New("invalid target lock state")
	errUnknownOwnersType            = errors.New("unknown owners")
	errCantSign                     = errors.New("can't sign")
	errInputsCredentialsMismatch    = errors.New("number of inputs is different from number of credentials")
	errInputsUTXOSMismatch          = errors.New("number of inputs is different from number of utxos")
	errWrongCredentials             = errors.New("wrong credentials")
)

// spend the provided amount while deducting the provided fee.
// Arguments:
// - [keys] are the owners of the funds
// - [totalAmountToSpend] is the amount of funds that are trying to be spended (changed their state)
// - [totalAmountToBurn] is the amount of AVAX that should be burned
// - [appliedLockState] is lockState that will be applied to consumed outs state
// Returns:
// - [inputs] the inputs that should be consumed to fund the outputs
// - [outputs] the outputs produced as result of spending
// - [inputIndexes] input indexes that produced outputs (output[i] produced by inputs[inputIndexes[i]])
// - [signers] the proof of ownership of the funds being moved
func (vm *VM) spend(
	keys []*crypto.PrivateKeySECP256K1R,
	totalAmountToSpend uint64,
	totalAmountToBurn uint64,
	appliedLockState LockState,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // outputs
	[]uint32, // inputIndexes
	[][]*crypto.PrivateKeySECP256K1R, // signers
	error,
) {
	if appliedLockState != LockStateBonded && appliedLockState != LockStateDeposited {
		return nil, nil, nil, nil, errInvalidTargetLockState
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
	outs := []*avax.TransferableOutput{}
	signers := [][]*crypto.PrivateKeySECP256K1R{}
	outInputIDs := []ids.ID{}

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

		out, ok := utxo.Out.(*LockedOut)
		if !ok {
			// This output isn't locked, so it will be handled during the next
			// iteration of the UTXO set
			continue
		} else if appliedLockState&^out.LockState != appliedLockState {
			// This output can't be locked with target lockState
			continue
		}

		innerOut, ok := out.TransferableOut.(*secp256k1fx.TransferOutput)
		if !ok {
			// We only know how to clone secp256k1 outputs for now
			continue
		}

		inIntf, inSigners, err := kc.Spend(innerOut, now)
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
			In: &LockedIn{
				LockState:      out.LockState,
				TransferableIn: in,
			},
		})
		inputID := utxo.InputID()

		// Add the output to the transitioned outputs
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
			Out: &LockedOut{
				LockState: out.LockState | appliedLockState,
				TransferableOut: &secp256k1fx.TransferOutput{
					Amt:          amountToSpend,
					OutputOwners: innerOut.OutputOwners,
				},
			},
		})
		outInputIDs = append(outInputIDs, inputID)

		if remainingValue > 0 {
			// This input provided more value than was needed to be spended.
			// Some of it must be returned
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &LockedOut{
					LockState: out.LockState,
					TransferableOut: &secp256k1fx.TransferOutput{
						Amt:          remainingValue,
						OutputOwners: innerOut.OutputOwners,
					},
				},
			})
			outInputIDs = append(outInputIDs, inputID)
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

		if _, ok := utxo.Out.(*LockedOut); ok {
			// This output is currently locked, so this output can't be
			// burned. Additionally, it may have already been consumed
			// above. Regardless, we skip to the next UTXO
			continue
		}

		innerOut, ok := utxo.Out.(*secp256k1fx.TransferOutput)
		if !ok {
			// We only know how to clone secp256k1 outputs for now
			continue
		}

		inIntf, inSigners, err := kc.Spend(innerOut, now)
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
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &LockedOut{
					LockState: appliedLockState,
					TransferableOut: &secp256k1fx.TransferOutput{
						Amt:          amountToSpend,
						OutputOwners: innerOut.OutputOwners,
					},
				},
			})
			outInputIDs = append(outInputIDs, inputID)
		}

		if remainingValue > 0 {
			// This input had extra value, so some of it must be returned
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt:          remainingValue,
					OutputOwners: innerOut.OutputOwners,
				},
			})
			outInputIDs = append(outInputIDs, inputID)
		}

		// Add the signers needed for this input to the set of signers
		signers = append(signers, inSigners)
	}

	if amountBurned < totalAmountToBurn || amountSpended < totalAmountToSpend {
		return nil, nil, nil, nil, fmt.Errorf(
			"provided keys have balance (unlocked, locked) (%d, %d) but need (%d, %d)",
			amountBurned, amountSpended, totalAmountToBurn, totalAmountToSpend)
	}

	avax.SortTransferableInputsWithSigners(ins, signers) // sort inputs and keys
	// ! @evlekht sort logic is partially duplicated, unhappy with this
	sort.Sort(&innerSortTransferableOutputsAndInputIDs{ // outputs and inputIDs
		outs:     outs,
		inputIDs: outInputIDs,
		codec:    Codec,
	})

	inputIndexesMap := make(map[ids.ID]uint32, len(ins))
	for inputIndex, in := range ins {
		inputIndexesMap[in.InputID()] = uint32(inputIndex)
	}

	inputIndexes := make([]uint32, len(outInputIDs))
	for i, inputID := range outInputIDs {
		inputIndexes[i] = inputIndexesMap[inputID]
	}

	return ins, outs, inputIndexes, signers, nil
}

// unlock consumes locked utxos created by lock transactions and owned by keys and produce unlocked outs
//
// Arguments:
// - [lockTxIDs] ids of lock transactions
// - [keys] owners of the funds
// - [removedLockState] is type of lock that that function will try to unlock (it's either Bonded or Deposited)
// - [needSigners] do inputs need to be signed
//
// Returns:
// - [inputs] produced inputs
// - [outputs] produced outputs
// - [inputIndexes] input indexes that produced outputs (output[i] produced by inputs[inputIndexes[i]])
// - [signers] the proof of ownership of the funds being moved
func (vm *VM) unlock(
	state MutableState,
	lockTxIDs []ids.ID,
	removedLockState LockState, //nolint // * @evlekht must be fixed with deposit PR
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // outputs
	[]uint32, // lockedOutInIndexes
	error,
) {
	if removedLockState != LockStateBonded && removedLockState != LockStateDeposited {
		return nil, nil, nil, errInvalidTargetLockState
	}

	lockedUTXOsChainState := state.LockedUTXOsChainState()

	var utxos []*avax.UTXO
	for _, lockTxID := range lockTxIDs {
		bondedUTXOIDs := lockedUTXOsChainState.GetBondedUTXOIDs(lockTxID)
		for utxoID := range bondedUTXOIDs {
			utxo, err := state.GetUTXO(utxoID)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("couldn't get UTXO %s: %w", utxoID, err)
			}
			utxos = append(utxos, utxo)
		}
	}

	return vm.unlockUTXOs(utxos, removedLockState)
}

// unlockUTXOs consumes locked utxos owned by keys and produce unlocked outs
// Arguments:
// - [utxos] utxos that will be used to consume and unlock
// - [keys] owners of the funds
// - [removedLockState] is type of lock that that function will try to unlock
//               (it's either Bonded or Deposited)
// - [needSigners] do inputs need to be signed
// Returns:
// - [inputs] produced inputs
// - [outputs] produced outputs
// - [inputIndexes] input indexes that produced outputs (output[i] produced by inputs[inputIndexes[i]])
// - [signers] the proof of ownership of the funds being moved
func (vm *VM) unlockUTXOs(
	utxos []*avax.UTXO,
	removedLockState LockState,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // outputs
	[]uint32, // outInputIndexes
	error,
) {
	if removedLockState != LockStateBonded && removedLockState != LockStateDeposited {
		return nil, nil, nil, errInvalidTargetLockState
	}

	ins := []*avax.TransferableInput{}
	outs := []*avax.TransferableOutput{}
	signers := [][]*crypto.PrivateKeySECP256K1R{}
	outInputIDs := []ids.ID{}

	for _, utxo := range utxos {
		out, ok := utxo.Out.(*LockedOut)
		if !ok {
			// This output isn't locked
			continue
		} else if removedLockState&out.LockState != removedLockState {
			// This output doesn't have required lockState
			continue
		}

		innerOut, ok := out.TransferableOut.(*secp256k1fx.TransferOutput)
		if !ok {
			// We only know how to clone secp256k1 outputs for now
			continue
		}

		// Add the input to the consumed inputs
		ins = append(ins, &avax.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
			In: &LockedIn{
				LockState: out.LockState,
				TransferableIn: &secp256k1fx.TransferInput{
					Amt:   out.Amount(),
					Input: secp256k1fx.Input{},
				},
			},
		})
		inputID := utxo.InputID()

		if newLockState := out.LockState &^ removedLockState; newLockState.isLocked() {
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &LockedOut{
					LockState: newLockState,
					TransferableOut: &secp256k1fx.TransferOutput{
						Amt:          innerOut.Amount(),
						OutputOwners: innerOut.OutputOwners,
					},
				},
			})
		} else {
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt:          innerOut.Amount(),
					OutputOwners: innerOut.OutputOwners,
				},
			})
		}
		outInputIDs = append(outInputIDs, inputID)
	}

	avax.SortTransferableInputsWithSigners(ins, signers) // sort inputs and keys
	sort.Sort(&innerSortTransferableOutputsAndInputIDs{  // sort outputs and inputIDs
		outs:     outs,
		inputIDs: outInputIDs,
		codec:    Codec,
	})

	inputIndexesMap := make(map[ids.ID]uint32, len(ins))
	for inputIndex, in := range ins {
		inputIndexesMap[in.InputID()] = uint32(inputIndex)
	}

	inputIndexes := make([]uint32, len(outInputIDs))
	for i, inputID := range outInputIDs {
		inputIndexes[i] = inputIndexesMap[inputID]
	}

	return ins, outs, inputIndexes, nil
}

type innerSortTransferableOutputsAndInputIDs struct {
	outs     []*avax.TransferableOutput
	inputIDs []ids.ID
	codec    codec.Manager
}

func (outs *innerSortTransferableOutputsAndInputIDs) Less(i, j int) bool {
	iOut := outs.outs[i]
	jOut := outs.outs[j]

	iAssetID := iOut.AssetID()
	jAssetID := jOut.AssetID()

	switch bytes.Compare(iAssetID[:], jAssetID[:]) {
	case -1:
		return true
	case 1:
		return false
	}

	iBytes, err := outs.codec.Marshal(CodecVersion, &iOut.Out)
	if err != nil {
		return false
	}
	jBytes, err := outs.codec.Marshal(CodecVersion, &jOut.Out)
	if err != nil {
		return false
	}
	return bytes.Compare(iBytes, jBytes) == -1
}
func (outs *innerSortTransferableOutputsAndInputIDs) Len() int { return len(outs.outs) }
func (outs *innerSortTransferableOutputsAndInputIDs) Swap(i, j int) {
	outs.outs[j], outs.outs[i] = outs.outs[i], outs.outs[j]
	outs.inputIDs[j], outs.inputIDs[i] = outs.inputIDs[i], outs.inputIDs[j]
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
// [utxoDB] should not be committed if an error is returned
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

// TODO
// Verify that [tx] is semantically valid.
// Verify that [tx] is semantically valid. Meaning:
//
// - consumed no less tokens, than produced;
// - ins len equal to creds len;
// - ins and utxos have expected assetID;
// - ins have same lockState as utxos;
// - transfer from utxo to in is valid
//
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
		return errInputsCredentialsMismatch
	}
	if len(ins) != len(utxos) {
		return errInputsUTXOSMismatch
	}
	for _, cred := range creds { // Verify credentials are well-formed.
		if err := cred.Verify(); err != nil {
			return errWrongCredentials
		}
	}

	// Track the amount of unlocked transfers
	producedAmount := feeAmount
	consumedAmount := uint64(0)

	for index, input := range ins {
		utxo := utxos[index] // The UTXO consumed by [input]

		if assetID := utxo.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}
		if assetID := input.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}

		out := utxo.Out
		lockState := LockStateUnlocked
		// Set [lockState] to this UTXO's lockState, if applicable
		if inner, ok := out.(*LockedOut); ok {
			out = inner.TransferableOut
			lockState = inner.LockState
		}

		in := input.In
		// The UTXO says it's locked, but this input, which consumes it,
		// is not locked - this is invalid.
		if inner, ok := in.(*LockedIn); lockState.isLocked() && !ok {
			return errLockedFundsNotMarkedAsLocked
		} else if ok {
			if inner.LockState != lockState {
				// This input is locked, but its lockState is wrong
				return errWrongLockState
			}
			in = inner.TransferableIn
		}

		// Verify that this tx's credentials allow [in] to be spent
		if err := vm.fx.VerifyTransfer(tx, in, creds[index], out); err != nil {
			return fmt.Errorf("failed to verify transfer: %w", err)
		}

		amount := in.Amount()

		newUnlockedConsumed, err := math.Add64(consumedAmount, amount)
		if err != nil {
			return err
		}
		consumedAmount = newUnlockedConsumed
	}

	for _, out := range outs {
		if assetID := out.AssetID(); assetID != feeAssetID {
			return errAssetIDMismatch
		}

		amount := out.Out.Amount()

		newUnlockedProduced, err := math.Add64(producedAmount, amount)
		if err != nil {
			return err
		}
		producedAmount = newUnlockedProduced
	}

	// More unlocked tokens produced than consumed. Invalid.
	if producedAmount > consumedAmount {
		return errWrongProducedAmount
	}
	return nil
}

// Verify that [inputs], their [inputIndexes] and [outputs] are syntacticly valid in conjunction for applied/removed [lockState].
// Arguments:
// - [inputs] are inputs that produced [outputs]
// - [inputIndexes] inputs[inputIndexes[i]] produce output[i]
// - [lockState] that expected to be applied/removed to/from inputs lock state in produced outputs
// - [lock] true if we'r locking, false if unlocking
func syntacticVerifyLock(
	inputs []*avax.TransferableInput,
	inputIndexes []uint32,
	outputs []*avax.TransferableOutput,
	lockState LockState,
	lock bool,
) error {
	if lockState != LockStateBonded && lockState != LockStateDeposited {
		return errInvalidTargetLockState
	}

	// ? @evlekht do we need this check?
	if len(inputs) > 4_294_967_295 { // max uint32
		return fmt.Errorf("inputs len is to big")
	}

	if len(outputs) != len(inputIndexes) {
		return errWrongInputIndexesLen
	}

	producedAmount := make([]uint64, len(inputs))

	for outputIndex, out := range outputs {
		inputIndex := inputIndexes[outputIndex]
		in := inputs[inputIndex]

		if in.AssetID() != out.AssetID() {
			return fmt.Errorf("input[%d] assetID isn't equal to produced output[%d] assetID",
				inputIndex, outputIndex)
		}

		inputLockState := LockStateUnlocked
		if lockedIn, ok := in.In.(*LockedIn); ok {
			inputLockState = lockedIn.LockState
		}

		outputLockState := LockStateUnlocked
		if lockedOut, ok := out.Out.(*LockedOut); ok {
			outputLockState = lockedOut.LockState
		}

		// checking that inputLockState is valid for applied/removed lockState
		// checking that outputLockState is valid for inputLockState and applied/removed lockState
		if !(lock &&
			inputLockState&lockState != lockState &&
			(outputLockState == inputLockState|lockState ||
				outputLockState == inputLockState) ||

			// only valid if we don't expect unlocked ins for fee burning
			!lock &&
				inputLockState&lockState == lockState &&
				(outputLockState == inputLockState&^lockState ||
					outputLockState == inputLockState)) { //nolint // newline is intended

			return errWrongLockState
		}

		newProducedAmount, err := math.Add64(producedAmount[inputIndex], out.Out.Amount())
		if err != nil {
			return err
		}
		producedAmount[inputIndex] = newProducedAmount
	}

	// Input state should be checked in previous loop
	// If not - input is burned to zero
	// So we still need to check all inputs and see if their state is valid
	// Also check that ins consumed amount is in accordance with produced amount
	for i, in := range inputs {
		inputLockState := LockStateUnlocked
		if lockedIn, ok := in.In.(*LockedIn); ok {
			inputLockState = lockedIn.LockState
		}

		// if input already locked this way, so it can't be used for lock / burn
		if lock && inputLockState&lockState == lockState {
			return errLockingLockedUTXO
		}

		if producedAmount[i] > in.In.Amount() {
			return errWrongProducedAmount
		}

		// if input won't possibly be fully unlocked after unlock
		// we can't allow burning of this input
		if (!lock && inputLockState&^lockState != LockStateUnlocked || lock && inputLockState.isLocked()) &&
			producedAmount[i] < in.In.Amount() {
			return errBurningLockedUTXO
		}
	}

	return nil
}

func verifyInsAndOutsUnlocked(ins []*avax.TransferableInput, outs []*avax.TransferableOutput) error {
	for _, out := range outs {
		if _, ok := out.Out.(*LockedOut); ok {
			return errLockedInsOrOuts
		}
	}

	for _, in := range ins {
		if _, ok := in.In.(*LockedIn); ok {
			return errLockedInsOrOuts
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
