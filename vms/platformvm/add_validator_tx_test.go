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
	"math"
	"testing"
	"time"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/platformvm/status"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
	"github.com/stretchr/testify/assert"
)

func TestAddValidatorTxExecuteBonding(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{keys[0].PublicKey().Address()},
	}

	defAmt := defaultValidatorStake * 2 // 10000000

	// Setting up UTXOs for tests
	utxos := make([]*avax.UTXO, 5)
	utxoToStateMap := map[int]LockState{
		0: LockStateUnlocked,
		1: LockStateDeposited,
		2: LockStateBonded,
		3: LockStateDepositedBonded,
		4: LockStateUnlocked,
	}
	for i := 0; i < len(utxos); i++ {
		utxos[i] = generateTestUTXO(ids.ID{byte(i)}, vm.ctx.AVAXAssetID, defAmt, outputOwners, utxoToStateMap[i])
	}

	unlockedUTXO := utxos[0]
	depositedUTXO := utxos[1]
	bondedUTXO := utxos[2]
	depositedAndBondedUTXO := utxos[3]
	unlockedSmallUTXO := utxos[4]
	unlockedSmallUTXO.Out.(*secp256k1fx.TransferOutput).Amt = defAmt / 4 // = defaultValidatorStake / 2 < defaultValidatorStake

	utxosByID := map[ids.ID]*avax.UTXO{
		unlockedUTXO.InputID():           unlockedUTXO,
		depositedUTXO.InputID():          depositedUTXO,
		bondedUTXO.InputID():             bondedUTXO,
		depositedAndBondedUTXO.InputID(): depositedAndBondedUTXO,
		unlockedSmallUTXO.InputID():      unlockedSmallUTXO,
	}

	for _, utxo := range utxos {
		vm.internalState.AddUTXO(utxo)
	}
	lockedUTXOsState := &lockedUTXOsChainStateImpl{
		bonds: map[ids.ID]ids.Set{
			bondedUTXO.TxID: map[ids.ID]struct{}{bondedUTXO.InputID(): {}},
		},
		deposits: map[ids.ID]ids.Set{
			depositedUTXO.TxID:          map[ids.ID]struct{}{depositedUTXO.InputID(): {}},
			depositedAndBondedUTXO.TxID: map[ids.ID]struct{}{depositedAndBondedUTXO.InputID(): {}},
		},
		lockedUTXOs: map[ids.ID]utxoLockState{
			bondedUTXO.InputID():             {BondTxID: &bondedUTXO.TxID},
			depositedUTXO.InputID():          {DepositTxID: &depositedUTXO.TxID},
			depositedAndBondedUTXO.InputID(): {DepositTxID: &depositedAndBondedUTXO.TxID},
		},
	}
	lockedUTXOsState.Apply(vm.internalState)
	err := vm.internalState.Commit()
	assert.NoError(t, err)

	// Other common data

	inputSigners := []*crypto.PrivateKeySECP256K1R{keys[0]}
	nodeKey, nodeID := generateNodeKeyAndID()
	startTime := uint64(defaultGenesisTime.Add(1 * time.Second).Unix())
	endTime := uint64(defaultGenesisTime.Add(1*time.Second + defaultMinStakingDuration).Unix())

	fee := defaultTxFee

	// Test cases

	tests := map[string]struct {
		inputUTXOIDs  []ids.ID
		outAmounts    []output
		inputIndexes  []uint32
		expectedError error
	}{
		"Happy path bonding": {
			inputUTXOIDs:  []ids.ID{unlockedUTXO.InputID()},
			outAmounts:    []output{{LockStateUnlocked, defAmt/2 - fee}, {LockStateBonded, defAmt / 2}},
			inputIndexes:  []uint32{0, 0},
			expectedError: nil,
		},
		"Happy path bond deposited": {
			inputUTXOIDs:  []ids.ID{unlockedUTXO.InputID(), depositedUTXO.InputID()},
			outAmounts:    []output{{LockStateUnlocked, defAmt - fee}, {LockStateDeposited, defAmt / 2}, {LockStateDepositedBonded, defAmt / 2}},
			inputIndexes:  []uint32{0, 1, 1},
			expectedError: nil,
		},
		// "Burning bonded utxo" skipped cause if will fail with errLockingLockedUTXO
		"Bonding bonded UTXO": {
			inputUTXOIDs:  []ids.ID{unlockedUTXO.InputID(), bondedUTXO.InputID()},
			outAmounts:    []output{{LockStateUnlocked, defAmt - fee}, {LockStateBonded, defAmt / 2}},
			inputIndexes:  []uint32{0, 1},
			expectedError: errWrongLockState,
		},
		"Burning deposited UTXO (not for fee, just burning)": {
			inputUTXOIDs:  []ids.ID{unlockedUTXO.InputID(), depositedUTXO.InputID()},
			outAmounts:    []output{{LockStateUnlocked, defAmt/2 - fee}, {LockStateDeposited, defAmt / 2}, {LockStateBonded, defAmt / 2}},
			inputIndexes:  []uint32{0, 1, 0},
			expectedError: errBurningLockedUTXO,
		},
		"Fee burning bonded UTXO": {
			inputUTXOIDs:  []ids.ID{unlockedUTXO.InputID(), bondedUTXO.InputID()},
			outAmounts:    []output{{LockStateUnlocked, defAmt / 2}, {LockStateBonded, defAmt/2 - fee}, {LockStateBonded, defAmt / 2}},
			inputIndexes:  []uint32{0, 1, 0},
			expectedError: errWrongLockState,
		},
		"Fee burning deposited UTXO": {
			inputUTXOIDs:  []ids.ID{unlockedUTXO.InputID(), depositedUTXO.InputID()},
			outAmounts:    []output{{LockStateUnlocked, defAmt / 2}, {LockStateDeposited, defAmt/2 - fee}, {LockStateBonded, defAmt / 2}},
			inputIndexes:  []uint32{0, 1, 0},
			expectedError: errBurningLockedUTXO,
		},
		"Produced out linked with input by index has amount greater than input's ": {
			inputUTXOIDs:  []ids.ID{unlockedUTXO.InputID(), unlockedSmallUTXO.InputID()},
			outAmounts:    []output{{LockStateUnlocked, defAmt/2 - fee}, {LockStateBonded, defAmt / 2}},
			inputIndexes:  []uint32{0, 1},
			expectedError: errWrongProducedAmount,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// Preparing ins, outs and signers
			signers := make([][]*crypto.PrivateKeySECP256K1R, len(tt.inputUTXOIDs)+1)
			ins := make([]*avax.TransferableInput, len(tt.inputUTXOIDs))
			outs := make([]*avax.TransferableOutput, len(tt.outAmounts))
			totalBondAmount := uint64(0)

			for i, inputID := range tt.inputUTXOIDs {
				utxo := utxosByID[inputID]
				ins[i] = generateTestInFromUTXO(vm.ctx.AVAXAssetID, utxo, []uint32{0})
				signers[i] = inputSigners
			}

			for i, outAmount := range tt.outAmounts {
				outs[i] = generateTestOut(vm.ctx.AVAXAssetID, outAmount.state, outAmount.amount, outputOwners)
				if outAmount.state.isBonded() {
					totalBondAmount += outAmount.amount
				}
			}

			signers[len(signers)-1] = []*crypto.PrivateKeySECP256K1R{nodeKey}

			// Preparing tx
			utx := &UnsignedAddValidatorTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    vm.ctx.NetworkID,
					BlockchainID: vm.ctx.ChainID,
					Ins:          ins,
					Outs:         outs,
				}},
				Validator: Validator{
					NodeID: nodeID,
					Start:  startTime,
					End:    endTime,
					Wght:   totalBondAmount,
				},
				InputIndexes: tt.inputIndexes,
				RewardsOwner: &outputOwners,
			}
			tx := &Tx{UnsignedTx: utx}

			if err := tx.Sign(Codec, signers); err != nil {
				t.Fatal(err)
			}

			// Testing execute
			_, _, err = tx.UnsignedTx.(*UnsignedAddValidatorTx).Execute(vm, vm.internalState, tx)

			if tt.expectedError != nil {
				assert.ErrorIs(err, tt.expectedError)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestAddValidatorTxExecuteUnitTesting(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	key, err := vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	nodeID := key.PublicKey().Address()

	// Case: Validator's weight is less than the minimum amount
	tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)

	tx.UnsignedTx.(*UnsignedAddValidatorTx).Validator.Wght = 0
	if err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because validator's weight is less than the minimum amount")
	}

	// Case: Validator's weight is more than the maximum amount
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)

	tx.UnsignedTx.(*UnsignedAddValidatorTx).Validator.Wght = defaultValidatorStake + 1
	if err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because validator's weight is more than the maximum amount")
	}

	// Case: Validator in pending validator set of primary network
	key2, err := vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	startTime := defaultGenesisTime.Add(1 * time.Second)
	tx, err = vm.newAddValidatorTx(
		uint64(startTime.Unix()),                                // start time
		uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,                     // node ID
		key2.PublicKey().Address(), // reward address
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	if err != nil {
		t.Fatal(err)
	}

	vm.internalState.AddCurrentStaker(tx, 0)
	vm.internalState.AddTx(tx, status.Committed)
	if err := vm.internalState.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := vm.internalState.(*internalStateImpl).loadCurrentValidators(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should have failed because validator in pending validator set")
	}

	// Case: Validator in pending validator set of primary network
	key2, err = vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	startTime = defaultGenesisTime.Add(1 * time.Second)
	tx, err = vm.newAddValidatorTx(
		uint64(startTime.Unix()),                                // start time
		uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,                     // node ID
		key2.PublicKey().Address(), // reward address
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	if err != nil {
		t.Fatal(err)
	}

	vm.internalState.AddPendingStaker(tx)
	vm.internalState.AddTx(tx, status.Committed)
	if err := vm.internalState.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := vm.internalState.(*internalStateImpl).loadPendingValidators(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should have failed because validator in pending validator set")
	}
}

func TestAddValidatorTxSyntacticVerify(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	nodeKey, nodeID := generateNodeKeyAndID()

	// Case: tx is nil
	var unsignedTx *UnsignedAddValidatorTx
	if err := unsignedTx.SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because tx is nil")
	}

	// Case 3: Wrong Network ID
	tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	)
	if err != nil {
		t.Fatal(err)
	}
	tx.UnsignedTx.(*UnsignedAddValidatorTx).NetworkID++
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because the wrong network ID was used")
	}

	// Case: Stake owner has no addresses
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	)
	if err != nil {
		t.Fatal(err)
	}
	tx.UnsignedTx.(*UnsignedAddValidatorTx).Outs = append(tx.UnsignedTx.(*UnsignedAddValidatorTx).Outs, &avax.TransferableOutput{
		Asset: avax.Asset{ID: avaxAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt: vm.internalState.GetValidatorBondAmount(),
			OutputOwners: secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     nil,
			},
		},
	})

	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because stake owner has no addresses")
	}

	// Case: Rewards owner has no addresses
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	)
	if err != nil {
		t.Fatal(err)
	}
	tx.UnsignedTx.(*UnsignedAddValidatorTx).RewardsOwner = &secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     nil,
	}
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because rewards owner has no addresses")
	}

	// Case: Valid
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	); err != nil {
		t.Fatal(err)
	} else if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err != nil {
		t.Fatal(err)
	}
}

