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
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	currentStakers := vm.internalState.CurrentStakerChainState()
	stakerToRemoveTx, _, err := currentStakers.GetNextStaker()
	if err != nil {
		t.Fatal(err)
	}
	addValidatorTxID := stakerToRemoveTx.ID()
	addValidatorTx, ok := stakerToRemoveTx.UnsignedTx.(*UnsignedAddValidatorTx)
	assert.True(t, ok)

	t.Run("Chain timestamp is wrong", func(t *testing.T) {
		assert := assert.New(t)
		stx, err := vm.newRewardValidatorTx(addValidatorTxID)
		assert.NoError(err)
		utx, ok := stx.UnsignedTx.(*UnsignedRewardValidatorTx)
		assert.True(ok)
		_, _, err = utx.Execute(vm, vm.internalState, stx)
		assert.ErrorIs(err, errToEarlyValidatorRemoval)
	})

	// Advance chain timestamp to time that next validator leaves
	vm.internalState.SetTimestamp(addValidatorTx.EndTime())

	t.Run("Not zero creds", func(t *testing.T) {
		assert := assert.New(t)
		stx, err := vm.newRewardValidatorTx(addValidatorTxID)
		assert.NoError(err)
		stx.Creds = append(stx.Creds, &secp256k1fx.Credential{})
		utx, ok := stx.UnsignedTx.(*UnsignedRewardValidatorTx)
		assert.True(ok)
		_, _, err = utx.Execute(vm, vm.internalState, stx)
		assert.ErrorIs(err, errWrongNumberOfCredentials)
	})

	t.Run("Wrong validator", func(t *testing.T) {
		assert := assert.New(t)
		utx := &UnsignedRewardValidatorTx{
			ValidatorTxID: ids.GenerateTestID(),
		}
		stx := &Tx{UnsignedTx: utx}
		err := stx.Sign(Codec, nil)
		assert.NoError(err)
		_, _, err = utx.Execute(vm, vm.internalState, stx)
		assert.ErrorIs(err, errWrongValidatorRemoval)
	})

	t.Run("Invalid ins len (one excess)", func(t *testing.T) {
		assert := assert.New(t)
		ins, outs, err := vm.unlock(vm.internalState, []ids.ID{addValidatorTxID}, LockStateBonded)
		assert.NoError(err)

		ins = append(ins, &avax.TransferableInput{In: &secp256k1fx.TransferInput{}}) // invalid len

		utx := &UnsignedRewardValidatorTx{
			Ins:           ins, // invalid len
			Outs:          outs,
			ValidatorTxID: addValidatorTxID,
		}
		stx := &Tx{UnsignedTx: utx}
		err = stx.Sign(Codec, nil)
		assert.NoError(err)
		_, _, err = utx.Execute(vm, vm.internalState, stx)
		assert.ErrorIs(err, errTxBodyMissmatch)
	})

	t.Run("Invalid outs len (one excess)", func(t *testing.T) {
		assert := assert.New(t)
		ins, outs, err := vm.unlock(vm.internalState, []ids.ID{addValidatorTxID}, LockStateBonded) // invalid len
		assert.NoError(err)

		outs = append(outs, &avax.TransferableOutput{Out: &secp256k1fx.TransferOutput{}})

		utx := &UnsignedRewardValidatorTx{
			Ins:           ins,
			Outs:          outs, // invalid len
			ValidatorTxID: addValidatorTxID,
		}
		stx := &Tx{UnsignedTx: utx}
		err = stx.Sign(Codec, nil)
		assert.NoError(err)
		_, _, err = utx.Execute(vm, vm.internalState, stx)
		assert.ErrorIs(err, errTxBodyMissmatch)
	})

	t.Run("Invalid input", func(t *testing.T) {
		assert := assert.New(t)
		ins, outs, err := vm.unlock(vm.internalState, []ids.ID{addValidatorTxID}, LockStateBonded)
		assert.NoError(err)

		assert.Greater(len(ins), 0)
		inputLockIDs := LockIDs{}
		if lockedIn, ok := ins[0].In.(*LockedIn); ok {
			inputLockIDs = lockedIn.LockIDs
		}
		ins[0] = &avax.TransferableInput{
			UTXOID: ins[0].UTXOID,
			Asset:  ins[0].Asset,
			In: &LockedIn{
				LockIDs: inputLockIDs,
				TransferableIn: &secp256k1fx.TransferInput{
					Amt: ins[0].In.Amount() - 1, // invalid
				},
			},
		}

		utx := &UnsignedRewardValidatorTx{
			Ins:           ins, // invalid
			Outs:          outs,
			ValidatorTxID: addValidatorTxID,
		}
		stx := &Tx{UnsignedTx: utx}
		err = stx.Sign(Codec, nil)
		assert.NoError(err)
		_, _, err = utx.Execute(vm, vm.internalState, stx)
		assert.ErrorIs(err, errTxBodyMissmatch)
	})

	t.Run("Invalid output", func(t *testing.T) {
		assert := assert.New(t)
		ins, outs, err := vm.unlock(vm.internalState, []ids.ID{addValidatorTxID}, LockStateBonded)
		assert.NoError(err)

		assert.Greater(len(outs), 0)
		validOut := outs[0].Out
		if lockedOut, ok := validOut.(*LockedOut); ok {
			validOut = lockedOut.TransferableOut
		}
		secpOut, ok := validOut.(*secp256k1fx.TransferOutput)
		assert.True(ok)

		var invalidOut avax.TransferableOut = &secp256k1fx.TransferOutput{
			Amt:          secpOut.Amt - 1, // invalid
			OutputOwners: secpOut.OutputOwners,
		}
		if lockedOut, ok := validOut.(*LockedOut); ok {
			invalidOut = &LockedOut{
				LockIDs:         lockedOut.LockIDs,
				TransferableOut: invalidOut,
			}
		}
		outs[0] = &avax.TransferableOutput{
			Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
			Out:   invalidOut,
		}

		utx := &UnsignedRewardValidatorTx{
			Ins:           ins,
			Outs:          outs, // invalid
			ValidatorTxID: addValidatorTxID,
		}
		stx := &Tx{UnsignedTx: utx}
		err = stx.Sign(Codec, nil)
		assert.NoError(err)
		_, _, err = utx.Execute(vm, vm.internalState, stx)
		assert.ErrorIs(err, errTxBodyMissmatch)
	})

	t.Run("Happy path", func(t *testing.T) {
		assert := assert.New(t)
		tx, err := vm.newRewardValidatorTx(addValidatorTxID)
		assert.NoError(err)

		rewardValidatorTx, ok := tx.UnsignedTx.(*UnsignedRewardValidatorTx)
		assert.True(ok)

		onCommitState, _, err := rewardValidatorTx.Execute(vm, vm.internalState, tx)
		assert.NoError(err)

		// Checking onCommitState

		onCommitCurrentStakers := onCommitState.CurrentStakerChainState()
		nextToRemoveTx, _, err := onCommitCurrentStakers.GetNextStaker()
		assert.NoError(err)
		assert.NotEqual(addValidatorTxID, nextToRemoveTx.ID(),
			"Should have removed the previous validator")

		// check that stake/reward is given back
		innerOut := addValidatorTx.Outs[0].Out.(*LockedOut)
		stakeOwners := innerOut.TransferableOut.(*secp256k1fx.TransferOutput).AddressesSet()

		// Get old balances
		oldBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
		assert.NoError(err)

		onCommitState.Apply(vm.internalState)
		err = vm.internalState.Commit()
		assert.NoError(err)

		onCommitBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
		assert.NoError(err)

		const reward uint64 = 13773

		assert.Equalf(onCommitBalance, oldBalance+reward,
			"on commit, should have old balance (%d) + staked amount (%d) + reward (%d) but have %d",
			oldBalance, addValidatorTx.Validator.Weight(), reward, onCommitBalance)

		rewardValidatorTxID := tx.ID()
		bondedOuts := addValidatorTx.Bond()
		// TODO@ test bond unbonded
		//  on commit, utxos bonded by addValidatorTxID should consumed to produce unbonded
		for i := range bondedOuts {
			_, err := vm.internalState.GetUTXO(addValidatorTxID.Prefix(uint64(i)))
			assert.ErrorIs(err, database.ErrNotFound)
		}

		assertOutsProducedUTXOs(
			assert,
			vm.internalState,
			rewardValidatorTxID,
			rewardValidatorTx.Outs,
		)
	})
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

	assert := assert.New(t)

	currentStakers := vm.internalState.CurrentStakerChainState()
	stakerToRemoveTx, _, err := currentStakers.GetNextStaker()
	assert.NoError(err)
	addValidatorTxID := stakerToRemoveTx.ID()
	addValidatorTx, ok := stakerToRemoveTx.UnsignedTx.(*UnsignedAddValidatorTx)
	assert.True(ok)

	// Advance chain timestamp to time that next validator leaves
	vm.internalState.SetTimestamp(addValidatorTx.EndTime())

	tx, err := vm.newRewardValidatorTx(addValidatorTxID)
	assert.NoError(err)

	rewardValidatorTx, ok := tx.UnsignedTx.(*UnsignedRewardValidatorTx)
	assert.True(ok)

	_, onAbortState, err := rewardValidatorTx.Execute(vm, vm.internalState, tx)
	assert.NoError(err)

	onAbortCurrentStakers := onAbortState.CurrentStakerChainState()
	nextToRemoveTx, _, err := onAbortCurrentStakers.GetNextStaker()
	assert.NoError(err)
	assert.NotEqual(addValidatorTxID, nextToRemoveTx.ID(),
		"Should have removed the previous validator")

	// check that stake/reward isn't given back
	innerOut := addValidatorTx.Outs[0].Out.(*LockedOut)
	stakeOwners := innerOut.TransferableOut.(*secp256k1fx.TransferOutput).AddressesSet()

	// Get old balances
	oldBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	assert.NoError(err)

	onAbortState.Apply(vm.internalState)
	err = vm.internalState.Commit()
	assert.NoError(err)

	onAbortBalance, err := avax.GetBalance(vm.internalState, stakeOwners)
	assert.NoError(err)

	assert.Equalf(onAbortBalance, oldBalance,
		"on abort, should have old balance (%d) + staked amount (%d) but have %d",
		oldBalance, addValidatorTx.Validator.Weight(), onAbortBalance)

	bondedOuts := addValidatorTx.Bond()
	for i := range bondedOuts {
		_, err := vm.internalState.GetUTXO(addValidatorTxID.Prefix(uint64(i)))
		assert.ErrorIs(err, database.ErrNotFound)
	}

	assertOutsProducedUTXOs(assert, vm.internalState, tx.ID(), rewardValidatorTx.Outs)
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
	_, err = currentStakers.GetValidator(nodeIDs[0])
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
	_, err = currentStakers.GetValidator(nodeIDs[0])
	if err != database.ErrNotFound {
		t.Fatal("should have removed a genesis validator")
	}
}

func TestRewardValidatorTxSyntacticVerify(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	stakerToRemoveTx, _, err := vm.internalState.CurrentStakerChainState().GetNextStaker()
	assert.NoError(t, err)
	addValidatorTxID := stakerToRemoveTx.ID()

	t.Run("valid", func(t *testing.T) {
		_, err := vm.newRewardValidatorTx(addValidatorTxID)
		assert.NoError(t, err)
	})

	t.Run("tx is nil", func(t *testing.T) {
		var tx *UnsignedRewardValidatorTx
		err := tx.SyntacticVerify(vm.ctx)
		assert.ErrorIs(t, err, errNilTx)
	})

	t.Run("empty validatorTxID", func(t *testing.T) {
		tx, err := vm.newRewardValidatorTx(addValidatorTxID)
		assert.NoError(t, err)
		tx.UnsignedTx.(*UnsignedRewardValidatorTx).ValidatorTxID = ids.Empty
		tx.UnsignedTx.(*UnsignedRewardValidatorTx).syntacticallyVerified = false
		err = tx.SyntacticVerify(vm.ctx)
		assert.ErrorIs(t, err, errInvalidID)
	})
}
