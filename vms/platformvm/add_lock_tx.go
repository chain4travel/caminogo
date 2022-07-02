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
	"errors"
	"fmt"
	"time"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/snow"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/utils/math"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

var (
	errLockTooShort       = errors.New("locking period is too short")
	errLockTooLong        = errors.New("locking period is too long")
	errLockAmountTooSmall = errors.New("amount of this lock is too low")

	_ UnsignedProposalTx = &UnsignedAddDelegatorTx{}
	_ TimedTx            = &UnsignedAddDelegatorTx{}
)

// UnsignedAddLockTx is an unsigned addLockTx
type UnsignedAddLockTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	// Where to send locked tokens when they unlock
	LockedOuts []*avax.TransferableOutput `serialize:"true" json:"stake"`

	// Unix time this lock starts
	Start uint64 `serialize:"true" json:"startTime"`

	// Unix time this lock finishes (unlocks)
	End uint64 `serialize:"true" json:"endTime"`

	Amount uint64 `serialize:"true" json:"amount"`

	// Where to send lock rewards when lock finished
	RewardsOwner Owner `serialize:"true" json:"rewardsOwner"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedAddLockTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedAddLockTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
	for _, out := range tx.LockedOuts {
		out.FxID = secp256k1fx.ID
		out.InitCtx(ctx)
	}
	tx.RewardsOwner.InitCtx(ctx)
}

// StartTime is the time when this lock begins
func (tx *UnsignedAddLockTx) StartTime() time.Time { return time.Unix(int64(tx.Start), 0) }

// EndTime is the time when this lock ends (unlocks)
func (tx *UnsignedAddLockTx) EndTime() time.Time { return time.Unix(int64(tx.End), 0) }

// Duration is the amount of time that this lock will exist (EndTime - StartTime)
func (tx *UnsignedAddLockTx) Duration() time.Duration { return tx.EndTime().Sub(tx.StartTime()) }

// Amount of locked tokens
func (tx *UnsignedAddLockTx) Weight() uint64 {
	return tx.Amount
}

// SyntacticVerify returns nil iff [tx] is valid
func (tx *UnsignedAddLockTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return errNilTx
	case tx.syntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}
	if err := verify.All(tx.RewardsOwner); err != nil {
		return fmt.Errorf("failed to verify rewards owner: %w", err)
	}

	totalLockAmount := uint64(0)
	for _, out := range tx.LockedOuts {
		if err := out.Verify(); err != nil {
			return fmt.Errorf("output verification failed: %w", err)
		}
		newLockAmount, err := math.Add64(totalLockAmount, out.Output().Amount())
		if err != nil {
			return err
		}
		totalLockAmount = newLockAmount
	}

	switch {
	case !avax.IsSortedTransferableOutputs(tx.LockedOuts, Codec):
		return errOutputsNotSorted
	}

	// cache that this is valid
	tx.syntacticallyVerified = true
	return nil
}

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedAddLockTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	startTime := tx.StartTime()
	maxLocalStartTime := vm.clock.Time().Add(maxFutureStartTime)
	if startTime.After(maxLocalStartTime) {
		return errFutureStartTime
	}

	_, _, err := tx.Execute(vm, parentState, stx)
	// We ignore [errFutureStartTime] here because an advanceTimeTx will be
	// issued before this transaction is issued.
	if errors.Is(err, errFutureStartTime) {
		return nil
	}
	return err
}

// Execute this transaction.
func (tx *UnsignedAddLockTx) Execute(
	vm *VM,
	parentState MutableState,
	stx *Tx,
) (
	VersionedState,
	VersionedState,
	error,
) {
	// Verify the tx is well-formed
	if err := tx.SyntacticVerify(vm.ctx); err != nil {
		return nil, nil, err
	}

	duration := tx.Duration()
	switch {
	case duration < vm.LockRewardConfig.MinLockDuration: // Ensure locking length is not too short
		return nil, nil, errLockTooShort
	case duration > vm.LockRewardConfig.MaxLockDuration: // Ensure locking length is not too long
		return nil, nil, errLockTooLong
	case tx.Amount < vm.MinLockAmount: // Ensure user is locking at least the minimum amount
		return nil, nil, errLockAmountTooSmall
	}

	outs := make([]*avax.TransferableOutput, len(tx.Outs)+len(tx.LockedOuts))
	copy(outs, tx.Outs)
	copy(outs[len(tx.Outs):], tx.LockedOuts)

	if vm.bootstrapped.GetValue() {
		currentTimestamp := parentState.GetTimestamp()
		// Ensure the lock starts after the current timestamp
		lockStartTime := tx.StartTime()
		if !currentTimestamp.Before(lockStartTime) {
			return nil, nil, fmt.Errorf(
				"chain timestamp (%s) not before lock's start time (%s)",
				currentTimestamp,
				lockStartTime,
			)
		}

		// Verify the flowcheck
		if err := vm.semanticVerifySpend(parentState, tx, tx.Ins, outs, stx.Creds, vm.AddLockTxFee, vm.ctx.AVAXAssetID); err != nil {
			return nil, nil, fmt.Errorf("failed semanticVerifySpend: %w", err)
		}

		// Make sure the tx doesn't start too far in the future. This is done
		// last to allow SemanticVerification to explicitly check for this
		// error.
		maxStartTime := vm.clock.Time().Add(maxFutureStartTime)
		if lockStartTime.After(maxStartTime) {
			return nil, nil, errFutureStartTime
		}
	}

	currentStakers := parentState.CurrentStakerChainState()
	pendingStakers := parentState.PendingStakerChainState()
	lockState := parentState.LockChainState()
	newlyLockState := lockState.AddLock(stx, vm.lockRewardCalculator.CalculateReward(tx.Duration(), tx.Amount))

	// Set up the state if this tx is committed
	onCommitState := newVersionedState(parentState, currentStakers, pendingStakers, newlyLockState)

	// Consume the UTXOS
	consumeInputs(onCommitState, tx.Ins)
	// Produce the UTXOS
	txID := tx.ID()
	produceOutputs(onCommitState, txID, vm.ctx.AVAXAssetID, tx.Outs)

	// Set up the state if this tx is aborted
	onAbortState := newVersionedState(parentState, currentStakers, pendingStakers, lockState)
	// Consume the UTXOS
	consumeInputs(onAbortState, tx.Ins)
	// Produce the UTXOS
	produceOutputs(onAbortState, txID, vm.ctx.AVAXAssetID, outs)

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if the lock start time is
// after the current wall clock time
func (tx *UnsignedAddLockTx) InitiallyPrefersCommit(vm *VM) bool {
	return tx.StartTime().After(vm.clock.Time())
}

// Creates a new transaction
func (vm *VM) newAddLockTx(
	lockAmount, // Amount the user locks
	startTime, // Unix time the tokens lock
	endTime uint64, // Unix time tokens unlock
	rewardAddress ids.ShortID, // Address to send reward to, if applicable
	keys []*crypto.PrivateKeySECP256K1R, // Keys providing the locked tokens
	changeAddr ids.ShortID, // Address to send change to, if there is any
) (*Tx, error) {
	ins, unlockedOuts, lockedOuts, signers, err := vm.stake(keys, lockAmount, vm.AddLockTxFee, changeAddr)
	if err != nil {
		return nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}
	// Create the tx
	utx := &UnsignedAddLockTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         unlockedOuts,
		}},
		Start:      startTime,
		End:        endTime,
		Amount:     lockAmount,
		LockedOuts: lockedOuts,
		RewardsOwner: &secp256k1fx.OutputOwners{
			Locktime:  0,
			Threshold: 1,
			Addrs:     []ids.ShortID{rewardAddress},
		},
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, signers); err != nil {
		return nil, err
	}
	return tx, utx.SyntacticVerify(vm.ctx)
}
