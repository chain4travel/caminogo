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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/constants"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/utils/hashing"
	"github.com/chain4travel/caminogo/utils/units"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

func TestUnsignedCreateChainTxVerify(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	type test struct {
		description   string
		shouldErr     bool
		subnetID      ids.ID
		genesisData   []byte
		vmID          ids.ID
		fxIDs         []ids.ID
		chainName     string
		keys          []*crypto.PrivateKeySECP256K1R
		setup         func(*UnsignedCreateChainTx) *UnsignedCreateChainTx
		expectedError error
	}

	tests := []test{
		{
			description:   "tx is nil",
			shouldErr:     true,
			subnetID:      testSubnet1.ID(),
			genesisData:   nil,
			vmID:          constants.AVMID,
			fxIDs:         nil,
			chainName:     "yeet",
			keys:          []*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
			setup:         func(*UnsignedCreateChainTx) *UnsignedCreateChainTx { return nil },
			expectedError: errNilTx,
		},
		{
			description:   "vm ID is empty",
			shouldErr:     true,
			subnetID:      testSubnet1.ID(),
			genesisData:   nil,
			vmID:          constants.AVMID,
			fxIDs:         nil,
			chainName:     "yeet",
			keys:          []*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
			setup:         func(tx *UnsignedCreateChainTx) *UnsignedCreateChainTx { tx.VMID = ids.ID{}; return tx },
			expectedError: errInvalidVMID,
		},
		{
			description:   "subnet ID is empty",
			shouldErr:     true,
			subnetID:      testSubnet1.ID(),
			genesisData:   nil,
			vmID:          constants.AVMID,
			fxIDs:         nil,
			chainName:     "yeet",
			keys:          []*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
			setup:         func(tx *UnsignedCreateChainTx) *UnsignedCreateChainTx { tx.SubnetID = ids.ID{}; return tx },
			expectedError: errDSCantValidate,
		},
		{
			description:   "subnet ID is platform chain's ID",
			shouldErr:     true,
			subnetID:      testSubnet1.ID(),
			genesisData:   nil,
			vmID:          constants.AVMID,
			fxIDs:         nil,
			chainName:     "yeet",
			keys:          []*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
			setup:         func(tx *UnsignedCreateChainTx) *UnsignedCreateChainTx { tx.SubnetID = vm.ctx.ChainID; return tx },
			expectedError: errDSCantValidate,
		},
		{
			description: "chain name is too long",
			shouldErr:   true,
			subnetID:    testSubnet1.ID(),
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
			setup: func(tx *UnsignedCreateChainTx) *UnsignedCreateChainTx {
				tx.ChainName = string(make([]byte, maxNameLen+1))
				return tx
			},
			expectedError: errNameTooLong,
		},
		{
			description: "chain name has invalid character",
			shouldErr:   true,
			subnetID:    testSubnet1.ID(),
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
			setup: func(tx *UnsignedCreateChainTx) *UnsignedCreateChainTx {
				tx.ChainName = "⌘"
				return tx
			},
			expectedError: errIllegalNameCharacter,
		},
		{
			description: "genesis data is too long",
			shouldErr:   true,
			subnetID:    testSubnet1.ID(),
			genesisData: nil,
			vmID:        constants.AVMID,
			fxIDs:       nil,
			chainName:   "yeet",
			keys:        []*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
			setup: func(tx *UnsignedCreateChainTx) *UnsignedCreateChainTx {
				tx.GenesisData = make([]byte, maxGenesisLen+1)
				return tx
			},
			expectedError: errGenesisTooLong,
		},
	}

	for _, test := range tests {
		tx, err := vm.newCreateChainTx(
			test.subnetID,
			test.genesisData,
			test.vmID,
			test.fxIDs,
			test.chainName,
			test.keys,
		)
		assert.NoError(t, err)

		tx.UnsignedTx.(*UnsignedCreateChainTx).syntacticallyVerified = false
		tx.UnsignedTx = test.setup(tx.UnsignedTx.(*UnsignedCreateChainTx))

		err = tx.UnsignedTx.(*UnsignedCreateChainTx).SyntacticVerify(vm.ctx)

		if !test.shouldErr {
			assert.NoError(t, err)
		} else {
			assert.ErrorIs(t, err, test.expectedError)
		}
	}
}

