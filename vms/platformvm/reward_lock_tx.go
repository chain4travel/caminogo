// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************

// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"fmt"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/snow"
	"github.com/chain4travel/caminogo/utils/math"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
)

var (
	_ UnsignedProposalTx = &UnsignedRewardLockTx{}
)

// UnsignedRewardLockTx is a transaction that represents a proposal to
// send rewards to owner of unlocked tokens.
//
// If this transaction is accepted and the next block accepted is a Commit
// block, the reward out will be created owned by with locked tokens owner
// and tokens will unlock.
//
// If this transaction is accepted and the next block accepted is an Abort // TODO@evlekht confirm with team
// block, the locked tokens will unlock, but no reward out will be created
type UnsignedRewardLockTx struct {
	avax.Metadata

	// ID of the tx that locked tokens
	TxID ids.ID `serialize:"true" json:"txID"`
}

func (tx *UnsignedRewardLockTx) InitCtx(*snow.Context) {}

func (tx *UnsignedRewardLockTx) InputIDs() ids.Set { return nil }

func (tx *UnsignedRewardLockTx) SyntacticVerify(*snow.Context) error { return nil }

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedRewardLockTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	_, _, err := tx.Execute(vm, parentState, stx)
	return err
}

// Execute this transaction.
//
// The current validating set must have at least one member. // TODO@evlekht correct the description comment
// The next validator to be removed must be the validator specified in this block.
// The next validator to be removed must be have an end time equal to the current
// chain timestamp.
func (tx *UnsignedRewardLockTx) Execute(
	vm *VM,
	parentState MutableState,
	stx *Tx,
) (
	VersionedState,
	VersionedState,
	error,
) {
	switch {
	case tx == nil:
		return nil, nil, errNilTx
	case tx.TxID == ids.Empty:
		return nil, nil, errInvalidID
	case len(stx.Creds) != 0:
		return nil, nil, errWrongNumberOfCredentials
	}

	currentLocksState := parentState.CurrentLocksChainState()
	lockTx, lockReward, err := currentLocksState.GetNextLock()
	switch {
	case err == database.ErrNotFound:
		return nil, nil, fmt.Errorf("failed to get next lock stop time: %w", err)
	case err != nil:
		return nil, nil, err
	}

	lockTxID := lockTx.ID()
	if lockTxID != tx.TxID {
		return nil, nil, fmt.Errorf(
			"attempting to unlock TxID: %s. Should be unlocking %s",
			tx.TxID,
			lockTxID,
		)
	}

	// Verify that the chain's timestamp is the validator's end time
	currentTime := parentState.GetTimestamp()
	lockTimedTx, ok := lockTx.UnsignedTx.(TimedTx)
	if !ok {
		return nil, nil, errWrongTxType
	}
	if endTime := lockTimedTx.EndTime(); !endTime.Equal(currentTime) {
		return nil, nil, fmt.Errorf(
			"attempting to unlock TxID: %s before their end time %s",
			tx.TxID,
			endTime,
		)
	}

	newlyCurrentLocksState, err := currentLocksState.DeleteNextLock()
	if err != nil {
		return nil, nil, err
	}

	newlyCurrentStakerState := parentState.CurrentStakerChainState()
	pendingStakerState := parentState.PendingStakerChainState()

	onCommitState := newVersionedState(parentState, newlyCurrentStakerState, pendingStakerState, newlyCurrentLocksState)
	onAbortState := newVersionedState(parentState, newlyCurrentStakerState, pendingStakerState, newlyCurrentLocksState)

	// If the reward is aborted, then the current supply should be decreased.
	currentSupply := onAbortState.GetCurrentSupply()
	newSupply, err := math.Sub64(currentSupply, lockReward)
	if err != nil {
		return nil, nil, err
	}
	onAbortState.SetCurrentSupply(newSupply)

	switch uLockTx := lockTx.UnsignedTx.(type) {
	case *UnsignedAddLockTx:
		// Refund the stake here
		for i, out := range uLockTx.LockedAmount {
			utxo := &avax.UTXO{
				UTXOID: avax.UTXOID{
					TxID:        tx.TxID,
					OutputIndex: uint32(len(uLockTx.Outs) + i),
				},
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out:   out.Output(),
			}
			onCommitState.AddUTXO(utxo)
			onAbortState.AddUTXO(utxo)
		}

		// Provide the reward here
		if lockReward > 0 { // TODO@evlekht may be wrap into function - its used in validator rewards
			outIntf, err := vm.fx.CreateOutput(lockReward, uLockTx.RewardsOwner)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create output: %w", err)
			}
			out, ok := outIntf.(verify.State)
			if !ok {
				return nil, nil, errInvalidState
			}

			utxo := &avax.UTXO{
				UTXOID: avax.UTXOID{
					TxID:        tx.TxID,
					OutputIndex: uint32(len(uLockTx.Outs) + len(uLockTx.LockedAmount)),
				},
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out:   out,
			}

			onCommitState.AddUTXO(utxo)
			onCommitState.AddRewardUTXO(tx.TxID, utxo)
		}
	default:
		return nil, nil, errShouldBeDSValidator // TODO@evlekht change err ?
	}

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if this node thinks the locked tokens owner
// should receive a reward.
func (tx *UnsignedRewardLockTx) InitiallyPrefersCommit(*VM) bool {
	return true
}

// RewardLockTx creates a new transaction that proposes to send rewards to owner of locked tokens.
func (vm *VM) newRewardLockTx(txID ids.ID) (*Tx, error) {
	tx := &Tx{UnsignedTx: &UnsignedRewardLockTx{
		TxID: txID,
	}}
	return tx, tx.Sign(Codec, nil)
}
