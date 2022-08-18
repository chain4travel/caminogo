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
	"github.com/chain4travel/caminogo/vms/components/avax"
)

var _ lockedUTXOsChainState = &lockedUTXOsChainStateImpl{}

type lockState struct {
	bondTxID    *ids.ID `serialize:"true"`
	depositTxID *ids.ID `serialize:"true"`
}

func (ls lockState) isLocked() bool    { return ls.bondTxID != nil || ls.depositTxID != nil }
func (ls lockState) isBonded() bool    { return ls.bondTxID != nil }
func (ls lockState) isDeposited() bool { return ls.depositTxID != nil }

type lockedUTXOsChainState interface {
	UpdateUTXOs(updatedUTXOStates map[ids.ID]lockState) (lockedUTXOsChainState, error)
	GetBondedUTXOs(bondTxID ids.ID) ids.Set // ? @evlekht rename GetBondedUTXOIDs ?
	GetDepositedUTXOs(depositTxID ids.ID) ids.Set
	GetUTXOLockState(utxoID ids.ID) *lockState
	UpdateAndProduceUTXOs(
		inputs []*avax.TransferableInput,
		inputIndexes []int,
		bondedOuts []*avax.TransferableOutput,
		notBondedOuts []*avax.TransferableOutput,
		txID ids.ID,
		bond bool,
	) (lockedUTXOsChainState, []*avax.UTXO, error)
	Unbond(bondTxID ids.ID) (lockedUTXOsChainState, error)

	Apply(InternalState)
}

// lockedUTXOsChainStateImpl is a copy on write implementation for versioning
// the lock state of utxos. None of the slices, maps, or pointers should be modified
// after initialization.
type lockedUTXOsChainStateImpl struct {
	bonds       map[ids.ID]ids.Set   // bondTx.ID -> bondedUTXO.ID -> nil
	deposits    map[ids.ID]ids.Set   // depositTx.ID -> depositedUTXO.ID -> nil
	lockedUTXOs map[ids.ID]lockState // lockedUTXO.ID -> { bondTx.ID, depositTx.ID }

	updatedUTXOs map[ids.ID]lockState // utxo.ID -> { bondTx.ID, depositTx.ID }
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

// TODO@ check for overlaps
func (cs *lockedUTXOsChainStateImpl) UpdateUTXOs(updatedUTXOStates map[ids.ID]lockState) (lockedUTXOsChainState, error) {
	newCS := &lockedUTXOsChainStateImpl{
		bonds:        make(map[ids.ID]ids.Set, len(cs.bonds)+1),
		deposits:     make(map[ids.ID]ids.Set, len(cs.deposits)+1),
		lockedUTXOs:  make(map[ids.ID]lockState, len(cs.lockedUTXOs)+1),
		updatedUTXOs: updatedUTXOStates,
	}

	for bondTxID, utxoIDs := range cs.bonds {
		newCS.bonds[bondTxID] = utxoIDs
	}
	for depositTxID, utxoIDs := range cs.deposits {
		newCS.deposits[depositTxID] = utxoIDs
	}
	for utxoID, lockIDs := range cs.lockedUTXOs {
		newCS.lockedUTXOs[utxoID] = lockIDs
	}

	for newLockedUTXOID, newLockedUTXO := range updatedUTXOStates {
		oldLockedUTXOState := newCS.lockedUTXOs[newLockedUTXOID]

		// updating bond state
		if oldLockedUTXOState.bondTxID == nil && newLockedUTXO.bondTxID != nil {
			// bonding not-bonded utxo
			bondTxID := *newLockedUTXO.bondTxID
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				bond = ids.Set{}
				newCS.bonds[bondTxID] = bond
			}
			bond.Add(newLockedUTXOID)
		} else if oldLockedUTXOState.bondTxID != nil && newLockedUTXO.bondTxID == nil {
			// unbonding bonded utxo
			bondTxID := *oldLockedUTXOState.bondTxID
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				return nil, fmt.Errorf("old utxo lock state has not-nil bondTxID, but there is no such bond: %v",
					bondTxID)
			}
			bond.Remove(newLockedUTXOID)
		} else {
			if oldLockedUTXOState.bondTxID == nil {
				return nil, fmt.Errorf("Attempt to unbond not-bonded utxo (utxoID: %v)",
					newLockedUTXOID)
			}
			return nil, fmt.Errorf("Attempt to bond bonded utxo (utxoID: %v, oldBondID: %v, newBondID: %v)",
				newLockedUTXOID, oldLockedUTXOState.bondTxID, newLockedUTXO.bondTxID)
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
			deposit.Add(newLockedUTXOID)
		} else if oldLockedUTXOState.depositTxID != nil && newLockedUTXO.depositTxID == nil {
			// undepositing deposited utxo
			depositTxID := *oldLockedUTXOState.depositTxID
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				return nil, fmt.Errorf("old utxo lock state has not-nil depositTxID, but there is no such deposit: %v",
					depositTxID)
			}
			deposit.Remove(newLockedUTXOID)
		} else {
			if oldLockedUTXOState.depositTxID == nil {
				return nil, fmt.Errorf("Attempt to undeposit not-deposited utxo (utxoID: %v)",
					newLockedUTXOID)
			}
			return nil, fmt.Errorf("Attempt to deposit deposited utxo (utxoID: %v, oldDepositID: %v, newDepositID: %v)",
				newLockedUTXOID, oldLockedUTXOState.depositTxID, newLockedUTXO.depositTxID)
		}

		// updating utxo lock state
		if newLockedUTXO.bondTxID == nil && newLockedUTXO.depositTxID == nil {
			delete(newCS.lockedUTXOs, newLockedUTXOID)
		} else {
			newCS.lockedUTXOs[newLockedUTXOID] = lockState{
				bondTxID:    newLockedUTXO.bondTxID,
				depositTxID: newLockedUTXO.depositTxID,
			}
		}
	}

	return newCS, nil
}

