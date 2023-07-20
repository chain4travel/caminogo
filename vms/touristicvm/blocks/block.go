// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
)

var (
	_ Block = &StandardBlock{}
)

type Block interface {
	ID() ids.ID
	Parent() ids.ID
	Height() uint64
	Timestamp() time.Time
	Bytes() []byte
	Txs() []*txs.Tx
	initialize(bytes []byte, cm codec.Manager) error
}

// StandardBlock is a block on the chain.
// Each block contains:
// 1) ParentID
// 2) Height
// 3) Timestamp
// 4) A piece of data (a string)
type StandardBlock struct {
	PrntID ids.ID `serialize:"true" json:"parentID"`  // parent's ID
	Hght   uint64 `serialize:"true" json:"height"`    // This block's height. The genesis block is at height 0.
	Tmstmp uint64 `serialize:"true" json:"timestamp"` // Time this block was proposed at

	id     ids.ID         // hold this block's ID
	bytes  []byte         // this block's encoded bytes
	status choices.Status // block's status

	Transactions []*txs.Tx `serialize:"true" json:"txs"`
}

func (b *StandardBlock) Txs() []*txs.Tx {
	return b.Transactions
}

func (b *StandardBlock) initialize(bytes []byte, cm codec.Manager) error {
	b.id = hashing.ComputeHash256Array(bytes)
	b.bytes = bytes
	for _, tx := range b.Transactions {
		if err := tx.Initialize(cm); err != nil {
			return fmt.Errorf("failed to initialize tx: %w", err)
		}
	}
	return nil
}

// Initialize sets [b.bytes] to [bytes], [b.id] to hash([b.bytes]),
// [b.status] to [status] and [b.vm] to [vm]
func (b *StandardBlock) Initialize(bytes []byte, status choices.Status) {
	b.bytes = bytes
	b.id = hashing.ComputeHash256Array(b.bytes)
	b.status = status
}

// ID returns the ID of this block
func (b *StandardBlock) ID() ids.ID { return b.id }

// ParentID returns [b]'s parent's ID
func (b *StandardBlock) Parent() ids.ID { return b.PrntID }

// Height returns this block's height. The genesis block has height 0.
func (b *StandardBlock) Height() uint64 { return b.Hght }

// Timestamp returns this block's time. The genesis block has time 0.
func (b *StandardBlock) Timestamp() time.Time { return time.Unix(int64(b.Tmstmp), 0) }

// Status returns the status of this block
func (b *StandardBlock) Status() choices.Status { return b.status }

// Bytes returns the byte repr. of this block
func (b *StandardBlock) Bytes() []byte { return b.bytes }

// Data returns the data of this block
// SetStatus sets the status of this block
func (b *StandardBlock) SetStatus(status choices.Status) { b.status = status }
