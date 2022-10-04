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
)

func TestUnsignedRewardValidatorTxExecuteOnCommit(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	currentStakers := vm.internalState.CurrentStakerChainState()
	toRemoveTx, _, err := currentStakers.GetNextStaker()
	if err != nil {
		t.Fatal(err)
	}
	toRemove := toRemoveTx.UnsignedTx.(*UnsignedAddValidatorTx)

	// Case 1: Chain timestamp is wrong
	stx, err := vm.newRewardValidatorTx(toRemove.ID())
	if err != nil {
		t.Fatal(err)
	}
	utx, ok := stx.UnsignedTx.(*UnsignedRewardValidatorTx)
	if !ok {
		t.Fatal("Could not cast rewardValidatorTx to an UnsignedRewardValidatorTx")
	}
	if _, _, err = utx.Execute(vm, vm.internalState, stx); err == nil {
		t.Fatalf("should have failed because validator end time doesn't match chain timestamp")
	}

	// Advance chain timestamp to time that next validator leaves
	vm.internalState.SetTimestamp(toRemove.EndTime())

	// Case 2: Wrong validator
	stx, err = vm.newRewardValidatorTx(ids.GenerateTestID())
	if err != nil {
		t.Fatal(err)
	}
	utx, ok = stx.UnsignedTx.(*UnsignedRewardValidatorTx)
	if !ok {
		t.Fatal("Could not cast rewardValidatorTx to an UnsignedRewardValidatorTx")
	}
	if _, _, err = utx.Execute(vm, vm.internalState, stx); err == nil {
		t.Fatalf("should have failed because validator ID is wrong")
	}

	// Case 3: Happy path
	tx, err := vm.newRewardValidatorTx(toRemove.ID())
	if err != nil {
		t.Fatal(err)
	}

	onCommitState, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	if err != nil {
		t.Fatal(err)
	}

	onCommitCurrentStakers := onCommitState.CurrentStakerChainState()
	nextToRemoveTx, _, err := onCommitCurrentStakers.GetNextStaker()
	if err != nil {
		t.Fatal(err)
	}
	if toRemove.ID() == nextToRemoveTx.ID() {
		t.Fatalf("Should have removed the previous validator")
	}

	// check that stake/reward is given back
	stakeOwners := toRemove.Stake[0].Out.(*secp256k1fx.TransferOutput).AddressesSet()

	// Get old balances
	oldBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	if err != nil {
		t.Fatal(err)
	}

	onCommitState.Apply(vm.internalState)
	if err := vm.internalState.Commit(); err != nil {
		t.Fatal(err)
	}

	onCommitBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	if err != nil {
		t.Fatal(err)
	}

	if onCommitBalance != oldBalance+toRemove.Validator.Weight()+13773 {
		t.Fatalf("on commit, should have old balance (%d) + staked amount (%d) + reward (%d) but have %d",
			oldBalance, toRemove.Validator.Weight(), 13773, onCommitBalance)
	}
}

