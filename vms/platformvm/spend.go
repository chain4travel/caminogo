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
	"github.com/chain4travel/caminogo/vms/platformvm/status"
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
	errLockingLockedUTXO            = errors.New("utxo consumed for locking are already locked")
	errLockedInsOrOuts              = errors.New("transaction body has locked inputs or outs, but that's now allowed")
	errWrongProducedAmount          = errors.New("produced more tokens, than input had")
	errNotBurnedEnough              = errors.New("burned less tokens, than we need to")
)

// spend the provided amount while deducting the provided fee.
// Arguments:
// - [keys] are the owners of the funds
// - [totalAmountToSpend] is the amount of funds that are trying to be spent (changed their state)
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
	[][]*crypto.PrivateKeySECP256K1R, // signers
	error,
) {
	if appliedLockState != LockStateBonded && appliedLockState != LockStateDeposited {
		return nil, nil, nil, errInvalidTargetLockState
	}

	addrs := ids.NewShortSet(len(keys)) // The addresses controlled by [keys]
	for _, key := range keys {
		addrs.Add(key.PublicKey().Address())
	}
	utxos, err := avax.GetAllUTXOs(vm.internalState, addrs) // The UTXOs controlled by [keys]
	if err != nil {
		return nil, nil, nil, fmt.Errorf("couldn't get UTXOs: %w", err)
	}

	kc := secp256k1fx.NewKeychain(keys...) // Keychain consumes UTXOs and creates new ones

	// Minimum time this transaction will be issued at
	now := uint64(vm.clock.Time().Unix())

	ins := []*avax.TransferableInput{}
	outs := []*avax.TransferableOutput{}
	signers := [][]*crypto.PrivateKeySECP256K1R{}

	// Amount of AVAX that has been spent
	amountSpent := uint64(0)

	// Consume locked UTXOs
	for _, utxo := range utxos {
		// If we have consumed more AVAX than we are trying to spend, then we
		// have no need to consume more locked AVAX
		if amountSpent >= totalAmountToSpend {
			break
		}

		if assetID := utxo.AssetID(); assetID != vm.ctx.AVAXAssetID {
			continue // We only care about staking AVAX, so ignore other assets
		}

		lockedOut, ok := utxo.Out.(*LockedOut)
		if !ok {
			// This output isn't locked, so it will be handled during the next
			// iteration of the UTXO set
			continue
		} else if lockedOut.LockState().IsLockedWith(appliedLockState) {
			// This output can't be locked with target lockState
			continue
		}

		innerOut, ok := lockedOut.TransferableOut.(*secp256k1fx.TransferOutput)
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

		// Spend any value that should be spent
		amountToSpend := math.Min64(
			totalAmountToSpend-amountSpent, // Amount we still need to spend
			remainingValue,                 // Amount available to spend
		)
		amountSpent += amountToSpend
		remainingValue -= amountToSpend

		// Add the input to the consumed inputs
		ins = append(ins, &avax.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
			In: &LockedIn{
				LockIDs:        lockedOut.LockIDs,
				TransferableIn: in,
			},
		})

		// Add the output to the transitioned outputs
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
			Out: &LockedOut{
				LockIDs: lockedOut.LockIDs.Lock(appliedLockState),
				TransferableOut: &secp256k1fx.TransferOutput{
					Amt:          amountToSpend,
					OutputOwners: innerOut.OutputOwners,
				},
			},
		})

		if remainingValue > 0 {
			// This input provided more value than was needed to be spent.
			// Some of it must be returned
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &LockedOut{
					LockIDs: lockedOut.LockIDs,
					TransferableOut: &secp256k1fx.TransferOutput{
						Amt:          remainingValue,
						OutputOwners: innerOut.OutputOwners,
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
		// If we have burned more AVAX then we need to, then we have no need to
		// consume more AVAX
		if amountSpent >= totalAmountToSpend && amountBurned >= totalAmountToBurn {
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

		// Spend any value that should be spent
		amountToSpend := math.Min64(
			totalAmountToSpend-amountSpent, // Amount we still need to spend
			remainingValue,                 // Amount available to spend
		)
		amountSpent += amountToSpend
		remainingValue -= amountToSpend

		// Add the input to the consumed inputs
		ins = append(ins, &avax.TransferableInput{
			UTXOID: utxo.UTXOID,
			Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
			In:     in,
		})

		if amountToSpend > 0 {
			// Some of this input was put for spending
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &LockedOut{
					LockIDs: LockIDs{}.Lock(appliedLockState),
					TransferableOut: &secp256k1fx.TransferOutput{
						Amt:          amountToSpend,
						OutputOwners: innerOut.OutputOwners,
					},
				},
			})
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
		}

		// Add the signers needed for this input to the set of signers
		signers = append(signers, inSigners)
	}

	if amountBurned < totalAmountToBurn || amountSpent < totalAmountToSpend {
		return nil, nil, nil, fmt.Errorf(
			"provided keys have balance (unlocked, locked) (%d, %d) but need (%d, %d)",
			amountBurned, amountSpent, totalAmountToBurn, totalAmountToSpend)
	}

	avax.SortTransferableInputsWithSigners(ins, signers) // sort inputs and keys
	avax.SortTransferableOutputs(outs, Codec)            // sort outputs

	return ins, outs, signers, nil
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
// - [signers] the proof of ownership of the funds being moved
func (vm *VM) unlock(
	state MutableState,
	lockTxIDs []ids.ID,
	removedLockState LockState, //nolint // * @evlekht must be fixed with deposit PR
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // outputs
	error,
) {
	if removedLockState != LockStateBonded && removedLockState != LockStateDeposited {
		return nil, nil, errInvalidTargetLockState
	}

	lockTxIDsSet := ids.NewSet(len(lockTxIDs))
	addrs := ids.ShortSet{}
	for _, lockTxID := range lockTxIDs {
		lockTxIDsSet.Add(lockTxID)

		tx, s, err := state.GetTx(lockTxID) // @jax this call might be expensive
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch lockedTx %s: %v", lockTxID, err)
		}
		if s != status.Committed {
			return nil, nil, fmt.Errorf("%s is not a commited tx", lockTxID)
		}

		addValTx, ok := tx.UnsignedTx.(*UnsignedAddValidatorTx) // @jax this is not generic
		if !ok {
			return nil, nil, fmt.Errorf("tx %s is not a addValidatorTx", lockTxID)
		}

		for i, valOut := range addValTx.Outs {
			lockedOut, ok := valOut.Out.(*LockedOut)
			if !ok {
				return nil, nil, fmt.Errorf("could not cast out no. %d to locked out from tx %s", i, lockTxID)
			}
			out, ok := lockedOut.TransferableOut.(*secp256k1fx.TransferOutput)
			if !ok {
				return nil, nil, fmt.Errorf("could not cast locked out no. %d to transerfableOut from tx %s", i, lockTxID)
			}
			addrs.Add(out.Addrs...)
		}

	}

	// TODO@ think on optimizing it to get not ALL allUTXOs
	allUTXOs, err := avax.GetAllUTXOs(vm.internalState, addrs)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't get UTXOs: %w", err)
	}

	var utxos []*avax.UTXO
	for _, utxo := range allUTXOs {
		lockedOut, ok := utxo.Out.(*LockedOut)
		if !ok {
			continue
		}

		if lockTxIDsSet.Contains(lockedOut.DepositTxID) ||
			lockTxIDsSet.Contains(lockedOut.BondTxID) {
			utxos = append(utxos, utxo)
		}
	}

	return vm.unlockUTXOs(utxos, removedLockState)
}

