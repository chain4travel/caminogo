// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
)

var (
	_ UnsignedTx = (*CreateProposalTx)(nil)
)

// CreateProposalTx is an unsigned CreateProposalTx
type CreateProposalTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// The proposal to create
	Proposal dao.Proposal
}

func (tx *CreateProposalTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *CreateProposalTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := locked.VerifyNoLocks(tx.Ins, tx.Outs); err != nil {
		return err
	}

	if err := tx.Proposal.Verify(); err != nil {
		return err
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *CreateProposalTx) Visit(visitor Visitor) error {
	return visitor.CreateProposalTx(tx)
}
