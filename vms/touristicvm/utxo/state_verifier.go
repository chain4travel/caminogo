// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.
package utxo

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/stakeable"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"

	"golang.org/x/exp/slices"

	safemath "github.com/ava-labs/avalanchego/utils/math"
)

var (
	errWrongAmounts = errors.New("wrong amounts")
)

type Verifier interface {
	// Verify that lock [tx] is semantically valid.
	// Arguments:
	// - [ins] and [outs] are the inputs and outputs of [tx].
	// - [creds] are the credentials of [tx], which allow [ins] to be spent.
	// - [ins] must have at least ([mintedAmount] - [burnedAmount]) less than the [outs].
	// - [assetID] is id of allowed asset, ins/outs with other assets will return error
	// - [appliedLockState] are lockState that was applied to [ins] lockState to produce [outs]
	//
	// Precondition: [tx] has already been syntactically verified.
	VerifyLock(
		tx txs.UnsignedTx,
		utxoDB avax.UTXOGetter,
		ins []*avax.TransferableInput,
		outs []*avax.TransferableOutput,
		creds []verify.Verifiable,
		mintedAmount uint64,
		burnedAmount uint64,
		assetID ids.ID,
		appliedLockState locked.State,
	) error

	VerifyLockUTXOs(
		tx txs.UnsignedTx,
		utxos []*avax.UTXO,
		ins []*avax.TransferableInput,
		outs []*avax.TransferableOutput,
		creds []verify.Verifiable,
		mintedAmount uint64,
		burnedAmount uint64,
		assetID ids.ID,
		appliedLockState locked.State) error

	VerifyUnlock(
		eligibleUTXOs []*avax.UTXO,
		utxoDB avax.UTXOGetter,
		tx txs.UnsignedTx,
		ins []*avax.TransferableInput,
		outs []*avax.TransferableOutput,
		creds []verify.Verifiable,
		burnedAmount uint64,
		amountToUnlock uint64,
		assetID ids.ID,
	) error
}

func (h *handler) VerifyLock(
	tx txs.UnsignedTx,
	utxoDB avax.UTXOGetter,
	ins []*avax.TransferableInput,
	outs []*avax.TransferableOutput,
	creds []verify.Verifiable,
	mintedAmount uint64,
	burnedAmount uint64,
	assetID ids.ID,
	appliedLockState locked.State,
) error {
	//msigState, ok := utxoDB.(secp256k1fx.AliasGetter)
	//if !ok {
	//	return secp256k1fx.ErrNotAliasGetter
	//}

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

	return h.VerifyLockUTXOs(tx, utxos, ins, outs, creds, mintedAmount, burnedAmount, assetID, appliedLockState)
}

