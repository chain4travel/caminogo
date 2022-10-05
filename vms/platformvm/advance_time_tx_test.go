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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/constants"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/platformvm/status"
)

// Ensure semantic verification fails when proposed timestamp is at or before current timestamp
func TestAdvanceTimeTxTimestampTooEarly(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	tx, err := vm.newAdvanceTimeTx(defaultGenesisTime)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, ErrorNotAfterCurrentTimestamp)
}

// Ensure semantic verification fails when proposed timestamp is after next validator set change time
func TestAdvanceTimeTxTimestampTooLate(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()

	// Case: Timestamp is after next validator start time
	// Add a pending validator
	pendingValidatorStartTime := defaultGenesisTime.Add(1 * time.Second)
	pendingValidatorEndTime := pendingValidatorStartTime.Add(defaultMinStakingDuration)
	nodeIDKey, _ := vm.factory.NewPrivateKey()
	nodeID := nodeIDKey.PublicKey().Address()
	_, err := addPendingValidator(vm, pendingValidatorStartTime, pendingValidatorEndTime, nodeID, []*crypto.PrivateKeySECP256K1R{keys[0]})
	assert.NoError(t, err)

	tx, err := vm.newAdvanceTimeTx(pendingValidatorStartTime.Add(1 * time.Second))
	assert.NoError(t, err)
	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, ErrorLaterThanNextStaker)

	err = vm.Shutdown()
	assert.NoError(t, err)
	vm.ctx.Lock.Unlock()

	// Case: Timestamp is after next validator end time
	vm, _, _ = defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	// fast forward clock to 10 seconds before genesis validators stop validating
	vm.clock.Set(defaultValidateEndTime.Add(-10 * time.Second))

	// Proposes advancing timestamp to 1 second after genesis validators stop validating
	tx, err = vm.newAdvanceTimeTx(defaultValidateEndTime.Add(1 * time.Second))
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, ErrorTooFarInTheFuture)
}

// Ensure semantic verification updates the current and pending staker set
// for the primary network
func TestAdvanceTimeTxUpdatePrimaryNetworkStakers(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	// Case: Timestamp is after next validator start time
	// Add a pending validator
	pendingValidatorStartTime := defaultGenesisTime.Add(1 * time.Second)
	pendingValidatorEndTime := pendingValidatorStartTime.Add(defaultMinStakingDuration)
	nodeIDKey, _ := vm.factory.NewPrivateKey()
	nodeID := nodeIDKey.PublicKey().Address()
	addPendingValidatorTx, err := addPendingValidator(vm, pendingValidatorStartTime, pendingValidatorEndTime, nodeID, []*crypto.PrivateKeySECP256K1R{keys[0]})
	assert.NoError(t, err)

	tx, err := vm.newAdvanceTimeTx(pendingValidatorStartTime)
	assert.NoError(t, err)

	onCommit, onAbort, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.NoError(t, err)

	onCommitCurrentStakers := onCommit.CurrentStakerChainState()
	validator, err := onCommitCurrentStakers.GetValidator(nodeID)
	assert.NoError(t, err)

	assert.Equal(t, validator.AddValidatorTx().ID(), addPendingValidatorTx.ID())

	onCommitPendingStakers := onCommit.PendingStakerChainState()
	_, err = onCommitPendingStakers.GetValidatorTx(nodeID)

	assert.Equal(t, err, database.ErrNotFound)

	_, reward, err := onCommitCurrentStakers.GetNextStaker()
	assert.NoError(t, err)

	assert.Equal(t, reward, uint64(1370))

	onAbortCurrentStakers := onAbort.CurrentStakerChainState()
	_, err = onAbortCurrentStakers.GetValidator(nodeID)
	assert.Equal(t, err, database.ErrNotFound)

	onAbortPendingStakers := onAbort.PendingStakerChainState()
	vdr, err := onAbortPendingStakers.GetValidatorTx(nodeID)
	assert.NoError(t, err)

	assert.Equal(t, vdr.ID(), addPendingValidatorTx.ID())

	// Test VM validators
	onCommit.Apply(vm.internalState)
	assert.NoError(t, vm.internalState.Commit())
	assert.True(t, vm.Validators.Contains(constants.PrimaryNetworkID, nodeID))
}

