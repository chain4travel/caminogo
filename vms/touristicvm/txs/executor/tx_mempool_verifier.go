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
// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/touristicvm/state"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
)

var _ txs.Visitor = (*MempoolTxVerifier)(nil)

type MempoolTxVerifier struct {
	*Backend
	ParentID      ids.ID
	StateVersions state.Versions
	Tx            *txs.Tx
}

func (v *MempoolTxVerifier) BaseTx(tx *txs.BaseTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) ImportTx(tx *txs.ImportTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) LockMessengerFundsTx(tx *txs.LockMessengerFundsTx) error {
	return v.standardTx(tx)
}
func (v *MempoolTxVerifier) CashoutChequeTx(tx *txs.CashoutChequeTx) error {
	return v.standardTx(tx)
}

func (v *MempoolTxVerifier) standardTx(tx txs.UnsignedTx) error {
	baseState, err := v.standardBaseState()
	if err != nil {
		return err
	}

	executor := StandardTxExecutor{
		Backend: v.Backend,
		State:   baseState,
		Tx:      v.Tx,
	}
	return tx.Visit(&executor)
}

// Upon Banff activation, txs are not verified against current chain time
// but against the block timestamp. [baseTime] calculates
// the right timestamp to be used to mempool tx verification
func (v *MempoolTxVerifier) standardBaseState() (state.Diff, error) {
	state, err := state.NewDiff(v.ParentID, v.StateVersions)
	if err != nil {
		return nil, err
	}

	nextBlkTime, err := v.nextBlockTime(state)
	if err != nil {
		return nil, err
	}

	state.SetTimestamp(nextBlkTime)

	return state, nil
}

func (v *MempoolTxVerifier) nextBlockTime(state state.Diff) (time.Time, error) {
	var (
		parentTime  = state.GetTimestamp()
		nextBlkTime = v.Clk.Time()
	)
	if parentTime.After(nextBlkTime) {
		nextBlkTime = parentTime
	}
	return nextBlkTime, nil
}
