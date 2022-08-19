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

type utxoLockState struct {
	BondTxID    *ids.ID
	DepositTxID *ids.ID
}

func (ls utxoLockState) isLocked() bool    { return ls.BondTxID != nil || ls.DepositTxID != nil }
func (ls utxoLockState) isBonded() bool    { return ls.BondTxID != nil }
func (ls utxoLockState) isDeposited() bool { return ls.DepositTxID != nil }

type lockedUTXOsChainState interface {
	GetUTXOLockState(utxoID ids.ID) *utxoLockState
	UpdateAndProduceUTXOs(
		inputs []*avax.TransferableInput,
		inputIndexes []int,
		bondedOuts []*avax.TransferableOutput,
		notBondedOuts []*avax.TransferableOutput,
		txID ids.ID,
		bond bool,
	) (lockedUTXOsChainState, []*avax.UTXO, error)
	Unbond(bondTxID ids.ID) (lockedUTXOsChainState, error)
	Undeposit(depositTxID ids.ID) (lockedUTXOsChainState, error)
	SemanticVerifyLockInputs(inputs []*avax.TransferableInput, bond bool) error

	Apply(InternalState)
}

// lockedUTXOsChainStateImpl is a copy on write implementation for versioning
// the lock state of utxos. None of the slices, maps, or pointers should be modified
// after initialization.
type lockedUTXOsChainStateImpl struct {
	bonds       map[ids.ID]ids.Set       // bondTx.ID -> bondedUTXO.ID -> nil
	deposits    map[ids.ID]ids.Set       // depositTx.ID -> depositedUTXO.ID -> nil
	lockedUTXOs map[ids.ID]utxoLockState // lockedUTXO.ID -> { bondTx.ID, depositTx.ID }

	updatedUTXOs map[ids.ID]utxoLockState // utxo.ID -> { bondTx.ID, depositTx.ID }
}

func (cs *lockedUTXOsChainStateImpl) GetUTXOLockState(utxoID ids.ID) *utxoLockState {
	utxoLockState, ok := cs.lockedUTXOs[utxoID]
	if !ok {
		return nil
	}
	return &utxoLockState
}

func (cs *lockedUTXOsChainStateImpl) updateUTXOs(updatedUTXOStates map[ids.ID]utxoLockState) (lockedUTXOsChainState, error) {
	newCS := &lockedUTXOsChainStateImpl{
		bonds:        make(map[ids.ID]ids.Set, len(cs.bonds)+1),
		deposits:     make(map[ids.ID]ids.Set, len(cs.deposits)+1),
		lockedUTXOs:  make(map[ids.ID]utxoLockState, len(cs.lockedUTXOs)+1),
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
		switch {
		case oldLockedUTXOState.BondTxID == nil && newLockedUTXO.BondTxID != nil:
			// bonding not-bonded utxo
			bondTxID := *newLockedUTXO.BondTxID
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				bond = ids.Set{}
				newCS.bonds[bondTxID] = bond
			}
			bond.Add(newLockedUTXOID)
		case oldLockedUTXOState.BondTxID != nil && newLockedUTXO.BondTxID == nil:
			// unbonding bonded utxo
			bondTxID := *oldLockedUTXOState.BondTxID
			bond := newCS.bonds[bondTxID]
			if bond == nil {
				return nil, fmt.Errorf("old utxo lock state has not-nil bondTxID, but there is no such bond in state: %v",
					bondTxID)
			}
			bond.Remove(newLockedUTXOID)
		case oldLockedUTXOState.BondTxID != newLockedUTXO.BondTxID:
			return nil, fmt.Errorf("attempt to bond bonded utxo (utxoID: %v, oldBondID: %v, newBondID: %v)",
				newLockedUTXOID, oldLockedUTXOState.BondTxID, newLockedUTXO.BondTxID)
		}

		// updating deposit state
		switch {
		case oldLockedUTXOState.DepositTxID == nil && newLockedUTXO.DepositTxID != nil:
			// depositing not-deposited utxo
			depositTxID := *newLockedUTXO.DepositTxID
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				deposit = ids.Set{}
				newCS.deposits[depositTxID] = deposit
			}
			deposit.Add(newLockedUTXOID)
		case oldLockedUTXOState.DepositTxID != nil && newLockedUTXO.DepositTxID == nil:
			// undepositing deposited utxo
			depositTxID := *oldLockedUTXOState.DepositTxID
			deposit := newCS.deposits[depositTxID]
			if deposit == nil {
				return nil, fmt.Errorf("old utxo lock state has not-nil depositTxID, but there is no such deposit in state: %v",
					depositTxID)
			}
			deposit.Remove(newLockedUTXOID)
		case oldLockedUTXOState.DepositTxID != newLockedUTXO.DepositTxID:
			return nil, fmt.Errorf("attempt to deposit deposited utxo (utxoID: %v, oldDepositID: %v, newDepositID: %v)",
				newLockedUTXOID, oldLockedUTXOState.DepositTxID, newLockedUTXO.DepositTxID)
		}

		// updating utxo lock state
		if newLockedUTXO.BondTxID == nil && newLockedUTXO.DepositTxID == nil {
			delete(newCS.lockedUTXOs, newLockedUTXOID)
		} else {
			newCS.lockedUTXOs[newLockedUTXOID] = utxoLockState{
				BondTxID:    newLockedUTXO.BondTxID,
				DepositTxID: newLockedUTXO.DepositTxID,
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
// - [inputIndexes] Indexes of inputs that produced outputs. First for notLockedOuts, then for lockedOuts
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
	updatedUTXOLockStates := make(map[ids.ID]utxoLockState, lockedUTXOsCount*2)

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
			updatedUTXOLockStates[utxo.InputID()] = utxoLockState{
				BondTxID:    nil,
				DepositTxID: cs.GetUTXOLockState(consumedUTXOID).DepositTxID,
			}
		} else {
			updatedUTXOLockStates[utxo.InputID()] = utxoLockState{
				BondTxID:    cs.GetUTXOLockState(consumedUTXOID).BondTxID,
				DepositTxID: nil,
			}
		}

		// removing consumed utxo from lock state
		updatedUTXOLockStates[consumedUTXOID] = utxoLockState{
			BondTxID:    nil,
			DepositTxID: nil,
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
			updatedUTXOLockStates[utxo.InputID()] = utxoLockState{
				BondTxID:    &txID,
				DepositTxID: cs.GetUTXOLockState(consumedUTXOID).DepositTxID,
			}
		} else {
			updatedUTXOLockStates[utxo.InputID()] = utxoLockState{
				BondTxID:    cs.GetUTXOLockState(consumedUTXOID).BondTxID,
				DepositTxID: &txID,
			}
		}

		// removing consumed utxo from lock state
		if _, ok := updatedUTXOLockStates[consumedUTXOID]; !ok {
			updatedUTXOLockStates[consumedUTXOID] = utxoLockState{
				BondTxID:    nil,
				DepositTxID: nil,
			}
		}
		outIndex++
	}

	newlyLockedUTXOsState, err := cs.updateUTXOs(updatedUTXOLockStates)
	if err != nil {
		return nil, nil, err
	}

	return newlyLockedUTXOsState, producedUTXOs, nil
}

