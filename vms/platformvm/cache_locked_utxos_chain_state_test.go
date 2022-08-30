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
			cs.lockedUTXOs[tt.args.inputs[0].InputID()] = tt.utxoLockState
			err := cs.SemanticVerifyLockInputs(tt.args.inputs, tt.args.bond)
			assert.Equal(t, err != nil, tt.wantErr, tt.msg)
		})
	}
}
