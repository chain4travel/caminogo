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
	"math"
	"testing"
	"time"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/platformvm/status"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
	"github.com/stretchr/testify/assert"
)

func TestAddValidatorTxSyntacticVerify(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	key, err := vm.factory.NewPrivateKey()
	assert.NoError(t, err)

	nodeID := key.PublicKey().Address()

	// Case: tx is nil
	var unsignedTx *UnsignedAddValidatorTx
	err = unsignedTx.SyntacticVerify(vm.ctx)
	assert.ErrorIs(t, err, errNilTx)

	// Case 3: Wrong Network ID
	tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	tx.UnsignedTx.(*UnsignedAddValidatorTx).NetworkID++
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	err = tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx)
	unwrapped := errors.Unwrap(err) // needs to be done due to multiple wrapping
	assert.Equal(t, errors.Unwrap(unwrapped), errWrongNetworkID)

	// Case: Stake owner has no addresses
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	tx.UnsignedTx.(*UnsignedAddValidatorTx).Stake = []*avax.TransferableOutput{{
		Asset: avax.Asset{ID: avaxAssetID},
		Out: &secp256k1fx.TransferOutput{
			Amt: vm.internalState.GetValidatorBondAmount(),
			OutputOwners: secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     nil,
			},
		},
	}}
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	err = tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx)
	assert.ErrorIs(t, err, secp256k1fx.ErrOutputUnspendable)

	// Case: Rewards owner has no addresses
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	tx.UnsignedTx.(*UnsignedAddValidatorTx).RewardsOwner = &secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     nil,
	}
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	err = tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx)
	assert.ErrorIs(t, err, secp256k1fx.ErrOutputUnspendable)

	// Case: Valid
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	err = tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx)
	assert.NoError(t, err)
}

// Test AddValidatorTx.Execute
func TestAddValidatorTxExecute(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	key, err := vm.factory.NewPrivateKey()
	assert.NoError(t, err)

	nodeID := key.PublicKey().Address()

	// Case: Validator's start time too early
	tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix())-1,
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errTimeBeforeCurrent)

	// Case: Validator's start time too far in the future
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Add(maxFutureStartTime).Unix()+1),
		uint64(defaultValidateStartTime.Add(maxFutureStartTime).Add(defaultMinStakingDuration).Unix()+1),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errFutureStakeTime)

	// Case: Validator already validating primary network
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID, // node ID
		nodeID, // reward address
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errTimeBeforeCurrent)

	// Case: Validator in pending validator set of primary network
	key2, err := vm.factory.NewPrivateKey()
	assert.NoError(t, err)

	startTime := defaultGenesisTime.Add(1 * time.Second)
	tx, err = vm.newAddValidatorTx(
		uint64(startTime.Unix()),                                // start time
		uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,                     // node ID
		key2.PublicKey().Address(), // reward address
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	vm.internalState.AddCurrentStaker(tx, 0)
	vm.internalState.AddTx(tx, status.Committed)
	err = vm.internalState.Commit()
	assert.NoError(t, err)

	err = vm.internalState.(*internalStateImpl).loadCurrentValidators()
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errNodeAlreadyValidator)

	// Case: Validator doesn't have enough tokens to cover stake amount
	_, err = vm.newAddValidatorTx( // create the tx
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		nodeID,
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
	)
	assert.NoError(t, err)

	// Remove all UTXOs owned by keys[0]
	utxoIDs, err := vm.internalState.UTXOIDs(keys[0].PublicKey().Address().Bytes(), ids.Empty, math.MaxInt32)
	assert.NoError(t, err)

	for _, utxoID := range utxoIDs {
		vm.internalState.DeleteUTXO(utxoID)
	}
	// Now keys[0] has no funds
	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errNodeAlreadyValidator)
}
