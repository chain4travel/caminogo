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
	"math"
	"testing"
	"time"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type dummyUnsignedTx struct {
	BaseTx
}

func (du *dummyUnsignedTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	return nil
}

func TestSemanticVerifySpendUTXOs(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	// The VM time during a test, unless [chainTimestamp] is set
	now := time.Unix(1607133207, 0)

	unsignedTx := dummyUnsignedTx{
		BaseTx: BaseTx{},
	}
	unsignedTx.Initialize([]byte{0}, []byte{1})

	existingTxID := ids.GenerateTestID()

	// Note that setting [chainTimestamp] also set's the VM's clock.
	// Adjust input/output locktimes accordingly.
	tests := []struct {
		description      string
		utxos            []*avax.UTXO
		ins              []*avax.TransferableInput
		outs             []*avax.TransferableOutput
		creds            []verify.Verifiable
		fee              uint64
		assetID          ids.ID
		appliedLockState LockState // TODO@ put into test cases
		wantErr          error
	}{
		{
			description: "no inputs, no outputs, no fee",
			utxos:       []*avax.UTXO{},
			ins:         []*avax.TransferableInput{},
			outs:        []*avax.TransferableOutput{},
			creds:       []verify.Verifiable{},
			fee:         0,
			assetID:     vm.ctx.AVAXAssetID,
		},
		{
			description: "no inputs, no outputs, positive fee",
			utxos:       []*avax.UTXO{},
			ins:         []*avax.TransferableInput{},
			outs:        []*avax.TransferableOutput{},
			creds:       []verify.Verifiable{},
			fee:         1,
			assetID:     vm.ctx.AVAXAssetID,
			wantErr:     errWrongProducedAmount,
		},
		{
			description: "one input, no outputs, positive fee",
			utxos: []*avax.UTXO{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: 1,
				},
			}},
			ins: []*avax.TransferableInput{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: 1,
				},
			}},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{},
			},
			fee:     1,
			assetID: vm.ctx.AVAXAssetID,
		},
		{
			description: "wrong number of credentials",
			utxos: []*avax.UTXO{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: 1,
				},
			}},
			ins: []*avax.TransferableInput{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: 1,
				},
			}},
			outs:    []*avax.TransferableOutput{},
			creds:   []verify.Verifiable{},
			fee:     1,
			assetID: vm.ctx.AVAXAssetID,
			wantErr: errInputsCredentialsMismatch,
		},
		{
			description: "wrong number of UTXOs",
			utxos:       []*avax.UTXO{},
			ins: []*avax.TransferableInput{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: 1,
				},
			}},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{},
			},
			fee:     1,
			assetID: vm.ctx.AVAXAssetID,
			wantErr: errInputsUTXOSMismatch,
		},
		{
			description: "invalid credential",
			utxos: []*avax.UTXO{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: 1,
				},
			}},
			ins: []*avax.TransferableInput{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: 1,
				},
			}},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				(*secp256k1fx.Credential)(nil),
			},
			fee:     1,
			assetID: vm.ctx.AVAXAssetID,
			wantErr: errWrongCredentials,
		},
		{
			description: "one input, no outputs, positive fee",
			utxos: []*avax.UTXO{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: 1,
				},
			}},
			ins: []*avax.TransferableInput{{
				Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: 1,
				},
			}},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{},
			},
			fee:     1,
			assetID: vm.ctx.AVAXAssetID,
		},
		{
			description: "one input, one output, positive fee",
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 2,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 2,
					},
				},
			},
			outs: []*avax.TransferableOutput{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{},
			},
			fee:     1,
			assetID: vm.ctx.AVAXAssetID,
		},
		{
			description: "one input, one output, zero fee",
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
					},
				},
			},
			outs: []*avax.TransferableOutput{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{},
			},
			fee:     0,
			assetID: vm.ctx.AVAXAssetID,
		},
		{
			description: "UTXO state is locked but input state is unlocked",
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					Out: &LockedOut{
						LockIDs: LockIDs{BondTxID: existingTxID},
						TransferableOut: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
					},
				},
			},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{},
			},
			fee:     0,
			assetID: vm.ctx.AVAXAssetID,
			wantErr: errLockedFundsNotMarkedAsLocked,
		},
		{
			description: "UTXO state has different locked state than the input",
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					Out: &LockedOut{
						LockIDs: LockIDs{BondTxID: existingTxID},
						TransferableOut: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
					In: &LockedIn{
						LockIDs: LockIDs{DepositTxID: existingTxID},
						TransferableIn: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
			},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{},
			},
			fee:     0,
			assetID: vm.ctx.AVAXAssetID,
			wantErr: errWrongLockState,
		},
	}

	for _, test := range tests {
		vm.clock.Set(now)

		t.Run(test.description, func(t *testing.T) {
			assert := assert.New(t)
			err := vm.semanticVerifySpendUTXOs(
				&unsignedTx,
				test.utxos,
				test.ins,
				test.outs,
				test.appliedLockState,
				test.creds,
				test.fee,
				test.assetID,
			)
			assert.ErrorIs(err, test.wantErr)
		})
	}
}

