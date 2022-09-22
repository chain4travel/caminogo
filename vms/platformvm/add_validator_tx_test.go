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
	"math"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/platformvm/reward"
	"github.com/chain4travel/caminogo/vms/platformvm/status"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
	"github.com/stretchr/testify/assert"
)

func TestTopLevelBondingCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
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

	defAmt := vm.MinValidatorStake * 2 // 10000000

	startTime := defaultGenesisTime.Add(1 * time.Second)

	utxo := avax.UTXO{}

	signers := [][]*crypto.PrivateKeySECP256K1R{}

	basic_key := keys[0]

	address := basic_key.PublicKey().Address()

	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{address},
	}

	type testCases struct {
		name            string
		inputIndexes    []uint32
		unlockedAmt     uint64
		lockedAmt       uint64
		validatorWeight uint64
		generateState   func(secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl)
		expectedError   error
	}

	tests := []testCases{
		{
			name:            "1. Happy path bonding",
			inputIndexes:    []uint32{0, 0},
			unlockedAmt:     defAmt/2 - defaultTxFee,
			lockedAmt:       defAmt / 2,
			validatorWeight: defAmt / 2,
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{1, 1},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{
					bonds:       map[ids.ID]ids.Set{},
					deposits:    map[ids.ID]ids.Set{},
					lockedUTXOs: map[ids.ID]utxoLockState{},
				}
				return utxos, lockedUTXOsState
			},
			expectedError: nil,
		},
		{
			name:            "2. Bond Deposited",
			inputIndexes:    []uint32{0, 1},
			unlockedAmt:     defAmt - defaultTxFee,
			lockedAmt:       defAmt,
			validatorWeight: defAmt,
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{2, 2},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{3, 3},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{
					bonds: map[ids.ID]ids.Set{},
					deposits: map[ids.ID]ids.Set{
						utxos[1].TxID: map[ids.ID]struct{}{utxos[1].InputID(): {}},
					},
					lockedUTXOs: map[ids.ID]utxoLockState{
						utxos[1].InputID(): {DepositTxID: &utxos[1].TxID},
					},
				}
				return utxos, lockedUTXOsState
			},
			expectedError: nil,
		},
		{
			name:            "3. Burned More",
			inputIndexes:    []uint32{0, 0},
			unlockedAmt:     defAmt/2 - defaultTxFee,
			lockedAmt:       defAmt / 2,
			validatorWeight: defAmt / 2,
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{4, 4},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{5, 5},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{
					bonds: map[ids.ID]ids.Set{},
					deposits: map[ids.ID]ids.Set{
						utxos[1].TxID: map[ids.ID]struct{}{utxos[1].InputID(): {}},
					},
					lockedUTXOs: map[ids.ID]utxoLockState{
						utxos[1].InputID(): {DepositTxID: &utxos[1].TxID},
					},
				}
				return utxos, lockedUTXOsState
			},
			expectedError: nil,
		},
		{
			name:            "4. Bonding Bonded UTXO",
			inputIndexes:    []uint32{0, 1},
			unlockedAmt:     defAmt - defaultTxFee,
			lockedAmt:       defAmt,
			validatorWeight: defAmt,
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{6, 6},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{7, 7},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{
					bonds: map[ids.ID]ids.Set{
						utxos[1].TxID: map[ids.ID]struct{}{utxos[1].InputID(): {}},
					},
					deposits: map[ids.ID]ids.Set{},
					lockedUTXOs: map[ids.ID]utxoLockState{
						utxos[1].InputID(): {BondTxID: &utxos[1].TxID},
					},
				}
				return utxos, lockedUTXOsState
			},
			expectedError: errors.New("utxo consumed for locking are already locked"),
		},
		{
			name:            "5. Fee Burning Bonded UTXO",
			inputIndexes:    []uint32{0, 1},
			unlockedAmt:     defAmt - defaultTxFee,
			lockedAmt:       defAmt,
			validatorWeight: defAmt,
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{9, 9},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{
					bonds: map[ids.ID]ids.Set{
						utxos[0].TxID: map[ids.ID]struct{}{utxos[0].InputID(): {}},
					},
					deposits: map[ids.ID]ids.Set{},
					lockedUTXOs: map[ids.ID]utxoLockState{
						utxos[0].InputID(): {BondTxID: &utxos[0].TxID},
					},
				}
				return utxos, lockedUTXOsState
			},
			expectedError: errors.New("utxo consumed for locking are already locked"),
		},
		{
			name:            "6. Fee Burning Deposited UTXO",
			inputIndexes:    []uint32{0, 1},
			unlockedAmt:     defAmt - defaultTxFee,
			lockedAmt:       defAmt,
			validatorWeight: defAmt,
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{10, 10},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt / 2,
							OutputOwners: outputOwners,
						},
					},
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{11, 11},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt / 2,
							OutputOwners: outputOwners,
						},
					},
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{
					bonds: map[ids.ID]ids.Set{},
					deposits: map[ids.ID]ids.Set{
						utxos[0].TxID: map[ids.ID]struct{}{utxos[0].InputID(): {}},
					},
					lockedUTXOs: map[ids.ID]utxoLockState{
						utxos[0].InputID(): {DepositTxID: &utxos[0].TxID},
					},
				}
				return utxos, lockedUTXOsState
			},
			expectedError: errors.New("produces more tokens than it consumes"),
		},
		{
			name:            "7. Produced Input is Bad",
			inputIndexes:    []uint32{0, 0},
			unlockedAmt:     defAmt/2 - defaultTxFee,
			lockedAmt:       defAmt / 2,
			validatorWeight: defAmt / 2,
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{12, 12},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt,
							OutputOwners: outputOwners,
						},
					},
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{13, 13},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          defAmt / 4,
							OutputOwners: outputOwners,
						},
					},
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{
					bonds:       map[ids.ID]ids.Set{},
					deposits:    map[ids.ID]ids.Set{},
					lockedUTXOs: map[ids.ID]utxoLockState{},
				}
				return utxos, lockedUTXOsState
			},
			expectedError: errors.New("utxo amount and input amount should be same"),
		},
	}

	inSigners := make([]*crypto.PrivateKeySECP256K1R, 0, outputOwners.Threshold)
	inSigners = append(inSigners, basic_key)

	generateInsAndOuts := func(l_utxos []avax.UTXO, tt testCases, unlockedAmt uint64, lockedAmt uint64) ([]*avax.TransferableInput, []*avax.TransferableOutput, []*avax.TransferableOutput) {
		ins := []*avax.TransferableInput{}

		unlockedOuts := []*avax.TransferableOutput{}

		lockedOuts := []*avax.TransferableOutput{}

		signers = [][]*crypto.PrivateKeySECP256K1R{}

		for _, l_utxo := range l_utxos {
			innerOut, _ := l_utxo.Out.(*secp256k1fx.TransferOutput)

			ins = append(ins, &avax.TransferableInput{
				UTXOID: l_utxo.UTXOID,
				Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: innerOut.Amt,
					Input: secp256k1fx.Input{
						SigIndices: []uint32{0},
					},
				},
			})

			signers = append(signers, inSigners)
		}
		unlockedOuts = append(unlockedOuts, &avax.TransferableOutput{
			Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
			Out: &secp256k1fx.TransferOutput{
				Amt:          unlockedAmt,
				OutputOwners: outputOwners,
			},
		})

		lockedOuts = append(lockedOuts, &avax.TransferableOutput{
			Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
			Out: &secp256k1fx.TransferOutput{
				Amt:          lockedAmt,
				OutputOwners: outputOwners,
			},
		})

		return ins, unlockedOuts, lockedOuts
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utxos, lockedUTXOsState := tt.generateState(outputOwners)
			for _, l_utxo := range utxos {
				utxo = l_utxo
				vm.internalState.AddUTXO(&utxo)
			}
			lockedUTXOsState.Apply(vm.internalState)
			err := vm.internalState.Commit()

			assert.NoError(t, err)

			ins, unlockedOuts, lockedOuts := generateInsAndOuts(utxos, tt, tt.unlockedAmt, tt.lockedAmt)

			utx := &UnsignedAddValidatorTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    vm.ctx.NetworkID,
					BlockchainID: vm.ctx.ChainID,
					Ins:          ins, // utxo
					Outs:         unlockedOuts,
				}},
				Validator: Validator{
					NodeID: nodeID,
					Start:  uint64(startTime.Unix()),
					End:    uint64(startTime.Add(defaultMinStakingDuration).Unix()),
					Wght:   tt.validatorWeight,
				},
				Bond:         lockedOuts,
				InputIndexes: tt.inputIndexes,
				RewardsOwner: &secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{address},
				},
				Shares: reward.PercentDenominator,
			}
			tx := &Tx{UnsignedTx: utx}

			if err := tx.Sign(Codec, signers); err != nil {
				t.Fatal(err)
			}

			_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)

			if err != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
			} else {
				assert.NoError(t, err)
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

	fmt.Println("nodeID : ", nodeID)
	fmt.Println("ids.ShortEmpty : ", ids.ShortEmpty)
	// Case: Validator's weight is less than the minimum amount
	tx, err := vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr
	)

	tx.UnsignedTx.(*UnsignedAddValidatorTx).Validator.Wght = 0
	if err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because validator's weight is less than the minimum amount")
	}

	// Case: Validator's weight is more than the maximum amount
	tx, err = vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr
	)

	tx.UnsignedTx.(*UnsignedAddValidatorTx).Validator.Wght = vm.MaxValidatorStake + 1
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
		vm.MinValidatorStake,     // stake amount
		uint64(startTime.Unix()), // start time
		uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,                     // node ID
		key2.PublicKey().Address(), // reward address
		reward.PercentDenominator,  // shares
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr // key
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
		vm.MinValidatorStake,     // stake amount
		uint64(startTime.Unix()), // start time
		uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,                     // node ID
		key2.PublicKey().Address(), // reward address
		reward.PercentDenominator,  // shares
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr // key
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

	key, err := vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	nodeID := key.PublicKey().Address()

	// Case: tx is nil
	var unsignedTx *UnsignedAddValidatorTx
	if err := unsignedTx.SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because tx is nil")
	}

	// Case 3: Wrong Network ID
	tx, err := vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr

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
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr

	)
	if err != nil {
		t.Fatal(err)
	}
	tx.UnsignedTx.(*UnsignedAddValidatorTx).Bond = []*avax.TransferableOutput{{
		Asset: avax.Asset{ID: avaxAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt: vm.MinValidatorStake,
			OutputOwners: secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     nil,
			},
		},
	}}
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because stake owner has no addresses")
	}

	// Case: Rewards owner has no addresses
	tx, err = vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr

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

	// Case: Too many shares
	tx, err = vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr

	)
	if err != nil {
		t.Fatal(err)
	}
	tx.UnsignedTx.(*UnsignedAddValidatorTx).Shares++ // 1 more than max amount
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because of too many shares")
	}

	// Case: Valid
	if tx, err := vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr

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

	key, err := vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	nodeID := key.PublicKey().Address()

	// Case: Validator's start time too early
	if tx, err := vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix())-1,
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because start time too early")
	}

	// Case: Validator's start time too far in the future
	if tx, err := vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Add(maxFutureStartTime).Unix()+1),
		uint64(defaultValidateStartTime.Add(maxFutureStartTime).Add(defaultMinStakingDuration).Unix()+1),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because start time too far in the future")
	}

	// Case: Validator already validating primary network
	if tx, err := vm.newAddValidatorTx(
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID, // node ID
		nodeID, // reward address
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because validator already validating")
	}

	// Case: Validator in pending validator set of primary network
	key2, err := vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	startTime := defaultGenesisTime.Add(1 * time.Second)
	tx, err := vm.newAddValidatorTx(
		vm.MinValidatorStake,     // stake amount
		uint64(startTime.Unix()), // start time
		uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,                     // node ID
		key2.PublicKey().Address(), // reward address
		reward.PercentDenominator,  // shares
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr // key
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
		vm.MinValidatorStake,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		reward.PercentDenominator,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		ids.ShortEmpty, // change addr
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
}
