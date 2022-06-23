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
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
)

var (
	_ currentLocksChainState = &currentLocksChainStateImpl{}
)

const strInvalidType = "expected add lock tx type but got %T"

type currentLocksChainState interface {
	// GetNextLock returns the next AddLockTx that is going to be removed
	// using a RewardLockTx.
	GetNextLock() (addLockTx *Tx, potentialReward uint64, err error)

	// UpdateStakers(
	// 	addValidators []*validatorReward,
	// 	addDelegators []*validatorReward,
	// 	addSubnetValidators []*Tx,
	// 	numTxsToRemove int,
	// ) (currentStakerChainState, error)

	// DeleteNextLock returns currentLocksChainState after moving AddLockTx from locks to deletedLocks.
	DeleteNextLock() (currentLocksChainState, error)

	// Locks returns the existing token locks on the network sorted in order of the
	// order of their future unlock.
	Locks() []*Tx

	Apply(InternalState)
}

// currentLocksChainStateImpl is a copy on write implementation for versioning // TODO@evlekht
// the validator set. None of the slices, maps, or pointers should be modified
// after initialization.
type currentLocksChainStateImpl struct {
	nextLockReward *validatorReward // TODO@evlekht ? rename type to something more unify and move its definition somewhere else ?

	// txID -> tx
	lockRewardsByTxID map[ids.ID]*validatorReward

	// list of current locks in order of their removal from the locks set
	locks []*Tx

	addedLocks   []*validatorReward
	deletedLocks []*Tx
}

// type validatorReward struct {
// 	addStakerTx     *Tx
// 	potentialReward uint64
// }

func (cs *currentLocksChainStateImpl) GetNextLock() (addLockTx *Tx, potentialReward uint64, err error) {
	if cs.nextLockReward == nil {
		return nil, 0, database.ErrNotFound
	}
	return cs.nextLockReward.addStakerTx, cs.nextLockReward.potentialReward, nil
}

// func (cs *currentStakerChainStateImpl) GetValidator(nodeID ids.ShortID) (currentValidator, error) {
// 	vdr, exists := cs.validatorsByNodeID[nodeID]
// 	if !exists {
// 		return nil, database.ErrNotFound
// 	}
// 	return vdr, nil
// }

// func (cs *currentStakerChainStateImpl) UpdateStakers(
// 	addValidatorTxs []*validatorReward,
// 	addDelegatorTxs []*validatorReward,
// 	addSubnetValidatorTxs []*Tx,
// 	numTxsToRemove int,
// ) (currentStakerChainState, error) {
// 	if numTxsToRemove > len(cs.validators) {
// 		return nil, errNotEnoughValidators
// 	}
// 	newCS := &currentStakerChainStateImpl{
// 		validatorsByNodeID: make(map[ids.ShortID]*currentValidatorImpl, len(cs.validatorsByNodeID)+len(addValidatorTxs)),
// 		validatorsByTxID:   make(map[ids.ID]*validatorReward, len(cs.validatorsByTxID)+len(addValidatorTxs)+len(addDelegatorTxs)+len(addSubnetValidatorTxs)),
// 		validators:         cs.validators[numTxsToRemove:], // sorted in order of removal

// 		addedStakers:   append(addValidatorTxs, addDelegatorTxs...),
// 		deletedStakers: cs.validators[:numTxsToRemove],
// 	}

// 	for nodeID, vdr := range cs.validatorsByNodeID {
// 		newCS.validatorsByNodeID[nodeID] = vdr
// 	}

// 	for txID, vdr := range cs.validatorsByTxID {
// 		newCS.validatorsByTxID[txID] = vdr
// 	}

// 	if numAdded := len(addValidatorTxs) + len(addDelegatorTxs) + len(addSubnetValidatorTxs); numAdded != 0 {
// 		numCurrent := len(newCS.validators)
// 		newSize := numCurrent + numAdded
// 		newValidators := make([]*Tx, newSize)
// 		copy(newValidators, newCS.validators)
// 		copy(newValidators[numCurrent:], addSubnetValidatorTxs)

// 		numStart := numCurrent + len(addSubnetValidatorTxs)
// 		for i, tx := range addValidatorTxs {
// 			newValidators[numStart+i] = tx.addStakerTx
// 		}

// 		numStart = numCurrent + len(addSubnetValidatorTxs) + len(addValidatorTxs)
// 		for i, tx := range addDelegatorTxs {
// 			newValidators[numStart+i] = tx.addStakerTx
// 		}

