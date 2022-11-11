// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"fmt"

	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
)

var _ ValidatorTx = (*CaminoAddValidatorTx)(nil)

// CaminoAddValidatorTx is an unsigned caminoAddValidatorTx
type CaminoAddValidatorTx struct {
	AddValidatorTx
}

// SyntacticVerify returns nil iff [tx] is valid
func (tx *CaminoAddValidatorTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	case tx.DelegationShares > reward.PercentDenominator: // Ensure delegators shares are in the allowed amount
		return errTooManyShares
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}
	if err := verify.All(&tx.Validator, tx.RewardsOwner); err != nil {
		return fmt.Errorf("failed to verify validator or rewards owner: %w", err)
	}

	totalStakeWeight := uint64(0)
	for _, out := range tx.Outs {
		lockedOut, ok := out.Out.(*locked.Out)
		if ok && lockedOut.IsNewlyLockedWith(locked.StateBonded) {
			newWeight, err := math.Add64(totalStakeWeight, lockedOut.Amount())
			if err != nil {
				return err
			}
			totalStakeWeight = newWeight

			assetID := out.AssetID()
			if assetID != ctx.AVAXAssetID {
				return fmt.Errorf("stake output must be AVAX but is %q", assetID)
			}
		}
	}

	switch {
	case !avax.IsSortedTransferableOutputs(tx.StakeOuts, Codec):
		return errOutputsNotSorted
	case totalStakeWeight != tx.Validator.Wght:
		// return fmt.Errorf("%w: weight %d != stake %d", errValidatorWeightMismatch, tx.Validator.Wght, totalStakeWeight)
	}

	// cache that this is valid
	tx.SyntacticallyVerified = true
	return nil
}