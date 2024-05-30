// Copyright (C) 2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/components/message"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs/mempool"
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

func (n *caminoNetwork) CrossChainAppRequest(ctx context.Context, chainID ids.ID, requestID uint32, _ time.Time, request []byte) error {
	n.log.Debug("called CrossChainAppRequest message handler",
		zap.Stringer("chainID", chainID),
		zap.Uint32("requestID", requestID),
		zap.Int("messageLen", len(request)),
	)

	msg := &message.CaminoRewardMessage{}
	if _, err := message.Codec.Unmarshal(request, msg); err != nil {
		return errUnknownCrossChainMessage // this would be fatal
	}

	if err := n.appSender.SendCrossChainAppResponse(
		ctx,
		chainID,
		requestID,
		[]byte(n.caminoRewardMessage()),
	); err != nil {
		n.log.Error("caminoCrossChainAppRequest failed to send response", zap.Error(err))
		// we don't want fatal here: response is for logging only, so
		// its better to not respond properly, than crash the whole node
		return nil
	}

	return nil
}

func (n *caminoNetwork) caminoRewardMessage() string {
	tx, err := n.newRewardsImportTx()
	if err != nil {
		return err.Error()
	}

	utx, ok := tx.Unsigned.(*txs.RewardsImportTx)
	if !ok {
		// should never happen
		err = fmt.Errorf("unexpected tx type: expected *txs.RewardsImportTx, got %T", utx)
		n.log.Error("caminoCrossChainAppRequest failed to create rewardsImportTx", zap.Error(err))
		return fmt.Sprintf("caminoCrossChainAppRequest failed to issue rewardsImportTx: %s", err)
	}

	if err := n.issueTx(tx); err != nil {
		n.log.Error("caminoCrossChainAppRequest failed to issue rewardsImportTx", zap.Error(err))
		return fmt.Sprintf("caminoCrossChainAppRequest failed to issue rewardsImportTx: %s", err)
	}

	amts := make([]uint64, len(utx.Ins))
	for i := range utx.Ins {
		amts[i] = utx.Ins[i].In.Amount()
	}

	return fmt.Sprintf("caminoCrossChainAppRequest issued rewardsImportTx with utxos with %v nCAM", amts)
}

func (n *caminoNetwork) newRewardsImportTx() (*txs.Tx, error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	tx, err := n.txBuilder.NewRewardsImportTx()
	if err != nil {
		n.log.Error("caminoCrossChainAppRequest failed to create rewardsImportTx", zap.Error(err))
		return nil, fmt.Errorf("caminoCrossChainAppRequest failed to create rewardsImportTx: %w", err)
	}
	return tx, nil
}