// TODO@ merge to TestSemanticVerifySpendUTXOs
// func TestSyntacticVerifyLock(t *testing.T) {
// 	outputOwners := secp256k1fx.OutputOwners{
// 		Locktime:  0,
// 		Threshold: 1,
// 		Addrs:     []ids.ShortID{keys[0].PublicKey().Address()},
// 	}
// 	type args struct {
// 		inputs       []*avax.TransferableInput
// 		inputIndexes []uint32
// 		outputs      []*avax.TransferableOutput
// 		lockState    LockState
// 		lock         bool
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		wantErr error
// 	}{
// 		{
// 			name: "Happy path bond",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, ids.Empty, ids.Empty),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, ids.Empty, someBondTxID),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateBonded,
// 				lock:         true,
// 			},
// 		},
// 		{
// 			name: "Happy path deposit",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, ids.Empty, ids.Empty),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, someDepositTxID, ids.Empty),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateDeposited,
// 				lock:         true,
// 			},
// 		},
// 		{
// 			name: "Happy path bonding deposited amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, someDepositTxID, ids.Empty),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, someDepositTxID, someBondTxID),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateBonded,
// 				lock:         true,
// 			},
// 		},
// 		{
// 			name: "Happy path depositing bonded amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, ids.Empty, someBondTxID),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, someDepositTxID, someBondTxID),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateDeposited,
// 				lock:         true,
// 			},
// 		},
// 		{
// 			name: "Happy path unbonding bonded amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, ids.Empty, someBondTxID),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, ids.Empty, ids.Empty),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateBonded,
// 			},
// 		},
// 		{
// 			name: "Happy path undepositing deposited amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, someDepositTxID, ids.Empty),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, ids.Empty, ids.Empty),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateDeposited,
// 			},
// 		},
// 		{
// 			name: "Bonding bonded amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, ids.Empty, someBondTxID),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, ids.Empty, someBondTxID),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateBonded,
// 				lock:         true,
// 			},
// 			wantErr: errWrongLockState,
// 		},
// 		{
// 			name: "Depositing deposited amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, someDepositTxID, ids.Empty),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, someDepositTxID, ids.Empty),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateDeposited,
// 				lock:         true,
// 			},
// 			wantErr: errWrongLockState,
// 		},
// 		{
// 			name: "Undepositing bonded amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, ids.Empty, someBondTxID),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, ids.Empty, ids.Empty),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateDeposited,
// 			},
// 			wantErr: errWrongLockState,
// 		},
// 		{
// 			name: "Unbonding deposited amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, someDepositTxID, ids.Empty),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 1, outputOwners, ids.Empty, ids.Empty),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateBonded,
// 			},
// 			wantErr: errWrongLockState,
// 		},
// 		{
// 			name: "Wrong amount",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, 1, ids.Empty, ids.Empty),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, 2, outputOwners, ids.Empty, someBondTxID),
// 				},
// 				inputIndexes: []uint32{0},
// 				lockState:    LockStateBonded,
// 				lock:         true,
// 			},
// 			wantErr: errWrongProducedAmount,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			err := syntacticVerifyLock(tt.args.inputs, tt.args.inputIndexes, tt.args.outputs, tt.args.lockState, tt.args.lock)
// 			assert.Equal(t, tt.wantErr, err)
// 		})
// 	}
// }

