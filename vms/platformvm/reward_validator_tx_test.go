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
	"testing"

	"github.com/chain4travel/caminogo/chains"
	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/database/manager"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/snow"
	"github.com/chain4travel/caminogo/snow/engine/common"
	"github.com/chain4travel/caminogo/snow/uptime"
	"github.com/chain4travel/caminogo/snow/validators"
	"github.com/chain4travel/caminogo/version"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/platformvm/status"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
	"github.com/stretchr/testify/assert"
)

func TestUnsignedRewardValidatorTxExecuteOnCommit(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	currentStakers := vm.internalState.CurrentStakerChainState()
	toRemoveTx, _, err := currentStakers.GetNextStaker()
	assert.NoError(t, err)

	toRemove := toRemoveTx.UnsignedTx.(*UnsignedAddValidatorTx)

	// Case 1: Chain timestamp is wrong
	tx, err := vm.newRewardValidatorTx(toRemove.ID())
	assert.NoError(t, err)

	_, _, err = toRemove.Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errTimeBeforeCurrent)

	// Advance chain timestamp to time that next validator leaves
	vm.internalState.SetTimestamp(toRemove.EndTime())

	// Case 2: Wrong validator
	tx, err = vm.newRewardValidatorTx(ids.GenerateTestID())
	assert.NoError(t, err)

	_, _, err = toRemove.Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errTimeBeforeCurrent)

	// Case 3: Happy path
	tx, err = vm.newRewardValidatorTx(toRemove.ID())
	assert.NoError(t, err)

	onCommitState, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.NoError(t, err)

	onCommitCurrentStakers := onCommitState.CurrentStakerChainState()
	nextToRemoveTx, _, err := onCommitCurrentStakers.GetNextStaker()
	assert.NoError(t, err)

	assert.NotEqual(t, toRemove.ID(), nextToRemoveTx.ID())

	// check that stake/reward is given back
	stakeOwners := toRemove.Stake[0].Out.(*secp256k1fx.TransferOutput).AddressesSet()

	// Get old balances
	oldBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	assert.NoError(t, err)

	onCommitState.Apply(vm.internalState)
	err = vm.internalState.Commit()
	assert.NoError(t, err)

	onCommitBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	assert.NoError(t, err)

	assert.Equal(t, onCommitBalance, oldBalance+toRemove.Validator.Weight()+13773)
}

func TestUnsignedRewardValidatorTxExecuteOnAbort(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	currentStakers := vm.internalState.CurrentStakerChainState()
	toRemoveTx, _, err := currentStakers.GetNextStaker()
	assert.NoError(t, err)

	toRemove := toRemoveTx.UnsignedTx.(*UnsignedAddValidatorTx)

	// Case 1: Chain timestamp is wrong
	tx, err := vm.newRewardValidatorTx(toRemove.ID())
	assert.NoError(t, err)

	_, _, err = toRemove.Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errTimeBeforeCurrent)

	// Advance chain timestamp to time that next validator leaves
	vm.internalState.SetTimestamp(toRemove.EndTime())

	// Case 2: Wrong validator
	tx, err = vm.newRewardValidatorTx(ids.GenerateTestID())
	assert.NoError(t, err)

	_, _, err = toRemove.Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errTimeBeforeCurrent)

	// Case 3: Happy path
	tx, err = vm.newRewardValidatorTx(toRemove.ID())
	assert.NoError(t, err)

	_, onAbortState, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.NoError(t, err)

	onAbortCurrentStakers := onAbortState.CurrentStakerChainState()
	nextToRemoveTx, _, err := onAbortCurrentStakers.GetNextStaker()
	assert.NoError(t, err)

	assert.NotEqual(t, toRemove.ID(), nextToRemoveTx.ID())

	// check that stake/reward isn't given back
	stakeOwners := toRemove.Stake[0].Out.(*secp256k1fx.TransferOutput).AddressesSet()

	// Get old balances
	oldBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	assert.NoError(t, err)

	onAbortState.Apply(vm.internalState)
	err = vm.internalState.Commit()
	assert.NoError(t, err)

	onAbortBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	assert.NoError(t, err)

	assert.Equal(t, onAbortBalance, oldBalance+toRemove.Validator.Weight())
}

