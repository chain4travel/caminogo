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

var _ lockedOutputsChainState = &lockedOutputChainStateImpl{}

// lockedOutputsChainState manages the set of stakers (both validators and
// delegators) that are slated to start staking in the future.
type lockedOutputsChainState interface {
	UpdateOutputs(updatedUTXOStates [][3]*ids.ID) lockedOutputsChainState
	GetBondedUTXOs(bondTxID ids.ID) *ids.Set
	GetDepositedUTXOs(depositTxID ids.ID) *ids.Set

	Apply(InternalState)
}

// lockedOutputChainStateImpl is a copy on write implementation for versioning
// the validator set. None of the slices, maps, or pointers should be modified
// after initialization.
type lockedOutputChainStateImpl struct {
	bonds       map[ids.ID]*ids.Set   // bondTx.ID -> bondedUTXO.ID -> nil
	deposits    map[ids.ID]*ids.Set   // depositTx.ID -> depositedUTXO.ID -> nil
	lockedUTXOs map[ids.ID][2]*ids.ID // lockedUTXO.ID -> [ bondTx.ID, depositTx.ID ]

	updatedUTXOs [][3]*ids.ID // { utxo.ID, bondTx.ID, depositTx.ID }
}

func (cs *lockedOutputChainStateImpl) GetBondedUTXOs(bondTxID ids.ID) *ids.Set {
	return cs.bonds[bondTxID]
}

func (cs *lockedOutputChainStateImpl) GetDepositedUTXOs(depositTxID ids.ID) *ids.Set {
	return cs.deposits[depositTxID]
}

func (cs *lockedOutputChainStateImpl) UpdateOutputs(updatedUTXOStates [][3]*ids.ID) lockedOutputsChainState {
	newCS := &lockedOutputChainStateImpl{
		bonds:        make(map[ids.ID]*ids.Set, len(cs.bonds)+1),
		deposits:     make(map[ids.ID]*ids.Set, len(cs.deposits)+1),
		lockedUTXOs:  make(map[ids.ID][2]*ids.ID, len(cs.lockedUTXOs)+1),
		updatedUTXOs: make([][3]*ids.ID, len(updatedUTXOStates)),
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
		if newLockedUTXO[0] == nil {
			panic("utxoID should always be non-nil")
		}
		utxoID := *newLockedUTXO[0]
		lockedUTXOState := newCS.lockedUTXOs[utxoID]

		if lockedUTXOState[0] == nil && newLockedUTXO[1] != nil {
			bondTxID := *newLockedUTXO[1]
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				newSet := ids.NewSet(0) // TODO@
				bond = &newSet
				newCS.bonds[bondTxID] = bond
			}
			bond.Add(utxoID)
		} else if lockedUTXOState[0] != nil && newLockedUTXO[1] == nil {
			bondTxID := *lockedUTXOState[0]
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				panic("") // TODO@
			}
			bond.Remove(utxoID)
		} else {
			panic("") // TODO@
		}

		if lockedUTXOState[1] == nil && newLockedUTXO[2] != nil {
			depositTxID := *newLockedUTXO[2]
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				newSet := ids.NewSet(0) // TODO@
				deposit = &newSet
				newCS.deposits[depositTxID] = deposit
			}
			deposit.Add(utxoID)
		} else if lockedUTXOState[1] != nil && newLockedUTXO[2] == nil {
			depositTxID := *lockedUTXOState[1]
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				panic("") // TODO@
			}
			deposit.Remove(utxoID)
		} else {
			panic("") // TODO@
		}

		if newLockedUTXO[1] != nil && newLockedUTXO[2] != nil {
			delete(newCS.lockedUTXOs, utxoID)
		} else {
			newCS.lockedUTXOs[utxoID] = [2]*ids.ID{newLockedUTXO[1], newLockedUTXO[2]}
		}
	}

	return newCS
}

func (cs *lockedOutputChainStateImpl) Apply(is InternalState) {
	for _, updatedUTXO := range cs.updatedUTXOs {
		is.UpdateLockedOutput(updatedUTXO)
	}

	is.SetLockedOutputsChainState(cs)

	cs.updatedUTXOs = nil
}