// Ensure semantic verification updates the current and pending staker sets correctly.
// Namely, it should add pending stakers whose start time is at or before the timestamp.
// It will not remove primary network stakers; that happens in rewardTxs.
func TestAdvanceTimeTxUpdateStakers(t *testing.T) {
	type stakerStatus uint
	const (
		pending stakerStatus = iota
		current
	)

	type staker struct {
		nodeID             ids.ShortID
		startTime, endTime time.Time
	}
	type test struct {
		description           string
		stakers               []staker
		subnetStakers         []staker
		advanceTimeTo         []time.Time
		expectedStakers       map[ids.ShortID]stakerStatus
		expectedSubnetStakers map[ids.ShortID]stakerStatus
	}

	// Chronological order: staker1 start, staker2 start, staker3 start and staker 4 start,
	//  staker3 and staker4 end, staker2 end and staker5 start, staker1 end
	staker1 := staker{
		nodeID:    ids.GenerateTestShortID(),
		startTime: defaultGenesisTime.Add(1 * time.Minute),
		endTime:   defaultGenesisTime.Add(10 * defaultMinStakingDuration).Add(1 * time.Minute),
	}
	staker2 := staker{
		nodeID:    ids.GenerateTestShortID(),
		startTime: staker1.startTime.Add(1 * time.Minute),
		endTime:   staker1.startTime.Add(1 * time.Minute).Add(defaultMinStakingDuration),
	}
	staker3 := staker{
		nodeID:    ids.GenerateTestShortID(),
		startTime: staker2.startTime.Add(1 * time.Minute),
		endTime:   staker2.endTime.Add(1 * time.Minute),
	}
	staker3Sub := staker{
		nodeID:    staker3.nodeID,
		startTime: staker3.startTime.Add(1 * time.Minute),
		endTime:   staker3.endTime.Add(-1 * time.Minute),
	}
	staker4 := staker{
		nodeID:    ids.GenerateTestShortID(),
		startTime: staker3.startTime,
		endTime:   staker3.endTime,
	}
	staker5 := staker{
		nodeID:    ids.GenerateTestShortID(),
		startTime: staker2.endTime,
		endTime:   staker2.endTime.Add(defaultMinStakingDuration),
	}

	tests := []test{
		{
			description:   "advance time to before staker1 start with subnet",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			subnetStakers: []staker{staker1, staker2, staker3, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime.Add(-1 * time.Second)},
			expectedStakers: map[ids.ShortID]stakerStatus{
				staker1.nodeID: pending, staker2.nodeID: pending, staker3.nodeID: pending, staker4.nodeID: pending, staker5.nodeID: pending,
			},
			expectedSubnetStakers: map[ids.ShortID]stakerStatus{
				staker1.nodeID: pending, staker2.nodeID: pending, staker3.nodeID: pending, staker4.nodeID: pending, staker5.nodeID: pending,
			},
		},
		{
			description:   "advance time to staker 1 start with subnet",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			subnetStakers: []staker{staker1},
			advanceTimeTo: []time.Time{staker1.startTime},
			expectedStakers: map[ids.ShortID]stakerStatus{
				staker2.nodeID: pending, staker3.nodeID: pending, staker4.nodeID: pending, staker5.nodeID: pending,
				staker1.nodeID: current,
			},
			expectedSubnetStakers: map[ids.ShortID]stakerStatus{
				staker2.nodeID: pending, staker3.nodeID: pending, staker4.nodeID: pending, staker5.nodeID: pending,
				staker1.nodeID: current,
			},
		},
		{
			description:   "advance time to the staker2 start",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime},
			expectedStakers: map[ids.ShortID]stakerStatus{
				staker3.nodeID: pending, staker4.nodeID: pending, staker5.nodeID: pending,
				staker1.nodeID: current, staker2.nodeID: current,
			},
		},
		{
			description:   "staker3 should validate only primary network",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			subnetStakers: []staker{staker1, staker2, staker3Sub, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime, staker3.startTime},
			expectedStakers: map[ids.ShortID]stakerStatus{
				staker5.nodeID: pending,
				staker1.nodeID: current, staker2.nodeID: current, staker3.nodeID: current, staker4.nodeID: current,
			},
			expectedSubnetStakers: map[ids.ShortID]stakerStatus{
				staker5.nodeID: pending, staker3Sub.nodeID: pending,
				staker1.nodeID: current, staker2.nodeID: current, staker4.nodeID: current,
			},
		},
		{
			description:   "advance time to staker3 start with subnet",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			subnetStakers: []staker{staker1, staker2, staker3Sub, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime, staker3.startTime, staker3Sub.startTime},
			expectedStakers: map[ids.ShortID]stakerStatus{
				staker5.nodeID: pending,
				staker1.nodeID: current, staker2.nodeID: current, staker3.nodeID: current, staker4.nodeID: current,
			},
			expectedSubnetStakers: map[ids.ShortID]stakerStatus{
				staker5.nodeID: pending,
				staker1.nodeID: current, staker2.nodeID: current, staker3.nodeID: current, staker4.nodeID: current,
			},
		},
		{
			description:   "advance time to staker5 end",
			stakers:       []staker{staker1, staker2, staker3, staker4, staker5},
			advanceTimeTo: []time.Time{staker1.startTime, staker2.startTime, staker3.startTime, staker5.startTime},
			expectedStakers: map[ids.ShortID]stakerStatus{
				staker1.nodeID: current, staker2.nodeID: current, staker3.nodeID: current, staker4.nodeID: current, staker5.nodeID: current,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(ts *testing.T) {
			assert := assert.New(ts)
			vm, _, _ := defaultVM()
			vm.ctx.Lock.Lock()
			defer func() {
				err := vm.Shutdown()
				assert.NoError(err)
				vm.ctx.Lock.Unlock()
			}()
			vm.WhitelistedSubnets.Add(testSubnet1.ID())

			for _, staker := range test.stakers {
				_, err := addPendingValidator(vm, staker.startTime, staker.endTime, staker.nodeID, []*crypto.PrivateKeySECP256K1R{keys[0]})
				assert.NoError(err)
			}

			for _, staker := range test.subnetStakers {
				tx, err := vm.newAddSubnetValidatorTx(
					10, // Weight
					uint64(staker.startTime.Unix()),
					uint64(staker.endTime.Unix()),
					staker.nodeID,    // validator ID
					testSubnet1.ID(), // Subnet ID
					[]*crypto.PrivateKeySECP256K1R{keys[0], keys[1]}, // Keys
				)
				assert.NoError(err)
				vm.internalState.AddPendingStaker(tx)
				vm.internalState.AddTx(tx, status.Committed)
			}
			err := vm.internalState.Commit()
			assert.NoError(err)

			err = vm.internalState.(*internalStateImpl).loadPendingValidators()
			assert.NoError(err)

			for _, newTime := range test.advanceTimeTo {
				vm.clock.Set(newTime)
				tx, err := vm.newAdvanceTimeTx(newTime)
				assert.NoError(err)

				onCommitState, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
				assert.NoError(err)
				onCommitState.Apply(vm.internalState)
			}

			assert.NoError(vm.internalState.Commit())

			// Check that the validators we expect to be in the current staker set are there
			currentStakers := vm.internalState.CurrentStakerChainState()
			// Check that the validators we expect to be in the pending staker set are there
			pendingStakers := vm.internalState.PendingStakerChainState()
			for stakerNodeID, status := range test.expectedStakers {
				switch status {
				case pending:
					_, err := pendingStakers.GetValidatorTx(stakerNodeID)
					assert.NoError(err)
					assert.False(vm.Validators.Contains(constants.PrimaryNetworkID, stakerNodeID))
				case current:
					_, err := currentStakers.GetValidator(stakerNodeID)
					assert.NoError(err)
					assert.True(vm.Validators.Contains(constants.PrimaryNetworkID, stakerNodeID))
				}
			}

			for stakerNodeID, status := range test.expectedSubnetStakers {
				switch status {
				case pending:
					assert.False(vm.Validators.Contains(testSubnet1.ID(), stakerNodeID))
				case current:
					assert.True(vm.Validators.Contains(testSubnet1.ID(), stakerNodeID))
				}
			}
		})
	}
}