func (cs *lockedUTXOsChainStateImpl) Unbond(bondTxID ids.ID) (lockedUTXOsChainState, error) {
	bondedUTXOIDs := cs.bonds[bondTxID]
	updatedUTXOLockStates := make(map[ids.ID]utxoLockState, len(bondedUTXOIDs))
	for utxoID := range bondedUTXOIDs {
		updatedUTXOLockStates[utxoID] = utxoLockState{
			BondTxID:    nil,
			DepositTxID: cs.GetUTXOLockState(utxoID).DepositTxID,
		}
	}

	newlyLockedUTXOsState, err := cs.updateUTXOs(updatedUTXOLockStates)
	if err != nil {
		return nil, err
	}

	return newlyLockedUTXOsState, nil
}

// ! @evlekht we'll also need more flexible method for graduall unlock
// ! most likely will be done with spending
func (cs *lockedUTXOsChainStateImpl) Undeposit(depositTxID ids.ID) (lockedUTXOsChainState, error) {
	depositedUTXOIDs := cs.deposits[depositTxID]
	updatedUTXOLockStates := make(map[ids.ID]utxoLockState, len(depositedUTXOIDs))
	for utxoID := range depositedUTXOIDs {
		updatedUTXOLockStates[utxoID] = utxoLockState{
			BondTxID:    cs.GetUTXOLockState(utxoID).BondTxID,
			DepositTxID: nil,
		}
	}

	newlyLockedUTXOsState, err := cs.updateUTXOs(updatedUTXOLockStates)
	if err != nil {
		return nil, err
	}

	return newlyLockedUTXOsState, nil
}

// Verify that [inputs] are semantically valid for locking.
// Arguments:
// - [inputs] Inputs that will produce locked outs
// - [bond] true if we'r bonding, false if depositing
func (cs *lockedUTXOsChainStateImpl) SemanticVerifyLockInputs(
	inputs []*avax.TransferableInput,
	bond bool,
) error {
	for _, in := range inputs {
		consumedUTXOLockState := cs.GetUTXOLockState(in.InputID())
		if bond && consumedUTXOLockState.isBonded() ||
			!bond && consumedUTXOLockState.isDeposited() {
			return fmt.Errorf("utxo consumed for locking are already locked")
		}
	}
	return nil
}
