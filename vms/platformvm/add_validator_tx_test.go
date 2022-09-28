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
	"crypto/rsa"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/platformvm/status"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

func TestAddValidatorTxSyntacticVerify(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	key, err := vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	rsaPrivateKey, certBytes, nodeID := loadNodeKeyPair(testStakingPath, 1)

	// Case: tx is nil
	var unsignedTx *UnsignedAddValidatorTx
	if err := unsignedTx.SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because tx is nil")
	}

	// Case 3: Wrong Network ID
	tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr
	)
	if err != nil {
		t.Fatal(err)
	}
	tx.UnsignedTx.(*UnsignedAddValidatorTx).NetworkID++
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because the wrong network ID was used")
	}

	// Case: Stake owner has no addresses
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr

	)
	if err != nil {
		t.Fatal(err)
	}
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
	if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because stake owner has no addresses")
	}

	// Case: Rewards owner has no addresses
	tx, err = vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr

	)
	if err != nil {
		t.Fatal(err)
	}
	tx.UnsignedTx.(*UnsignedAddValidatorTx).RewardsOwner = &secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     nil,
	}
	// This tx was syntactically verified when it was created...pretend it wasn't so we don't use cache
	tx.UnsignedTx.(*UnsignedAddValidatorTx).syntacticallyVerified = false
	if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err == nil {
		t.Fatal("should have errored because rewards owner has no addresses")
	}

	// Case: Node certificate key doesn't match node ID
	_, _, tempNodeID := loadNodeKeyPair(testStakingPath, 2)
	if _, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		tempNodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr

	); err != errCertificateDontMatch {
		t.Fatalf("should have errored with: '%s' error", errCertificateDontMatch)
	}

	// Case: Valid
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr

	); err != nil {
		t.Fatal(err)
	} else if err := tx.UnsignedTx.(*UnsignedAddValidatorTx).SyntacticVerify(vm.ctx); err != nil {
		t.Fatal(err)
	}
}

// Test AddValidatorTx.Execute
func TestAddValidatorTxExecute(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	key, err := vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	rsaPrivateKey, certBytes, nodeID := loadNodeKeyPair(testStakingPath, 1)

	// Case: Valid
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix())+1,
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err != nil {
		t.Fatal(err)
	}

	// Case: Failed signature verification
	tempPrivateKey, _, _ := loadNodeKeyPair(testStakingPath, 5)
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix())+1,
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		tempPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); errors.Unwrap(err) != rsa.ErrVerification {
		t.Fatalf("should have errored with: '%s' error", rsa.ErrVerification)
	}

	// Case: Validator's start time too early
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix())-1,
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because start time too early")
	}

	// Case: Validator's start time too far in the future
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Add(maxFutureStartTime).Unix()+1),
		uint64(defaultValidateStartTime.Add(maxFutureStartTime).Add(defaultMinStakingDuration).Unix()+1),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because start time too far in the future")
	}

	// Case: Validator already validating primary network
	if tx, err := vm.newAddValidatorTx(
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID, // node ID
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	} else if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should've errored because validator already validating")
	}

	// Case: Validator in pending validator set of primary network
	key2, err := vm.factory.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	rsaPrivateKey2, certBytes2, nodeID2 := loadNodeKeyPair(testStakingPath, 2)

	startTime := defaultGenesisTime.Add(1 * time.Second)
	tx, err := vm.newAddValidatorTx(
		uint64(startTime.Unix()),                                // start time
		uint64(startTime.Add(defaultMinStakingDuration).Unix()), // end time
		nodeID2, // node ID
		key2.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey2,
		certBytes2,
		ids.ShortEmpty, // change addr // key
	)
	if err != nil {
		t.Fatal(err)
	}

	vm.internalState.AddCurrentStaker(tx, 0)
	vm.internalState.AddTx(tx, status.Committed)
	if err := vm.internalState.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := vm.internalState.(*internalStateImpl).loadCurrentValidators(); err != nil {
		t.Fatal(err)
	}

	if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should have failed because validator in pending validator set")
	}

	// Case: Validator doesn't have enough tokens to cover stake amount
	if _, err := vm.newAddValidatorTx( // create the tx
		uint64(defaultValidateStartTime.Unix()),
		uint64(defaultValidateEndTime.Unix()),
		nodeID,
		key.PublicKey().Address(),
		[]*crypto.PrivateKeySECP256K1R{keys[0]},
		rsaPrivateKey,
		certBytes,
		ids.ShortEmpty, // change addr
	); err != nil {
		t.Fatal(err)
	}
	// Remove all UTXOs owned by keys[0]
	utxoIDs, err := vm.internalState.UTXOIDs(keys[0].PublicKey().Address().Bytes(), ids.Empty, math.MaxInt32)
	if err != nil {
		t.Fatal(err)
	}
	for _, utxoID := range utxoIDs {
		vm.internalState.DeleteUTXO(utxoID)
	}
	// Now keys[0] has no funds
	if _, _, err := tx.UnsignedTx.(UnsignedProposalTx).Execute(vm, vm.internalState, tx); err == nil {
		t.Fatal("should have failed because tx fee paying key has no funds")
	}
}