func TestSpend(t *testing.T) {
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
	assert.NoError(t, err)
	secpKey, ok := key.(*crypto.PrivateKeySECP256K1R)
	assert.True(t, ok)
	address := key.PublicKey().Address()
	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{address},
	}
	existingTxID := ids.GenerateTestID()

	type args struct {
		totalAmountToSpend uint64
		totalAmountToBurn  uint64
		appliedLockState   LockState
	}
	type want struct {
		ins  []*avax.TransferableInput
		outs []*avax.TransferableOutput
	}
	tests := map[string]struct {
		utxos        []*avax.UTXO
		args         args
		generateWant func([]*avax.UTXO) want
		expectError  bool // TODO@ specific errors ?
		msg          string
	}{
		"Happy path bonding": {
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 10, outputOwners, ids.Empty, ids.Empty),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(utxos[0], []uint32{0}),
						generateTestInFromUTXO(utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
						generateTestOut(avaxAssetID, 4, outputOwners, ids.Empty, thisTxID),
						generateTestOut(avaxAssetID, 5, outputOwners, ids.Empty, thisTxID),
					},
				}
			},
			msg: "Happy path bonding",
		},
		"Happy path bonding (different output order)": {
			// In this test, spend function consumes the UTXOs with the given order,
			// but the outputs should be sorted with ascending order (based on amount)
			args: args{
				totalAmountToSpend: 10,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 10, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(utxos[0], []uint32{0}),
						generateTestInFromUTXO(utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, 4, outputOwners, ids.Empty, ids.Empty),
						generateTestOut(avaxAssetID, 1, outputOwners, ids.Empty, thisTxID),
						generateTestOut(avaxAssetID, 9, outputOwners, ids.Empty, thisTxID),
					},
				}
			},
			msg: "Happy path bonding (different output order)",
		},
		"Happy path bonding deposited amount": {
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 10, outputOwners, existingTxID, ids.Empty),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(utxos[0], []uint32{0}),
						generateTestInFromUTXO(utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, 4, outputOwners, ids.Empty, ids.Empty),
						generateTestOut(avaxAssetID, 1, outputOwners, existingTxID, ids.Empty),
						generateTestOut(avaxAssetID, 9, outputOwners, existingTxID, thisTxID),
					},
				}
			},
			msg: "Happy path bonding deposited amount",
		},
		"Happy path bonding deposited amount (different input order)": {
			// In this test, spend function consumes the UTXOs with the given order,
			// but the inputs should be sorted with ascending order (based on UTXO's TxID)
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 10, outputOwners, existingTxID, ids.Empty),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(utxos[1], []uint32{0}),
						generateTestInFromUTXO(utxos[0], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, 4, outputOwners, ids.Empty, ids.Empty),
						generateTestOut(avaxAssetID, 1, outputOwners, existingTxID, ids.Empty),
						generateTestOut(avaxAssetID, 9, outputOwners, existingTxID, thisTxID),
					},
				}
			},
			msg: "Happy path bonding deposited amount (different input order)",
		},
		"Bonding already bonded amount": {
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 10, outputOwners, ids.Empty, existingTxID),
			},
			expectError: true,
			msg:         "Bonding already bonded amount",
		},
		"Not enough balance to bond": {
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
			},
			expectError: true,
			msg:         "Not enough balance to bond",
		},
		"Happy path depositing": {
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateDeposited,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 10, outputOwners, ids.Empty, ids.Empty),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(utxos[0], []uint32{0}),
						generateTestInFromUTXO(utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
						generateTestOut(avaxAssetID, 4, outputOwners, thisTxID, ids.Empty),
						generateTestOut(avaxAssetID, 5, outputOwners, thisTxID, ids.Empty),
					},
				}
			},
			msg: "Happy path depositing",
		},
		"Happy path depositing bonded amount": {
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateDeposited,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 10, outputOwners, ids.Empty, existingTxID),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(utxos[0], []uint32{0}),
						generateTestInFromUTXO(utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, 4, outputOwners, ids.Empty, ids.Empty),
						generateTestOut(avaxAssetID, 1, outputOwners, ids.Empty, existingTxID),
						generateTestOut(avaxAssetID, 9, outputOwners, thisTxID, existingTxID),
					},
				}
			},
			msg: "Happy path depositing bonded amount",
		},
		"Depositing already deposited amount": {
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateDeposited,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 1, outputOwners, existingTxID, ids.Empty),
			},
			expectError: true,
			msg:         "Depositing already deposited amount",
		},
		"Not enough balance to deposit": {
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateDeposited,
			},
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
			},
			expectError: true,
			msg:         "Not enough balance to depoist",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			internalState := NewMockInternalState(ctrl)
			utxoIDs := []ids.ID{}
			want := tt.generateWant(tt.utxos)
			var expectedSigners [][]*crypto.PrivateKeySECP256K1R
			if tt.expectError {
				expectedSigners = make([][]*crypto.PrivateKeySECP256K1R, len(want.ins))
				for i := range want.ins {
					expectedSigners[i] = []*crypto.PrivateKeySECP256K1R{secpKey}
				}
			}

			for _, utxo := range tt.utxos {
				utxo := utxo
				vm.internalState.AddUTXO(utxo)
				utxoIDs = append(utxoIDs, utxo.InputID())
				internalState.EXPECT().GetUTXO(utxo.InputID()).Return(vm.internalState.GetUTXO(utxo.InputID()))
			}
			internalState.EXPECT().UTXOIDs(address.Bytes(), ids.Empty, math.MaxInt).Return(utxoIDs, nil)
			err := vm.internalState.Commit()
			assert.NoError(err)

			// keep the original internalState to a variable in order to assign it back and shut it down in defer func
			oldInternalState := vm.internalState
			// set the mocked internalState as the vm's internalState
			vm.internalState = internalState

			ins, outs, signers, err := vm.spend(
				[]*crypto.PrivateKeySECP256K1R{secpKey},
				tt.args.totalAmountToSpend,
				tt.args.totalAmountToBurn,
				tt.args.appliedLockState,
			)

			assert.Equal(want.ins, ins)
			assert.Equal(want.outs, outs)
			assert.Equal(expectedSigners, signers)
			assert.Equal(tt.expectError, err != nil, tt.msg)
			vm.internalState = oldInternalState
		})
	}
}

