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
	"github.com/chain4travel/caminogo/utils/units"
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
	vm.AddStakerTxFee = units.NanoAvax

	// Common data

	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{keys[0].PublicKey().Address()},
	}

	fee := vm.AddStakerTxFee
	validatorBond := vm.internalState.GetValidatorBondAmount()

	inputSigners := []*crypto.PrivateKeySECP256K1R{keys[0]}
	nodeKey, nodeID := generateNodeKeyAndID()
	startTime := uint64(defaultGenesisTime.Add(1 * time.Second).Unix())
	endTime := uint64(defaultGenesisTime.Add(1*time.Second + defaultMinStakingDuration).Unix())

	existingTxID := ids.GenerateTestID()

	// Test cases

	tests := map[string]struct {
		utxos         []*avax.UTXO
		outputs       []*avax.TransferableOutput
		inputIndexes  []uint32
		expectedError error
	}{
		"Happy path bonding": {
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond*2, outputOwners, ids.Empty, ids.Empty),
			},
			outputs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, validatorBond-fee, outputOwners, ids.Empty, ids.Empty),
				generateTestOut(avaxAssetID, validatorBond, outputOwners, ids.Empty, thisTxID),
			},
			inputIndexes:  []uint32{0, 0},
			expectedError: nil,
		},
		"Happy path bond deposited": {
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond*2, outputOwners, existingTxID, ids.Empty),
			},
			outputs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, validatorBond-fee, outputOwners, ids.Empty, ids.Empty),
				generateTestOut(avaxAssetID, validatorBond, outputOwners, existingTxID, ids.Empty),
				generateTestOut(avaxAssetID, validatorBond, outputOwners, existingTxID, thisTxID),
			},
			inputIndexes:  []uint32{0, 1, 1},
			expectedError: nil,
		},
		"Happy path bond deposited and unlocked": {
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond/2, outputOwners, existingTxID, ids.Empty),
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond, outputOwners, ids.Empty, ids.Empty),
			},
			outputs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, validatorBond/2-fee, outputOwners, ids.Empty, ids.Empty),
				generateTestOut(avaxAssetID, validatorBond/2, outputOwners, existingTxID, thisTxID),
				generateTestOut(avaxAssetID, validatorBond/2, outputOwners, existingTxID, thisTxID),
			},
			inputIndexes:  []uint32{0, 1, 1},
			expectedError: nil,
		},
		"Bonding bonded UTXO": {
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond, outputOwners, ids.Empty, existingTxID),
			},
			outputs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, validatorBond-fee, outputOwners, ids.Empty, ids.Empty),
				generateTestOut(avaxAssetID, validatorBond/2, outputOwners, ids.Empty, thisTxID),
			},
			inputIndexes:  []uint32{0, 1},
			expectedError: errWrongLockState,
		},
		"Fee burning bonded UTXO": {
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond, outputOwners, ids.Empty, existingTxID),
			},
			outputs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, validatorBond/2, outputOwners, ids.Empty, ids.Empty),
				generateTestOut(avaxAssetID, validatorBond/2-fee, outputOwners, ids.Empty, existingTxID),
				generateTestOut(avaxAssetID, validatorBond/2, outputOwners, ids.Empty, thisTxID),
			},
			inputIndexes:  []uint32{0, 1, 0},
			expectedError: errWrongLockState,
		},
		"Fee burning deposited UTXO": {
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.GenerateTestID(), avaxAssetID, validatorBond*2, outputOwners, existingTxID, ids.Empty),
			},
			outputs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, validatorBond-fee, outputOwners, existingTxID, ids.Empty),
				generateTestOut(avaxAssetID, validatorBond, outputOwners, existingTxID, thisTxID),
			},
			inputIndexes:  []uint32{0, 1, 0},
			expectedError: errNotBurnedEnough,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// Preparing tx data and utxos

			state := newVersionedState(
				vm.internalState,
				vm.internalState.CurrentStakerChainState(),
				vm.internalState.PendingStakerChainState(),
			)

			inputs := make([]*avax.TransferableInput, len(tt.utxos))
			signers := make([][]*crypto.PrivateKeySECP256K1R, len(tt.utxos)+1)
			for i, utxo := range tt.utxos {
				state.AddUTXO(utxo)
				inputs[i] = generateTestInFromUTXO(utxo, []uint32{0})
				signers[i] = inputSigners
			}

			totalBondAmount := uint64(0)
			for _, out := range tt.outputs {
				if lockedOut, ok := out.Out.(*LockedOut); ok && lockedOut.BondTxID == thisTxID {
					totalBondAmount += lockedOut.Amount()
				}
			}

			signers[len(signers)-1] = []*crypto.PrivateKeySECP256K1R{nodeKey}

			// avax.SortTransferableOutputs(tt.outputs, Codec)
			avax.SortTransferableInputsWithSigners(inputs, signers)

			// Preparing tx
			utx := &UnsignedAddValidatorTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    vm.ctx.NetworkID,
					BlockchainID: vm.ctx.ChainID,
					Ins:          inputs,
					Outs:         tt.outputs,
				}},
				Validator: Validator{
					NodeID: nodeID,
					Start:  startTime,
					End:    endTime,
					Wght:   totalBondAmount,
				},
				RewardsOwner: &outputOwners,
			}
			tx := &Tx{UnsignedTx: utx}

			err := tx.Sign(Codec, signers)
			assert.NoError(err)

			// Testing execute
			_, _, err = tx.UnsignedTx.(*UnsignedAddValidatorTx).Execute(vm, state, tx)
			assert.ErrorIs(err, tt.expectedError)
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
		generateTestUTXO(ids.ID{9, 9}, avaxAssetID, defaultValidatorStake, outputOwners, ids.Empty, ids.Empty),
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

	txID := tx.ID()

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

	// TODO@ balance check?

	assertOutsProducedUTXOs(assert, vm.internalState, txID, utx.Outs)
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
			Ins: []*avax.TransferableInput{
				generateTestInFromUTXO(utxo, []uint32{0}),
			},
			Outs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, defaultValidatorStake, outputOwners, ids.Empty, thisTxID),
			},
		}},
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
