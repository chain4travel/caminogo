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
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"github.com/ava-labs/avalanchego/vms/touristicvm/state"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
	"github.com/ava-labs/avalanchego/vms/touristicvm/utxo"
)

var (
	_                            txs.Visitor = (*StandardTxExecutor)(nil)
	errFlowCheckFailed                       = errors.New("flow check failed")
	errIssuerBeneficiaryMismatch             = errors.New("the issuer and the beneficiary should be different")
	errInvalidSignature                      = errors.New("invalid signature")
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
	if err := locked.VerifyNoLocks(tx.Ins, tx.Outs); err != nil {
		return err
	}
	if err := locked.VerifyNoLocks(tx.ImportedInputs, nil); err != nil {
		return err
	}
	if err := e.Tx.SyntacticVerify(e.Ctx); err != nil {
		return err
	}

	e.Inputs = set.NewSet[ids.ID](len(tx.ImportedInputs))
	utxoIDs := make([][]byte, len(tx.ImportedInputs))
	for i, in := range tx.ImportedInputs {
		utxoID := in.UTXOID.InputID()

		e.Inputs.Add(utxoID)
		utxoIDs[i] = utxoID[:]
	}

	if e.Bootstrapped.Get() {

		allUTXOBytes, err := e.Ctx.SharedMemory.Get(tx.SourceChain, utxoIDs)
		if err != nil {
			return fmt.Errorf("failed to get shared memory: %w", err)
		}

		utxos := make([]*avax.UTXO, len(tx.Ins)+len(tx.ImportedInputs))
		for index, input := range tx.Ins {
			utxo, err := e.State.GetUTXO(input.InputID())
			if err != nil {
				return fmt.Errorf("failed to get UTXO %s: %w", &input.UTXOID, err)
			}
			utxos[index] = utxo
		}
		for i, utxoBytes := range allUTXOBytes {
			utxo := &avax.UTXO{}
			if _, err := txs.Codec.Unmarshal(utxoBytes, utxo); err != nil {
				return fmt.Errorf("failed to unmarshal UTXO: %w", err)
			}
			utxos[i+len(tx.Ins)] = utxo
		}

		ins := make([]*avax.TransferableInput, len(tx.Ins)+len(tx.ImportedInputs))
		copy(ins, tx.Ins)
		copy(ins[len(tx.Ins):], tx.ImportedInputs)

		if err := e.FlowChecker.VerifyLockUTXOs(
			tx,
			utxos,
			ins,
			tx.Outs,
			e.Tx.Creds,
			0,
			e.Backend.Config.TxFee,
			e.Backend.Ctx.AVAXAssetID,
			locked.StateUnlocked,
		); err != nil {
			return err
		}
	}

	txID := e.Tx.ID()

	// Consume the UTXOS
	avax.Consume(e.State, tx.Ins)
	// Produce the UTXOS
	avax.Produce(e.State, txID, tx.Outs)

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

	// verify that the tx carries one and only one signature
	if tx.Cheque.Auth == nil || len(tx.Cheque.Auth.(*secp256k1fx.Credential).Sigs) != 1 {
		return fmt.Errorf("expected one signature, got %d", len(e.Tx.Creds))
	}

	// verify that the issuer and the beneficiary are different
	if tx.Cheque.Issuer == tx.Cheque.Beneficiary {
		return errIssuerBeneficiaryMismatch
	}

	// verify that the cheque is valid
	if signer, err := e.Fx.RecoverAddressFromSignedMessage(tx.Cheque.BuildMsgToSign(), tx.Cheque.Auth); err != nil || signer != tx.Cheque.Issuer {
		return errInvalidSignature
	}

	var (
		lastCashedOutCheque state.Cheque
		err                 error
	)
	issuerAgent := state.IssuerAgent{
		Issuer: tx.Cheque.Issuer,
		Agent:  tx.Cheque.Agent,
	}
	// check that the cheque is not already cashed out
	if lastCashedOutCheque, err = e.State.GetLastCheque(issuerAgent, tx.Cheque.Beneficiary); err != nil {
		if err != database.ErrNotFound {
			return err
		}

		lastCashedOutCheque = state.Cheque{
			Amount: 0,
		} // first attempt to cash out
	} else if lastCashedOutCheque.Amount >= tx.Cheque.Amount {
		return utxo.ErrAmountAlreadyPaidOut
	} else if tx.Cheque.SerialID <= lastCashedOutCheque.SerialID {
		return fmt.Errorf("new serial ID should be higher than  %d", lastCashedOutCheque.SerialID)
	}

	amountToBurn := uint64(0) // TODO nikos for now CashoutTx does not incur a e.Config.TxFee
	amountToUnlock := tx.Cheque.Amount - lastCashedOutCheque.Amount

	// check that the cheque is backed by enough funds
	lockedUtxos, err := e.State.LockedUTXOs(tx.Cheque.Issuer)
	if err != nil {
		return err
	}
	lockedBalance := e.FlowChecker.SumUpUtxos(lockedUtxos)
	if lockedBalance < amountToUnlock {
		return utxo.ErrNotEnoughLockedFunds
	}

	if err := e.FlowChecker.VerifyUnlock(
		lockedUtxos,
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
	e.State.SetLastCheque(issuerAgent, tx.Cheque.Beneficiary, state.Cheque{
		Amount:   tx.Cheque.Amount,
		SerialID: tx.Cheque.SerialID,
	})

	return nil
}