// Regression test for https://github.com/chain4travel/caminogo/pull/584
// that ensures it fixes a bug where subnet validators are not removed
// when timestamp is advanced and there is a pending staker whose start time
// is after the new timestamp
func TestAdvanceTimeTxRemoveSubnetValidator(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()
	vm.WhitelistedSubnets.Add(testSubnet1.ID())
	// Add a subnet validator to the staker set
	subnetValidatorNodeID := keys[0].PublicKey().Address()
	// Starts after the corre
	subnetVdr1StartTime := defaultValidateStartTime
	subnetVdr1EndTime := defaultValidateStartTime.Add(defaultMinStakingDuration)
	tx, err := vm.newAddSubnetValidatorTx(
		1,                                  // Weight
		uint64(subnetVdr1StartTime.Unix()), // Start time
		uint64(subnetVdr1EndTime.Unix()),   // end time
		subnetValidatorNodeID,              // Node ID
		testSubnet1.ID(),                   // Subnet ID
		[]*crypto.PrivateKeySECP256K1R{keys[0], keys[1]}, // Keys
	)
	assert.NoError(t, err)

	vm.internalState.AddCurrentStaker(tx, 0)
	vm.internalState.AddTx(tx, status.Committed)

	err = vm.internalState.Commit()
	assert.NoError(t, err)

	err = vm.internalState.(*internalStateImpl).loadCurrentValidators()
	assert.NoError(t, err)

	// The above validator is now part of the staking set

	// Queue a staker that joins the staker set after the above validator leaves
	subnetVdr2NodeID := keys[1].PublicKey().Address()
	tx, err = vm.newAddSubnetValidatorTx(
		1, // Weight
		uint64(subnetVdr1EndTime.Add(time.Second).Unix()),                                // Start time
		uint64(subnetVdr1EndTime.Add(time.Second).Add(defaultMinStakingDuration).Unix()), // end time
		subnetVdr2NodeID, // Node ID
		testSubnet1.ID(), // Subnet ID
		[]*crypto.PrivateKeySECP256K1R{keys[0], keys[1]}, // Keys
	)
	assert.NoError(t, err)

	vm.internalState.AddPendingStaker(tx)
	vm.internalState.AddTx(tx, status.Committed)

	err = vm.internalState.Commit()
	assert.NoError(t, err)

	err = vm.internalState.(*internalStateImpl).loadPendingValidators()
	assert.NoError(t, err)

	// The above validator is now in the pending staker set

	// Advance time to the first staker's end time.
	vm.clock.Set(subnetVdr1EndTime)
	tx, err = vm.newAdvanceTimeTx(subnetVdr1EndTime)
	assert.NoError(t, err)

	onCommitState, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.NoError(t, err)

	currentStakers := onCommitState.CurrentStakerChainState()
	vdr, err := currentStakers.GetValidator(subnetValidatorNodeID)
	assert.NoError(t, err)
	_, exists := vdr.SubnetValidators()[testSubnet1.ID()]

	// The first staker should now be removed. Verify that is the case.
	assert.False(t, exists)

	// Check VM Validators are removed successfully
	onCommitState.Apply(vm.internalState)
	assert.NoError(t, vm.internalState.Commit())
	assert.False(t, vm.Validators.Contains(testSubnet1.ID(), subnetVdr2NodeID))
	assert.False(t, vm.Validators.Contains(testSubnet1.ID(), subnetValidatorNodeID))
}

