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

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/utils/hashing"
	"github.com/chain4travel/caminogo/vms/platformvm/status"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
	"github.com/stretchr/testify/assert"
)

func TestAddSubnetValidatorTxSyntacticVerify(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	nodeID := keys[0].PublicKey().Address()

	// Case: tx is nil
	var unsignedTx *UnsignedAddSubnetValidatorTx
	err := unsignedTx.SyntacticVerify(vm.ctx)
	assert.ErrorIs(t, err, errNilTx)

	// Case: Wrong network ID
	tx, err := vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).NetworkID++
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).syntacticallyVerified = false
	err = tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SyntacticVerify(vm.ctx)
	assert.Equal(t, errors.Unwrap(err), errWrongNetworkID)

	// Case: Missing Subnet ID
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).Validator.Subnet = ids.ID{}
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).syntacticallyVerified = false
	err = tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SyntacticVerify(vm.ctx)
	assert.ErrorIs(t, err, errBadSubnetID)

	// Case: No weight
	tx, err = vm.newAddSubnetValidatorTx(
		1,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).Validator.Wght = 0
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).syntacticallyVerified = false
	err = tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SyntacticVerify(vm.ctx)
	assert.ErrorIs(t, err, errWeightTooSmall)

	// Case: Subnet auth indices not unique
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix())-1,
		nodeID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SubnetAuth.(*secp256k1fx.Input).SigIndices[0] = tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SubnetAuth.(*secp256k1fx.Input).SigIndices[1]
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).syntacticallyVerified = false
	err = tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SyntacticVerify(vm.ctx)
	assert.ErrorIs(t, err, secp256k1fx.ErrNotSortedUnique)

	// Case: Valid
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	err = tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SyntacticVerify(vm.ctx)
	assert.NoError(t, err)
}

