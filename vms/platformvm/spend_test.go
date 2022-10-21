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

	// Note that setting [chainTimestamp] also set's the VM's clock.
	// Adjust input/output locktimes accordingly.
	tests := []struct {
		description string
		utxos       []*avax.UTXO
		ins         []*avax.TransferableInput
		outs        []*avax.TransferableOutput
		creds       []verify.Verifiable
		fee         uint64
		assetID     ids.ID
		wantErr     error
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
						LockState: LockStateBonded,
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
						LockState: LockStateBonded,
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
						LockState: LockStateDeposited,
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
				test.creds,
				test.fee,
				test.assetID,
			)
			assert.ErrorIs(err, test.wantErr)
		})
	}
}

func TestSyntacticVerifyLock(t *testing.T) {
	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{keys[0].PublicKey().Address()},
	}
	type args struct {
		inputs       []*avax.TransferableInput
		inputIndexes []uint32
		outputs      []*avax.TransferableOutput
		lockState    LockState
		lock         bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "Happy path bond",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateUnlocked, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateBonded, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateBonded,
				lock:         true,
			},
		},
		{
			name: "Happy path deposit",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateUnlocked, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateDeposited, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateDeposited,
				lock:         true,
			},
		},
		{
			name: "Happy path bonding deposited amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateDeposited, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateDepositedBonded, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateBonded,
				lock:         true,
			},
		},
		{
			name: "Happy path depositing bonded amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateBonded, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateDepositedBonded, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateDeposited,
				lock:         true,
			},
		},
		{
			name: "Happy path unbonding bonded amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateBonded, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateUnlocked, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateBonded,
			},
		},
		{
			name: "Happy path undepositing deposited amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateDeposited, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateUnlocked, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateDeposited,
			},
		},
		{
			name: "Bonding bonded amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateBonded, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateBonded, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateBonded,
				lock:         true,
			},
			wantErr: errWrongLockState,
		},
		{
			name: "Depositing deposited amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateDeposited, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateDeposited, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateDeposited,
				lock:         true,
			},
			wantErr: errWrongLockState,
		},
		{
			name: "Undepositing bonded amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateBonded, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateUnlocked, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateDeposited,
			},
			wantErr: errWrongLockState,
		},
		{
			name: "Unbonding deposited amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateDeposited, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateUnlocked, 1, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateBonded,
			},
			wantErr: errWrongLockState,
		},
		{
			name: "Wrong amount",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateUnlocked, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateBonded, 2, outputOwners),
				},
				inputIndexes: []uint32{0},
				lockState:    LockStateBonded,
				lock:         true,
			},
			wantErr: errWrongProducedAmount,
		},
		{
			name: "Wrong input indexes",
			args: args{
				inputs: []*avax.TransferableInput{
					generateTestIn(avaxAssetID, LockStateUnlocked, 1),
				},
				outputs: []*avax.TransferableOutput{
					generateTestOut(avaxAssetID, LockStateBonded, 1, outputOwners),
				},
				lockState: LockStateBonded,
				lock:      true,
			},
			wantErr: errWrongInputIndexesLen,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := syntacticVerifyLock(tt.args.inputs, tt.args.inputIndexes, tt.args.outputs, tt.args.lockState, tt.args.lock)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

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

	type args struct {
		totalAmountToSpend uint64
		totalAmountToBurn  uint64
		appliedLockState   LockState
	}
	type want struct {
		ins          []*avax.TransferableInput
		outs         []*avax.TransferableOutput
		inputIndexes []uint32
		signers      [][]*crypto.PrivateKeySECP256K1R
		err          bool
	}
	tests := []struct {
		name          string
		args          args
		generateState func(secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl)
		generateWant  func([]avax.UTXO, secp256k1fx.OutputOwners) want
		msg           string
	}{
		{
			name: "Happy path bonding",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
					*generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 10, outputOwners, LockStateUnlocked),
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{}
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(avaxAssetID, &utxos[0], []uint32{0}),
						generateTestInFromUTXO(avaxAssetID, &utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, LockStateUnlocked, 5, outputOwners),
						generateTestOut(avaxAssetID, LockStateBonded, 4, outputOwners),
						generateTestOut(avaxAssetID, LockStateBonded, 5, outputOwners),
					},
					inputIndexes: []uint32{1, 0, 1},
				}
			},
			msg: "Happy path bonding",
		},
		{
			// In this test, spend function consumes the UTXOs with the given order,
			// but the outputs should be sorted with ascending order (based on amount)
			name: "Happy path bonding (different output order)",
			args: args{
				totalAmountToSpend: 10,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 10, outputOwners, LockStateUnlocked),
					*generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{}
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(avaxAssetID, &utxos[0], []uint32{0}),
						generateTestInFromUTXO(avaxAssetID, &utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, LockStateUnlocked, 4, outputOwners),
						generateTestOut(avaxAssetID, LockStateBonded, 1, outputOwners),
						generateTestOut(avaxAssetID, LockStateBonded, 9, outputOwners),
					},
					inputIndexes: []uint32{1, 1, 0},
				}
			},
			msg: "Happy path bonding (different output order)",
		},
		{
			name: "Happy path bonding deposited amount",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
					*generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 10, outputOwners, LockStateDeposited),
				}
				lockedUTXOsState := generateTestLockedState([]map[ids.ID]utxoLockState{{
					utxos[1].InputID(): {DepositTxID: &utxos[1].TxID},
				}}, false)
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(avaxAssetID, &utxos[0], []uint32{0}),
						generateTestInFromUTXO(avaxAssetID, &utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, LockStateUnlocked, 4, outputOwners),
						generateTestOut(avaxAssetID, LockStateDeposited, 1, outputOwners),
						generateTestOut(avaxAssetID, LockStateDepositedBonded, 9, outputOwners),
					},
					inputIndexes: []uint32{0, 1, 1},
				}
			},
			msg: "Happy path bonding deposited amount",
		},
		{
			// In this test, spend function consumes the UTXOs with the given order,
			// but the inputs should be sorted with ascending order (based on UTXO's TxID)
			name: "Happy path bonding deposited amount (different input order)",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 10, outputOwners, LockStateDeposited),
				}
				lockedUTXOsState := generateTestLockedState([]map[ids.ID]utxoLockState{{
					utxos[1].InputID(): {DepositTxID: &utxos[1].TxID},
				}}, false)
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(avaxAssetID, &utxos[1], []uint32{0}),
						generateTestInFromUTXO(avaxAssetID, &utxos[0], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, LockStateUnlocked, 4, outputOwners),
						generateTestOut(avaxAssetID, LockStateDeposited, 1, outputOwners),
						generateTestOut(avaxAssetID, LockStateDepositedBonded, 9, outputOwners),
					},
					inputIndexes: []uint32{1, 0, 0},
				}
			},
			msg: "Happy path bonding deposited amount (different input order)",
		},
		{
			name: "Bonding already bonded amount",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 10, outputOwners, LockStateBonded),
				}
				lockedUTXOsState := generateTestLockedState([]map[ids.ID]utxoLockState{{
					utxos[0].InputID(): {BondTxID: &utxos[0].TxID},
				}}, false)
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					err: true,
				}
			},
			msg: "Bonding already bonded amount",
		},
		{
			name: "Not enough balance to bond",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateBonded,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{}
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					err: true,
				}
			},
			msg: "Not enough balance to bond",
		},
		{
			name: "Happy path depositing",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateDeposited,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
					*generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 10, outputOwners, LockStateUnlocked),
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{}
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(avaxAssetID, &utxos[0], []uint32{0}),
						generateTestInFromUTXO(avaxAssetID, &utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, LockStateUnlocked, 5, outputOwners),
						generateTestOut(avaxAssetID, LockStateDeposited, 4, outputOwners),
						generateTestOut(avaxAssetID, LockStateDeposited, 5, outputOwners),
					},
					inputIndexes: []uint32{1, 0, 1},
				}
			},
			msg: "Happy path depositing",
		},
		{
			name: "Happy path depositing bonded amount",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateDeposited,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
					*generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 10, outputOwners, LockStateBonded),
				}
				lockedUTXOsState := generateTestLockedState([]map[ids.ID]utxoLockState{{
					utxos[1].InputID(): {BondTxID: &utxos[1].TxID},
				}}, false)
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(avaxAssetID, &utxos[0], []uint32{0}),
						generateTestInFromUTXO(avaxAssetID, &utxos[1], []uint32{0}),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, LockStateUnlocked, 4, outputOwners),
						generateTestOut(avaxAssetID, LockStateBonded, 1, outputOwners),
						generateTestOut(avaxAssetID, LockStateDepositedBonded, 9, outputOwners),
					},
					inputIndexes: []uint32{0, 1, 1},
				}
			},
			msg: "Happy path depositing bonded amount",
		},
		{
			name: "Depositing already deposited amount",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateDeposited,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 1, outputOwners, LockStateDeposited),
				}
				lockedUTXOsState := generateTestLockedState([]map[ids.ID]utxoLockState{{
					utxos[0].InputID(): {DepositTxID: &utxos[0].TxID},
				}}, false)
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					err: true,
				}
			},
			msg: "Depositing already deposited amount",
		},
		{
			name: "Not enough balance to deposit",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				appliedLockState:   LockStateDeposited,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					*generateTestUTXO(ids.ID{8, 8}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
				}
				lockedUTXOsState := &lockedUTXOsChainStateImpl{}
				return utxos, lockedUTXOsState
			},
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					err: true,
				}
			},
			msg: "Not enough balance to depoist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			internalState := NewMockInternalState(ctrl)
			key, _ := vm.factory.NewPrivateKey()
			address := key.PublicKey().Address()
			outputOwners := secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     []ids.ShortID{address},
			}
			UTXOIDs := []ids.ID{}
			utxos, lockedUTXOsState := tt.generateState(outputOwners)
			want := tt.generateWant(utxos, outputOwners)
			if !want.err {
				for range want.ins {
					want.signers = append(want.signers, []*crypto.PrivateKeySECP256K1R{key.(*crypto.PrivateKeySECP256K1R)})
				}
			}
			for _, utxo := range utxos {
				utxo := utxo
				vm.internalState.AddUTXO(&utxo)
				UTXOIDs = append(UTXOIDs, utxo.InputID())
				internalState.EXPECT().GetUTXO(utxo.InputID()).Return(vm.internalState.GetUTXO(utxo.InputID()))
			}
			internalState.EXPECT().UTXOIDs(address.Bytes(), ids.Empty, math.MaxInt).Return(UTXOIDs, nil)
			vm.internalState.SetLockedUTXOsChainState(lockedUTXOsState)
			err := vm.internalState.Commit()
			assert.NoError(t, err)
			// keep the original internalState to a variable in order to assign it back and shut it down in defer func
			oldInternalState := vm.internalState
			// set the mocked internalState as the vm's internalState
			vm.internalState = internalState
			ins, outs, inputIndexes, signers, err := vm.spend(
				[]*crypto.PrivateKeySECP256K1R{key.(*crypto.PrivateKeySECP256K1R)},
				tt.args.totalAmountToSpend,
				tt.args.totalAmountToBurn,
				tt.args.appliedLockState)
			assert.Equal(t, want.ins, ins)
			assert.Equal(t, want.outs, outs)
			assert.Equal(t, want.inputIndexes, inputIndexes)
			assert.Equal(t, want.signers, signers)
			assert.Equal(t, want.err, err != nil, tt.msg)
			vm.internalState = oldInternalState
		})
	}
}