func TestWhitelistedSubnet(t *testing.T) {
	for _, whitelist := range []bool{true, false} {
		t.Run(fmt.Sprintf("whitelisted %t", whitelist), func(ts *testing.T) {
			vm, _, _ := defaultVM()
			vm.ctx.Lock.Lock()
			defer func() {
				err := vm.Shutdown()
				assert.NoError(t, err)
				vm.ctx.Lock.Unlock()
			}()

			if whitelist {
				vm.WhitelistedSubnets.Add(testSubnet1.ID())
			}
			// Add a subnet validator to the staker set
			subnetValidatorNodeID := keys[0].PublicKey().Address()

			subnetVdr1StartTime := defaultGenesisTime.Add(1 * time.Minute)
			subnetVdr1EndTime := defaultGenesisTime.Add(10 * defaultMinStakingDuration).Add(1 * time.Minute)
			tx, err := vm.newAddSubnetValidatorTx(
				1,                                  // Weight
				uint64(subnetVdr1StartTime.Unix()), // Start time
				uint64(subnetVdr1EndTime.Unix()),   // end time
				subnetValidatorNodeID,              // Node ID
				testSubnet1.ID(),                   // Subnet ID
				[]*crypto.PrivateKeySECP256K1R{keys[0], keys[1]}, // Keys
			)
			assert.NoError(t, err)

			vm.internalState.AddPendingStaker(tx)
			vm.internalState.AddTx(tx, status.Committed)

			err = vm.internalState.Commit()
			assert.NoError(t, err)

			err = vm.internalState.(*internalStateImpl).loadPendingValidators()
			assert.NoError(t, err)

			// Advance time to the staker's start time.
			vm.clock.Set(subnetVdr1StartTime)
			tx, err = vm.newAdvanceTimeTx(subnetVdr1StartTime)
			assert.NoError(t, err)

			onCommitState, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
			assert.NoError(t, err)

			onCommitState.Apply(vm.internalState)
			assert.NoError(t, vm.internalState.Commit())
			assert.Equal(t, whitelist, vm.Validators.Contains(testSubnet1.ID(), subnetValidatorNodeID))
		})
	}
}