func TestUnsignedRewardValidatorTxExecuteOnAbort(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	currentStakers := vm.internalState.CurrentStakerChainState()
	toRemoveTx, _, err := currentStakers.GetNextStaker()
	if err != nil {
		t.Fatal(err)
	}
	toRemove := toRemoveTx.UnsignedTx.(*UnsignedAddValidatorTx)

	// Case 1: Chain timestamp is wrong
	stx, err := vm.newRewardValidatorTx(toRemove.ID())
	if err != nil {
		t.Fatal(err)
	}
	utx, ok := stx.UnsignedTx.(*UnsignedRewardValidatorTx)
	if !ok {
		t.Fatal("Could not cast rewardValidatorTx to an UnsignedRewardValidatorTx")
	}
	if _, _, err = utx.Execute(vm, vm.internalState, stx); err == nil {
		t.Fatalf("should have failed because validator end time doesn't match chain timestamp")
	}

	// Advance chain timestamp to time that next validator leaves
	vm.internalState.SetTimestamp(toRemove.EndTime())

	// Case 2: Wrong validator
	stx, err = vm.newRewardValidatorTx(ids.GenerateTestID())
	if err != nil {
		t.Fatal(err)
	}
	utx, ok = stx.UnsignedTx.(*UnsignedRewardValidatorTx)
	if !ok {
		t.Fatal("Could not cast rewardValidatorTx to an UnsignedRewardValidatorTx")
	}
	if _, _, err = utx.Execute(vm, vm.internalState, stx); err == nil {
		t.Fatalf("should have failed because validator ID is wrong")
	}

	// Case 3: Happy path
	tx, err := vm.newRewardValidatorTx(toRemove.ID())
	if err != nil {
		t.Fatal(err)
	}

	_, onAbortState, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	if err != nil {
		t.Fatal(err)
	}

	onAbortCurrentStakers := onAbortState.CurrentStakerChainState()
	nextToRemoveTx, _, err := onAbortCurrentStakers.GetNextStaker()
	if err != nil {
		t.Fatal(err)
	}
	if toRemove.ID() == nextToRemoveTx.ID() {
		t.Fatalf("Should have removed the previous validator")
	}

	// check that stake/reward isn't given back
	stakeOwners := toRemove.Stake[0].Out.(*secp256k1fx.TransferOutput).AddressesSet()

	// Get old balances
	oldBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	if err != nil {
		t.Fatal(err)
	}

	onAbortState.Apply(vm.internalState)
	if err := vm.internalState.Commit(); err != nil {
		t.Fatal(err)
	}

	onAbortBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	if err != nil {
		t.Fatal(err)
	}

	if onAbortBalance != oldBalance+toRemove.Validator.Weight() {
		t.Fatalf("on abort, should have old balance (%d) + staked amount (%d) but have %d",
			oldBalance, toRemove.Validator.Weight(), onAbortBalance)
	}
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
	if err := firstVM.Initialize(firstCtx, firstDB, genesisBytes, nil, nil, firstMsgChan, nil, nil); err != nil {
		t.Fatal(err)
	}

	firstVM.clock.Set(defaultGenesisTime)
	firstVM.uptimeManager.(uptime.TestManager).SetTime(defaultGenesisTime)

	if err := firstVM.SetState(snow.Bootstrapping); err != nil {
		t.Fatal(err)
	}

	if err := firstVM.SetState(snow.NormalOp); err != nil {
		t.Fatal(err)
	}

	// Fast forward clock to time for genesis validators to leave
	firstVM.uptimeManager.(uptime.TestManager).SetTime(defaultValidateEndTime)

	if err := firstVM.Shutdown(); err != nil {
		t.Fatal(err)
	}
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
		if err := secondVM.Shutdown(); err != nil {
			t.Fatal(err)
		}
		secondCtx.Lock.Unlock()
	}()

	secondMsgChan := make(chan common.Message, 1)
	if err := secondVM.Initialize(secondCtx, secondDB, genesisBytes, nil, nil, secondMsgChan, nil, nil); err != nil {
		t.Fatal(err)
	}

	secondVM.clock.Set(defaultValidateStartTime.Add(2 * defaultMinStakingDuration))
	secondVM.uptimeManager.(uptime.TestManager).SetTime(defaultValidateStartTime.Add(2 * defaultMinStakingDuration))

	if err := secondVM.SetState(snow.Bootstrapping); err != nil {
		t.Fatal(err)
	}

	if err := secondVM.SetState(snow.NormalOp); err != nil {
		t.Fatal(err)
	}

	secondVM.clock.Set(defaultValidateEndTime)
	secondVM.uptimeManager.(uptime.TestManager).SetTime(defaultValidateEndTime)

	blk, err := secondVM.BuildBlock() // should contain proposal to advance time
	if err != nil {
		t.Fatal(err)
	} else if err := blk.Verify(); err != nil {
		t.Fatal(err)
	}

	// Assert preferences are correct
	block := blk.(*ProposalBlock)
	options, err := block.Options()
	if err != nil {
		t.Fatal(err)
	}

	commit, ok := options[0].(*CommitBlock)
	if !ok {
		t.Fatal(errShouldPrefCommit)
	}

	abort, ok := options[1].(*AbortBlock)
	if !ok {
		t.Fatal(errShouldPrefCommit)
	}

	if err := block.Accept(); err != nil {
		t.Fatal(err)
	}
	if err := commit.Verify(); err != nil {
		t.Fatal(err)
	}
	if err := abort.Verify(); err != nil {
		t.Fatal(err)
	}

	onAbortState := abort.onAccept()
	_, txStatus, err := onAbortState.GetTx(block.Tx.ID())
	if err != nil {
		t.Fatal(err)
	}
	if txStatus != status.Aborted {
		t.Fatalf("status should be Aborted but is %s", txStatus)
	}

	if err := commit.Accept(); err != nil { // advance the timestamp
		t.Fatal(err)
	}

	_, txStatus, err = secondVM.internalState.GetTx(block.Tx.ID())
	if err != nil {
		t.Fatal(err)
	}
	if txStatus != status.Committed {
		t.Fatalf("status should be Committed but is %s", txStatus)
	}

	// Verify that chain's timestamp has advanced
	timestamp := secondVM.internalState.GetTimestamp()
	if !timestamp.Equal(defaultValidateEndTime) {
		t.Fatal("expected timestamp to have advanced")
	}

	blk, err = secondVM.BuildBlock() // should contain proposal to reward genesis validator
	if err != nil {
		t.Fatal(err)
	}
	if err := blk.Verify(); err != nil {
		t.Fatal(err)
	}

	block = blk.(*ProposalBlock)
	options, err = block.Options()
	if err != nil {
		t.Fatal(err)
	}

	commit, ok = options[1].(*CommitBlock)
	if !ok {
		t.Fatal(errShouldPrefAbort)
	}

	abort, ok = options[0].(*AbortBlock)
	if !ok {
		t.Fatal(errShouldPrefAbort)
	}

	if err := blk.Accept(); err != nil {
		t.Fatal(err)
	}
	if err := commit.Verify(); err != nil {
		t.Fatal(err)
	}

	onCommitState := commit.onAccept()
	_, txStatus, err = onCommitState.GetTx(block.Tx.ID())
	if err != nil {
		t.Fatal(err)
	}
	if txStatus != status.Committed {
		t.Fatalf("status should be Committed but is %s", txStatus)
	}

	if err := abort.Verify(); err != nil {
		t.Fatal(err)
	}
	if err := abort.Accept(); err != nil { // do not reward the genesis validator
		t.Fatal(err)
	}

	_, txStatus, err = secondVM.internalState.GetTx(block.Tx.ID())
	if err != nil {
		t.Fatal(err)
	}
	if txStatus != status.Aborted {
		t.Fatalf("status should be Aborted but is %s", txStatus)
	}

	currentStakers := secondVM.internalState.CurrentStakerChainState()
	_, err = currentStakers.GetValidator(nodeIDs[4])
	if err != database.ErrNotFound {
		t.Fatal("should have removed a genesis validator")
	}
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
	if err := vm.Initialize(ctx, db, genesisBytes, nil, nil, msgChan, nil, appSender); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		ctx.Lock.Unlock()
	}()

	vm.clock.Set(defaultGenesisTime)
	vm.uptimeManager.(uptime.TestManager).SetTime(defaultGenesisTime)

	if err := vm.SetState(snow.Bootstrapping); err != nil {
		t.Fatal(err)
	}

	if err := vm.SetState(snow.NormalOp); err != nil {
		t.Fatal(err)
	}

	// Fast forward clock to time for genesis validators to leave
	vm.clock.Set(defaultValidateEndTime)
	vm.uptimeManager.(uptime.TestManager).SetTime(defaultValidateEndTime)

	blk, err := vm.BuildBlock() // should contain proposal to advance time
	if err != nil {
		t.Fatal(err)
	} else if err := blk.Verify(); err != nil {
		t.Fatal(err)
	}

	// first the time will be advanced.
	block := blk.(*ProposalBlock)
	options, err := block.Options()
	if err != nil {
		t.Fatal(err)
	}

	commit, ok := options[0].(*CommitBlock)
	if !ok {
		t.Fatal(errShouldPrefCommit)
	}
	abort, ok := options[1].(*AbortBlock)
	if !ok {
		t.Fatal(errShouldPrefCommit)
	}

	if err := block.Accept(); err != nil {
		t.Fatal(err)
	}
	if err := commit.Verify(); err != nil {
		t.Fatal(err)
	}
	if err := abort.Verify(); err != nil {
		t.Fatal(err)
	}

	// advance the timestamp
	if err := commit.Accept(); err != nil {
		t.Fatal(err)
	}

	// Verify that chain's timestamp has advanced
	timestamp := vm.internalState.GetTimestamp()
	if !timestamp.Equal(defaultValidateEndTime) {
		t.Fatal("expected timestamp to have advanced")
	}

	// should contain proposal to reward genesis validator
	blk, err = vm.BuildBlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := blk.Verify(); err != nil {
		t.Fatal(err)
	}

	block = blk.(*ProposalBlock)
	options, err = block.Options()
	if err != nil {
		t.Fatal(err)
	}

	abort, ok = options[0].(*AbortBlock)
	if !ok {
		t.Fatal(errShouldPrefAbort)
	}
	commit, ok = options[1].(*CommitBlock)
	if !ok {
		t.Fatal(errShouldPrefAbort)
	}

	if err := blk.Accept(); err != nil {
		t.Fatal(err)
	}
	if err := commit.Verify(); err != nil {
		t.Fatal(err)
	}
	if err := abort.Verify(); err != nil {
		t.Fatal(err)
	}

	// do not reward the genesis validator
	if err := abort.Accept(); err != nil {
		t.Fatal(err)
	}

	currentStakers := vm.internalState.CurrentStakerChainState()
	_, err = currentStakers.GetValidator(nodeIDs[4])
	if err != database.ErrNotFound {
		t.Fatal("should have removed a genesis validator")
	}
}