// unlockUTXOs consumes locked utxos owned by keys and produce unlocked outs
// Arguments:
//   - [utxos] utxos that will be used to consume and unlock
//   - [keys] owners of the funds
//   - [removedLockState] is type of lock that that function will try to unlock
//     (it's either Bonded or Deposited)
//   - [needSigners] do inputs need to be signed
//
// Returns:
// - [inputs] produced inputs
// - [outputs] produced outputs
// - [signers] the proof of ownership of the funds being moved
func (vm *VM) unlockUTXOs(
	utxos []*avax.UTXO,
	removedLockState LockState,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // outputs
	error,
) {
	if removedLockState != LockStateBonded && removedLockState != LockStateDeposited {
		return nil, nil, errInvalidTargetLockState
	}

	ins := []*avax.TransferableInput{}
	outs := []*avax.TransferableOutput{}

	for _, utxo := range utxos {
		out, ok := utxo.Out.(*LockedOut)
		if !ok {
			// This output isn't locked
			continue
		} else if !out.LockState().IsLockedWith(removedLockState) {
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
				LockIDs: out.LockIDs,
				TransferableIn: &secp256k1fx.TransferInput{
					Amt:   out.Amount(),
					Input: secp256k1fx.Input{},
				},
			},
		})

		if newLockIDs := out.LockIDs.Unlock(removedLockState); newLockIDs.LockState().IsLocked() {
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &LockedOut{
					LockIDs: newLockIDs,
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
	}

	avax.SortTransferableInputs(ins)          // sort inputs
	avax.SortTransferableOutputs(outs, Codec) // sort outputs

	return ins, outs, nil
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

// Verify that lock [tx] is semanticly valid.
// Arguments:
// - [utxoDB] should not be committed if an error is returned
// - [creds] are the credentials of [tx], which allow [inputs] to be spent.
// - [inputs] are inputs that produced [outputs] by [tx].
// - [appliedLockState] is the lock state that expected to be applied to inputs lock state in produced output
// - [unlockedMustBurnAmount] is the minimum amout the we expect to burn
// Precondition: [tx] has already been syntactically verified
func (vm *VM) semanticVerifySpend(
	utxoDB UTXOGetter,
	tx UnsignedTx,
	inputs []*avax.TransferableInput,
	outputs []*avax.TransferableOutput,
	appliedLockState LockState,
	creds []verify.Verifiable,
	unlockedMustBurnAmount uint64,
	assetID ids.ID,
) error {
	utxos := make([]*avax.UTXO, len(inputs))
	for i, input := range inputs {
		utxo, err := utxoDB.GetUTXO(input.InputID())
		if err != nil {
			return fmt.Errorf(
				"failed to read consumed UTXO %s due to: %w",
				&input.UTXOID,
				err,
			)
		}
		utxos[i] = utxo
	}

	return vm.semanticVerifySpendUTXOs(
		tx,
		utxos,
		inputs,
		outputs,
		appliedLockState,
		creds,
		unlockedMustBurnAmount,
		assetID,
	)
}

// Verify that lock [tx] is semanticly valid.
// Arguments:
// - [utxos] are consumed by [inputs]
// - [creds] are the credentials of [tx], which allow [inputs] to be spent.
// - [inputs] are inputs that produced [outputs] by [tx].
// - [appliedLockState] is the lock state that expected to be applied to inputs lock state in produced output
// - [unlockedMustBurnAmount] is the minimum amout the we expect to burn
// Precondition: [tx] has already been syntactically verified
func (vm *VM) semanticVerifySpendUTXOs(
	tx UnsignedTx,
	utxos []*avax.UTXO,
	inputs []*avax.TransferableInput,
	outputs []*avax.TransferableOutput,
	appliedLockState LockState,
	creds []verify.Verifiable,
	unlockedMustBurnAmount uint64,
	assetID ids.ID,
) error {
	if appliedLockState != LockStateBonded && appliedLockState != LockStateDeposited {
		return errInvalidTargetLockState
	}

	if len(inputs) != len(creds) {
		return fmt.Errorf(
			"there are %d inputs and %d credentials: %w",
			len(inputs),
			len(creds),
			errInputsCredentialsMismatch,
		)
	}

	if len(inputs) != len(utxos) {
		return fmt.Errorf(
			"there are %d inputs and %d utxos: %w",
			len(inputs),
			len(utxos),
			errInputsUTXOSMismatch,
		)
	}

	for _, cred := range creds {
		if err := cred.Verify(); err != nil {
			return errWrongCredentials
		}
	}

	// ownerID -> LockIDs -> amount
	consumedAmounts := map[ids.ID]map[LockIDs]uint64{}

	for i, input := range inputs {
		utxo := utxos[i]

		if utxo.AssetID() != assetID || input.AssetID() != assetID {
			return errAssetIDMismatch
		}

		consumedOut := utxo.Out
		utxoLockIDs := LockIDs{}
		// Set [lockState] to this UTXO's lockState, if applicable
		if lockedOut, ok := consumedOut.(*LockedOut); ok {
			consumedOut = lockedOut.TransferableOut
			utxoLockIDs = lockedOut.LockIDs
		}

		in := input.In
		if lockedIn, ok := in.(*LockedIn); ok {
			if lockedIn.LockIDs.LockState().IsLockedWith(appliedLockState) {
				return errLockingLockedUTXO
			}
			// This input is locked, but its lockState is wrong
			if utxoLockIDs != lockedIn.LockIDs {
				return errWrongLockState
			}
			in = lockedIn.TransferableIn
		} else if utxoLockIDs.LockState().IsLocked() {
			// The UTXO says it's locked, but this input, which consumes it,
			// is not locked - this is invalid.
			return errLockedFundsNotMarkedAsLocked
		}

		// Verify that this tx's credentials allow [in] to be spent
		if err := vm.fx.VerifyTransfer(tx, in, creds[i], consumedOut); err != nil {
			return fmt.Errorf("failed to verify transfer: %w", err)
		}

		ownerID, err := getOwnerID(consumedOut)
		if err != nil {
			return err
		}

		newConsumedAmounts, err := math.Add64(consumedAmounts[ownerID][utxoLockIDs], in.Amount())
		if err != nil {
			return err
		}
		consumedAmounts[ownerID][utxoLockIDs] = newConsumedAmounts
	}

	producedAmounts := map[ids.ID]map[LockIDs]uint64{}

	for _, out := range outputs {
		if out.AssetID() != assetID {
			return errAssetIDMismatch
		}

		ownerID, err := getOwnerID(out.Out)
		if err != nil {
			return err
		}
		lockIDs := LockIDs{}
		if lockedOut, ok := out.Out.(*LockedOut); ok {
			lockIDs = lockedOut.LockIDs
		}

		newProducedAmount, err := math.Add64(producedAmounts[ownerID][lockIDs], out.Out.Amount())
		if err != nil {
			return err
		}
		producedAmounts[ownerID][lockIDs] = newProducedAmount
	}

	unlockedBurned := uint64(0)
	for ownerID, ownerProducedAmounts := range producedAmounts {
		for lockIDs, producedAmount := range ownerProducedAmounts {
			consumedAmount := uint64(0)
			changedConsumedLockIDs := lockIDs.Unlock(appliedLockState)
			if ownerConsumedAmounts, ok := consumedAmounts[ownerID]; ok {
				consumedAmount = ownerConsumedAmounts[changedConsumedLockIDs]                       // state changed
				newConsumedAmount, err := math.Add64(consumedAmount, ownerConsumedAmounts[lockIDs]) // state not changed
				if err != nil {
					return err
				}
				consumedAmount = newConsumedAmount
			}
			if producedAmount > consumedAmount {
				return fmt.Errorf(
					"ownerID %s produces %d unlocked and consumes %d unlocked for lockIDs %+v with applied lockState %s: %w",
					ownerID,
					producedAmount,
					consumedAmount,
					lockIDs,
					appliedLockState,
					errWrongProducedAmount,
				)
			}
			if !lockIDs.LockState().IsLocked() && producedAmount != consumedAmount {
				burnedAmount, err := math.Sub64(consumedAmount, producedAmount)
				if err != nil {
					return err
				}
				newUnlockedBurned, err := math.Add64(unlockedBurned, burnedAmount)
				if err != nil {
					return err
				}
				unlockedBurned = newUnlockedBurned
			}
		}
	}

	if unlockedBurned < unlockedMustBurnAmount {
		return errNotBurnedEnough
	}

	return nil
}

func getOwnerID(out interface{}) (ids.ID, error) {
	owned, ok := out.(Owned)
	if !ok {
		return ids.Empty, errUnknownOwnersType
	}
	owner := owned.Owners()
	ownerBytes, err := Codec.Marshal(CodecVersion, owner)
	if err != nil {
		return ids.Empty, fmt.Errorf("couldn't marshal owner: %w", err)
	}

	return hashing.ComputeHash256Array(ownerBytes), nil
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
	for index, output := range outs {
		out := output.Out
		if lockedOut, ok := out.(*LockedOut); ok {
			lockedOut := &LockedOut{
				LockIDs:         lockedOut.LockIDs,
				TransferableOut: lockedOut.TransferableOut,
			}
			if lockedOut.BondTxID == thisTxID {
				lockedOut.BondTxID = txID
			}
			if lockedOut.DepositTxID == thisTxID {
				lockedOut.DepositTxID = txID
			}
			out = lockedOut
		}
		utxoDB.AddUTXO(&avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(index),
			},
			Asset: avax.Asset{ID: assetID},
			Out:   out,
		})
	}
}
