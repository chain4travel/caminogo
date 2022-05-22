// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"fmt"

	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

var (
	_ currentValidator = &currentValidatorImpl{}
)

type currentValidator interface {
	validator

	AddValidatorTx() *UnsignedAddValidatorTx
	VerifyCredsIntersection(vm *VM, tx *Tx) (bool, error)

	// Weight of delegations to this validator. Doesn't include the stake
	// provided by this validator.
	DelegatorWeight() uint64

	PotentialReward() uint64
}

type currentValidatorImpl struct {
	// delegators are sorted in order of removal.
	validatorImpl

	addValidatorTx  *UnsignedAddValidatorTx
	delegatorWeight uint64
	potentialReward uint64
}

func (v *currentValidatorImpl) AddValidatorTx() *UnsignedAddValidatorTx {
	return v.addValidatorTx
}

func (v *currentValidatorImpl) DelegatorWeight() uint64 {
	return v.delegatorWeight
}

func (v *currentValidatorImpl) PotentialReward() uint64 {
	return v.potentialReward
}

func (v *currentValidatorImpl) VerifyCredsIntersection(vm *VM, tx *Tx) (bool, error) {
	// get all public keys from these signatures
	sigAddr := secp256k1fx.NewSignatureKeyMap()
	for _, cred := range tx.Creds {
		if fxCreds, ok := cred.(*secp256k1fx.Credential); ok {
			if err := vm.fx.GetPublicKeys(tx, fxCreds, sigAddr); err != nil {
				return false, err
			}
		} else {
			return false, fmt.Errorf("no secp256k1fx.Credential found")
		}
	}

	if sigAddr.Empty() {
		return false, fmt.Errorf("no signatures found")
	}

	for _, stake := range v.addValidatorTx.Stake {
		out := stake.Out
		if test, ok := out.(*StakeableLockOut); ok {
			out = test.TransferableOut
		}
		secpOut, ok := out.(*secp256k1fx.TransferOutput)
		if !ok {
			return false, fmt.Errorf("cannot get secp256k1fx.TransferOutput")
		}
		for owner := range secpOut.AddressesSet() {
			if sigAddr.Contains(owner) {
				return true, nil
			}
		}
	}
	return false, nil
}