func TestUptimeDisallowedWithRestart(t *testing.T) {
	_, genesisBytes := defaultGenesis()
	db := manager.NewMemDB(version.DefaultVersion1_0_0)

	firstDB := db.NewPrefixDBManager([]byte{})
	firstVM := &VM{Factory: Factory{
		Chains:                 chains.MockManager{},
		UptimePercentage:       .2,
		RewardConfig:           defaultRewardConfig,
		Validators:             validators.NewManager(),
		UptimeLockedCalculator: uptime.NewLockedCalculator(),
	}}

	firstCtx := defaultContext()
	firstCtx.Lock.Lock()

	firstMsgChan := make(chan common.Message, 1)
	err := firstVM.Initialize(firstCtx, firstDB, genesisBytes, nil, nil, firstMsgChan, nil, nil)
	assert.NoError(t, err)

	firstVM.clock.Set(defaultGenesisTime)
	firstVM.uptimeManager.(uptime.TestManager).SetTime(defaultGenesisTime)

	err = firstVM.SetState(snow.Bootstrapping)
	assert.NoError(t, err)

	err = firstVM.SetState(snow.NormalOp)
	assert.NoError(t, err)

	// Fast forward clock to time for genesis validators to leave
	firstVM.uptimeManager.(uptime.TestManager).SetTime(defaultValidateEndTime)

	err = firstVM.Shutdown()
	assert.NoError(t, err)
	firstCtx.Lock.Unlock()

	secondDB := db.NewPrefixDBManager([]byte{})
	secondVM := &VM{Factory: Factory{
		Chains:                 chains.MockManager{},
		UptimePercentage:       .21,
		Validators:             validators.NewManager(),
		UptimeLockedCalculator: uptime.NewLockedCalculator(),
	}}

	secondCtx := defaultContext()
	secondCtx.Lock.Lock()
	defer func() {
		err := secondVM.Shutdown()
		assert.NoError(t, err)
		secondCtx.Lock.Unlock()
	}()

	secondMsgChan := make(chan common.Message, 1)
	err = secondVM.Initialize(secondCtx, secondDB, genesisBytes, nil, nil, secondMsgChan, nil, nil)
	assert.NoError(t, err)

	secondVM.clock.Set(defaultValidateStartTime.Add(2 * defaultMinStakingDuration))
	secondVM.uptimeManager.(uptime.TestManager).SetTime(defaultValidateStartTime.Add(2 * defaultMinStakingDuration))

	err = secondVM.SetState(snow.Bootstrapping)
	assert.NoError(t, err)

	err = secondVM.SetState(snow.NormalOp)
	assert.NoError(t, err)

	secondVM.clock.Set(defaultValidateEndTime)
	secondVM.uptimeManager.(uptime.TestManager).SetTime(defaultValidateEndTime)

	blk, err := secondVM.BuildBlock() // should contain proposal to advance time
	assert.NoError(t, err)

	err = blk.Verify()
	assert.NoError(t, err)

	// Assert preferences are correct
	block := blk.(*ProposalBlock)
	options, err := block.Options()
	assert.NoError(t, err)

	commit, ok := options[0].(*CommitBlock)
	assert.True(t, ok, errShouldPrefCommit)

	abort, ok := options[1].(*AbortBlock)
	assert.True(t, ok, errShouldPrefCommit)

	err = block.Accept()
	assert.NoError(t, err)

	err = commit.Verify()
	assert.NoError(t, err)

	err = abort.Verify()
	assert.NoError(t, err)

	onAbortState := abort.onAccept()
	_, txStatus, err := onAbortState.GetTx(block.Tx.ID())
	assert.NoError(t, err)

	assert.Equal(t, txStatus, status.Aborted)

	err = commit.Accept()
	assert.NoError(t, err)

	_, txStatus, err = secondVM.internalState.GetTx(block.Tx.ID())
	assert.NoError(t, err)

	assert.Equal(t, txStatus, status.Committed)

	// Verify that chain's timestamp has advanced
	timestamp := secondVM.internalState.GetTimestamp()
	assert.True(t, timestamp.Equal(defaultValidateEndTime))

	blk, err = secondVM.BuildBlock() // should contain proposal to reward genesis validator
	assert.NoError(t, err)

	err = blk.Verify()
	assert.NoError(t, err)

	block = blk.(*ProposalBlock)
	options, err = block.Options()
	assert.NoError(t, err)

	commit, ok = options[1].(*CommitBlock)
	assert.True(t, ok, errShouldPrefAbort)

	abort, ok = options[0].(*AbortBlock)
	assert.True(t, ok, errShouldPrefAbort)

	err = blk.Accept()
	assert.NoError(t, err)

	err = commit.Verify()
	assert.NoError(t, err)

	onCommitState := commit.onAccept()
	_, txStatus, err = onCommitState.GetTx(block.Tx.ID())
	assert.NoError(t, err)

	assert.Equal(t, txStatus, status.Committed)

	err = abort.Verify()
	assert.NoError(t, err)

	err = abort.Accept()
	assert.NoError(t, err)

	_, txStatus, err = secondVM.internalState.GetTx(block.Tx.ID())
	assert.NoError(t, err)

	assert.Equal(t, txStatus, status.Aborted)

	currentStakers := secondVM.internalState.CurrentStakerChainState()
	_, err = currentStakers.GetValidator(keys[0].PublicKey().Address())
	assert.ErrorIs(t, err, database.ErrNotFound)
}