func (h *handler) VerifyLockUTXOs(
	tx txs.UnsignedTx,
	utxos []*avax.UTXO,
	ins []*avax.TransferableInput,
	outs []*avax.TransferableOutput,
	creds []verify.Verifiable,
	mintedAmount uint64,
	burnedAmount uint64,
	assetID ids.ID,
	appliedLockState locked.State,
) error {
	if appliedLockState != locked.StateLocked &&
		appliedLockState != locked.StateUnlocked {
		return errInvalidTargetLockState
	}

	if len(ins) != len(creds) {
		return fmt.Errorf(
			"there are %d inputs and %d credentials: %w",
			len(ins),
			len(creds),
			errInputsCredentialsMismatch,
		)
	}

	if len(ins) != len(utxos) {
		return fmt.Errorf(
			"there are %d inputs and %d utxos: %w",
			len(ins),
			len(utxos),
			errInputsUTXOsMismatch,
		)
	}

	for _, cred := range creds {
		if err := cred.Verify(); err != nil {
			return errBadCredentials
		}
	}

	// Track the amount of transfers and their owners
	// if appliedLockState == bond, then otherLockTxID is depositTxID and vice versa
	// ownerID -> otherLockTxID -> amount
	consumed := uint64(0)
	produced := uint64(0)

	for index, input := range ins {
		utxo := utxos[index] // The UTXO consumed by [input]

		if utxoAssetID := utxo.AssetID(); utxoAssetID != assetID {
			return fmt.Errorf(
				"utxo %d has asset ID %s but expect %s: %w",
				index,
				utxoAssetID,
				assetID,
				errAssetIDMismatch,
			)
		}

		if inputAssetID := input.AssetID(); inputAssetID != assetID {
			return fmt.Errorf(
				"input %d has asset ID %s but expect %s: %w",
				index,
				inputAssetID,
				assetID,
				errAssetIDMismatch,
			)
		}

		out := utxo.Out
		if _, ok := out.(*stakeable.LockOut); ok {
			return errWrongUTXOOutType
		}

		lockIDs := &locked.IDsEmpty
		if lockedOut, ok := out.(*locked.Out); ok {
			// can only spend unlocked utxos, if appliedLockState is unlocked
			if appliedLockState == locked.StateUnlocked {
				return errLockedUTXO
				// utxo is already locked with appliedLockState, so it can't be locked it again
			} else if lockedOut.IsLockedWith(appliedLockState) {
				return errLockingLockedUTXO
			}
			out = lockedOut.TransferableOut
			lockIDs = &lockedOut.IDs
		}

		in := input.In
		if _, ok := in.(*stakeable.LockIn); ok {
			return errWrongInType
		}

		if _, ok := in.(*locked.In); ok {
			return fmt.Errorf("cannot consumed locked inputs")
		} else if lockIDs.IsLocked() {
			// The UTXO says it's locked, but this input, which consumes it,
			// is not locked - this is invalid.
			return errLockedFundsNotMarkedAsLocked
		}

		if err := h.fx.VerifyTransfer(tx, in, creds[index], out); err != nil {
			return fmt.Errorf("failed to verify transfer: %w", err)
		}

		amount := in.Amount()
		newAmount, err := safemath.Add64(consumed, amount)
		if err != nil {
			return err
		}
		consumed = newAmount
	}

	for index, output := range outs {
		if outputAssetID := output.AssetID(); outputAssetID != assetID {
			return fmt.Errorf(
				"output %d has asset ID %s but expect %s: %w",
				index,
				outputAssetID,
				assetID,
				errAssetIDMismatch,
			)
		}

		out := output.Out
		if _, ok := out.(*stakeable.LockOut); ok {
			return errWrongOutType
		}

		if lockedOut, ok := out.(*locked.Out); ok {
			out = lockedOut.TransferableOut
		}

		producedAmount := out.Amount()
		newAmount, err := safemath.Add64(produced, producedAmount)
		if err != nil {
			return err
		}
		produced = newAmount
	}
	if consumed < produced {
		return fmt.Errorf(
			"produces %d and consumes %d with lock '%s': %w",
			produced,
			consumed,
			appliedLockState,
			errWrongProducedAmount,
		)
	}

	if consumed < burnedAmount {
		return fmt.Errorf(
			"asset %s burned %d unlocked, but needed to burn %d: %w",
			assetID,
			consumed,
			burnedAmount,
			errNotBurnedEnough,
		)
	}

	return nil
}

func (h *handler) VerifyUnlock(eligibleUTXOs []*avax.UTXO, utxoDB avax.UTXOGetter, tx txs.UnsignedTx, ins []*avax.TransferableInput, outs []*avax.TransferableOutput, creds []verify.Verifiable, burnedAmount uint64, amountToUnlock uint64, assetID ids.ID) error {
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

	return h.VerifyUnlockUTXOs(eligibleUTXOs, tx, utxos, ins, outs, creds, burnedAmount, amountToUnlock, assetID)
}

