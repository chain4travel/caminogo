// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
)

var (
	_ UnsignedTx = (*FinishProposalsTx)(nil)

	errNoFinishedProposals = errors.New("no expired or successful proposals")
	errNotUniqueProposalID = errors.New("not unique proposal id")
)

// FinishProposalsTx is an unsigned removeExpiredProposalsTx
type FinishProposalsTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// Proposals that will be removed. Successful proposals will be executed.
	FinishedProposalIDs []ids.ID `serialize:"true" json:"finishedProposalIDs"`
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *FinishProposalsTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	case len(tx.FinishedProposalIDs) == 0:
		return errNoFinishedProposals
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}

	uniqueProposals := set.NewSet[ids.ID](len(tx.FinishedProposalIDs))
	for _, proposalID := range tx.FinishedProposalIDs {
		if uniqueProposals.Contains(proposalID) {
			return errNotUniqueProposalID
		}
		uniqueProposals.Add(proposalID)
	}

	if err := locked.VerifyLockMode(tx.Ins, tx.Outs, true); err != nil {
		return err
	}

	// cache that this is valid
	tx.SyntacticallyVerified = true
	return nil
}

func (tx *FinishProposalsTx) Visit(visitor Visitor) error {
	return visitor.FinishProposalsTx(tx)
}
