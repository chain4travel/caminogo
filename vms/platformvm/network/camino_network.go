// Copyright (C) 2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/components/message"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs/mempool"
	"github.com/prometheus/client_golang/prometheus"
)

var errUnknownCrossChainMessage = errors.New("unknown cross-chain message")

type SystemTxBuilder interface {
	NewRewardsImportTx() (*txs.Tx, error)
}

type caminoNetwork struct {
	*network
	txBuilder SystemTxBuilder
	lock      sync.Locker
}

func NewCamino(
	log logging.Logger,
	nodeID ids.NodeID,
	subnetID ids.ID,
	vdrs validators.State,
	txVerifier TxVerifier,
	mempool mempool.Mempool,
	partialSyncPrimaryNetwork bool,
	appSender common.AppSender,
	registerer prometheus.Registerer,
	config Config,
	txBuilder SystemTxBuilder,
	lock sync.Locker,
) (Network, error) {
	avaxNetwork, err := New(
		log,
		nodeID,
		subnetID,
		vdrs,
		txVerifier,
		mempool,
		partialSyncPrimaryNetwork,
		appSender,
		registerer,
		config,
	)
	if err != nil {
		return nil, err
	}

	return &caminoNetwork{
		network:   avaxNetwork.(*network),
		txBuilder: txBuilder,
		lock:      lock,
	}, nil
}

func (n *caminoNetwork) CrossChainAppRequest(_ context.Context, chainID ids.ID, _ uint32, _ time.Time, request []byte) error {
	n.log.Debug("called CrossChainAppRequest message handler",
		zap.Stringer("chainID", chainID),
		zap.Int("messageLen", len(request)),
	)

	msg := &message.CaminoRewardMessage{}
	if _, err := message.Codec.Unmarshal(request, msg); err != nil {
		return errUnknownCrossChainMessage // this would be fatal
	}

	tx := n.newRewardsImportTx()
	if tx == nil {
		return nil
	}

	if err := n.issueTx(tx); err != nil {
		n.log.Error("caminoCrossChainAppRequest couldn't issue rewardsImportTx", zap.Error(err))
		// we don't want fatal here: its better to have network running
		// and try to repair stalled reward imports, than crash the whole network
		// TODO@ evlekht check if it still fatals
	}

	return nil
}

func (n *caminoNetwork) newRewardsImportTx() *txs.Tx {
	n.lock.Lock()
	defer n.lock.Unlock()

	tx, err := n.txBuilder.NewRewardsImportTx()
	if err != nil {
		n.log.Error("caminoCrossChainAppRequest couldn't create rewardsImportTx", zap.Error(err))
		return nil // we don't want fatal here // TODO@ evlekht check if it still fatals
	}
	return tx
}
