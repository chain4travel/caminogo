// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.
package metrics

import (
	"fmt"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/avalanchego/utils/wrappers"
)

var _ txs.Visitor = (*txMetrics)(nil)

type txMetrics struct {
	numBaseTxs               prometheus.Counter
	numImportTxs             prometheus.Counter
	numLockMessengerFundsTxs prometheus.Counter
	numCashoutChequeTxs      prometheus.Counter
}

func newTxMetrics(
	namespace string,
	registerer prometheus.Registerer,
) (*txMetrics, error) {
	errs := wrappers.Errs{}
	m := &txMetrics{
		numBaseTxs:               newTxMetric(namespace, "base", registerer, &errs),
		numImportTxs:             newTxMetric(namespace, "import", registerer, &errs),
		numLockMessengerFundsTxs: newTxMetric(namespace, "lock_messenger_funds", registerer, &errs),
		numCashoutChequeTxs:      newTxMetric(namespace, "cash_out", registerer, &errs),
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
func (m *txMetrics) BaseTx(*txs.BaseTx) error {
	m.numBaseTxs.Inc()
	return nil
}
func (m *txMetrics) ImportTx(*txs.ImportTx) error {
	m.numImportTxs.Inc()
	return nil
}

func (m *txMetrics) LockMessengerFundsTx(*txs.LockMessengerFundsTx) error {
	m.numLockMessengerFundsTxs.Inc()
	return nil
}

func (m *txMetrics) CashoutChequeTx(*txs.CashoutChequeTx) error {
	m.numCashoutChequeTxs.Inc()
	return nil
}