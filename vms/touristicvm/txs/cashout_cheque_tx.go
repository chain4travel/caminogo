// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/verify"

	"github.com/ava-labs/avalanchego/snow"
)

var _ UnsignedTx = (*CashoutChequeTx)(nil)

// CashoutChequeTx is an unsigned cashoutChequeTx
type CashoutChequeTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	Amount      uint64            `serialize:"true" json:"amount"`
	Beneficiary ids.ShortID       `serialize:"true" json:"beneficiary"`
	Issuer      ids.ShortID       `serialize:"true" json:"issuer"`
	IssuerAuth  verify.Verifiable `serialize:"true" json:"issuerAuth"`
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
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}

	// cache that this is valid
	tx.SyntacticallyVerified = true
	return nil
}

func (tx *CashoutChequeTx) Visit(visitor Visitor) error {
	return visitor.CashoutChequeTx(tx)
}