// Test AddValidatorTx.Execute
func TestAddValidatorTxExecute(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	nodeKey, nodeID := generateNodeKeyAndID()

	// Case: Valid
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix())+1,
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err != nil {
		t.Fatal(err)
	}

	// Case: Failed node signature verification
	// In this case the Tx will not even be signed from the node's key
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix())+1,
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKeys[1]},
	); err != nil {
		t.Fatal(err)
	} else if _, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); !errors.Is(err, errNodeSigVerificationFailed) {
		t.Fatalf("should have errored with: '%s' error", errNodeSigVerificationFailed)
	}

	// Case: Validator's start time too early
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix())-1,
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because start time too early")
	}

	// Case: Validator's start time too far in the future
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Add(maxFutureStartTime).Unix()+1),
		uint64(defaultValidateStartTime.Add(maxFutureStartTime).Add(defaultMinStakingDuration).Unix()+1),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because start time too far in the future")
	}

	// Case: Validator already validating primary network
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID, // node ID
		nodeID, // reward address
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because validator already validating")
	}

	// Case: Validator in pending validator set of primary network
	nodeKey1, nodeID1 := generateNodeKeyAndID()
	startTime := defaultGenesisTime.Add(1 * time.Second)
	tx, err := vm.newAddValidatorTx(
		uint64(startTime.Unix()),                                // start time
		uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,  // node ID
		nodeID1, // reward address
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey1},
	)
	if err != nil {
		t.Fatal(err)
	}

	vm.internalState.AddCurrentStaker(tx, 0)
	vm.internalState.AddTx(tx, status.Committed)
	if err := vm.internalState.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := vm.internalState.(*internalStateImpl).loadCurrentValidators(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should have failed because validator in pending validator set")
	}

	// Case: Validator doesn't have enough tokens to cover stake amount
	if _, err := vm.newAddValidatorTx( // create the tx
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0], nodeKey},
	); err != nil {
		t.Fatal(err)
	}
	// Remove all UTXOs owned by keys[0]
	utxoIDs, err := vm.internalState.UTXOIDs(keys[0].PublicKey().Address().Bytes(), ids.Empty, math.MaxInt32)
	if err != nil {
		t.Fatal(err)
	}
	for _, utxoID := range utxoIDs {
		vm.internalState.DeleteUTXO(utxoID)
	}
	// Now keys[0] has no funds
	if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should have failed because tx fee paying key has no funds")
	}

	// Happy path
	key, err := vm.factory.NewPrivateKey()
	privKey, ok := key.(*crypto.PrivateKeySECP256K1R)
	nodeID = key.PublicKey().Address()
	if !ok {
		t.Fatal(err)
	}
	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{key.PublicKey().Address()},
	}

	// Add tokens to key in order to be able to execute AddValidator Tx
	utxos := []*avax.UTXO{
		generateTestUTXO(ids.ID{9, 9}, avaxAssetID, defaultValidatorStake, outputOwners, LockStateUnlocked),
	}
	for _, utxo := range utxos {
		vm.internalState.AddUTXO(utxo)
	}
	err = vm.internalState.Commit()
	if err != nil {
		t.Fatal(err)
	}

	if tx, err = vm.newAddValidatorTx( // create the tx
		uint64(defaultValidateStartTime.Unix()+1),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{privKey},
	); err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)

	utx, ok := tx.UnsignedTx.(*UnsignedAddValidatorTx)
	assert.True(ok)

	onCommitState, _, err := utx.Execute(vm, vm.internalState, tx)
	if err != nil {
		t.Fatal(err)
	}

	addValidatorTxID := tx.ID()
	lockChainState := vm.internalState.LockedUTXOsChainState()
	bondedUTXOIDs := lockChainState.GetBondedUTXOIDs(addValidatorTxID)
	assert.Empty(bondedUTXOIDs)

	onCommitState.Apply(vm.internalState)
	err = vm.internalState.Commit()
	if err != nil {
		t.Fatal(err)
	}

	pendingStakers := vm.internalState.PendingStakerChainState()
	_, err = pendingStakers.GetValidatorTx(nodeID)
	if err != nil {
		t.Fatal(err)
	}

	// Check balance and lock state from lockedChainState
	lockChainState = vm.internalState.LockedUTXOsChainState()
	bondedUTXOIDs = lockChainState.GetBondedUTXOIDs(addValidatorTxID)

	assert.Equal(len(bondedUTXOIDs), len(utx.Outs))

	outIndexesMap := map[uint32]struct{}{}

	for bondedUTXOID := range bondedUTXOIDs {
		bondedUTXO, err := vm.internalState.GetUTXO(bondedUTXOID)
		assert.NoError(err)

		assert.Equal(bondedUTXO.Asset, utx.Outs[bondedUTXO.OutputIndex].Asset)
		assert.Equal(bondedUTXO.Out, utx.Outs[bondedUTXO.OutputIndex].Out)
		assert.Equal(bondedUTXO.TxID, addValidatorTxID)
		assert.NotContains(outIndexesMap, bondedUTXO.OutputIndex)

		outIndexesMap[bondedUTXO.OutputIndex] = struct{}{}

		bondedUTXOLockState := lockChainState.GetUTXOLockState(bondedUTXOID)
		assert.NotNil(bondedUTXOLockState.BondTxID)
		assert.Equal(*bondedUTXOLockState.BondTxID, addValidatorTxID)
		assert.Nil(bondedUTXOLockState.DepositTxID) // cause we created consumed utxo that way
	}
}

