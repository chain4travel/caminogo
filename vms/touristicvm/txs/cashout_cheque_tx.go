// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"encoding/binary"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

var _ UnsignedTx = (*CashoutChequeTx)(nil)

type Cheque struct {
	Issuer      ids.ShortID `serialize:"true" json:"issuer"`
	Beneficiary ids.ShortID `serialize:"true" json:"beneficiary"`
	Amount      uint64      `serialize:"true" json:"amount"`
	SerialID    uint64      `serialize:"true" json:"serialID"`
	Agent       ids.ShortID `serialize:"true" json:"agent"` // Agent is the node that issued the cheque
}

// A cheque's msg payload to be signed results from concatenating the following fields:
// 1. Issuer
// 2. Beneficiary
// 3. Amount
// 4. SerialID
// 5. Agent
func (c *Cheque) BuildMsgToSign() []byte {
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, c.Amount)

	serialIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(serialIDBytes, c.SerialID)

	msgToSign := append(c.Issuer.Bytes(), c.Beneficiary.Bytes()...)
	msgToSign = append(msgToSign, amountBytes...)
	msgToSign = append(msgToSign, serialIDBytes...)
	msgToSign = append(msgToSign, c.Agent.Bytes()...)
	return msgToSign
}

type SignedCheque struct {
	Cheque `serialize:"true"`
	Auth   secp256k1fx.CredentialIntf `serialize:"true" json:"auth"`
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
