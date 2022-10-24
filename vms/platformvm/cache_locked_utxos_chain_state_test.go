// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

// import (
// 	"testing"

// 	"github.com/chain4travel/caminogo/vms/secp256k1fx"

// 	"github.com/chain4travel/caminogo/ids"
// 	"github.com/chain4travel/caminogo/vms/components/avax"
// 	"github.com/stretchr/testify/assert"
// )

// func TestUpdateUTXOs(t *testing.T) {
// 	tests := []struct {
// 		name              string
// 		lockedStateImpl   lockedUTXOsChainStateImpl
// 		updatedUTXOStates map[ids.ID]utxoLockState
// 		want              lockedUTXOsChainState
// 		wantErr           bool
// 		msg               string
// 	}{
// 		{
// 			name: "Happy path bonding",
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
// 				BondTxID: &ids.ID{9, 9},
// 			}},
// 			want: generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {BondTxID: &ids.ID{9, 9}},
// 			}}, true),
// 			msg: "Happy path bonding",
// 		},
// 		{
// 			name: "Happy path bonding deposited tokens",
// 			lockedStateImpl: *generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {DepositTxID: &ids.ID{9, 9}},
// 			}}, false),
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
// 				BondTxID: &ids.ID{8, 8},
// 			}},
// 			want: generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {BondTxID: &ids.ID{8, 8}},
// 			}}, true),
// 			msg: "Happy path bonding deposited tokens",
// 		},
// 		{
// 			name: "Happy path unbonding",
// 			lockedStateImpl: *generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {BondTxID: &ids.ID{9, 9}},
// 			}}, false),
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {}},
// 			want: &lockedUTXOsChainStateImpl{
// 				// bonds:        map[ids.ID]ids.Set{},
// 				// deposits:     map[ids.ID]ids.Set{},
// 				lockedUTXOs:  map[ids.ID]utxoLockState{},
// 				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {}},
// 			},
// 			msg: "Happy path unbonding",
// 		},
// 		{
// 			name: "BondTx exists no bond state",
// 			lockedStateImpl: lockedUTXOsChainStateImpl{
// 				// bonds:    map[ids.ID]ids.Set{},
// 				// deposits: map[ids.ID]ids.Set{},
// 				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
// 					BondTxID: &ids.ID{9, 9},
// 				}},
// 			},
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {}},
// 			wantErr:           true,
// 			msg:               "Should have failed because bonding tx exists but there is no such bond in state",
// 		},
// 		{
// 			name: "Bonding already bonded UTXO",
// 			lockedStateImpl: *generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {BondTxID: &ids.ID{9, 9}},
// 			}}, false),
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
// 				BondTxID: &ids.ID{8, 8},
// 			}},
// 			wantErr: true,
// 			msg:     "Should have failed because UTXO is already bonded",
// 		},
// 		{
// 			name: "Happy path depositing",
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
// 				DepositTxID: &ids.ID{9, 9},
// 			}},
// 			want: generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {DepositTxID: &ids.ID{9, 9}},
// 			}}, true),
// 			msg: "Happy path depositing",
// 		},
// 		{
// 			name: "Happy path depositing bonded tokens",
// 			lockedStateImpl: *generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {BondTxID: &ids.ID{9, 9}},
// 			}}, false),
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
// 				DepositTxID: &ids.ID{8, 8},
// 			}},
// 			want: generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {DepositTxID: &ids.ID{8, 8}},
// 			}}, true),
// 			msg: "Happy path depositing bonded tokens",
// 		},
// 		{
// 			name: "Happy path undepositing",
// 			lockedStateImpl: *generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {DepositTxID: &ids.ID{9, 9}},
// 			}}, false),
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {}},
// 			want: &lockedUTXOsChainStateImpl{
// 				lockedUTXOs:  map[ids.ID]utxoLockState{},
// 				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {}},
// 			},
// 			msg: "Happy path undepositing",
// 		},
// 		{
// 			name: "DepositTx exists no deposit state",
// 			lockedStateImpl: lockedUTXOsChainStateImpl{
// 				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
// 					DepositTxID: &ids.ID{9, 9},
// 				}},
// 			},
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {}},
// 			wantErr:           true,
// 			msg:               "Should have failed because depositing tx exists but there is no such deposit in state",
// 		},
// 		{
// 			name: "Depositing already deposited UTXO",
// 			lockedStateImpl: *generateTestLockedState([]map[ids.ID]utxoLockState{{
// 				{1, 1}: {DepositTxID: &ids.ID{9, 9}},
// 			}}, false),
// 			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
// 				DepositTxID: &ids.ID{8, 8},
// 			}},
// 			wantErr: true,
// 			msg:     "Should have failed because UTXO is already deposited",
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := tt.lockedStateImpl.UpdateLockState(tt.updatedUTXOStates)
// 			assert.Equal(t, tt.wantErr, err != nil, tt.msg)
// 			assert.Equalf(t, tt.want, got, tt.msg)
// 		})
// 	}
// }

