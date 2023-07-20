// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
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

package builder

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/timestampvm/blocks"
	"github.com/ava-labs/avalanchego/vms/timestampvm/state"
	"github.com/ava-labs/avalanchego/vms/timestampvm/txs"
	"github.com/ava-labs/avalanchego/vms/timestampvm/txs/mempool"

	blockexecutor "github.com/ava-labs/avalanchego/vms/timestampvm/blocks/executor"
	txbuilder "github.com/ava-labs/avalanchego/vms/timestampvm/txs/builder"
	txexecutor "github.com/ava-labs/avalanchego/vms/timestampvm/txs/executor"
)

// targetBlockSize is maximum number of transaction bytes to place into a
// StandardBlock
const targetBlockSize = 128 * units.KiB

var (
	_ Builder = (*builder)(nil)

	errEndOfTime       = errors.New("program time is suspiciously far in the future")
	errNoPendingBlocks = errors.New("no pending blocks")
	errChainNotSynced  = errors.New("chain not synced")
)

type Builder interface {
	mempool.Mempool
	Network

	// set preferred block on top of which we'll build next
	SetPreference(blockID ids.ID)

	// get preferred block on top of which we'll build next
	Preferred() (snowman.Block, error)

	// AddUnverifiedTx verifier the tx before adding it to mempool
	AddUnverifiedTx(tx *txs.Tx) error

	// BuildBlock is called on timer clock to attempt to create
	// next block
	BuildBlock(context.Context) (snowman.Block, error)
}

// builder implements a simple builder to convert txs into valid blocks
type builder struct {
	mempool.Mempool
	Network

	txBuilder         txbuilder.Builder
	txExecutorBackend *txexecutor.Backend
	blkManager        blockexecutor.Manager

	// ID of the preferred block to build on top of
	preferredBlockID ids.ID

	// channel to send messages to the consensus engine
	toEngine chan<- common.Message
}

func New(
	mempool mempool.Mempool,
	txBuilder txbuilder.Builder,
	txExecutorBackend *txexecutor.Backend,
	blkManager blockexecutor.Manager,
	toEngine chan<- common.Message,
	appSender common.AppSender,
) Builder {
	builder := &builder{
		Mempool:           mempool,
		txBuilder:         txBuilder,
		txExecutorBackend: txExecutorBackend,
		blkManager:        blkManager,
		toEngine:          toEngine,
	}

	builder.Network = NewNetwork(
		txExecutorBackend.Ctx,
		builder,
		appSender,
	)

	return builder
}

func (b *builder) SetPreference(blockID ids.ID) {
	b.preferredBlockID = blockID
}

func (b *builder) Preferred() (snowman.Block, error) {
	return b.blkManager.GetBlock(b.preferredBlockID)
}

// AddUnverifiedTx verifies a transaction and attempts to add it to the mempool
func (b *builder) AddUnverifiedTx(tx *txs.Tx) error {
	if !b.txExecutorBackend.Bootstrapped.Get() {
		return errChainNotSynced
	}

	txID := tx.ID()
	if b.Mempool.Has(txID) {
		// If the transaction is already in the mempool - then it looks the same
		// as if it was successfully added
		return nil
	}

	verifier := txexecutor.MempoolTxVerifier{
		Backend:       b.txExecutorBackend,
		ParentID:      b.preferredBlockID, // We want to build off of the preferred block
		StateVersions: b.blkManager,
		Tx:            tx,
	}
	if err := tx.Unsigned.Visit(&verifier); err != nil {
		b.MarkDropped(txID, err)
		return err
	}

	if err := b.Mempool.Add(tx); err != nil {
		return err
	}
	return b.GossipTx(tx)
}

// BuildBlock builds a block to be added to consensus.
// This method removes the transactions from the returned
// blocks from the mempool.
func (b *builder) BuildBlock(context.Context) (snowman.Block, error) {
	b.Mempool.DisableAdding()
	defer func() {
		b.Mempool.EnableAdding()
	}()

	ctx := b.txExecutorBackend.Ctx
	ctx.Log.Debug("starting to attempt to build a block")

	statelessBlk, err := b.buildBlock()
	if err != nil {
		return nil, err
	}

	// Remove selected txs from mempool now that we are returning the block to
	// the consensus engine.
	txs := statelessBlk.Txs()
	b.Mempool.Remove(txs)
	return b.blkManager.NewBlock(statelessBlk), nil
}

// Returns the block we want to build and issue.
// Only modifies state to remove expired proposal txs.
func (b *builder) buildBlock() (blocks.Block, error) {
	// Get the block to build on top of and retrieve the new block's context.
	preferred, err := b.Preferred()
	if err != nil {
		return nil, err
	}
	preferredID := preferred.ID()
	nextHeight := preferred.Height() + 1
	preferredState, ok := b.blkManager.GetState(preferredID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", state.ErrMissingParentState, preferredID)
	}

	timestamp := b.txExecutorBackend.Clk.Time()
	if parentTime := preferred.Timestamp(); parentTime.After(timestamp) {
		timestamp = parentTime
	}

	return buildBlock(
		b,
		preferredID,
		nextHeight,
		timestamp,
		preferredState,
	)
}

// notifyBlockReady tells the consensus engine that a new block is ready to be
// created
func (b *builder) notifyBlockReady() {
	select {
	case b.toEngine <- common.PendingTxs:
	default:
		b.txExecutorBackend.Ctx.Log.Debug("dropping message to consensus engine")
	}
}

// [timestamp] is min(max(now, parent timestamp), next staker change time)
func buildBlock(
	builder *builder,
	parentID ids.ID,
	height uint64,
	timestamp time.Time,
	parentState state.Chain,
) (blocks.Block, error) {

	// If there is no reason to build a block, don't.
	if !builder.Mempool.HasTxs() {
		builder.txExecutorBackend.Ctx.Log.Debug("no pending txs to issue into a block")
		return nil, errNoPendingBlocks
	}

	// Issue a block with as many transactions as possible.
	return &blocks.StandardBlock{
		PrntID:       parentID,
		Hght:         height,
		Tmstmp:       uint64(timestamp.Unix()),
		Transactions: builder.Mempool.PeekTxs(targetBlockSize),
	}, nil
}