func (cs *lockedUTXOsChainStateImpl) Apply(is InternalState) {
	for utxoID, utxoLockState := range cs.updatedUTXOs {
		is.UpdateLockedUTXO(utxoID, utxoLockState)
	}

	is.SetLockedUTXOsChainState(cs)

	cs.updatedUTXOs = nil
}

// Creates utxos from given outs and update lock state for them and for utxos consumed by inputs
// Arguments:
// - [inputs] Inputs that produced this outputs
// - [inputIndexes] Indexes of inputs that produced outputs. First for notBondedOuts, then for bondedOuts
// - [notLockedOuts] Outputs that won't be locked
// - [lockedOuts] Outputs that will be locked
// - [txID] ID for transaction that produced this inputs and outputs
// - [bond] true if we'r bonding, false if depositing
// Returns:
// - [newlyLockedUTXOsState] updated locked UTXOs chain state
// - [producedUTXOs] utxos with produced outputs
func (cs *lockedUTXOsChainStateImpl) UpdateAndProduceUTXOs(
	inputs []*avax.TransferableInput,
	inputIndexes []int,
	notLockedOuts []*avax.TransferableOutput,
	lockedOuts []*avax.TransferableOutput,
	txID ids.ID,
	bond bool,
) (
	lockedUTXOsChainState, // newlyLockedUTXOsState
	[]*avax.UTXO, // producedUTXOs
	error,
) {
	lockedUTXOsCount := len(lockedOuts)
	notLockedUTXOsCount := len(notLockedOuts)
	producedUTXOs := make([]*avax.UTXO, notLockedUTXOsCount+lockedUTXOsCount)
	updatedUTXOLockStates := make(map[ids.ID]lockState, lockedUTXOsCount*2)

	// updating lock state for not locked utxos
	outIndex := 0
	for ; outIndex < notLockedUTXOsCount; outIndex++ {
		input := inputs[inputIndexes[outIndex]]
		consumedUTXOID := input.InputID()
		// produce new utxo
		utxo := &avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(outIndex),
			},
			Asset: avax.Asset{ID: input.AssetID()},
			Out:   notLockedOuts[outIndex].Output(),
		}
		producedUTXOs[outIndex] = utxo

		// adding produced not locked utxo to lock state
		if bond {
			updatedUTXOLockStates[utxo.InputID()] = lockState{
				bondTxID:    nil,
				depositTxID: cs.GetUTXOLockState(consumedUTXOID).depositTxID,
			}
		} else {
			updatedUTXOLockStates[utxo.InputID()] = lockState{
				bondTxID:    cs.GetUTXOLockState(consumedUTXOID).bondTxID,
				depositTxID: nil,
			}
		}

		// removing consumed utxo from lock state
		updatedUTXOLockStates[consumedUTXOID] = lockState{
			bondTxID:    nil,
			depositTxID: nil,
		}
	}

	// updating lock state for locked utxos
	for lockedOutIndex := 0; lockedOutIndex < lockedUTXOsCount; lockedOutIndex++ {
		input := inputs[inputIndexes[outIndex]]
		consumedUTXOID := input.InputID()
		// produce new utxo
		utxo := &avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(outIndex),
			},
			Asset: avax.Asset{ID: input.AssetID()},
			Out:   lockedOuts[lockedOutIndex].Output(),
		}
		producedUTXOs[outIndex] = utxo

		// adding produced locked utxo to lock state
		if bond {
			updatedUTXOLockStates[utxo.InputID()] = lockState{
				bondTxID:    &txID,
				depositTxID: cs.GetUTXOLockState(consumedUTXOID).depositTxID,
			}
		} else {
			updatedUTXOLockStates[utxo.InputID()] = lockState{
				bondTxID:    cs.GetUTXOLockState(consumedUTXOID).bondTxID,
				depositTxID: &txID,
			}
		}

		// removing consumed utxo from lock state
		if _, ok := updatedUTXOLockStates[consumedUTXOID]; !ok {
			updatedUTXOLockStates[consumedUTXOID] = lockState{
				bondTxID:    nil,
				depositTxID: nil,
			}
		}
		outIndex++
	}

	newlyLockedUTXOsState, err := cs.UpdateUTXOs(updatedUTXOLockStates)
	if err != nil {
		return nil, nil, err
	}

	return newlyLockedUTXOsState, producedUTXOs, nil
}