func TestAddValidatorTxManuallyWrongSignature(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()
	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{keys[0].PublicKey().Address()},
	}
	nodeKey, _ := generateNodeKeyAndID()
	_, nodeID := generateNodeKeyAndID()
	signers := [][]*crypto.PrivateKeySECP256K1R{{keys[0]}, {nodeKey}}

	utxo := &avax.UTXO{
		UTXOID: avax.UTXOID{TxID: ids.ID{byte(1)}},
		Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt:          defaultValidatorStake,
			OutputOwners: outputOwners,
		},
	}
	vm.internalState.AddUTXO(utxo)
	err := vm.internalState.Commit()
	assert.NoError(t, err)

	utx := &UnsignedAddValidatorTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins: []*avax.TransferableInput{{
				UTXOID: utxo.UTXOID,
				Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt:   defaultValidatorStake,
					Input: secp256k1fx.Input{SigIndices: []uint32{0}},
				},
			}},
			Outs: []*avax.TransferableOutput{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &LockedOut{
					LockState: LockStateBonded,
					TransferableOut: &secp256k1fx.TransferOutput{
						Amt:          defaultValidatorStake,
						OutputOwners: outputOwners,
					},
				},
			}},
		}},
		InputIndexes: []uint32{0},
		Validator: Validator{
			NodeID: nodeID,
			Start:  uint64(defaultGenesisTime.Add(1 * time.Second).Unix()),
			End:    uint64(defaultGenesisTime.Add(1*time.Second + defaultMinStakingDuration).Unix()),
			Wght:   defaultValidatorStake,
		},
		RewardsOwner: &outputOwners,
	}
	tx := &Tx{UnsignedTx: utx}

	if err := tx.Sign(Codec, signers); err != nil {
		t.Fatal(err)
	}

	// Testing execute
	_, _, err = tx.UnsignedTx.(*UnsignedAddValidatorTx).Execute(vm, vm.internalState, tx)
	assert.Equal(t, errNodeSigVerificationFailed, err)
}