func TestAddSubnetValidatorTxExecute(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	nodeID := keys[0].PublicKey().Address()

	// Case: Proposed validator currently validating primary network
	// but stops validating subnet after stops validating primary network
	// (note that keys[0] is a genesis validator)
	tx, err := vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix())+1,
		nodeID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, ErrorAtOrAfterCurrentTimestamp)

	// Case: Proposed validator currently validating primary network
	// and proposed subnet validation period is subset of
	// primary network validation period
	// (note that keys[0] is a genesis validator)
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(defaultValidateStartTime.Unix()+1),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.NoError(t, err)

	// Add a validator to pending validator set of primary network
	key, err := vm.factory.NewPrivateKey()
	assert.NoError(t, err)

	pendingDSValidatorID := key.PublicKey().Address()

	// starts validating primary network 10 seconds after genesis
	DSStartTime := defaultGenesisTime.Add(10 * time.Second)
	DSEndTime := DSStartTime.Add(5 * defaultMinStakingDuration)

	addDSTx, err := vm.newAddValidatorTx(
		uint64(DSStartTime.Unix()),              // start time
		uint64(DSEndTime.Unix()),                // end time
		pendingDSValidatorID,                    // node ID
		nodeID,                                  // reward address
		[]*crypto.PrivateKeySECP256K1R{keys[0]}, // key

	)
	assert.NoError(t, err)

	// Case: Proposed validator isn't in pending or current validator sets
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(DSStartTime.Unix()), // start validating subnet before primary network
		uint64(DSEndTime.Unix()),
		pendingDSValidatorID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errDSValidatorSubset)

	vm.internalState.AddCurrentStaker(addDSTx, 0)
	vm.internalState.AddTx(addDSTx, status.Committed)

	err = vm.internalState.Commit()
	assert.NoError(t, err)

	err = vm.internalState.(*internalStateImpl).loadCurrentValidators()
	assert.NoError(t, err)

	// Node with ID key.PublicKey().Address() now a pending validator for primary network

	// Case: Proposed validator is pending validator of primary network
	// but starts validating subnet before primary network
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(DSStartTime.Unix())-1, // start validating subnet before primary network
		uint64(DSEndTime.Unix()),
		pendingDSValidatorID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errDSValidatorSubset)

	// Case: Proposed validator is pending validator of primary network
	// but stops validating subnet after primary network
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(DSStartTime.Unix()),
		uint64(DSEndTime.Unix())+1, // stop validating subnet after stopping validating primary network
		pendingDSValidatorID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errDSValidatorSubset)

	// Case: Proposed validator is pending validator of primary network
	// and period validating subnet is subset of time validating primary network
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(DSStartTime.Unix()), // same start time as for primary network
		uint64(DSEndTime.Unix()),   // same end time as for primary network
		pendingDSValidatorID,
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.NoError(t, err)

	// Case: Proposed validator start validating at/before current timestamp
	// First, advance the timestamp
	newTimestamp := defaultGenesisTime.Add(2 * time.Second)
	vm.internalState.SetTimestamp(newTimestamp)

	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,               // weight
		uint64(newTimestamp.Unix()), // start time
		uint64(newTimestamp.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,           // node ID
		testSubnet1.ID(), // subnet ID
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, ErrorAtOrAfterCurrentTimestamp)

	// reset the timestamp
	vm.internalState.SetTimestamp(defaultGenesisTime)

	// Case: Proposed validator already validating the subnet
	// First, add validator as validator of subnet
	subnetTx, err := vm.newAddSubnetValidatorTx(
		defaultWeight,                           // weight
		uint64(defaultValidateStartTime.Unix()), // start time
		uint64(defaultValidateEndTime.Unix()),   // end time
		nodeID,                                  // node ID
		testSubnet1.ID(),                        // subnet ID
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	vm.internalState.AddCurrentStaker(subnetTx, 0)
	vm.internalState.AddTx(subnetTx, status.Committed)

	err = vm.internalState.Commit()
	assert.NoError(t, err)

	err = vm.internalState.(*internalStateImpl).loadCurrentValidators()
	assert.NoError(t, err)

	// Node with ID nodeIDKey.PublicKey().Address() now validating subnet with ID testSubnet1.ID
	duplicateSubnetTx, err := vm.newAddSubnetValidatorTx(
		defaultWeight,                           // weight
		uint64(defaultValidateStartTime.Unix()), // start time
		uint64(defaultValidateEndTime.Unix()),   // end time
		nodeID,                                  // node ID
		testSubnet1.ID(),                        // subnet ID
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	_, _, err = duplicateSubnetTx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, duplicateSubnetTx)
	assert.ErrorIs(t, err, ErrorAtOrAfterCurrentTimestamp)

	vm.internalState.DeleteCurrentStaker(subnetTx)
	err = vm.internalState.Commit()
	assert.NoError(t, err)

	err = vm.internalState.(*internalStateImpl).loadCurrentValidators()
	assert.NoError(t, err)

	// Case: Too many signatures
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,                     // weight
		uint64(defaultGenesisTime.Unix()), // start time
		uint64(defaultGenesisTime.Add(defaultMinStakingDuration).Unix())+1, // end time
		nodeID,           // node ID
		testSubnet1.ID(), // subnet ID
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1], testSubnet1ControlKeys[2]},
	)
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, ErrorAtOrAfterCurrentTimestamp)

	// Case: Too few signatures
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,                     // weight
		uint64(defaultGenesisTime.Unix()), // start time
		uint64(defaultGenesisTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,           // node ID
		testSubnet1.ID(), // subnet ID
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[2]},
	)
	assert.NoError(t, err)

	// Remove a signature
	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SubnetAuth.(*secp256k1fx.Input).SigIndices = tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SubnetAuth.(*secp256k1fx.Input).SigIndices[1:]
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).syntacticallyVerified = false

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, ErrorAtOrAfterCurrentTimestamp)

	// Case: Control Signature from invalid key (keys[3] is not a control key)
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,                     // weight
		uint64(defaultGenesisTime.Unix()), // start time
		uint64(defaultGenesisTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID,           // node ID
		testSubnet1.ID(), // subnet ID
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], keys[1]},
	)
	assert.NoError(t, err)

	// Replace a valid signature with one from keys[3]
	sig, err := keys[3].SignHash(hashing.ComputeHash256(tx.UnsignedBytes()))
	assert.NoError(t, err)

	copy(tx.Creds[0].(*secp256k1fx.Credential).Sigs[0][:], sig)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, ErrorAtOrAfterCurrentTimestamp)

	// Case: Proposed validator in pending validator set for subnet
	// First, add validator to pending validator set of subnet
	tx, err = vm.newAddSubnetValidatorTx(
		defaultWeight,                       // weight
		uint64(defaultGenesisTime.Unix())+1, // start time
		uint64(defaultGenesisTime.Add(defaultMinStakingDuration).Unix())+1, // end time
		nodeID,           // node ID
		testSubnet1.ID(), // subnet ID
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	vm.internalState.AddCurrentStaker(tx, 0)
	vm.internalState.AddTx(tx, status.Committed)

	err = vm.internalState.Commit()
	assert.NoError(t, err)

	err = vm.internalState.(*internalStateImpl).loadCurrentValidators()
	assert.NoError(t, err)

	_, _, err = tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx)
	assert.ErrorIs(t, err, errAlreadyValidatingSubnet)
}

// Test that marshalling/unmarshalling works
func TestAddSubnetValidatorMarshal(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		err := vm.Shutdown()
		assert.NoError(t, err)
		vm.ctx.Lock.Unlock()
	}()

	var unmarshaledTx Tx

	// valid tx
	tx, err := vm.newAddSubnetValidatorTx(
		defaultWeight,
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		keys[0].PublicKey().Address(),
		testSubnet1.ID(),
		[]*crypto.PrivateKeySECP256K1R{testSubnet1ControlKeys[0], testSubnet1ControlKeys[1]},
	)
	assert.NoError(t, err)

	txBytes, err := Codec.Marshal(CodecVersion, tx)
	assert.NoError(t, err)

	_, err = Codec.Unmarshal(txBytes, &unmarshaledTx)
	assert.NoError(t, err)

	err = unmarshaledTx.Sign(Codec, nil)
	assert.NoError(t, err)

	err = unmarshaledTx.UnsignedTx.(*UnsignedAddSubnetValidatorTx).SyntacticVerify(vm.ctx)
	assert.NoError(t, err)
}
