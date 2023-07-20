// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import "github.com/ava-labs/avalanchego/vms/timestampvm/txs"

var _ txs.Visitor = (*remover)(nil)

type remover struct {
	m  *mempool
	tx *txs.Tx
}

func (r *remover) ImportTx(*txs.ImportTx) error {
	r.m.removeDecisionTxs([]*txs.Tx{r.tx})
	return nil
}
