// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/vms/components/message"
	"github.com/ava-labs/avalanchego/vms/platformvm/block/executor"
	txBuilder "github.com/ava-labs/avalanchego/vms/platformvm/txs/builder"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs/mempool"
)

var errUnknownCrossChainMessage = errors.New("unknown cross-chain message")

type caminoNetwork struct {
	network
	txBuilder txBuilder.CaminoBuilder
}

func NewCamino(
	ctx *snow.Context,
	manager executor.Manager,
	mempool mempool.Mempool,
	partialSyncPrimaryNetwork bool,
	appSender common.AppSender,
	txBuilder txBuilder.CaminoBuilder,
) Network {
	return &caminoNetwork{
		network: network{
			AppHandler: common.NewNoOpAppHandler(ctx.Log),

			ctx:                       ctx,
			manager:                   manager,
			mempool:                   mempool,
			partialSyncPrimaryNetwork: partialSyncPrimaryNetwork,
			appSender:                 appSender,
			recentTxs:                 &cache.LRU[ids.ID, struct{}]{Size: recentCacheSize},
		},
		txBuilder: txBuilder,
	}
}

func (n *caminoNetwork) CrossChainAppRequest(_ context.Context, chainID ids.ID, _ uint32, _ time.Time, request []byte) error {
	n.ctx.Log.Debug("called CrossChainAppRequest message handler",
		zap.Stringer("chainID", chainID),
		zap.Int("messageLen", len(request)),
	)

	msg := &message.CaminoRewardMessage{}
	if _, err := message.Codec.Unmarshal(request, msg); err != nil {
		return errUnknownCrossChainMessage // this would be fatal
	}

	n.ctx.Lock.Lock()
	defer n.ctx.Lock.Unlock() // TODO@ need to include issueTx() ? or we can do it only for tx builder?

	tx, err := n.txBuilder.NewRewardsImportTx()
	if err != nil {
		n.ctx.Log.Error("caminoCrossChainAppRequest couldn't create rewardsImportTx", zap.Error(err))
		return nil // we don't want fatal here
	}

	if err := n.issueTx(tx); err != nil {
		n.ctx.Log.Error("caminoCrossChainAppRequest couldn't issue rewardsImportTx", zap.Error(err))
		// we don't want fatal here: its better to have network running
		// and try to repair stalled reward imports, than crash the whole network
	}

	return nil
}
