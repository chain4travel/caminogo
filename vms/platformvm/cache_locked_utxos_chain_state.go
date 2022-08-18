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
	"fmt"

	"github.com/chain4travel/caminogo/ids"
)

var _ lockedUTXOsChainState = &lockedUTXOsChainStateImpl{}

type lockedUTXOState struct {
	utxoID ids.ID
	lockState
}

type lockState struct {
	bondTxID    *ids.ID `serialize:"true"`
	depositTxID *ids.ID `serialize:"true"`
}

// lockedUTXOsChainState manages the set of stakers (both validators and
// delegators) that are slated to start staking in the future.
type lockedUTXOsChainState interface {
	UpdateUTXOs(updatedUTXOStates []lockedUTXOState) (lockedUTXOsChainState, error)
	GetBondedUTXOs(bondTxID ids.ID) ids.Set // ? @evlekht rename GetBondedUTXOIDs ?
	GetDepositedUTXOs(depositTxID ids.ID) ids.Set
	GetUTXOLockState(utxoID ids.ID) *lockState

	Apply(InternalState)
}

// lockedUTXOsChainStateImpl is a copy on write implementation for versioning
// the validator set. None of the slices, maps, or pointers should be modified
// after initialization.
type lockedUTXOsChainStateImpl struct {
	bonds       map[ids.ID]ids.Set   // bondTx.ID -> bondedUTXO.ID -> nil
	deposits    map[ids.ID]ids.Set   // depositTx.ID -> depositedUTXO.ID -> nil
	lockedUTXOs map[ids.ID]lockState // lockedUTXO.ID -> { bondTx.ID, depositTx.ID }

	updatedUTXOs []lockedUTXOState // { utxo.ID, bondTx.ID, depositTx.ID }
}

func (cs *lockedUTXOsChainStateImpl) GetBondedUTXOs(bondTxID ids.ID) ids.Set {
	return cs.bonds[bondTxID]
}

func (cs *lockedUTXOsChainStateImpl) GetDepositedUTXOs(depositTxID ids.ID) ids.Set {
	return cs.deposits[depositTxID]
}

func (cs *lockedUTXOsChainStateImpl) GetUTXOLockState(utxoID ids.ID) *lockState {
	utxoLockState, ok := cs.lockedUTXOs[utxoID]
	if !ok {
		return nil
	}
	return &utxoLockState
}

func (cs *lockedUTXOsChainStateImpl) UpdateUTXOs(updatedUTXOStates []lockedUTXOState) (lockedUTXOsChainState, error) {
	newCS := &lockedUTXOsChainStateImpl{
		bonds:        make(map[ids.ID]ids.Set, len(cs.bonds)+1),
		deposits:     make(map[ids.ID]ids.Set, len(cs.deposits)+1),
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
		oldLockedUTXOState := newCS.lockedUTXOs[newLockedUTXO.utxoID]

		// updating bond state
		if oldLockedUTXOState.bondTxID == nil && newLockedUTXO.bondTxID != nil {
			// bonding not-bonded utxo
			bondTxID := *newLockedUTXO.bondTxID
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				bond = ids.Set{}
				newCS.bonds[bondTxID] = bond
			}
			bond.Add(newLockedUTXO.utxoID)
		} else if oldLockedUTXOState.bondTxID != nil && newLockedUTXO.bondTxID == nil {
			// unbonding bonded utxo
			bondTxID := *oldLockedUTXOState.bondTxID
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				return nil, fmt.Errorf("old utxo lock state has not-nil bondTxID, but there is no such bond: %v",
					bondTxID)
			}
			bond.Remove(newLockedUTXO.utxoID)
		} else {
			if oldLockedUTXOState.bondTxID == nil {
				return nil, fmt.Errorf("Attempt to unbond not-bonded utxo (utxoID: %v)",
					newLockedUTXO.utxoID)
			}
			return nil, fmt.Errorf("Attempt to bond bonded utxo (utxoID: %v, oldBondID: %v, newBondID: %v)",
				newLockedUTXO.utxoID, oldLockedUTXOState.bondTxID, newLockedUTXO.bondTxID)
		}

		// updating deposit state
		if oldLockedUTXOState.depositTxID == nil && newLockedUTXO.depositTxID != nil {
			// depositing not-deposited utxo
			depositTxID := *newLockedUTXO.depositTxID
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				deposit = ids.Set{}
				newCS.deposits[depositTxID] = deposit
			}
			deposit.Add(newLockedUTXO.utxoID)
		} else if oldLockedUTXOState.depositTxID != nil && newLockedUTXO.depositTxID == nil {
			// undepositing deposited utxo
			depositTxID := *oldLockedUTXOState.depositTxID
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				return nil, fmt.Errorf("old utxo lock state has not-nil depositTxID, but there is no such deposit: %v",
					depositTxID)
			}
			deposit.Remove(newLockedUTXO.utxoID)
		} else {
			if oldLockedUTXOState.depositTxID == nil {
				return nil, fmt.Errorf("Attempt to undeposit not-deposited utxo (utxoID: %v)",
					newLockedUTXO.utxoID)
			}
			return nil, fmt.Errorf("Attempt to deposit deposited utxo (utxoID: %v, oldDepositID: %v, newDepositID: %v)",
				newLockedUTXO.utxoID, oldLockedUTXOState.depositTxID, newLockedUTXO.depositTxID)
		}

		// updating utxo lock state
		if newLockedUTXO.bondTxID == nil && newLockedUTXO.depositTxID == nil {
			delete(newCS.lockedUTXOs, newLockedUTXO.utxoID)
		} else {
			newCS.lockedUTXOs[newLockedUTXO.utxoID] = lockState{
				bondTxID:    newLockedUTXO.bondTxID,
				depositTxID: newLockedUTXO.depositTxID,
			}
		}
	}

	return newCS, nil
}

func (cs *lockedUTXOsChainStateImpl) Apply(is InternalState) {
	for _, updatedUTXO := range cs.updatedUTXOs {
		is.UpdateLockedUTXO(updatedUTXO)
	}

	is.SetLockedUTXOsChainState(cs)

	cs.updatedUTXOs = nil
}
