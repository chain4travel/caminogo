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

	"github.com/golang/mock/gomock"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
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
		shouldErr   bool
	}{
		{
			description: "no inputs, no outputs, no fee",
			utxos:       []*avax.UTXO{},
			ins:         []*avax.TransferableInput{},
			outs:        []*avax.TransferableOutput{},
			creds:       []verify.Verifiable{},
			fee:         0,
			assetID:     vm.ctx.AVAXAssetID,
			shouldErr:   false,
		},
		{
			description: "no inputs, no outputs, positive fee",
			utxos:       []*avax.UTXO{},
			ins:         []*avax.TransferableInput{},
			outs:        []*avax.TransferableOutput{},
			creds:       []verify.Verifiable{},
			fee:         1,
			assetID:     vm.ctx.AVAXAssetID,
			shouldErr:   true,
		},
		{
			description: "no inputs, no outputs, positive fee",
			utxos:       []*avax.UTXO{},
			ins:         []*avax.TransferableInput{},
			outs:        []*avax.TransferableOutput{},
			creds:       []verify.Verifiable{},
			fee:         1,
			assetID:     vm.ctx.AVAXAssetID,
			shouldErr:   true,
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
			fee:       1,
			assetID:   vm.ctx.AVAXAssetID,
			shouldErr: false,
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
			outs:      []*avax.TransferableOutput{},
			creds:     []verify.Verifiable{},
			fee:       1,
			assetID:   vm.ctx.AVAXAssetID,
			shouldErr: true,
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
			fee:       1,
			assetID:   vm.ctx.AVAXAssetID,
			shouldErr: true,
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
			fee:       1,
			assetID:   vm.ctx.AVAXAssetID,
			shouldErr: true,
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
			fee:       1,
			assetID:   vm.ctx.AVAXAssetID,
			shouldErr: false,
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
			fee:       1,
			assetID:   vm.ctx.AVAXAssetID,
			shouldErr: false,
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
			fee:       0,
			assetID:   vm.ctx.AVAXAssetID,
			shouldErr: false,
		},
	}

	for _, test := range tests {
		vm.clock.Set(now)

		t.Run(test.description, func(t *testing.T) {
			err := vm.semanticVerifySpendUTXOs(
				&unsignedTx,
				test.utxos,
				test.ins,
				test.outs,
				test.creds,
				test.fee,
				test.assetID,
			)

			if err == nil && test.shouldErr {
				t.Fatalf("expected error but got none")
			} else if err != nil && !test.shouldErr {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}

func TestSyntacticVerifyInputIndexes(t *testing.T) {
	type args struct {
		inputs       []*avax.TransferableInput
		inputIndexes []uint32
		outputs      []*avax.TransferableOutput
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		msg     string
	}{
		{
			name: "One input one output",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				inputIndexes: []uint32{0},
				outputs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
				},
			},
			wantErr: false,
			msg:     "One input one output",
		},
		{
			name: "One input two outputs",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 2,
						},
					},
				},
				inputIndexes: []uint32{0, 0},
				outputs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
				},
			},
			wantErr: false,
			msg:     "One input two outputs",
		},
		{
			name: "Two inputs one output",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
					{
						Asset: avax.Asset{ID: avaxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				inputIndexes: []uint32{0},
				outputs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
				},
			},
			wantErr: false,
			msg:     "Two inputs one output",
		},
		{
			name: "Two inputs two outputs",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
					{
						Asset: avax.Asset{ID: avaxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				inputIndexes: []uint32{0, 1},
				outputs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
				},
			},
			wantErr: false,
			msg:     "Two inputs two outputs",
		},
		{
			name: "Wrong assetId",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: ids.ID{}},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				inputIndexes: []uint32{0, 0},
				outputs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 100,
						},
					},
				},
			},
			wantErr: true,
			msg:     "Should have failed because of input and output assetId mismatch",
		},
		{
			name: "Input amount and consumed amount mismatch",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				inputIndexes: []uint32{0, 0},
				outputs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 100,
						},
					},
				},
			},
			wantErr: true,
			msg:     "Should have failed because of input amount and consumed amount mismatch",
		},
		{
			name: "Amount overflow",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				inputIndexes: []uint32{0, 0},
				outputs: []*avax.TransferableOutput{
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: math.MaxUint64,
						},
					},
					{
						Asset: avax.Asset{ID: avaxAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt: 1,
						},
					},
				},
			},
			wantErr: true,
			msg:     "Should have failed because of output amount overflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := syntacticVerifyInputIndexes(tt.args.inputs, tt.args.inputIndexes, tt.args.outputs)
			assert.Equal(t, tt.wantErr, err != nil, tt.msg)
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
		spendMode          spendMode
	}
	type want struct {
		ins          []*avax.TransferableInput
		unlockedOuts []*avax.TransferableOutput
		lockedOuts   []*avax.TransferableOutput
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
				spendMode:          spendModeBond,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          5,
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
							Amt:          10,
							OutputOwners: outputOwners,
						},
					},
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
						{
							UTXOID: utxos[0].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   5,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
						{
							UTXOID: utxos[1].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   10,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
					},
					unlockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          5,
								OutputOwners: outputOwners,
							},
						},
					},
					lockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          4,
								OutputOwners: outputOwners,
							},
						},
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          5,
								OutputOwners: outputOwners,
							},
						},
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
				spendMode:          spendModeBond,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          10,
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
							Amt:          5,
							OutputOwners: outputOwners,
						},
					},
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
						{
							UTXOID: utxos[0].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   10,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
						{
							UTXOID: utxos[1].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   5,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
					},
					unlockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          4,
								OutputOwners: outputOwners,
							},
						},
					},
					lockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          1,
								OutputOwners: outputOwners,
							},
						},
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          9,
								OutputOwners: outputOwners,
							},
						},
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
				spendMode:          spendModeBond,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          5,
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
							Amt:          10,
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
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						{
							UTXOID: utxos[0].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   5,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
						{
							UTXOID: utxos[1].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   10,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
					},
					unlockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          1,
								OutputOwners: outputOwners,
							},
						},
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          4,
								OutputOwners: outputOwners,
							},
						},
					},
					lockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          9,
								OutputOwners: outputOwners,
							},
						},
					},
					inputIndexes: []uint32{1, 0, 1},
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
				spendMode:          spendModeBond,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{9, 9},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          5,
							OutputOwners: outputOwners,
						},
					},
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          10,
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
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						{
							UTXOID: utxos[1].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   10,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
						{
							UTXOID: utxos[0].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   5,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
					},
					unlockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          1,
								OutputOwners: outputOwners,
							},
						},
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          4,
								OutputOwners: outputOwners,
							},
						},
					},
					lockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          9,
								OutputOwners: outputOwners,
							},
						},
					},
					inputIndexes: []uint32{0, 1, 0},
				}
			},
			msg: "Happy path bonding deposited amount (different input order)",
		},
		{
			name: "Bonding already bonded amount",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				spendMode:          spendModeBond,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          10,
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
				spendMode:          spendModeBond,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          5,
							OutputOwners: outputOwners,
						},
					},
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
				spendMode:          spendModeDeposit,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          5,
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
							Amt:          10,
							OutputOwners: outputOwners,
						},
					},
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
						{
							UTXOID: utxos[0].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   5,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
						{
							UTXOID: utxos[1].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   10,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
					},
					unlockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          5,
								OutputOwners: outputOwners,
							},
						},
					},
					lockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          4,
								OutputOwners: outputOwners,
							},
						},
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          5,
								OutputOwners: outputOwners,
							},
						},
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
				spendMode:          spendModeDeposit,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          5,
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
							Amt:          10,
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
			generateWant: func(utxos []avax.UTXO, outputOwners secp256k1fx.OutputOwners) want {
				for index := range utxos {
					utxos[index].InputID()
				}
				return want{
					ins: []*avax.TransferableInput{
						{
							UTXOID: utxos[0].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   5,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
						{
							UTXOID: utxos[1].UTXOID,
							Asset:  avax.Asset{ID: vm.ctx.AVAXAssetID},
							In: &secp256k1fx.TransferInput{
								Amt:   10,
								Input: secp256k1fx.Input{SigIndices: []uint32{0}},
							},
						},
					},
					unlockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          1,
								OutputOwners: outputOwners,
							},
						},
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          4,
								OutputOwners: outputOwners,
							},
						},
					},
					lockedOuts: []*avax.TransferableOutput{
						{
							Asset: avax.Asset{ID: avaxAssetID},
							Out: &secp256k1fx.TransferOutput{
								Amt:          9,
								OutputOwners: outputOwners,
							},
						},
					},
					inputIndexes: []uint32{1, 0, 1},
				}
			},
			msg: "Happy path depositing bonded amount",
		},
		{
			name: "Depositing already deposited amount",
			args: args{
				totalAmountToSpend: 9,
				totalAmountToBurn:  1,
				spendMode:          spendModeDeposit,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          10,
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
				spendMode:          spendModeDeposit,
			},
			generateState: func(outputOwners secp256k1fx.OutputOwners) ([]avax.UTXO, *lockedUTXOsChainStateImpl) {
				utxos := []avax.UTXO{
					{
						UTXOID: avax.UTXOID{
							TxID:        ids.ID{8, 8},
							OutputIndex: 0,
						},
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						Out: &secp256k1fx.TransferOutput{
							Amt:          5,
							OutputOwners: outputOwners,
						},
					},
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
			// Set the mocked internalState to return the lockedUTXOState from the real internalState
			internalState.EXPECT().LockedUTXOsChainState().Return(vm.internalState.LockedUTXOsChainState())
			// keep the original internalState to a variable in order to assign it back and shut it down in defer func
			oldInternalState := vm.internalState
			// set the mocked internalState as the vm's internalState
			vm.internalState = internalState
			ins, unlockedOuts, lockedOuts, inputIndexes, signers, err := vm.spend(
				[]*crypto.PrivateKeySECP256K1R{key.(*crypto.PrivateKeySECP256K1R)},
				tt.args.totalAmountToSpend,
				tt.args.totalAmountToBurn,
				address,
				tt.args.spendMode)
			assert.Equal(t, want.ins, ins)
			assert.Equal(t, want.unlockedOuts, unlockedOuts)
			assert.Equal(t, want.lockedOuts, lockedOuts)
			assert.Equal(t, want.inputIndexes, inputIndexes)
			assert.Equal(t, want.signers, signers)
			assert.Equal(t, want.err, err != nil, tt.msg)
			vm.internalState = oldInternalState
		})
	}
}
