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
	// errInvalidState = errors.New("generated output isn't valid state")

	_ UnsignedProposalTx = &UnsignedAddDelegatorTx{}
	_ TimedTx            = &UnsignedAddDelegatorTx{}
)

// UnsignedAddLockTx is an unsigned addLockTx
type UnsignedAddLockTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	// Where to send locked tokens when they unlock
	LockedAmount []*avax.TransferableOutput `serialize:"true" json:"stake"`

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
	for _, out := range tx.LockedAmount {
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
	for _, out := range tx.LockedAmount {
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
	case !avax.IsSortedTransferableOutputs(tx.LockedAmount, Codec):
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
	case duration < vm.MinStakeDuration: // Ensure locking length is not too short
		return nil, nil, errStakeTooShort
	case duration > vm.MaxStakeDuration: // Ensure locking length is not too long
		return nil, nil, errStakeTooLong
	case tx.Weight() < vm.MinDelegatorStake: // Ensure user is locking at least the minimum amount
		return nil, nil, errWeightTooSmall
	}

	outs := make([]*avax.TransferableOutput, len(tx.Outs)+len(tx.LockedAmount))
	copy(outs, tx.Outs)
	copy(outs[len(tx.Outs):], tx.LockedAmount)

	currentStakers := parentState.CurrentStakerChainState()
	pendingStakers := parentState.PendingStakerChainState()

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
		if err := vm.semanticVerifySpend(parentState, tx, tx.Ins, outs, stx.Creds, vm.AddStakerTxFee, vm.ctx.AVAXAssetID); err != nil { // TODO@evlekht fee
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
	currentLockState := parentState.CurrentLocksChainState()

	// Set up the state if this tx is committed
	onCommitState := newVersionedState(parentState, currentStakers, pendingStakers, currentLockState)

	// Consume the UTXOS
	consumeInputs(onCommitState, tx.Ins)
	// Produce the UTXOS
	txID := tx.ID()
	produceOutputs(onCommitState, txID, vm.ctx.AVAXAssetID, tx.Outs)

	// Set up the state if this tx is aborted
	onAbortState := newVersionedState(parentState, currentStakers, pendingStakers, currentLockState)
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
	ins, unlockedOuts, lockedOuts, signers, err := vm.stake(keys, lockAmount, vm.AddStakerTxFee, changeAddr)
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
		Start:        startTime,
		End:          endTime,
		Amount:       lockAmount,
		LockedAmount: lockedOuts,
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

// // Return the maximum amount of stake on a node (including delegations) at any
// // given time between [startTime] and [endTime] given that:
// // * The amount of stake on the node right now is [currentStake]
// // * The delegations currently on this node are [current]
// // * [current] is sorted in order of increasing delegation end time.
// // * The stake delegated in [current] are already included in [currentStake]
// // * [startTime] is in the future, and [endTime] > [startTime]
// // * The delegations that will be on this node in the future are [pending]
// // * The start time of all delegations in [pending] are in the future
// // * [pending] is sorted in order of increasing delegation start time
// func maxStakeAmount(
// 	current,
// 	pending []*UnsignedAddDelegatorTx, // sorted by next start time first
// 	startTime time.Time,
// 	endTime time.Time,
// 	currentStake uint64,
// ) (uint64, error) {
// 	// Keep track of which delegators should be removed next so that we can
// 	// efficiently remove delegators and keep the current stake updated.
// 	toRemoveHeap := validatorHeap{}
// 	for _, currentDelegator := range current {
// 		toRemoveHeap.Add(&currentDelegator.Validator)
// 	}

// 	var (
// 		err error
// 		// [maxStake] is the max stake at any point between now [starTime] and [endTime]
// 		maxStake uint64
// 	)

// 	// Calculate what the amount staked will be when each pending delegation
// 	// starts.
// 	for _, nextPending := range pending { // Iterates in order of increasing start time
// 		// Calculate what the amount staked will be when this delegation starts.
// 		nextPendingStartTime := nextPending.StartTime()

// 		if nextPendingStartTime.After(endTime) {
// 			// This delegation starts after [endTime].
// 			// Since we're calculating the max amount staked in
// 			// [startTime, endTime], we can stop. (Recall that [pending] is
// 			// sorted in order of increasing end time.)
// 			break
// 		}

// 		// Subtract from [currentStake] all of the current delegations that will
// 		// have ended by the time that the delegation [nextPending] starts.
// 		for toRemoveHeap.Len() > 0 {
// 			// Get the next current delegation that will end.
// 			toRemove := toRemoveHeap.Peek()
// 			toRemoveEndTime := toRemove.EndTime()
// 			if toRemoveEndTime.After(nextPendingStartTime) {
// 				break
// 			}
// 			// This current delegation [toRemove] ends before [nextPending]
// 			// starts, so its stake should be subtracted from [currentStake].

// 			// Changed in AP3:
// 			// If the new delegator has started, then this current delegator
// 			// should have an end time that is > [startTime].
// 			newDelegatorHasStartedBeforeFinish := toRemoveEndTime.After(startTime)
// 			if newDelegatorHasStartedBeforeFinish && currentStake > maxStake {
// 				// Only update [maxStake] if it's after [startTime]
// 				maxStake = currentStake
// 			}

// 			currentStake, err = math.Sub64(currentStake, toRemove.Wght)
// 			if err != nil {
// 				return 0, err
// 			}

// 			// Changed in AP3:
// 			// Remove the delegator from the heap and update the heap so that
// 			// the top of the heap is the next delegator to remove.
// 			toRemoveHeap.Remove()
// 		}

// 		// Add to [currentStake] the stake of this pending delegator to
// 		// calculate what the stake will be when this pending delegation has
// 		// started.
// 		currentStake, err = math.Add64(currentStake, nextPending.Validator.Wght)
// 		if err != nil {
// 			return 0, err
// 		}

// 		// Changed in AP3:
// 		// If the new delegator has started, then this pending delegator should
// 		// have a start time that is >= [startTime]. Otherwise, the delegator
// 		// hasn't started yet and the [currentStake] shouldn't count towards the
// 		// [maximumStake] during the delegators delegation period.
// 		newDelegatorHasStarted := !nextPendingStartTime.Before(startTime)
// 		if newDelegatorHasStarted && currentStake > maxStake {
// 			// Only update [maxStake] if it's after [startTime]
// 			maxStake = currentStake
// 		}

// 		// This pending delegator is a current delegator relative
// 		// when considering later pending delegators that start late
// 		toRemoveHeap.Add(&nextPending.Validator)
// 	}

// 	// [currentStake] is now the amount staked before the next pending delegator
// 	// whose start time is after [endTime].

// 	// If there aren't any delegators that will be added before the end of our
// 	// delegation period, we should advance through time until our delegation
// 	// period starts.
// 	for toRemoveHeap.Len() > 0 {
// 		toRemove := toRemoveHeap.Peek()
// 		toRemoveEndTime := toRemove.EndTime()
// 		if toRemoveEndTime.After(startTime) {
// 			break
// 		}

// 		currentStake, err = math.Sub64(currentStake, toRemove.Wght)
// 		if err != nil {
// 			return 0, err
// 		}

// 		// Changed in AP3:
// 		// Remove the delegator from the heap and update the heap so that the
// 		// top of the heap is the next delegator to remove.
// 		toRemoveHeap.Remove()
// 	}

// 	// We have advanced time to be inside the delegation window.
// 	// Make sure that the max stake is updated accordingly.
// 	if currentStake > maxStake {
// 		maxStake = currentStake
// 	}
// 	return maxStake, nil
// }

// func (vm *VM) maxStakeAmount(
// 	subnetID ids.ID,
// 	nodeID ids.ShortID,
// 	startTime time.Time,
// 	endTime time.Time,
// ) (uint64, error) {
// 	if startTime.After(endTime) {
// 		return 0, errStartAfterEndTime
// 	}
// 	if timestamp := vm.internalState.GetTimestamp(); startTime.Before(timestamp) {
// 		return 0, errStartTimeTooEarly
// 	}
// 	if subnetID == constants.PrimaryNetworkID {
// 		return vm.maxPrimarySubnetStakeAmount(nodeID, startTime, endTime)
// 	}
// 	return vm.maxSubnetStakeAmount(subnetID, nodeID, startTime, endTime)
// }

// func (vm *VM) maxSubnetStakeAmount(
// 	subnetID ids.ID,
// 	nodeID ids.ShortID,
// 	startTime time.Time,
// 	endTime time.Time,
// ) (uint64, error) {
// 	var (
// 		vdrTx  *UnsignedAddSubnetValidatorTx
// 		exists bool
// 	)

// 	pendingStakers := vm.internalState.PendingStakerChainState()
// 	pendingValidator, _ := pendingStakers.GetValidator(nodeID)

// 	currentStakers := vm.internalState.CurrentStakerChainState()
// 	currentValidator, err := currentStakers.GetValidator(nodeID)
// 	switch err {
// 	case nil:
// 		vdrTx, exists = currentValidator.SubnetValidators()[subnetID]
// 		if !exists {
// 			vdrTx = pendingValidator.SubnetValidators()[subnetID]
// 		}
// 	case database.ErrNotFound:
// 		vdrTx = pendingValidator.SubnetValidators()[subnetID]
// 	default:
// 		return 0, err
// 	}

// 	if vdrTx == nil {
// 		return 0, nil
// 	}
// 	if vdrTx.StartTime().After(endTime) {
// 		return 0, nil
// 	}
// 	if vdrTx.EndTime().Before(startTime) {
// 		return 0, nil
// 	}
// 	return vdrTx.Weight(), nil
// }

// func (vm *VM) maxPrimarySubnetStakeAmount(
// 	nodeID ids.ShortID,
// 	startTime time.Time,
// 	endTime time.Time,
// ) (uint64, error) {
// 	currentStakers := vm.internalState.CurrentStakerChainState()
// 	pendingStakers := vm.internalState.PendingStakerChainState()

// 	pendingValidator, _ := pendingStakers.GetValidator(nodeID)
// 	currentValidator, err := currentStakers.GetValidator(nodeID)

// 	switch err {
// 	case nil:
// 		vdrTx := currentValidator.AddValidatorTx()
// 		if vdrTx.StartTime().After(endTime) {
// 			return 0, nil
// 		}
// 		if vdrTx.EndTime().Before(startTime) {
// 			return 0, nil
// 		}

// 		currentWeight := vdrTx.Weight()
// 		currentWeight, err = math.Add64(currentWeight, currentValidator.DelegatorWeight())
// 		if err != nil {
// 			return 0, err
// 		}
// 		return maxStakeAmount(
// 			currentValidator.Delegators(),
// 			pendingValidator.Delegators(),
// 			startTime,
// 			endTime,
// 			currentWeight,
// 		)
// 	case database.ErrNotFound:
// 		futureValidator, err := pendingStakers.GetValidatorTx(nodeID)
// 		if err == database.ErrNotFound {
// 			return 0, nil
// 		}
// 		if err != nil {
// 			return 0, err
// 		}
// 		if futureValidator.StartTime().After(endTime) {
// 			return 0, nil
// 		}
// 		if futureValidator.EndTime().Before(startTime) {
// 			return 0, nil
// 		}

// 		return maxStakeAmount(
// 			nil,
// 			pendingValidator.Delegators(),
// 			startTime,
// 			endTime,
// 			futureValidator.Weight(),
// 		)
// 	default:
// 		return 0, err
// 	}
// }