// 		sortValidatorsByRemoval(newValidators)
// 		newCS.validators = newValidators

// 		for _, vdr := range addValidatorTxs {
// 			switch tx := vdr.addStakerTx.UnsignedTx.(type) {
// 			case *UnsignedAddValidatorTx:
// 				newCS.validatorsByNodeID[tx.Validator.NodeID] = &currentValidatorImpl{
// 					addValidatorTx:  tx,
// 					potentialReward: vdr.potentialReward,
// 				}
// 				newCS.validatorsByTxID[vdr.addStakerTx.ID()] = vdr
// 			default:
// 				return nil, errWrongTxType
// 			}
// 		}

// 		for _, vdr := range addDelegatorTxs {
// 			switch tx := vdr.addStakerTx.UnsignedTx.(type) {
// 			case *UnsignedAddDelegatorTx:
// 				oldVdr := newCS.validatorsByNodeID[tx.Validator.NodeID]
// 				newVdr := *oldVdr
// 				newVdr.delegators = make([]*UnsignedAddDelegatorTx, len(oldVdr.delegators)+1)
// 				copy(newVdr.delegators, oldVdr.delegators)
// 				newVdr.delegators[len(oldVdr.delegators)] = tx
// 				sortDelegatorsByRemoval(newVdr.delegators)
// 				newVdr.delegatorWeight += tx.Validator.Wght
// 				newCS.validatorsByNodeID[tx.Validator.NodeID] = &newVdr
// 				newCS.validatorsByTxID[vdr.addStakerTx.ID()] = vdr
// 			default:
// 				return nil, errWrongTxType
// 			}
// 		}

// 		for _, vdr := range addSubnetValidatorTxs {
// 			switch tx := vdr.UnsignedTx.(type) {
// 			case *UnsignedAddSubnetValidatorTx:
// 				oldVdr := newCS.validatorsByNodeID[tx.Validator.NodeID]
// 				newVdr := *oldVdr
// 				newVdr.subnets = make(map[ids.ID]*UnsignedAddSubnetValidatorTx, len(oldVdr.subnets)+1)
// 				for subnetID, addTx := range oldVdr.subnets {
// 					newVdr.subnets[subnetID] = addTx
// 				}
// 				newVdr.subnets[tx.Validator.Subnet] = tx
// 				newCS.validatorsByNodeID[tx.Validator.NodeID] = &newVdr
// 			default:
// 				return nil, errWrongTxType
// 			}

// 			wrappedTx := &validatorReward{
// 				addStakerTx: vdr,
// 			}
// 			newCS.validatorsByTxID[vdr.ID()] = wrappedTx
// 			newCS.addedStakers = append(newCS.addedStakers, wrappedTx)
// 		}
// 	}

// 	for i := 0; i < numTxsToRemove; i++ {
// 		removed := cs.validators[i]
// 		removedID := removed.ID()
// 		delete(newCS.validatorsByTxID, removedID)

// 		switch tx := removed.UnsignedTx.(type) {
// 		case *UnsignedAddSubnetValidatorTx:
// 			oldVdr := newCS.validatorsByNodeID[tx.Validator.NodeID]
// 			newVdr := *oldVdr
// 			newVdr.subnets = make(map[ids.ID]*UnsignedAddSubnetValidatorTx, len(oldVdr.subnets)-1)
// 			for subnetID, addTx := range oldVdr.subnets {
// 				if removedID != addTx.ID() {
// 					newVdr.subnets[subnetID] = addTx
// 				}
// 			}
// 			newCS.validatorsByNodeID[tx.Validator.NodeID] = &newVdr
// 		default:
// 			return nil, errWrongTxType
// 		}
// 	}

// 	newCS.setNextStaker()
// 	return newCS, nil
// }

func (cs *currentLocksChainStateImpl) DeleteNextLock() (currentLocksChainState, error) {
	removedTx, _, err := cs.GetNextLock()
	if err != nil {
		return nil, err
	}
	removedTxID := removedTx.ID()

	newCS := &currentLocksChainStateImpl{
		lockRewardsByTxID: make(map[ids.ID]*validatorReward, len(cs.lockRewardsByTxID)-1),
		locks:             cs.locks[1:], // sorted in order of removal

		deletedLocks: []*Tx{removedTx},
	}

	for txID, lockReward := range cs.lockRewardsByTxID {
		if txID != removedTxID {
			newCS.lockRewardsByTxID[txID] = lockReward
		}
	}

	newCS.setNextLock()
	return newCS, nil
}