// Ensure Execute fails when there are not enough control sigs
func TestCreateChainTxInsufficientControlSigs(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	tx, err := vm.newCreateChainTx(
		testSubnet1.ID(),
		nil,
		constants.AVMID,
		nil,
		"chain name",
		[]*crypto.PrivateKeySECP256K1R{keys[0], keys[1]},
	)
	assert.NoError(t, err)

	vs := newVersionedState(
		vm.internalState,
		vm.internalState.CurrentStakerChainState(),
		vm.internalState.PendingStakerChainState(),
	)

	// Remove a signature
	tx.Creds[0].(*secp256k1fx.Credential).Sigs = tx.Creds[0].(*secp256k1fx.Credential).Sigs[1:]

	_, err = tx.UnsignedTx.(UnsignedDecisionTx).Execute(vm, vs, tx)
	assert.ErrorIs(t, err, secp256k1fx.ErrInputCredentialSignersMismatch)
}

// Ensure Execute fails when an incorrect control signature is given
func TestCreateChainTxWrongControlSig(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	tx, err := vm.newCreateChainTx( // create a tx
		testSubnet1.ID(),
		nil,
		constants.AVMID,
		nil,
		"chain name",
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	// Generate new, random key to sign tx with
	factory := crypto.FactorySECP256K1R{}
	key, err := factory.NewPrivateKey()
	assert.NoError(t, err)

	vs := newVersionedState(
		vm.internalState,
		vm.internalState.CurrentStakerChainState(),
		vm.internalState.PendingStakerChainState(),
	)

	// Replace a valid signature with one from another key
	sig, err := key.SignHash(hashing.ComputeHash256(tx.UnsignedBytes()))
	assert.NoError(t, err)

	copy(tx.Creds[0].(*secp256k1fx.Credential).Sigs[0][:], sig)
	_, err = tx.UnsignedTx.(UnsignedDecisionTx).Execute(vm, vs, tx)
	assert.ErrorIs(t, errors.Unwrap(err), secp256k1fx.ErrExpectedSignature)
}

// Ensure Execute fails when the Subnet the blockchain specifies as
// its validator set doesn't exist
func TestCreateChainTxNoSuchSubnet(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	tx, err := vm.newCreateChainTx(
		testSubnet1.ID(),
		nil,
		constants.AVMID,
		nil,
		"chain name",
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	vs := newVersionedState(
		vm.internalState,
		vm.internalState.CurrentStakerChainState(),
		vm.internalState.PendingStakerChainState(),
	)

	tx.UnsignedTx.(*UnsignedCreateChainTx).SubnetID = ids.GenerateTestID()
	_, err = tx.UnsignedTx.(UnsignedDecisionTx).Execute(vm, vs, tx)
	assert.ErrorIs(t, errors.Unwrap(err), errSubnetNotExist)
}

// Ensure valid tx passes semanticVerify
func TestCreateChainTxValid(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	// create a valid tx
	tx, err := vm.newCreateChainTx(
		testSubnet1.ID(),
		nil,
		constants.AVMID,
		nil,
		"chain name",
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	vs := newVersionedState(
		vm.internalState,
		vm.internalState.CurrentStakerChainState(),
		vm.internalState.PendingStakerChainState(),
	)

	_, err = tx.UnsignedTx.(UnsignedDecisionTx).Execute(vm, vs, tx)
	assert.NoError(t, err)
}

func TestCreateChainTxAP3FeeChange(t *testing.T) {
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

			ins, outs, _, signers, err := vm.stake(keys, 0, test.fee)
			assert.NoError(err)

			subnetAuth, subnetSigners, err := vm.authorize(vm.internalState, testSubnet1.ID(), keys)
			assert.NoError(err)

			signers = append(signers, subnetSigners)

			// Create the tx

			utx := &UnsignedCreateChainTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    vm.ctx.NetworkID,
					BlockchainID: vm.ctx.ChainID,
					Ins:          ins,
					Outs:         outs,
				}},
				SubnetID:   testSubnet1.ID(),
				VMID:       constants.AVMID,
				SubnetAuth: subnetAuth,
			}
			tx := &Tx{UnsignedTx: utx}
			err = tx.Sign(Codec, signers)
			assert.NoError(err)

			vs := newVersionedState(
				vm.internalState,
				vm.internalState.CurrentStakerChainState(),
				vm.internalState.PendingStakerChainState(),
			)
			vs.SetTimestamp(test.time)

			_, err = utx.Execute(vm, vs, tx)
			assert.Equal(test.expectsError, err != nil)
		})
	}
}
