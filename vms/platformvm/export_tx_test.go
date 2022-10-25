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
	"testing"
	"time"

	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"

	"github.com/stretchr/testify/assert"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
)

func TestNewExportTx(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	type test struct {
		description        string
		destinationChainID ids.ID
		sourceKeys         []*crypto.PrivateKeySECP256K1R
		timestamp          time.Time
		shouldErr          bool
		shouldVerify       bool
	}

	sourceKey := keys[0]

	tests := []test{
		{
			description:        "P->X export",
			destinationChainID: xChainID,
			sourceKeys:         []*crypto.PrivateKeySECP256K1R{sourceKey},
			timestamp:          defaultValidateStartTime,
			shouldErr:          false,
			shouldVerify:       true,
		},
		{
			description:        "P->C export",
			destinationChainID: cChainID,
			sourceKeys:         []*crypto.PrivateKeySECP256K1R{sourceKey},
			timestamp:          vm.ApricotPhase5Time,
			shouldErr:          false,
			shouldVerify:       true,
		},
	}

	to := ids.GenerateTestShortID()
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			assert := assert.New(t)
			tx, err := vm.newExportTx(defaultBalance-defaultTxFee, tt.destinationChainID, to, tt.sourceKeys)
			if tt.shouldErr {
				assert.Error(err)
				return
			}
			assert.NoError(err)

			// Get the preferred block (which we want to build off)
			preferred, err := vm.Preferred()
			assert.NoError(err)

			preferredDecision, ok := preferred.(decision)
			assert.True(ok)

			preferredState := preferredDecision.onAccept()
			fakedState := newVersionedState(
				preferredState,
				preferredState.CurrentStakerChainState(),
				preferredState.PendingStakerChainState(),
			)
			fakedState.SetTimestamp(tt.timestamp)

			err = tx.UnsignedTx.SemanticVerify(vm, fakedState, tx)
			if tt.shouldVerify {
				assert.NoError(err)
			} else {
				assert.Error(err)
			}
		})
	}
}

func TestExportLockedInsOrLockedOuts(t *testing.T) {
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
	signers := [][]*crypto.PrivateKeySECP256K1R{{keys[0]}}

	type test struct {
		name string
		outs []*avax.TransferableOutput
		ins  []*avax.TransferableInput
		err  error
	}

	tests := []test{
		{
			name: "Locked out",
			outs: []*avax.TransferableOutput{
				generateTestOut(vm.ctx.AVAXAssetID, LockStateBonded, 10, outputOwners),
			},
			ins: []*avax.TransferableInput{},
			err: errLockedInsOrOuts,
		},
		{
			name: "Locked in",
			outs: []*avax.TransferableOutput{},
			ins: []*avax.TransferableInput{
				generateTestIn(vm.ctx.AVAXAssetID, LockStateBonded, 10),
			},
			err: errLockedInsOrOuts,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utx := &UnsignedExportTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    vm.ctx.NetworkID,
					BlockchainID: vm.ctx.ChainID,
					Outs:         tt.outs,
					Ins:          tt.ins,
				}},
				DestinationChain: vm.ctx.XChainID,
				ExportedOutputs: []*avax.TransferableOutput{
					generateTestOut(vm.ctx.AVAXAssetID, LockStateUnlocked, 10, outputOwners),
				},
			}
			tx := &Tx{UnsignedTx: utx}

			err := tx.Sign(Codec, signers)
			assert.NoError(t, err)

			// Get the preferred block (which we want to build off)
			preferred, err := vm.Preferred()
			assert.NoError(t, err)

			preferredDecision, ok := preferred.(decision)
			assert.True(t, ok)

			preferredState := preferredDecision.onAccept()
			fakedState := newVersionedState(
				preferredState,
				preferredState.CurrentStakerChainState(),
				preferredState.PendingStakerChainState(),
			)

			// Testing execute
			_, err = utx.Execute(vm, fakedState, tx)
			assert.Equal(t, tt.err, err)
		})
	}
}
