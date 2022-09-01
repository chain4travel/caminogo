// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

var _ currentValidator = &currentValidatorImpl{}

type currentValidator interface {
	validator

	AddValidatorTx() *UnsignedAddValidatorTx

	PotentialReward() uint64
}

type currentValidatorImpl struct {
	validatorImpl

	addValidatorTx  *UnsignedAddValidatorTx
	potentialReward uint64
}

func (v *currentValidatorImpl) AddValidatorTx() *UnsignedAddValidatorTx {
	return v.addValidatorTx
}

func (v *currentValidatorImpl) PotentialReward() uint64 {
	return v.potentialReward
}
