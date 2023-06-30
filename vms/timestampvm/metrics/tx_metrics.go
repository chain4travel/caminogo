// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.
package metrics

import (
	"fmt"
	"github.com/ava-labs/avalanchego/vms/timestampvm/txs"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/avalanchego/utils/wrappers"
)

var _ txs.Visitor = (*txMetrics)(nil)

type txMetrics struct {
	numExportTxs prometheus.Counter
}

func newTxMetrics(
	namespace string,
	registerer prometheus.Registerer,
) (*txMetrics, error) {
	errs := wrappers.Errs{}
	m := &txMetrics{
		numExportTxs: newTxMetric(namespace, "export", registerer, &errs),
	}
	return m, errs.Err
}

func newTxMetric(
	namespace string,
	txName string,
	registerer prometheus.Registerer,
	errs *wrappers.Errs,
) prometheus.Counter {
	txMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      fmt.Sprintf("%s_txs_accepted", txName),
		Help:      fmt.Sprintf("Number of %s transactions accepted", txName),
	})
	errs.Add(registerer.Register(txMetric))
	return txMetric
}
func (m *txMetrics) ExportTx(*txs.ExportTx) error {
	m.numExportTxs.Inc()
	return nil
}
