// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
)

var (
	_ UnsignedTx = (*LockMessengerFundsTx)(nil)

	errTooLargeAmountToLock = errors.New("too large amount to lock")
)

// LockMessengerFundsTx is an unsigned lockMessengerFundsTx
type LockMessengerFundsTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	amountToLock *uint64
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [LockMessengerFundsTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *LockMessengerFundsTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
}

func (tx *LockMessengerFundsTx) LockAmount() uint64 {
	if tx.amountToLock == nil {
		amountToLock := uint64(0)
		for _, out := range tx.Outs {
			if lockedOut, ok := out.Out.(*locked.Out); ok && lockedOut.IsNewlyLockedWith(locked.StateDeposited) {
				amountToLock += lockedOut.Amount()
			}
		}
		tx.amountToLock = &amountToLock
	}
	return *tx.amountToLock
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *LockMessengerFundsTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}

	amountToLock := uint64(0)
	for _, out := range tx.Outs {
		if lockedOut, ok := out.Out.(*locked.Out); ok && lockedOut.IsNewlyLockedWith(locked.StateDeposited) {
			newDepositAmount, err := math.Add64(amountToLock, lockedOut.Amount())
			if err != nil {
				return fmt.Errorf("%w: %s", errTooLargeAmountToLock, err)
			}
			amountToLock = newDepositAmount
		}
	}
	tx.amountToLock = &amountToLock

	// cache that this is valid
	tx.SyntacticallyVerified = true
	return nil
}

func (tx *LockMessengerFundsTx) Visit(visitor Visitor) error {
	return visitor.LockMessengerFundsTx(tx)
}
