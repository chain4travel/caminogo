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
	_ lockChainState = &lockChainStateImpl{}
)

const strInvalidType = "expected add lock tx type but got %T"

type lockChainState interface {
	// AddLock returns lockChainState after moving AddLockTx from locks to deletedLocks.
	AddLock(addLockTx *Tx) lockChainState

	// GetNextLock returns the next AddLockTx that is going to be removed
	// using a RewardLockTx.
	GetNextLock() (addLockTx *Tx, potentialReward uint64, err error)

	// DeleteNextLock returns lockChainState after moving AddLockTx from locks to deletedLocks.
	DeleteNextLock() (lockChainState, error)

	// Locks returns the existing token locks on the network sorted in order of the
	// order of their future unlock.
	Locks() []*Tx

	Apply(InternalState)
}

// lockChainStateImpl is a copy on write implementation for versioning
// the locks set. None of the slices, maps, or pointers should be modified
// after initialization.
type lockChainStateImpl struct {
	nextLockReward *validatorReward // ?@evlekht ? rename type to something more unify and move its definition somewhere else ?

	// txID -> tx
	lockRewardsByTxID map[ids.ID]*validatorReward

	// list of current locks in order of their removal from the locks set
	locks []*Tx

	addedLocks   []*validatorReward
	deletedLocks []*Tx
}

func (cs *lockChainStateImpl) AddLock(tx *Tx) lockChainState {
	newCS := &lockChainStateImpl{
		lockRewardsByTxID: make(map[ids.ID]*validatorReward, len(cs.lockRewardsByTxID)+1),
		locks:             make([]*Tx, len(cs.locks)+1),
		addedLocks:        []*validatorReward{{tx, 0}},
	}
	copy(newCS.locks, cs.locks)
	newCS.locks[len(cs.locks)] = tx
	sortLocksByRemoval(newCS.locks)

	switch tx.UnsignedTx.(type) {
	case *UnsignedAddLockTx:
		newCS.addedLocks[0].potentialReward = 100
	default:
		panic(fmt.Errorf("expected lock tx type but got %T", tx.UnsignedTx))
	}

	for txID, lockReward := range cs.lockRewardsByTxID {
		newCS.lockRewardsByTxID[txID] = lockReward
	}
	newCS.lockRewardsByTxID[tx.ID()] = newCS.addedLocks[0]

	newCS.setNextLock()

	return newCS
}

func (cs *lockChainStateImpl) GetNextLock() (tx *Tx, potentialReward uint64, err error) {
	if cs.nextLockReward == nil {
		return nil, 0, database.ErrNotFound
	}
	return cs.nextLockReward.addStakerTx, cs.nextLockReward.potentialReward, nil
}

func (cs *lockChainStateImpl) DeleteNextLock() (lockChainState, error) {
	removedTx, _, err := cs.GetNextLock()
	if err != nil {
		return nil, err
	}
	removedTxID := removedTx.ID()

	newCS := &lockChainStateImpl{
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

func (cs *lockChainStateImpl) Locks() []*Tx {
	return cs.locks
}

func (cs *lockChainStateImpl) Apply(is InternalState) {
	for _, added := range cs.addedLocks {
		is.AddLock(added.addStakerTx, added.potentialReward)
	}
	for _, deleted := range cs.deletedLocks {
		is.DeleteLock(deleted)
	}
	is.SetLockChainState(cs)

	// lock changes should only be applied once.
	cs.addedLocks = nil
	cs.deletedLocks = nil
}

// setNextLock to the next lock that will be removed using a RewardLockTx.
func (cs *lockChainStateImpl) setNextLock() {
	if len(cs.locks) > 0 {
		cs.nextLockReward = cs.lockRewardsByTxID[cs.locks[0].ID()]
	} else {
		cs.nextLockReward = nil
	}
}

/*
 ******************************************************
 ********************* Sorter *************************
 ******************************************************
 */

type innerSortLocksByRemoval []*Tx

func (s innerSortLocksByRemoval) Less(i, j int) bool { // ?@evlekht sort abstract interface objects, compare sort with validators
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