func (cs *currentLocksChainStateImpl) Locks() []*Tx {
	return cs.locks
}

func (cs *currentLocksChainStateImpl) Apply(is InternalState) {
	for _, added := range cs.addedLocks {
		is.AddCurrentLock(added.addStakerTx, added.potentialReward)
	}
	for _, deleted := range cs.deletedLocks {
		is.DeleteCurrentLock(deleted)
	}
	is.SetCurrentLockChainState(cs)

	// lock changes should only be applied once.
	cs.addedLocks = nil
	cs.deletedLocks = nil
}

// func (cs *currentStakerChainStateImpl) ValidatorSet(subnetID ids.ID) (validators.Set, error) {
// 	if subnetID == constants.PrimaryNetworkID {
// 		return cs.primaryValidatorSet()
// 	}
// 	return cs.subnetValidatorSet(subnetID)
// }

// func (cs *currentStakerChainStateImpl) primaryValidatorSet() (validators.Set, error) {
// 	vdrs := validators.NewSet()

// 	var err error
// 	for nodeID, vdr := range cs.validatorsByNodeID {
// 		vdrWeight := vdr.addValidatorTx.Validator.Wght
// 		vdrWeight, err = safemath.Add64(vdrWeight, vdr.delegatorWeight)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if err := vdrs.AddWeight(nodeID, vdrWeight); err != nil {
// 			return nil, err
// 		}
// 	}

// 	return vdrs, nil
// }

// func (cs *currentStakerChainStateImpl) subnetValidatorSet(subnetID ids.ID) (validators.Set, error) {
// 	vdrs := validators.NewSet()

// 	for nodeID, vdr := range cs.validatorsByNodeID {
// 		subnetVDR, exists := vdr.subnets[subnetID]
// 		if !exists {
// 			continue
// 		}
// 		if err := vdrs.AddWeight(nodeID, subnetVDR.Validator.Wght); err != nil {
// 			return nil, err
// 		}
// 	}

// 	return vdrs, nil
// }

// func (cs *currentStakerChainStateImpl) GetStaker(txID ids.ID) (tx *Tx, reward uint64, err error) {
// 	staker, exists := cs.validatorsByTxID[txID]
// 	if !exists {
// 		return nil, 0, database.ErrNotFound
// 	}
// 	return staker.addStakerTx, staker.potentialReward, nil
// }

// setNextStaker to the next staker that will be removed using a // TODO@evlekht comment
// RewardValidatorTx.
func (cs *currentLocksChainStateImpl) setNextLock() {
	for _, tx := range cs.locks {
		switch tx.UnsignedTx.(type) {
		case *UnsignedAddLockTx:
			cs.nextLockReward = cs.lockRewardsByTxID[tx.ID()]
			return
		}
	}
}

/*
 ******************************************************
 ********************* Sorter *************************
 ******************************************************
 */

type innerSortLocksByRemoval []*Tx

func (s innerSortLocksByRemoval) Less(i, j int) bool { // TODO@evlekht sort abstract interface objects, compare sort with validators
	iDel := s[i]
	jDel := s[j]

	var (
		iEndTime time.Time
	)
	switch tx := iDel.UnsignedTx.(type) {
	case *UnsignedAddLockTx:
		iEndTime = tx.EndTime()
	default:
		panic(fmt.Errorf(strInvalidType, iDel.UnsignedTx))
	}

	var (
		jEndTime time.Time
	)
	switch tx := jDel.UnsignedTx.(type) {
	case *UnsignedAddLockTx:
		jEndTime = tx.EndTime()
	default:
		panic(fmt.Errorf(strInvalidType, jDel.UnsignedTx))
	}

	if iEndTime.Before(jEndTime) {
		return true
	}
	if jEndTime.Before(iEndTime) {
		return false
	}

	// If the end times are the same, then we sort by the txID.
	iTxID := iDel.ID()
	jTxID := jDel.ID()
	return bytes.Compare(iTxID[:], jTxID[:]) == -1
}

func (s innerSortLocksByRemoval) Len() int {
	return len(s)
}

func (s innerSortLocksByRemoval) Swap(i, j int) {
	s[j], s[i] = s[i], s[j]
}

func sortLocksByRemoval(s []*Tx) {
	sort.Sort(innerSortLocksByRemoval(s))
}
