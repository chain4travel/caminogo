// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
)

var (
	_ txs.Visitor = (*issuer)(nil)
)

type issuer struct {
	m  *mempool
	tx *txs.Tx
}

func (i *issuer) ImportTx(*txs.ImportTx) error {
	i.m.addDecisionTx(i.tx)
	return nil
}
