package platformvm

import (
	"testing"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
	"github.com/stretchr/testify/assert"
)

func TestSemanticVerifyLockInputs(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	type args struct {
		inputs []*avax.TransferableInput
		bond   bool
	}
	tests := []struct {
		name          string
		args          args
		utxoLockState utxoLockState
		wantErr       bool
		msg           string
	}{
		{
			name: "Happy path bond",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				bond: true,
			},
			utxoLockState: utxoLockState{BondTxID: nil, DepositTxID: nil},
			wantErr:       false,
			msg:           "Happy path bond",
		},
		{
			name: "Happy path deposit",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				bond: false,
			},
			utxoLockState: utxoLockState{BondTxID: nil, DepositTxID: nil},
			wantErr:       false,
			msg:           "Happy path deposit",
		},
		{
			name: "Happy path bond deposited amount",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				bond: true,
			},
			utxoLockState: utxoLockState{BondTxID: nil, DepositTxID: &ids.ID{}},
			wantErr:       false,
			msg:           "Happy path bond deposited amount",
		},
		{
			name: "Happy path deposit bonded amount",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				bond: false,
			},
			utxoLockState: utxoLockState{BondTxID: &ids.ID{}, DepositTxID: nil},
			wantErr:       false,
			msg:           "Happy path deposit bonded amount",
		},
		{
			name: "Consumed UTXOs already bonded test",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				bond: true,
			},
			utxoLockState: utxoLockState{BondTxID: &ids.ID{}, DepositTxID: nil},
			wantErr:       true,
			msg:           "Should have failed because UTXOs consumed are already bonded",
		},
		{
			name: "Consumed UTXOs already deposited test",
			args: args{
				inputs: []*avax.TransferableInput{
					{
						Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
						In: &secp256k1fx.TransferInput{
							Amt: 1,
						},
					},
				},
				bond: false,
			},
			utxoLockState: utxoLockState{BondTxID: nil, DepositTxID: &ids.ID{}},
			wantErr:       true,
			msg:           "Should have failed because UTXOs consumed are already deposited",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &lockedUTXOsChainStateImpl{
				bonds:        make(map[ids.ID]ids.Set),
				deposits:     make(map[ids.ID]ids.Set),
				lockedUTXOs:  make(map[ids.ID]utxoLockState),
				updatedUTXOs: make(map[ids.ID]utxoLockState),
			}
			if tt.utxoLockState.isLocked() {
				cs.lockedUTXOs[tt.args.inputs[0].InputID()] = tt.utxoLockState
			}
			err := cs.SemanticVerifyLockInputs(tt.args.inputs, tt.args.bond)
			assert.Equal(t, tt.wantErr, err != nil, tt.msg)
		})
	}
}

