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

package metrics

import (
	"fmt"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/touristicvm/blocks"
	"github.com/prometheus/client_golang/prometheus"
)

type blockMetrics struct {
	txMetrics *txMetrics

	numStandardBlocks prometheus.Counter
}

func newBlockMetrics(
	namespace string,
	registerer prometheus.Registerer,
) (*blockMetrics, error) {
	txMetrics, err := newTxMetrics(namespace, registerer)
	errs := wrappers.Errs{Err: err}
	m := &blockMetrics{
		txMetrics:         txMetrics,
		numStandardBlocks: newBlockMetric(namespace, "standard", registerer, &errs),
	}
	return m, errs.Err
}

func newBlockMetric(
	namespace string,
	blockName string,
	registerer prometheus.Registerer,
	errs *wrappers.Errs,
) prometheus.Counter {
	blockMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      fmt.Sprintf("%s_blks_accepted", blockName),
		Help:      fmt.Sprintf("Number of %s blocks accepted", blockName),
	})
	errs.Add(registerer.Register(blockMetric))
	return blockMetric
}

// TODO add visitor
func (m *blockMetrics) StandardBlock(b *blocks.StandardBlock) error {
	m.numStandardBlocks.Inc()
	for _, tx := range b.Transactions {
		if err := tx.Unsigned.Visit(m.txMetrics); err != nil {
			return err
		}
	}
	return nil
}
