// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package utxo

import (
	"errors"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"sort"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

var (
	errLockedFundsNotMarkedAsLocked = errors.New("locked funds not marked as locked")
	errInvalidTargetLockState       = errors.New("invalid target lock state")
	errLockingLockedUTXO            = errors.New("utxo consumed for locking are already locked")
	errUnlockingUnlockedUTXO        = errors.New("utxo consumed for unlocking are already unlocked")
	errInsufficientBalance          = errors.New("insufficient balance")
	errWrongInType                  = errors.New("wrong input type")
	errWrongOutType                 = errors.New("wrong output type")
	errWrongUTXOOutType             = errors.New("wrong utxo output type")
	errWrongProducedAmount          = errors.New("produced more tokens, than input had")
	errInputsCredentialsMismatch    = errors.New("number of inputs is different from number of credentials")
	errInputsUTXOsMismatch          = errors.New("number of inputs is different from number of utxos")
	errBadCredentials               = errors.New("bad credentials")
	errNotBurnedEnough              = errors.New("burned less tokens, than needed to")
	errAssetIDMismatch              = errors.New("utxo/input/output assetID is different from expected asset id")
	errLockIDsMismatch              = errors.New("input lock ids is different from utxo lock ids")
	errFailToGetDeposit             = errors.New("couldn't get deposit")
	errLockedUTXO                   = errors.New("can't spend locked utxo")
	errNotLockedUTXO                = errors.New("can't spend unlocked utxo")
	errUTXOOutTypeOrAmtMismatch     = errors.New("inner out isn't *secp256k1fx.TransferOutput or inner out amount != input.Amt")
	errCantSpend                    = errors.New("can't spend utxo with given credential and input")
)

// Creates UTXOs from [outs] and adds them to the UTXO set.
// UTXOs with LockedOut will have 'thisTxID' replaced with [txID].
// [txID] is the ID of the tx that created [outs].
func ProduceLocked(
	utxoDB avax.UTXOAdder,
	txID ids.ID,
	outs []*avax.TransferableOutput,
) error {

	for index, output := range outs {
		out := output.Out
		if lockedOut, ok := out.(*locked.Out); ok {
			utxoLockedOut := *lockedOut
			utxoLockedOut.FixLockID(txID, locked.StateLocked)
			out = &utxoLockedOut
		}
		utxoDB.AddUTXO(&avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(index),
			},
			Asset: output.Asset,
			Out:   out,
		})
	}

	return nil
}

type CaminoSpender interface {
	Locker
	Unlocker
}

func (*handler) isMultisigTransferOutput(utxoDB avax.UTXOReader, out verify.State) bool {

	//TODO nikos add multisig alias getter and remove commented area

	//secpOut, ok := out.(*secp256k1fx.TransferOutput)
	//if !ok {
	//	// Conversion should succeed, otherwise it will be handled by the caller
	//	return false
	//}

	//state, ok := utxoDB.(state.Diff)
	//if !ok {
	//	return false
	//}
	//
	//for _, addr := range secpOut.Addrs {
	//	if _, err := state.GetMultisigAlias(addr); err == nil {
	//		return true
	//	}
	//}
	return false
}

type innerSortUTXOs struct {
	utxos          []*avax.UTXO
	allowedAssetID ids.ID
	lockState      locked.State
}

func (sort *innerSortUTXOs) Less(i, j int) bool {
	iUTXO := sort.utxos[i]
	jUTXO := sort.utxos[j]

	if iUTXO.AssetID() == sort.allowedAssetID && jUTXO.AssetID() != sort.allowedAssetID {
		return true
	}

	iOut := iUTXO.Out
	iLockIDs := &locked.IDsEmpty
	if lockedOut, ok := iOut.(*locked.Out); ok {
		iOut = lockedOut.TransferableOut
		iLockIDs = &lockedOut.IDs
	}

	jOut := jUTXO.Out
	jLockIDs := &locked.IDsEmpty
	if lockedOut, ok := jOut.(*locked.Out); ok {
		jOut = lockedOut.TransferableOut
		jLockIDs = &lockedOut.IDs
	}

	if sort.lockState == locked.StateUnlocked {
		// Sort all locks last
		iEmpty := *iLockIDs == locked.IDsEmpty
		if iEmpty != (*jLockIDs == locked.IDsEmpty) {
			return iEmpty
		}
	} else {
		iLockTxID := &iLockIDs.LockTxID
		jLockTxID := &jLockIDs.LockTxID

		if *iLockTxID == ids.Empty && *jLockTxID != ids.Empty {
			return true
		} else if *iLockTxID != ids.Empty && *jLockTxID == ids.Empty {
			return false
		}

	}

	iAmount := uint64(0)
	if amounter, ok := iOut.(avax.Amounter); ok {
		iAmount = amounter.Amount()
	}

	jAmount := uint64(0)
	if amounter, ok := jOut.(avax.Amounter); ok {
		jAmount = amounter.Amount()
	}

	return iAmount < jAmount
}

func (sort *innerSortUTXOs) Len() int {
	return len(sort.utxos)
}

func (sort *innerSortUTXOs) Swap(i, j int) {
	u := sort.utxos
	u[j], u[i] = u[i], u[j]
}

func sortUTXOs(utxos []*avax.UTXO, allowedAssetID ids.ID, lockState locked.State) {
	sort.Sort(&innerSortUTXOs{utxos: utxos, allowedAssetID: allowedAssetID, lockState: lockState})
}