// Test method InitiallyPrefersCommit
func TestAdvanceTimeTxInitiallyPrefersCommit(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	vm.clock.Set(defaultGenesisTime) // VM's clock reads the genesis time

	// Proposed advancing timestamp to 1 second after sync bound
	tx, err := vm.newAdvanceTimeTx(defaultGenesisTime.Add(1 * time.Second).Add(syncBound))
	assert.NoError(t, err)

	assert.False(t, tx.UnsignedTx.(UnsignedProposalTx).InitiallyPrefersCommit(vm))

	// advance wall clock time
	vm.clock.Set(defaultGenesisTime.Add(1 * time.Second))
	assert.True(t, tx.UnsignedTx.(UnsignedProposalTx).InitiallyPrefersCommit(vm))
}

// Ensure marshaling/unmarshaling works
func TestAdvanceTimeTxUnmarshal(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	tx, err := vm.newAdvanceTimeTx(defaultGenesisTime)
	assert.NoError(t, err)

	bytes, err := Codec.Marshal(CodecVersion, tx)
	assert.NoError(t, err)

	var unmarshaledTx Tx
	_, err = Codec.Unmarshal(bytes, &unmarshaledTx)
	assert.NoError(t, err)

	assert.Equal(t, tx.UnsignedTx.(*UnsignedAdvanceTimeTx).Time, unmarshaledTx.UnsignedTx.(*UnsignedAdvanceTimeTx).Time)
}

func addPendingValidator(vm *VM, startTime time.Time, endTime time.Time, nodeID ids.ShortID, keys []*crypto.PrivateKeySECP256K1R) (*Tx, error) {
	addPendingValidatorTx, err := vm.newAddValidatorTx(
		uint64(startTime.Unix()),
		uint64(endTime.Unix()),
		nodeID,
		nodeID,
		keys,
	)
	if err != nil {
		return nil, err
	}

	vm.internalState.AddPendingStaker(addPendingValidatorTx)
	vm.internalState.AddTx(addPendingValidatorTx, status.Committed)
	if err := vm.internalState.Commit(); err != nil {
		return nil, err
	}
	if err := vm.internalState.(*internalStateImpl).loadPendingValidators(); err != nil {
		return nil, err
	}
	return addPendingValidatorTx, err
}