func TestUpdateUTXOs(t *testing.T) {
	tests := []struct {
		name              string
		lockedStateImpl   lockedUTXOsChainStateImpl
		updatedUTXOStates map[ids.ID]utxoLockState
		want              lockedUTXOsChainState
		wantErr           bool
		msg               string
	}{
		{
			name: "Happy path bonding",
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    &ids.ID{9, 9},
				DepositTxID: nil,
			}},
			want: &lockedUTXOsChainStateImpl{
				bonds: map[ids.ID]ids.Set{
					{9, 9}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				deposits: map[ids.ID]ids.Set{},
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
			},
			wantErr: false,
			msg:     "Happy path bonding",
		},
		{
			name: "Happy path bonding deposited tokens",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds: make(map[ids.ID]ids.Set),
				deposits: map[ids.ID]ids.Set{
					{9, 9}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    &ids.ID{8, 8},
				DepositTxID: nil,
			}},
			want: &lockedUTXOsChainStateImpl{
				bonds: map[ids.ID]ids.Set{
					{8, 8}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				deposits: make(map[ids.ID]ids.Set),
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{8, 8},
					DepositTxID: nil,
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{8, 8},
					DepositTxID: nil,
				}},
			},
			wantErr: false,
			msg:     "Happy path bonding deposited tokens",
		},
		{
			name: "Happy path unbonding",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds: map[ids.ID]ids.Set{
					{9, 9}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				deposits: make(map[ids.ID]ids.Set),
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    nil,
				DepositTxID: nil,
			}},
			want: &lockedUTXOsChainStateImpl{
				bonds:       map[ids.ID]ids.Set{},
				deposits:    map[ids.ID]ids.Set{},
				lockedUTXOs: map[ids.ID]utxoLockState{},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: nil,
				}},
			},
			wantErr: false,
			msg:     "Happy path unbonding",
		},
		{
			name: "BondTx exists no bond state",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds:    make(map[ids.ID]ids.Set),
				deposits: make(map[ids.ID]ids.Set),
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    nil,
				DepositTxID: nil,
			}},
			want:    nil,
			wantErr: true,
			msg:     "Should have failed because bonding tx exists but there is no such bond in state",
		},
		{
			name: "Bonding already bonded UTXO",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds: map[ids.ID]ids.Set{
					{9, 9}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				deposits: make(map[ids.ID]ids.Set),
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    &ids.ID{8, 8},
				DepositTxID: nil,
			}},
			want:    nil,
			wantErr: true,
			msg:     "Should have failed because UTXO is already bonded",
		},
		{
			name: "Happy path depositing",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds:        make(map[ids.ID]ids.Set),
				deposits:     make(map[ids.ID]ids.Set),
				lockedUTXOs:  make(map[ids.ID]utxoLockState),
				updatedUTXOs: make(map[ids.ID]utxoLockState),
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    nil,
				DepositTxID: &ids.ID{9, 9},
			}},
			want: &lockedUTXOsChainStateImpl{
				bonds: map[ids.ID]ids.Set{},
				deposits: map[ids.ID]ids.Set{
					{9, 9}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
			},
			wantErr: false,
			msg:     "Happy path depositing",
		},
		{
			name: "Happy path depositing bonded tokens",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds: map[ids.ID]ids.Set{
					{9, 9}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				deposits: make(map[ids.ID]ids.Set),
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    &ids.ID{9, 9},
					DepositTxID: nil,
				}},
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    nil,
				DepositTxID: &ids.ID{8, 8},
			}},
			want: &lockedUTXOsChainStateImpl{
				bonds: make(map[ids.ID]ids.Set),
				deposits: map[ids.ID]ids.Set{
					{8, 8}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{8, 8},
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{8, 8},
				}},
			},
			wantErr: false,
			msg:     "Happy path depositing bonded tokens",
		},
		{
			name: "Happy path undepositing",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds: make(map[ids.ID]ids.Set),
				deposits: map[ids.ID]ids.Set{
					{9, 9}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    nil,
				DepositTxID: nil,
			}},
			want: &lockedUTXOsChainStateImpl{
				bonds:       map[ids.ID]ids.Set{},
				deposits:    map[ids.ID]ids.Set{},
				lockedUTXOs: map[ids.ID]utxoLockState{},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: nil,
				}},
			},
			wantErr: false,
			msg:     "Happy path undepositing",
		},
		{
			name: "DepositTx exists no deposit state",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds:    make(map[ids.ID]ids.Set),
				deposits: make(map[ids.ID]ids.Set),
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    nil,
				DepositTxID: nil,
			}},
			want:    nil,
			wantErr: true,
			msg:     "Should have failed because depositing tx exists but there is no such deposit in state",
		},
		{
			name: "Depositing already deposited UTXO",
			lockedStateImpl: lockedUTXOsChainStateImpl{
				bonds: make(map[ids.ID]ids.Set),
				deposits: map[ids.ID]ids.Set{
					{9, 9}: map[ids.ID]struct{}{{1, 1}: {}},
				},
				lockedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
				updatedUTXOs: map[ids.ID]utxoLockState{{1, 1}: {
					BondTxID:    nil,
					DepositTxID: &ids.ID{9, 9},
				}},
			},
			updatedUTXOStates: map[ids.ID]utxoLockState{{1, 1}: {
				BondTxID:    nil,
				DepositTxID: &ids.ID{8, 8},
			}},
			want:    nil,
			wantErr: true,
			msg:     "Should have failed because UTXO is already deposited",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.lockedStateImpl.updateUTXOs(tt.updatedUTXOStates)
			assert.Equal(t, tt.wantErr, err != nil, tt.msg)
			assert.Equalf(t, tt.want, got, tt.msg)
		})
	}
}