func TestUptimeDisallowedAfterNeverConnecting(t *testing.T) {
	_, genesisBytes := defaultGenesis()
	db := manager.NewMemDB(version.DefaultVersion1_0_0)

	vm := &VM{Factory: Factory{
		Chains:                 chains.MockManager{},
		UptimePercentage:       .2,
		RewardConfig:           defaultRewardConfig,
		Validators:             validators.NewManager(),
		UptimeLockedCalculator: uptime.NewLockedCalculator(),
	}}

	ctx := defaultContext()
	ctx.Lock.Lock()

	msgChan := make(chan common.Message, 1)
	appSender := &common.SenderTest{T: t}

	err := vm.Initialize(ctx, db, genesisBytes, nil, nil, msgChan, nil, appSender)
	assert.NoError(t, err)

	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		ctx.Lock.Unlock()
	}()

	vm.clock.Set(defaultGenesisTime)
	vm.uptimeManager.(uptime.TestManager).SetTime(defaultGenesisTime)

	err = vm.SetState(snow.Bootstrapping)
	assert.NoError(t, err)

	err = vm.SetState(snow.NormalOp)
	assert.NoError(t, err)
	if err := vm.SetState(snow.NormalOp); err != nil {
		t.Fatal(err)
	}

	// Fast forward clock to time for genesis validators to leave
	vm.clock.Set(defaultValidateEndTime)
	vm.uptimeManager.(uptime.TestManager).SetTime(defaultValidateEndTime)

	blk, err := vm.BuildBlock() // should contain proposal to advance time
	assert.NoError(t, err)

	err = blk.Verify()
	assert.NoError(t, err)

	// first the time will be advanced.
	block := blk.(*ProposalBlock)
	options, err := block.Options()
	assert.NoError(t, err)

	commit, ok := options[0].(*CommitBlock)
	assert.True(t, ok, errShouldPrefCommit)

	abort, ok := options[1].(*AbortBlock)
	assert.True(t, ok, errShouldPrefCommit)

	err = block.Accept()
	assert.NoError(t, err)

	err = commit.Verify()
	assert.NoError(t, err)

	err = abort.Verify()
	assert.NoError(t, err)

	// advance the timestamp
	err = commit.Accept()
	assert.NoError(t, err)

	// Verify that chain's timestamp has advanced
	timestamp := vm.internalState.GetTimestamp()
	assert.True(t, timestamp.Equal(defaultValidateEndTime))

	// should contain proposal to reward genesis validator
	blk, err = vm.BuildBlock()
	assert.NoError(t, err)

	err = blk.Verify()
	assert.NoError(t, err)

	block = blk.(*ProposalBlock)
	options, err = block.Options()
	assert.NoError(t, err)

	abort, ok = options[0].(*AbortBlock)
	assert.True(t, ok, errShouldPrefAbort)

	commit, ok = options[1].(*CommitBlock)
	assert.True(t, ok, errShouldPrefAbort)

	err = blk.Accept()
	assert.NoError(t, err)

	err = commit.Verify()
	assert.NoError(t, err)

	err = abort.Verify()
	assert.NoError(t, err)

	// do not reward the genesis validator
	err = abort.Accept()
	assert.NoError(t, err)

	currentStakers := vm.internalState.CurrentStakerChainState()
	_, err = currentStakers.GetValidator(keys[0].PublicKey().Address())
	assert.ErrorIs(t, err, database.ErrNotFound)
}