func TestUnlockUTXOs(t *testing.T) {
	type want struct {
		ins     []*avax.TransferableInput
		outs    []*avax.TransferableOutput
		indexes []uint32
	}
	tests := []struct {
		name          string
		lockState     LockState
		generateUTXOs func(secp256k1fx.OutputOwners) []*avax.UTXO
		generateWant  func([]*avax.UTXO, secp256k1fx.OutputOwners) want
		wantErr       error
	}{
		{
			name:      "Unbond bonded UTXOs",
			lockState: LockStateBonded,
			generateUTXOs: func(outputOwners secp256k1fx.OutputOwners) []*avax.UTXO {
				return []*avax.UTXO{
					generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, LockStateBonded),
				}
			},
			generateWant: func(utxos []*avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(avaxAssetID, utxos[0], nil),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, LockStateUnlocked, 5, outputOwners),
					},
					indexes: []uint32{0},
				}
			},
		},
		{
			name:      "Undeposit deposited UTXOs",
			lockState: LockStateDeposited,
			generateUTXOs: func(outputOwners secp256k1fx.OutputOwners) []*avax.UTXO {
				return []*avax.UTXO{
					generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, LockStateDeposited),
				}
			},
			generateWant: func(utxos []*avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					ins: []*avax.TransferableInput{
						generateTestInFromUTXO(avaxAssetID, utxos[0], nil),
					},
					outs: []*avax.TransferableOutput{
						generateTestOut(avaxAssetID, LockStateUnlocked, 5, outputOwners),
					},
					indexes: []uint32{0},
				}
			},
		},
		{
			name:      "Unbond deposited UTXOs",
			lockState: LockStateBonded,
			generateUTXOs: func(outputOwners secp256k1fx.OutputOwners) []*avax.UTXO {
				return []*avax.UTXO{
					generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, LockStateDeposited),
				}
			},
			generateWant: func(utxos []*avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					ins:     []*avax.TransferableInput{},
					outs:    []*avax.TransferableOutput{},
					indexes: []uint32{},
				}
			},
		},
		{
			name:      "Undeposit bonded UTXOs",
			lockState: LockStateDeposited,
			generateUTXOs: func(outputOwners secp256k1fx.OutputOwners) []*avax.UTXO {
				return []*avax.UTXO{
					generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, LockStateBonded),
				}
			},
			generateWant: func(utxos []*avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					ins:     []*avax.TransferableInput{},
					outs:    []*avax.TransferableOutput{},
					indexes: []uint32{},
				}
			},
		},
		{
			name:      "Unlock unlocked UTXOs",
			lockState: LockStateBonded,
			generateUTXOs: func(outputOwners secp256k1fx.OutputOwners) []*avax.UTXO {
				return []*avax.UTXO{
					generateTestUTXO(ids.ID{9, 9}, avaxAssetID, 5, outputOwners, LockStateUnlocked),
				}
			},
			generateWant: func(utxos []*avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{
					ins:     []*avax.TransferableInput{},
					outs:    []*avax.TransferableOutput{},
					indexes: []uint32{},
				}
			},
		},
		{
			name:      "Wrong state, lockStateUnlocked",
			lockState: LockStateUnlocked,
			generateUTXOs: func(outputOwners secp256k1fx.OutputOwners) []*avax.UTXO {
				return []*avax.UTXO{}
			},
			generateWant: func(utxos []*avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{}
			},
			wantErr: errInvalidTargetLockState,
		},
		{
			name:      "Wrong state, LockStateDepositedBonded",
			lockState: LockStateDepositedBonded,
			generateUTXOs: func(outputOwners secp256k1fx.OutputOwners) []*avax.UTXO {
				return []*avax.UTXO{}
			},
			generateWant: func(utxos []*avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				return want{}
			},
			wantErr: errInvalidTargetLockState,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm, _, _ := defaultVM()
			vm.ctx.Lock.Lock()
			defer func() {
				if err := vm.Shutdown(); err != nil {
					t.Fatal(err)
				}
				vm.ctx.Lock.Unlock()
			}()
			assert := assert.New(t)

			key, err := vm.factory.NewPrivateKey()
			assert.NoError(err)
			address := key.PublicKey().Address()
			outputOwners := secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     []ids.ShortID{address},
			}
			utxos := tt.generateUTXOs(outputOwners)

			ins, outs, indexes, err := vm.unlockUTXOs(utxos, tt.lockState)
			expected := tt.generateWant(utxos, outputOwners)

			assert.Equal(expected.ins, ins)
			assert.Equal(expected.outs, outs)
			assert.Equal(expected.indexes, indexes)
			assert.ErrorIs(tt.wantErr, err)
		})
	}
}
