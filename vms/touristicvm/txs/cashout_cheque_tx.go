// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

var _ UnsignedTx = (*CashoutChequeTx)(nil)

type Cheque struct {
	Issuer      ids.ShortID `serialize:"true" json:"issuer"`
	Beneficiary ids.ShortID `serialize:"true" json:"beneficiary"`
	Amount      uint64      `serialize:"true" json:"amount"`
	SerialID    uint64      `serialize:"true" json:"serialID"`
	Agent       ids.ShortID `serialize:"true" json:"agent"` // Agent is the node that issued the cheque
}

type SignedCheque struct {
	Cheque `serialize:"true"`
	Auth   verify.Verifiable `serialize:"true" json:"auth"`
}

// CashoutChequeTx is an unsigned cashoutChequeTx
type CashoutChequeTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// Cheque to cashout
	Cheque SignedCheque `serialize:"true" json:"cheque"`
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *CashoutChequeTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify CashoutChequeTx: %w", err)
	}

	// cache that this is valid
	tx.SyntacticallyVerified = true
	return nil
}

func (tx *CashoutChequeTx) Visit(visitor Visitor) error {
	return visitor.CashoutChequeTx(tx)
}
