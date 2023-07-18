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
	"context"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/chains/atomic"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/timestampvm/state"
	"github.com/ava-labs/avalanchego/vms/timestampvm/txs"
)

var (
	_ txs.Visitor = (*StandardTxExecutor)(nil)

	errEmptyNodeID              = errors.New("validator nodeID cannot be empty")
	errMaxStakeDurationTooLarge = errors.New("max stake duration must be less than or equal to the global max stake duration")
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

func (e *StandardTxExecutor) ImportTx(tx *txs.ImportTx) error {
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
		if err := verify.SameSubnet(context.TODO(), e.Ctx, tx.SourceChain); err != nil {
			return err
		}

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

		if err := e.FlowChecker.VerifySpendUTXOs(
			nil,
			tx,
			utxos,
			ins,
			tx.Outs,
			e.Tx.Creds,
			map[ids.ID]uint64{
				e.Ctx.AVAXAssetID: e.Config.TxFee,
			},
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
