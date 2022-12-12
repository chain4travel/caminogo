// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
)

var (
	_ UnsignedTx = (*CreateVoteTx)(nil)
)

// CreateProposalTx is an unsigned CreateVoteTx
type CreateVoteTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// The vote to create
	Vote dao.Vote `serialize:"true"`
	// The proposal to vote for
	ProposalID ids.ID `serialize:"true"`
}

func (tx *CreateVoteTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *CreateVoteTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := locked.VerifyNoLocks(tx.Ins, tx.Outs); err != nil {
		return err
	}

	if err := tx.Vote.Verify(); err != nil {
		return err
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *CreateVoteTx) Visit(visitor Visitor) error {
	return visitor.CreateVoteTx(tx)
}
