package platformvm

import (
	"testing"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
	"github.com/stretchr/testify/assert"
)

func TestGetInsOuts(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	owner1 := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
	}

	txID := ids.GenerateTestID()

	testCases := map[string]struct {
		utxos     []*avax.UTXO
		outs      []*avax.TransferableOutput
		lockState LockState
		err       error
		result    [][]int
	}{
		"1": {
			utxos: []*avax.UTXO{
				generateTestUTXO(txID, avaxAssetID, 100, owner1, LockStateUnlocked),
				generateTestUTXO(txID, avaxAssetID, 100, owner1, LockStateUnlocked),
			},
			outs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, LockStateBonded, 75, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 75, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 25, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 25, owner1),
			},
			lockState: LockStateBonded,
			err:       nil,
			result:    [][]int{{0, 2}, {1, 3}},
		},
		"2": {
			utxos: []*avax.UTXO{
				generateTestUTXO(txID, avaxAssetID, 100, owner1, LockStateUnlocked),
				generateTestUTXO(txID, avaxAssetID, 100, owner1, LockStateUnlocked),
			},
			outs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, LockStateBonded, 75, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 65, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 35, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 25, owner1),
			},
			lockState: LockStateBonded,
			err:       nil,
			result:    [][]int{{0, 3}, {1, 2}},
		},
		"3": {
			utxos: []*avax.UTXO{
				generateTestUTXO(txID, avaxAssetID, 100, owner1, LockStateUnlocked),
				generateTestUTXO(txID, avaxAssetID, 80, owner1, LockStateUnlocked),
			},
			outs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, LockStateBonded, 75, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 40, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 40, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 25, owner1),
			},
			lockState: LockStateBonded,
			err:       nil,
			result:    [][]int{{0, 3}, {1, 2}},
		},
		"4": {
			utxos: []*avax.UTXO{
				generateTestUTXO(txID, avaxAssetID, 100, owner1, LockStateUnlocked),
				generateTestUTXO(txID, avaxAssetID, 80, owner1, LockStateUnlocked),
			},
			outs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, LockStateBonded, 75, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 55, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 25, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 25, owner1),
			},
			lockState: LockStateBonded,
			err:       nil,
			result:    [][]int{{0, 2}, {1, 3}},
		},
		"5": {
			utxos: []*avax.UTXO{
				generateTestUTXO(txID, avaxAssetID, 100, owner1, LockStateUnlocked),
				generateTestUTXO(txID, avaxAssetID, 80, owner1, LockStateUnlocked),
			},
			outs: []*avax.TransferableOutput{
				generateTestOut(avaxAssetID, LockStateBonded, 55, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 55, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 45, owner1),
				generateTestOut(avaxAssetID, LockStateBonded, 25, owner1),
			},
			lockState: LockStateBonded,
			err:       nil,
			result:    [][]int{{0, 2}, {1, 3}},
		},
	}

	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			state := newVersionedState(vm.internalState, nil, nil, nil)
			for _, utxo := range tt.utxos {
				state.AddUTXO(utxo)
			}
			ins := make([]*avax.TransferableInput, len(tt.utxos))
			for i := 0; i < len(ins); i++ {
				ins[i] = generateTestInFromUTXO(tt.utxos[i], []uint32{})
			}

			assert := assert.New(t)
			result, err := GetInsOuts(state, ins, tt.outs, tt.lockState)
			assert.ErrorIs(err, tt.err)
			assert.Equal(tt.result, result)
		})
	}
}
