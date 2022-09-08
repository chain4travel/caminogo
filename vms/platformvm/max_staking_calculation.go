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
	"time"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/constants"
)

var errInvalidState = errors.New("generated output isn't valid state")

func (vm *VM) maxStakeAmount(
	subnetID ids.ID,
	nodeID ids.ShortID,
	startTime time.Time,
	endTime time.Time,
) (uint64, error) {
	if startTime.After(endTime) {
		return 0, errStartAfterEndTime
	}
	if timestamp := vm.internalState.GetTimestamp(); startTime.Before(timestamp) {
		return 0, errStartTimeTooEarly
	}
	if subnetID == constants.PrimaryNetworkID {
		return vm.maxPrimarySubnetStakeAmount(nodeID, startTime, endTime)
	}
	return vm.maxSubnetStakeAmount(subnetID, nodeID, startTime, endTime)
}

// ?@jax did not fully understand why this needs to be in a separate function
// ? the only difference is that subnets dont care about delegations
func (vm *VM) maxSubnetStakeAmount(
	subnetID ids.ID,
	nodeID ids.ShortID,
	startTime time.Time,
	endTime time.Time,
) (uint64, error) {
	var (
		vdrTx  *UnsignedAddSubnetValidatorTx
		exists bool
	)

	pendingStakers := vm.internalState.PendingStakerChainState()
	pendingValidator := pendingStakers.GetValidator(nodeID)

	currentStakers := vm.internalState.CurrentStakerChainState()
	currentValidator, err := currentStakers.GetValidator(nodeID)
	switch err {
	case nil:

		// ? @jax just as an idea, if this returns also if subnetID
		// ? is the primary network id we can just use this function
		vdrTx, exists = currentValidator.SubnetValidators()[subnetID]
		if !exists {
			vdrTx = pendingValidator.SubnetValidators()[subnetID]
		}
	case database.ErrNotFound:
		// ?@jax isnt this dangerous without the existence check?
		vdrTx = pendingValidator.SubnetValidators()[subnetID]
	default:
		return 0, err
	}

	if vdrTx == nil {
		return 0, nil
	}
	if vdrTx.StartTime().After(endTime) {
		return 0, nil
	}
	if vdrTx.EndTime().Before(startTime) {
		return 0, nil
	}
	return vdrTx.Weight(), nil
}

func (vm *VM) maxPrimarySubnetStakeAmount(
	nodeID ids.ShortID,
	startTime time.Time,
	endTime time.Time,
) (uint64, error) {
	currentStakers := vm.internalState.CurrentStakerChainState()
	pendingStakers := vm.internalState.PendingStakerChainState()

	currentValidator, err := currentStakers.GetValidator(nodeID)

	switch err {
	case nil:
		vdrTx := currentValidator.AddValidatorTx()
		if vdrTx.StartTime().After(endTime) {
			return 0, nil
		}
		if vdrTx.EndTime().Before(startTime) {
			return 0, nil
		}

		currentWeight := vdrTx.Weight()
		return currentWeight, nil

		// ?@jax integrate over time and sum up all delegations
		// ? there is some magic around when to add/remove delegations, but as we rip them out good
		// ? not an issue
	// If the node id is not in the active valiator pools
	// check if it will be in the future and count it if the time falls into the
	// start/endtime
	case database.ErrNotFound:
		futureValidator, err := pendingStakers.GetValidatorTx(nodeID)
		if err == database.ErrNotFound {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		if futureValidator.StartTime().After(endTime) {
			return 0, nil
		}
		if futureValidator.EndTime().Before(startTime) {
			return 0, nil
		}

		return futureValidator.Weight(), nil

	default:
		return 0, err
	}
}
