// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.
package metrics

import (
	"github.com/ava-labs/avalanchego/vms/timestampvm/blocks"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
)

var _ Metrics = (*metrics)(nil)

type Metrics interface {
	metric.APIInterceptor

	// Mark that an option vote that we initially preferred was accepted.
	MarkOptionVoteWon()
	// Mark that an option vote that we initially preferred was rejected.
	MarkOptionVoteLost()
	// Mark that the given block was accepted.
	MarkAccepted(blocks.StandardBlock) error
}

func New(
	namespace string,
	registerer prometheus.Registerer,
) (Metrics, error) {
	blockMetrics, err := newBlockMetrics(namespace, registerer)
	m := &metrics{
		blockMetrics: blockMetrics,

		percentConnected: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "percent_connected",
			Help:      "Percent of connected stake",
		}),
		subnetPercentConnected: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "percent_connected_subnet",
				Help:      "Percent of connected subnet weight",
			},
			[]string{"subnetID"},
		),

		numVotesWon: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "votes_won",
			Help:      "Total number of votes this node has won",
		}),
		numVotesLost: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "votes_lost",
			Help:      "Total number of votes this node has lost",
		}),
	}

	errs := wrappers.Errs{Err: err}
	apiRequestMetrics, err := metric.NewAPIInterceptor(namespace, registerer)
	m.APIInterceptor = apiRequestMetrics
	errs.Add(
		err,

		registerer.Register(m.percentConnected),
		registerer.Register(m.subnetPercentConnected),

		registerer.Register(m.numVotesWon),
		registerer.Register(m.numVotesLost),
	)

	return m, errs.Err
}

type metrics struct {
	metric.APIInterceptor

	blockMetrics *blockMetrics

	percentConnected       prometheus.Gauge
	subnetPercentConnected *prometheus.GaugeVec

	numVotesWon, numVotesLost prometheus.Counter
}

func (m *metrics) MarkOptionVoteWon() {
	m.numVotesWon.Inc()
}

func (m *metrics) MarkOptionVoteLost() {
	m.numVotesLost.Inc()
}

func (m *metrics) MarkAccepted(b blocks.StandardBlock) error {
	m.blockMetrics.numStandardBlocks.Inc()
	for _, tx := range b.Transactions {
		if err := tx.Unsigned.Visit(m.blockMetrics.txMetrics); err != nil {
			return err
		}
	}
	return nil
}
