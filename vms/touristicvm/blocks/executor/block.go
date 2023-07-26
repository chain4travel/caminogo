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

package executor

import (
	"context"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/utils/set"
	"time"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/chains/atomic"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/vms/touristicvm/blocks"
	"github.com/ava-labs/avalanchego/vms/touristicvm/state"
	"github.com/ava-labs/avalanchego/vms/touristicvm/status"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs/executor"
)

var (
	errTimestampTooEarly    = errors.New("block's timestamp is earlier than its parent's timestamp")
	errDatabaseGet          = errors.New("error while retrieving data from database")
	errTimestampTooLate     = errors.New("block's timestamp is more than 1 hour ahead of local time")
	errConflictingBatchTxs  = errors.New("block contains conflicting transactions")
	errConflictingParentTxs = errors.New("block contains a transaction that conflicts with a transaction in a parent block")

	errEmptyBlock               = errors.New("block contains no transactions")
	_             snowman.Block = (*Block)(nil)
)

// Exported for testing in touristicVM package.
type Block struct {
	blocks.Block
	manager *manager
}

// Verify returns nil iff this block is valid.
// To be valid, it must be that:
// b.parent.Timestamp < b.Timestamp <= [local time] + 1 hour
func (b *Block) Verify(context.Context) error {
	parentID := b.Parent()
	onAcceptState, err := state.NewDiff(parentID, b.manager.backend)
	if err != nil {
		return err
	}

	// Apply the changes, if any, from advancing the chain time.
	nextChainTime := b.Timestamp()

	if len(b.Txs()) == 0 {
		return errEmptyBlock
	}

	onAcceptState.SetTimestamp(nextChainTime)

	blkState := &blockState{
		statelessBlock: b,
		onAcceptState:  onAcceptState,
		timestamp:      onAcceptState.GetTimestamp(),
		atomicRequests: make(map[ids.ID]*atomic.Requests),
	}

	// Finally we process the transactions
	funcs := make([]func(), 0, len(b.Txs()))
	for _, tx := range b.Txs() {
		txExecutor := executor.StandardTxExecutor{
			Backend: b.manager.txExecutorBackend,
			State:   onAcceptState,
			Tx:      tx,
		}
		if err := tx.Unsigned.Visit(&txExecutor); err != nil {
			txID := tx.ID()
			b.manager.backend.Mempool.MarkDropped(txID, err) // cache tx as dropped
			return err
		}
		// ensure it doesn't overlap with current input batch
		if blkState.inputs.Overlaps(txExecutor.Inputs) {
			return errConflictingBatchTxs
		}
		// Add UTXOs to batch
		blkState.inputs.Union(txExecutor.Inputs)

		onAcceptState.AddTx(tx, status.Committed)
		if txExecutor.OnAccept != nil {
			funcs = append(funcs, txExecutor.OnAccept)
		}

		for chainID, txRequests := range txExecutor.AtomicRequests {
			// Add/merge in the atomic requests represented by [tx]
			chainRequests, exists := blkState.atomicRequests[chainID]
			if !exists {
				blkState.atomicRequests[chainID] = txRequests
				continue
			}

			chainRequests.PutRequests = append(chainRequests.PutRequests, txRequests.PutRequests...)
			chainRequests.RemoveRequests = append(chainRequests.RemoveRequests, txRequests.RemoveRequests...)
		}
	}

	if err := b.verifyUniqueInputs(b, blkState.inputs); err != nil {
		return err
	}

	if numFuncs := len(funcs); numFuncs == 1 {
		blkState.onAcceptFunc = funcs[0]
	} else if numFuncs > 1 {
		blkState.onAcceptFunc = func() {
			for _, f := range funcs {
				f()
			}
		}
	}

	blkID := b.ID()
	b.manager.blkIDToState[blkID] = blkState

	b.manager.Mempool.Remove(b.Txs())
	return nil
}