func (h *handler) VerifyUnlockUTXOs(
	eligibleUTXOs []*avax.UTXO,
	tx txs.UnsignedTx,
	utxos []*avax.UTXO,
	ins []*avax.TransferableInput,
	outs []*avax.TransferableOutput,
	creds []verify.Verifiable,
	burnedAmount uint64,
	amountToUnlock uint64,
	assetID ids.ID,
) error {
	if len(ins) != len(utxos) {
		return fmt.Errorf(
			"there are %d inputs and %d utxos: %w",
			len(ins),
			len(utxos),
			errInputsUTXOsMismatch,
		)
	}

	var err error
	//consumedLockedOwnerID := ids.Empty
	consumedLocked := uint64(0)
	consumedUnlocked := uint64(0)

	for index, input := range ins {
		utxo := utxos[index] // The UTXO consumed by [input]

		if utxoAssetID := utxo.AssetID(); utxoAssetID != assetID {
			return fmt.Errorf(
				"utxo %d has asset ID %s but expect %s: %w",
				index,
				utxoAssetID,
				assetID,
				errAssetIDMismatch,
			)
		}

		if inputAssetID := input.AssetID(); inputAssetID != assetID {
			return fmt.Errorf(
				"input %d has asset ID %s but expect %s: %w",
				index,
				inputAssetID,
				assetID,
				errAssetIDMismatch,
			)
		}
		out := utxo.Out
		if lockedOut, ok := out.(*locked.Out); ok {
			//// utxo isn't deposited, so it can't be unlocked
			//// bonded-not-deposited utxos are not allowed
			//if lockedOut.LockTxID == ids.Empty {
			//	return errUnlockingUnlockedUTXO
			//}
			out = lockedOut.TransferableOut
		}

		in, ok := input.In.(*locked.In)
		if ok {
			consumedLocked, err = safemath.Add64(consumedLocked, in.Amount())
			if err != nil {
				return err
			}
			// if it's a locked input, then we need to verify the signature of the signed message
			if !slices.Contains(eligibleUTXOs, utxo) {
				return fmt.Errorf("this utxo is not locked and thus cannot be used to unlock funds")
			}

			//consumedLockedOwnerID, err = txs.GetOutputOwnerID(out)
			if err != nil {
				return err
			}
		} else {
			consumedUnlocked, err = safemath.Add64(consumedUnlocked, input.In.Amount())
			if err != nil {
				return err
			}
			// if it's an unlocked input, then we need to verify the signature of the signed tx
			if err := h.fx.VerifyTransfer(tx, in, creds[index], out); err != nil {
				return fmt.Errorf("failed to verify transfer: %w: %s", errCantSpend, err)
			}
		}
	}
	producedLocked := uint64(0)
	producedUnlocked := uint64(0)

	for index, output := range outs {
		outputAssetID := output.AssetID()
		if outputAssetID != assetID {
			return fmt.Errorf(
				"output %d has asset ID %s but expect %s: %w",
				index,
				assetID,
				outputAssetID,
				errAssetIDMismatch,
			)
		}
		out := output.Out
		if lockedOut, ok := out.(*locked.Out); ok && lockedOut.LockTxID != ids.Empty {
			producedLocked, err = safemath.Add64(producedLocked, lockedOut.Amount())
			if err != nil {
				return err
			}

			//ownerID, err := txs.GetOwnerID(out)
			//if err != nil {
			//	return err
			//}
			//if ownerID != consumedLockedOwnerID {
			//	return fmt.Errorf("ownerID of locked output %s doesn't match ownerID of consumed locked input %s", ownerID, consumedLockedOwnerID)
			//}
			// TODO replace with check that does indeed what we want. This check is not correct
		} else {
			producedUnlocked, err = safemath.Add64(producedUnlocked, out.Amount())
			if err != nil {
				return err
			}
		}
	}

	// consumed locked +  consumed unlocked  = produced locked + produced unlocked + burnedAmount
	if consumedLocked+consumedUnlocked != producedLocked+producedUnlocked+burnedAmount {
		return fmt.Errorf("consumed locked +  consumed unlocked  = produced locked + produced unlocked + burnedAmount: %w", errWrongAmounts)
	}
	// consumed locked - amountToUnlock = produced locked
	if consumedLocked-amountToUnlock != producedLocked {
		return fmt.Errorf("consumed locked - amountToUnlock must be equal to produced locked: %w", errWrongAmounts)
	}

	// checking that we burned required amount
	if consumedUnlocked < burnedAmount {
		return fmt.Errorf(
			"asset %s burned %d unlocked, but needed to burn %d: %w",
			assetID,
			consumedUnlocked,
			burnedAmount,
			errNotBurnedEnough,
		)
	}

	// produced unlocked must be equal to amountToUnlock
	if producedUnlocked != amountToUnlock {
		return fmt.Errorf("produced unlocked must be equal to amountToUnlock: %w", errWrongAmounts)
	}

	return nil
}
