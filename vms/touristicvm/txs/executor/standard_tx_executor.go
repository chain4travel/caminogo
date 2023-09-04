// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************
// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/chains/atomic"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"github.com/ava-labs/avalanchego/vms/touristicvm/state"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
	"github.com/ava-labs/avalanchego/vms/touristicvm/utxo"
	"strconv"
)

var (
	_                  txs.Visitor = (*StandardTxExecutor)(nil)
	errFlowCheckFailed             = errors.New("flow check failed")
)

type StandardTxExecutor struct {
	// inputs, to be filled before visitor methods are called
	*Backend
	State state.Diff // state is expected to be modified
	Tx    *txs.Tx

	// outputs of visitor execution
	OnAccept       func() // may be nil
	Inputs         set.Set[ids.ID]
	AtomicRequests map[ids.ID]*atomic.Requests // may be nil
}

func (e *StandardTxExecutor) BaseTx(tx *txs.BaseTx) error {
	if err := e.Tx.SyntacticVerify(e.Ctx); err != nil {
		return err
	}

	if e.Bootstrapped.Get() {
		if err := e.Backend.FlowChecker.VerifyLock(
			tx,
			e.State,
			tx.Ins,
			tx.Outs,
			e.Tx.Creds,
			0,
			e.Backend.Config.TxFee,
			e.Backend.Ctx.AVAXAssetID,
			locked.StateUnlocked,
		); err != nil {
			return fmt.Errorf("%w: %s", errFlowCheckFailed, err)
		}
	}

	txID := e.Tx.ID()
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)
	return nil
}

func (e *StandardTxExecutor) ImportTx(tx *txs.ImportTx) error {
	if err := e.Tx.SyntacticVerify(e.Ctx); err != nil {
		return err
	}

	if err := e.BaseTx(&tx.BaseTx); err != nil {
		return err
	}

	// TODO nikos - check if
	utxoIDs := make([][]byte, len(tx.ImportedInputs))
	for i, in := range tx.ImportedInputs {
		utxoID := in.UTXOID.InputID()

		e.Inputs.Add(utxoID)
		utxoIDs[i] = utxoID[:]
	}
	e.AtomicRequests = map[ids.ID]*atomic.Requests{
		tx.SourceChain: {
			RemoveRequests: utxoIDs,
		},
	}
	return nil
}

func (e *StandardTxExecutor) LockMessengerFundsTx(tx *txs.LockMessengerFundsTx) error {
	if err := e.Tx.SyntacticVerify(e.Backend.Ctx); err != nil {
		return err
	}

	//TODO nikos
	//if err := e.Fx.VerifyMultisigOwner(
	//	&secp256k1fx.TransferOutput{
	//		OutputOwners: *rewardOwner,
	//	}, e.State,
	//); err != nil {
	//	return err
	//}

	if err := e.FlowChecker.VerifyLock(
		tx,
		e.State,
		tx.Ins,
		tx.Outs,
		e.Tx.Creds,
		0,
		e.Config.TxFee,
		e.Ctx.AVAXAssetID,
		locked.StateLocked,
	); err != nil {
		return fmt.Errorf("%w: %s", errFlowCheckFailed, err)
	}

	txID := e.Tx.ID()

	avax.Consume(e.State, tx.Ins)
	if err := utxo.ProduceLocked(e.State, txID, tx.Outs); err != nil {
		return err
	}

	return nil
}

func (e *StandardTxExecutor) CashoutChequeTx(tx *txs.CashoutChequeTx) error {
	if err := e.Tx.SyntacticVerify(e.Backend.Ctx); err != nil {
		return err
	}

	// signature is based on the concatenation of issuer, beneficiary and amount
	unsignedMsg := tx.Issuer.String() + tx.Beneficiary.String() + strconv.FormatUint(tx.Amount, 10)

	// verify that the cheque is valid
	if signer, err := e.Fx.RecoverAddressFromSignature(unsignedMsg, tx.IssuerAuth); err != nil || signer != tx.Issuer {
		return fmt.Errorf("invalid signature")
	}
	// check that the cheque is backed by enough funds
	lockedUtxos, err := e.State.LockedUTXOs(tx.Issuer)
	if err != nil {
		return err
	}
	lockedBalance := e.FlowChecker.SumUpUtxos(lockedUtxos)
	if lockedBalance < tx.Amount {
		return fmt.Errorf("the issuer has not enough locked funds")
	}

	// check that the cheque is not already cashed out
	var paidOut uint64
	if paidOut, err = e.State.GetPaidOut(tx.Issuer, tx.Beneficiary); err != nil {
		if err != database.ErrNotFound {
			return err
		}
		paidOut = 0 // first attempt to cash out
	} else if paidOut >= tx.Amount {
		return fmt.Errorf("amount already paid out")
	}

	amountToBurn := e.Config.TxFee
	amountToUnlock := tx.Amount - paidOut

	if err := e.FlowChecker.VerifyUnlock(
		[]byte(unsignedMsg),
		e.State,
		tx,
		tx.Ins,
		tx.Outs,
		e.Tx.Creds,
		amountToBurn,
		amountToUnlock,
		e.Ctx.AVAXAssetID,
	); err != nil {
		return fmt.Errorf("%w: %s", errFlowCheckFailed, err)
	}

	txID := e.Tx.ID()
	avax.Consume(e.State, tx.Ins)
	avax.Produce(e.State, txID, tx.Outs)
	e.State.SetPaidOut(tx.Issuer, tx.Beneficiary, tx.Amount)

	return nil
}