// Accept sets this block's status to Accepted and sets lastAccepted to this
// block's ID and saves this info to b.vm.DB
func (b *Block) Accept(context.Context) error {
	b.manager.ctx.Log.Debug(
		"accepting block",
		zap.String("blockType", "apricot standard"),
		zap.Stringer("blkID", b.ID()),
		zap.Uint64("height", b.Height()),
		zap.Stringer("parentID", b.Parent()),
	)
	blkID := b.ID()
	defer b.manager.backend.free(blkID)

	if err := b.commonAccept(); err != nil {
		return err
	}

	blkState, ok := b.manager.blkIDToState[blkID]
	if !ok {
		return fmt.Errorf("couldn't find state of block %s", blkID)
	}

	// Update the state to reflect the changes made in [onAcceptState].
	blkState.onAcceptState.Apply(b.manager.state)

	defer b.manager.state.Abort()
	batch, err := b.manager.state.CommitBatch()
	if err != nil {
		return fmt.Errorf(
			"failed to commit VM's database for block %s: %w",
			blkID,
			err,
		)
	}

	// Note that this method writes [batch] to the database.
	if err := b.manager.ctx.SharedMemory.Apply(blkState.atomicRequests, batch); err != nil {
		return fmt.Errorf("failed to apply vm's state to shared memory: %w", err)
	}

	if onAcceptFunc := blkState.onAcceptFunc; onAcceptFunc != nil {
		onAcceptFunc()
	}
	return nil
}

// Reject sets this block's status to Rejected and saves the status in state
// Recall that b.vm.DB.Commit() must be called to persist to the DB
func (b *Block) Reject(context.Context) error {

	blkID := b.ID()
	defer b.manager.free(blkID)

	b.manager.ctx.Log.Verbo(
		"rejecting block",
		zap.Stringer("blkID", blkID),
		zap.Uint64("height", b.Height()),
		zap.Stringer("parentID", b.Parent()),
	)

	for _, tx := range b.Txs() {
		if err := b.manager.Mempool.Add(tx); err != nil {
			b.manager.ctx.Log.Debug(
				"failed to reissue tx",
				zap.Stringer("txID", tx.ID()),
				zap.Stringer("blkID", blkID),
				zap.Error(err),
			)
		}
	}

	b.manager.state.AddStatelessBlock(b, choices.Rejected)
	return b.manager.state.Commit()
}

func (b *Block) Status() choices.Status {
	blkID := b.ID()
	// If this block is an accepted Proposal block with no accepted children, it
	// will be in [blkIDToState], but we should return accepted, not processing,
	// so we do this check.
	if b.manager.lastAccepted == blkID {
		return choices.Accepted
	}
	// Check if the block is in memory. If so, it's processing.
	if _, ok := b.manager.blkIDToState[blkID]; ok {
		return choices.Processing
	}
	// Block isn't in memory. Check in the database.
	_, status, err := b.manager.state.GetStatelessBlock(blkID)
	switch err {
	case nil:
		return status

	case database.ErrNotFound:
		// choices.Unknown means we don't have the bytes of the block.
		// In this case, we do, so we return choices.Processing.
		return choices.Processing

	default:
		// TODO: correctly report this error to the consensus engine.
		b.manager.ctx.Log.Error(
			"dropping unhandled database error",
			zap.Error(err),
		)
		return choices.Processing
	}
}

func (b *Block) Timestamp() time.Time {
	return b.manager.getTimestamp(b.ID())
}

// verifyUniqueInputs verifies that the inputs of the given block are not
// duplicated in any of the parent blocks pinned in memory.
func (b *Block) verifyUniqueInputs(block blocks.Block, inputs set.Set[ids.ID]) error {
	if inputs.Len() == 0 {
		return nil
	}

	// Check for conflicts in ancestors.
	for {
		parentID := block.Parent()
		parentState, ok := b.manager.blkIDToState[parentID]
		if !ok {
			// The parent state isn't pinned in memory.
			// This means the parent must be accepted already.
			return nil
		}

		if parentState.inputs.Overlaps(inputs) {
			return errConflictingParentTxs
		}

		block = parentState.statelessBlock
	}
}

func (b *Block) commonAccept() error {
	blkID := b.ID()

	if err := b.manager.metrics.MarkAccepted(b); err != nil {
		return fmt.Errorf("failed to accept block %s: %w", blkID, err)
	}

	b.manager.backend.lastAccepted = blkID
	b.manager.state.SetLastAccepted(blkID)
	//a.state.SetHeight(b.Height())
	b.manager.state.AddStatelessBlock(b, choices.Accepted)
	//a.recentlyAccepted.Add(blkID) // TODO nikos check if this is needed
	return nil
}