func TestUnlockUTXOs(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	key, err := vm.factory.NewPrivateKey()
	assert.NoError(t, err)
	address := key.PublicKey().Address()
	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{address},
	}
	existingTxID := ids.GenerateTestID()

	type want struct {
		ins  []*avax.TransferableInput
		outs []*avax.TransferableOutput
	}
	tests := map[string]struct {
		name          string
		lockState     LockState
		utxos         []*avax.UTXO
		generateWant  func([]*avax.UTXO) want
		expectedError error
	}{
		"Unbond bonded UTXOs": {
			lockState: LockStateBonded,
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, ids.Empty, existingTxID),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(utxos[0], nil),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
					},
				}
			},
		},
		"Undeposit deposited UTXOs": {
			lockState: LockStateDeposited,
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, existingTxID, ids.Empty),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(utxos[0], nil),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
					},
				}
			},
		},
		"Unbond deposited UTXOs": {
			lockState: LockStateBonded,
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, existingTxID, ids.Empty),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins:  []*avax.TransferableInput{},
					outs: []*avax.TransferableOutput{},
				}
			},
		},
		"Undeposit bonded UTXOs": {
			lockState: LockStateDeposited,
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, ids.Empty, existingTxID),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins:  []*avax.TransferableInput{},
					outs: []*avax.TransferableOutput{},
				}
			},
		},
		"Unlock unlocked UTXOs": {
			lockState: LockStateBonded,
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, ids.Empty, ids.Empty),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins:  []*avax.TransferableInput{},
					outs: []*avax.TransferableOutput{},
				}
			},
		},
		"Wrong state, lockStateUnlocked": {
			lockState:     LockStateUnlocked,
			generateWant:  func(utxos []*avax.UTXO) want { return want{} },
			expectedError: errInvalidTargetLockState,
		},
		"Wrong state, LockStateDepositedBonded": {
			lockState:     LockStateDepositedBonded,
			generateWant:  func(utxos []*avax.UTXO) want { return want{} },
			expectedError: errInvalidTargetLockState,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			expected := tt.generateWant(tt.utxos)
			ins, outs, err := vm.unlockUTXOs(tt.utxos, tt.lockState)

			assert.Equal(expected.ins, ins)
			assert.Equal(expected.outs, outs)
			assert.ErrorIs(tt.expectedError, err)
		})
	}
}
