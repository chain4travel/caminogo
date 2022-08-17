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
	"github.com/chain4travel/caminogo/ids"
)

var _ lockedUTXOsChainState = &lockedUTXOsChainStateImpl{}

// TODO@ serialize or json tags ?
type lockedUTXOState struct {
	utxoID ids.ID
	lockState
}

// TODO@ serialize or json tags ?
type lockState struct {
	bondTxID    *ids.ID
	depositTxID *ids.ID
}

// lockedUTXOsChainState manages the set of stakers (both validators and
// delegators) that are slated to start staking in the future.
type lockedUTXOsChainState interface {
	UpdateUTXOs(updatedUTXOStates []lockedUTXOState) lockedUTXOsChainState
	GetBondedUTXOs(bondTxID ids.ID) *ids.Set // TODO@ rename GetBondedUTXOIDs ?
	GetDepositedUTXOs(depositTxID ids.ID) *ids.Set
	GetUTXOLockState(utxoID ids.ID) lockState

	Apply(InternalState)
}

// lockedUTXOsChainStateImpl is a copy on write implementation for versioning
// the validator set. None of the slices, maps, or pointers should be modified
// after initialization.
type lockedUTXOsChainStateImpl struct {
	bonds       map[ids.ID]*ids.Set  // bondTx.ID -> bondedUTXO.ID -> nil
	deposits    map[ids.ID]*ids.Set  // depositTx.ID -> depositedUTXO.ID -> nil
	lockedUTXOs map[ids.ID]lockState // lockedUTXO.ID -> { bondTx.ID, depositTx.ID }

	updatedUTXOs []lockedUTXOState // { utxo.ID, bondTx.ID, depositTx.ID }
}

func (cs *lockedUTXOsChainStateImpl) GetBondedUTXOs(bondTxID ids.ID) *ids.Set {
	return cs.bonds[bondTxID]
}

func (cs *lockedUTXOsChainStateImpl) GetDepositedUTXOs(depositTxID ids.ID) *ids.Set {
	return cs.deposits[depositTxID]
}

func (cs *lockedUTXOsChainStateImpl) GetUTXOLockState(utxoID ids.ID) lockState {
	return cs.lockedUTXOs[utxoID]
}

func (cs *lockedUTXOsChainStateImpl) UpdateUTXOs(updatedUTXOStates []lockedUTXOState) lockedUTXOsChainState {
	newCS := &lockedUTXOsChainStateImpl{
		bonds:        make(map[ids.ID]*ids.Set, len(cs.bonds)+1),
		deposits:     make(map[ids.ID]*ids.Set, len(cs.deposits)+1),
		lockedUTXOs:  make(map[ids.ID]lockState, len(cs.lockedUTXOs)+1),
		updatedUTXOs: make([]lockedUTXOState, len(updatedUTXOStates)),
	}

	copy(newCS.updatedUTXOs, updatedUTXOStates)

	for bondTxID, utxoIDs := range cs.bonds {
		newCS.bonds[bondTxID] = utxoIDs
	}
	for depositTxID, utxoIDs := range cs.deposits {
		newCS.deposits[depositTxID] = utxoIDs
	}
	for utxoID, lockIDs := range cs.lockedUTXOs {
		newCS.lockedUTXOs[utxoID] = lockIDs
	}

	for _, newLockedUTXO := range updatedUTXOStates {
		lockedUTXOState := newCS.lockedUTXOs[newLockedUTXO.utxoID]

		if lockedUTXOState.bondTxID == nil && newLockedUTXO.bondTxID != nil {
			bondTxID := *newLockedUTXO.bondTxID
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				newSet := ids.NewSet(0) // TODO@
				bond = &newSet
				newCS.bonds[bondTxID] = bond
			}
			bond.Add(newLockedUTXO.utxoID)
		} else if lockedUTXOState.bondTxID != nil && newLockedUTXO.bondTxID == nil {
			bondTxID := *lockedUTXOState.bondTxID
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				panic("") // TODO@
			}
			bond.Remove(newLockedUTXO.utxoID)
		} else {
			panic("") // TODO@
		}

		if lockedUTXOState.depositTxID == nil && newLockedUTXO.depositTxID != nil {
			depositTxID := *newLockedUTXO.depositTxID
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				newSet := ids.NewSet(0) // TODO@
				deposit = &newSet
				newCS.deposits[depositTxID] = deposit
			}
			deposit.Add(newLockedUTXO.utxoID)
		} else if lockedUTXOState.depositTxID != nil && newLockedUTXO.depositTxID == nil {
			depositTxID := *lockedUTXOState.depositTxID
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				panic("") // TODO@
			}
			deposit.Remove(newLockedUTXO.utxoID)
		} else {
			panic("") // TODO@
		}

		if newLockedUTXO.bondTxID != nil && newLockedUTXO.depositTxID != nil { // TODO@ may be == ?
			delete(newCS.lockedUTXOs, newLockedUTXO.utxoID)
		} else {
			newCS.lockedUTXOs[newLockedUTXO.utxoID] = lockState{
				bondTxID:    newLockedUTXO.bondTxID,
				depositTxID: newLockedUTXO.depositTxID,
			}
		}
	}

	return newCS
}

func (cs *lockedUTXOsChainStateImpl) Apply(is InternalState) {
	for _, updatedUTXO := range cs.updatedUTXOs {
		is.UpdateLockedUTXO(updatedUTXO)
	}

	is.SetLockedUTXOsChainState(cs)

	cs.updatedUTXOs = nil
}
