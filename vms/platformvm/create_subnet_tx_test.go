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

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"

	"github.com/stretchr/testify/assert"

	"github.com/chain4travel/caminogo/utils/units"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

func TestCreateSubnetTxAP3FeeChange(t *testing.T) {
	ap3Time := defaultGenesisTime.Add(time.Hour)
	tests := []struct {
		name         string
		time         time.Time
		fee          uint64
		expectsError bool
	}{
		{
			name:         "pre-fork - correctly priced",
			time:         defaultGenesisTime,
			fee:          0,
			expectsError: false,
		},
		{
			name:         "post-fork - incorrectly priced",
			time:         ap3Time,
			fee:          100*defaultTxFee - 1*units.NanoAvax,
			expectsError: true,
		},
		{
			name:         "post-fork - correctly priced",
			time:         ap3Time,
			fee:          100 * defaultTxFee,
			expectsError: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			vm, _, _ := defaultVM()
			vm.ApricotPhase3Time = ap3Time

			vm.ctx.Lock.Lock()
			defer func() {
				err := vm.Shutdown()
				assert.NoError(err)
				vm.ctx.Lock.Unlock()
			}()

			ins, outs, _, signers, err := vm.spend(keys, 0, test.fee, LockStateDeposited)
			assert.NoError(err)

			// Create the tx
			utx := &UnsignedCreateSubnetTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    vm.ctx.NetworkID,
					BlockchainID: vm.ctx.ChainID,
					Ins:          ins,
					Outs:         outs,
				}},
				Owner: &secp256k1fx.OutputOwners{},
			}
			tx := &Tx{UnsignedTx: utx}
			err = tx.Sign(Codec, signers)
			assert.NoError(err)

			vs := newVersionedState(
				vm.internalState,
				vm.internalState.CurrentStakerChainState(),
				vm.internalState.PendingStakerChainState(),
				vm.internalState.LockedUTXOsChainState(),
			)
			vs.SetTimestamp(test.time)

			_, err = utx.Execute(vm, vs, tx)
			assert.Equal(test.expectsError, err != nil)
		})
	}
}

func TestCreateSubnetLockedInsOrLockedOuts(t *testing.T) {
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
			utx := &UnsignedCreateSubnetTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    vm.ctx.NetworkID,
					BlockchainID: vm.ctx.ChainID,
					Ins:          tt.ins,
					Outs:         tt.outs,
				}},
				Owner: &secp256k1fx.OutputOwners{},
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
				preferredState.LockedUTXOsChainState(),
			)

			// Testing execute
			_, err = utx.Execute(vm, fakedState, tx)
			assert.Equal(t, tt.err, err)
		})
	}
}