// func TestProduceUTXOs(t *testing.T) {
// 	outputOwners := secp256k1fx.OutputOwners{
// 		Locktime:  0,
// 		Threshold: 1,
// 		Addrs:     []ids.ShortID{keys[0].PublicKey().Address()},
// 	}
// 	type want struct {
// 		utxoLockedState map[ids.ID]utxoLockState
// 		utxos           []*avax.UTXO
// 	}
// 	type args struct {
// 		inputs    []*avax.TransferableInput
// 		outputs   []*avax.TransferableOutput
// 		lockState LockState
// 		txID      ids.ID
// 	}
// 	tests := []struct {
// 		name                 string
// 		args                 args
// 		generateCurrentState func(ins []*avax.TransferableInput) lockedUTXOsChainStateImpl
// 		generateWant         func(ins []*avax.TransferableInput, outs []*avax.TransferableOutput, producedUTXOs []*avax.UTXO) want
// 	}{
// 		{
// 			name: "Unlocked input bonded output",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, LockStateUnlocked, 10),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, LockStateBonded, 10, outputOwners),
// 				},
// 				lockState: LockStateBonded,
// 				txID:      ids.ID{1},
// 			},
// 			generateCurrentState: func(ins []*avax.TransferableInput) lockedUTXOsChainStateImpl {
// 				return lockedUTXOsChainStateImpl{}
// 			},
// 			generateWant: func(ins []*avax.TransferableInput, outs []*avax.TransferableOutput, producedUTXOs []*avax.UTXO) want {
// 				return want{
// 					utxoLockedState: map[ids.ID]utxoLockState{
// 						ins[0].InputID():           {},                     // consumedUTXO
// 						producedUTXOs[0].InputID(): {BondTxID: &ids.ID{1}}, // producedUTXO
// 					},
// 					utxos: []*avax.UTXO{
// 						generateTestUTXO(ids.ID{1}, avaxAssetID, 10, outputOwners, LockStateBonded),
// 					},
// 				}
// 			},
// 		},
// 		{
// 			name: "Unlocked input deposited output",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, LockStateUnlocked, 10),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, LockStateDeposited, 10, outputOwners),
// 				},
// 				lockState: LockStateDeposited,
// 				txID:      ids.ID{1},
// 			},
// 			generateCurrentState: func(ins []*avax.TransferableInput) lockedUTXOsChainStateImpl {
// 				return lockedUTXOsChainStateImpl{}
// 			},
// 			generateWant: func(ins []*avax.TransferableInput, outs []*avax.TransferableOutput, producedUTXOs []*avax.UTXO) want {
// 				return want{
// 					utxoLockedState: map[ids.ID]utxoLockState{
// 						ins[0].InputID():           {},                        // consumedUTXO
// 						producedUTXOs[0].InputID(): {DepositTxID: &ids.ID{1}}, // producedUTXO
// 					},
// 					utxos: []*avax.UTXO{
// 						generateTestUTXO(ids.ID{1}, avaxAssetID, 10, outputOwners, LockStateDeposited),
// 					},
// 				}
// 			},
// 		},
// 		{
// 			name: "Bonded input deposited and bonded output",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, LockStateBonded, 10),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, LockStateDepositedBonded, 7, outputOwners),
// 					generateTestOut(avaxAssetID, LockStateBonded, 3, outputOwners),
// 				},
// 				lockState: LockStateDeposited,
// 				txID:      ids.ID{1},
// 			},
// 			generateCurrentState: func(ins []*avax.TransferableInput) lockedUTXOsChainStateImpl {
// 				return *generateTestLockedState([]map[ids.ID]utxoLockState{{ins[0].InputID(): {BondTxID: &ids.ID{0}}}}, false)
// 			},
// 			generateWant: func(ins []*avax.TransferableInput, outs []*avax.TransferableOutput, producedUTXOs []*avax.UTXO) want {
// 				return want{
// 					utxoLockedState: map[ids.ID]utxoLockState{
// 						producedUTXOs[0].InputID(): {BondTxID: &ids.ID{0}, DepositTxID: &ids.ID{1}}, // producedUTXO
// 						ins[0].InputID():           {},                                              // consumedUTXO
// 						producedUTXOs[1].InputID(): {BondTxID: &ids.ID{0}},                          // producedUTXO
// 					},
// 					utxos: []*avax.UTXO{
// 						generateTestUTXO(ids.ID{1}, avaxAssetID, 7, outputOwners, LockStateDepositedBonded),
// 						generateTestUTXO(ids.ID{1}, avaxAssetID, 3, outputOwners, LockStateBonded),
// 					},
// 				}
// 			},
// 		},
// 		{
// 			name: "Deposited input deposited and bonded output",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, LockStateDeposited, 10),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, LockStateDepositedBonded, 7, outputOwners),
// 					generateTestOut(avaxAssetID, LockStateDeposited, 3, outputOwners),
// 				},
// 				lockState: LockStateBonded,
// 				txID:      ids.ID{1},
// 			},
// 			generateCurrentState: func(ins []*avax.TransferableInput) lockedUTXOsChainStateImpl {
// 				return *generateTestLockedState([]map[ids.ID]utxoLockState{{ins[0].InputID(): {DepositTxID: &ids.ID{0}}}}, false)
// 			},
// 			generateWant: func(ins []*avax.TransferableInput, outs []*avax.TransferableOutput, producedUTXOs []*avax.UTXO) want {
// 				return want{
// 					utxoLockedState: map[ids.ID]utxoLockState{
// 						producedUTXOs[0].InputID(): {DepositTxID: &ids.ID{0}, BondTxID: &ids.ID{1}}, // producedUTXO
// 						ins[0].InputID():           {},                                              // consumedUTXO
// 						producedUTXOs[1].InputID(): {DepositTxID: &ids.ID{0}},                       // producedUTXO
// 					},
// 					utxos: []*avax.UTXO{
// 						generateTestUTXO(ids.ID{1}, avaxAssetID, 7, outputOwners, LockStateDepositedBonded),
// 						generateTestUTXO(ids.ID{1}, avaxAssetID, 3, outputOwners, LockStateDeposited),
// 					},
// 				}
// 			},
// 		},
// 		{
// 			name: "Bonded input unlocked output",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, LockStateBonded, 10),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, LockStateUnlocked, 10, outputOwners),
// 				},
// 				lockState: LockStateBonded,
// 				txID:      ids.ID{1},
// 			},
// 			generateCurrentState: func(ins []*avax.TransferableInput) lockedUTXOsChainStateImpl {
// 				return *generateTestLockedState([]map[ids.ID]utxoLockState{{ins[0].InputID(): {BondTxID: &ids.ID{0}}}}, false)
// 			},
// 			generateWant: func(ins []*avax.TransferableInput, outs []*avax.TransferableOutput, producedUTXOs []*avax.UTXO) want {
// 				return want{
// 					utxoLockedState: map[ids.ID]utxoLockState{
// 						ins[0].InputID():           {}, // consumedUTXO
// 						producedUTXOs[0].InputID(): {}, // producedUTXO
// 					},
// 					utxos: []*avax.UTXO{
// 						generateTestUTXO(ids.ID{1}, avaxAssetID, 10, outputOwners, LockStateUnlocked),
// 					},
// 				}
// 			},
// 		},
// 		{
// 			name: "Deposited input unlocked output",
// 			args: args{
// 				inputs: []*avax.TransferableInput{
// 					generateTestIn(avaxAssetID, LockStateDeposited, 10),
// 				},
// 				outputs: []*avax.TransferableOutput{
// 					generateTestOut(avaxAssetID, LockStateUnlocked, 10, outputOwners),
// 				},
// 				lockState: LockStateDeposited,
// 				txID:      ids.ID{1},
// 			},
// 			generateCurrentState: func(ins []*avax.TransferableInput) lockedUTXOsChainStateImpl {
// 				return *generateTestLockedState([]map[ids.ID]utxoLockState{{ins[0].InputID(): {DepositTxID: &ids.ID{0}}}}, false)
// 			},
// 			generateWant: func(ins []*avax.TransferableInput, outs []*avax.TransferableOutput, producedUTXOs []*avax.UTXO) want {
// 				return want{
// 					utxoLockedState: map[ids.ID]utxoLockState{
// 						ins[0].InputID():           {}, // consumedUTXO
// 						producedUTXOs[0].InputID(): {}, // producedUTXO
// 					},
// 					utxos: []*avax.UTXO{
// 						generateTestUTXO(ids.ID{1}, avaxAssetID, 10, outputOwners, LockStateUnlocked),
// 					},
// 				}
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			currentLockedState := tt.generateCurrentState(tt.args.inputs)
// 			utxoLockedState, utxos, err := currentLockedState.ProduceUTXOsAndLockState(tt.args.inputs, tt.args.outputs, tt.args.lockState, tt.args.txID)
// 			assert.NoError(t, err) // TODO@
// 			generatedWant := tt.generateWant(tt.args.inputs, tt.args.outputs, utxos)

// 			assert.Equal(t, generatedWant.utxoLockedState, utxoLockedState)
// 			for index, utxo := range utxos {
// 				assert.Equal(t, generatedWant.utxos[index].Out, utxo.Out)
// 			}
// 		})
// 	}
// }
