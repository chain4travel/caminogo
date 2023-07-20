// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/vms/touristicvm/blocks"
)

const (
	lastAcceptedByte byte = iota
)

const (
	// maximum block capacity of the cache
	blockCacheSize = 8192
)

var (
	_ BlockState = &blockState{}
)

type BlockState interface {
	GetStatelessBlock(blockID ids.ID) (blocks.Block, choices.Status, error)
	AddStatelessBlock(block blocks.Block, status choices.Status)
}

// blockState implements BlocksState interface with database and cache.
type blockState struct {
	addedBlocks map[ids.ID]blkWrapper // map of blockID -> Block
	// cache to store blocks
	blockCache cache.Cacher[ids.ID, *blkWrapper]
	// block database
	blockDB      database.Database
	lastAccepted ids.ID
}

// blkWrapper wraps the actual blk bytes and status to persist them together
type blkWrapper struct {
	Bytes  []byte         `serialize:"true"`
	Status choices.Status `serialize:"true"`

	block blocks.Block
}

func (s *blockState) GetStatelessBlock(blockID ids.ID) (blocks.Block, choices.Status, error) {
	if blk, ok := s.addedBlocks[blockID]; ok {
		return blk.block, blk.Status, nil
	}
	if blkState, ok := s.blockCache.Get(blockID); ok {
		if blkState == nil {
			return nil, choices.Processing, database.ErrNotFound
		}
		return blkState.block, blkState.Status, nil
	}

	blkBytes, err := s.blockDB.Get(blockID[:])
	if err == database.ErrNotFound {
		s.blockCache.Put(blockID, nil)
		return nil, choices.Processing, database.ErrNotFound // status does not matter here
	} else if err != nil {
		return nil, choices.Processing, err // status does not matter here
	}

	// Note: stored blocks are verified, so it's safe to unmarshal them with GenesisCodec
	blkState := blkWrapper{}
	if _, err := blocks.GenesisCodec.Unmarshal(blkBytes, &blkState); err != nil {
		return nil, choices.Processing, err // status does not matter here
	}

	blkState.block, err = blocks.Parse(blocks.GenesisCodec, blkState.Bytes)
	if err != nil {
		return nil, choices.Processing, err
	}

	s.blockCache.Put(blockID, &blkState)
	return blkState.block, blkState.Status, nil
}

func (s *blockState) AddStatelessBlock(block blocks.Block, status choices.Status) {
	s.addedBlocks[block.ID()] = blkWrapper{
		block:  block,
		Bytes:  block.Bytes(),
		Status: status,
	}
}