func (cs *lockedUTXOsChainStateImpl) Unbond(bondTxID ids.ID) (lockedUTXOsChainState, error) {
	bondedUTXOIDs := cs.GetBondedUTXOs(bondTxID)
	updatedUTXOLockStates := make(map[ids.ID]lockState, len(bondedUTXOIDs))
	for utxoID := range bondedUTXOIDs {
		updatedUTXOLockStates[utxoID] = lockState{
			bondTxID:    nil,
			depositTxID: cs.GetUTXOLockState(utxoID).depositTxID,
		}
	}

	newlyLockedUTXOsState, err := cs.UpdateUTXOs(updatedUTXOLockStates)
	if err != nil {
		return nil, err
	}

	return newlyLockedUTXOsState, nil
}

// ! @evlekht we'll also need more flexible method for graduall unlock
// ! most likely will be done with spending
func (cs *lockedUTXOsChainStateImpl) Undeposit(depositTxID ids.ID) (lockedUTXOsChainState, error) {
	depositedUTXOIDs := cs.GetDepositedUTXOs(depositTxID)
	updatedUTXOLockStates := make(map[ids.ID]lockState, len(depositedUTXOIDs))
	for utxoID := range depositedUTXOIDs {
		updatedUTXOLockStates[utxoID] = lockState{
			bondTxID:    cs.GetUTXOLockState(utxoID).bondTxID,
			depositTxID: nil,
		}
	}

	newlyLockedUTXOsState, err := cs.UpdateUTXOs(updatedUTXOLockStates)
	if err != nil {
		return nil, err
	}

	return newlyLockedUTXOsState, nil
}
