// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"errors"
	"fmt"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/vms/components/avax"
)

var (
	errLockingLockedUTXO    = errors.New("utxo consumed for locking are already locked")
	errWrongInputIndexesLen = errors.New("inputIndexes len doesn't match outputs len")
	errBurningLockedUTXO    = errors.New("trying to burn locked utxo")
	errLockedInsOrOuts      = errors.New("transaction body has locked inputs or outs, but that's now allowed")
	errWrongProducedAmount  = errors.New("produced more tokens, than input had")

	_ lockedUTXOsChainState = &lockedUTXOsChainStateImpl{}
)

type utxoLockState struct {
	BondTxID    *ids.ID
	DepositTxID *ids.ID
}

type lockedUTXOsChainState interface {
	GetUTXOLockState(utxoID ids.ID) utxoLockState
	GetBondedUTXOIDs(bondTxID ids.ID) ids.Set
	GetDepositedUTXOIDs(depositTxID ids.ID) ids.Set
	ProduceUTXOsAndLockState(
		inputs []*avax.TransferableInput,
		inputIndexes []uint32,
		outputs []*avax.TransferableOutput,
		txID ids.ID,
	) (map[ids.ID]utxoLockState, []*avax.UTXO)
	UpdateLockState(updatedUTXOStates map[ids.ID]utxoLockState) (lockedUTXOsChainState, error)
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

func (cs *lockedUTXOsChainStateImpl) GetUTXOLockState(utxoID ids.ID) utxoLockState {
	return cs.lockedUTXOs[utxoID]
}

func (cs *lockedUTXOsChainStateImpl) GetBondedUTXOIDs(bondTxID ids.ID) ids.Set {
	return cs.bonds[bondTxID]
}

func (cs *lockedUTXOsChainStateImpl) GetDepositedUTXOIDs(depositTxID ids.ID) ids.Set {
	return cs.deposits[depositTxID]
}

// Creates the updated locked chain state from the produced utxos
// Arguments:
// - [updatedUTXOLockStates] locked state of produced utxos
// Returns:
// - [newlyLockedUTXOsState] updated locked UTXOs chain state
func (cs *lockedUTXOsChainStateImpl) UpdateLockState(updatedUTXOStates map[ids.ID]utxoLockState) (lockedUTXOsChainState, error) {
	newCS := &lockedUTXOsChainStateImpl{
		bonds:        make(map[ids.ID]ids.Set, len(cs.bonds)),
		deposits:     make(map[ids.ID]ids.Set, len(cs.deposits)),
		lockedUTXOs:  make(map[ids.ID]utxoLockState, len(cs.lockedUTXOs)),
		updatedUTXOs: updatedUTXOStates,
	}

	for bondTxID, utxoIDs := range cs.bonds {
		newCS.bonds[bondTxID] = utxoIDs.Clone()
	}
	for depositTxID, utxoIDs := range cs.deposits {
		newCS.deposits[depositTxID] = utxoIDs.Clone()
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
			if bond.Len() == 0 {
				delete(newCS.bonds, bondTxID)
			}
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
			if deposit.Len() == 0 {
				delete(newCS.deposits, depositTxID)
			}
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

// Creates utxos and utxoLockStates from given ins and outs
// Arguments:
// - [inputs] Inputs that produced this outputs
// - [inputIndexes] Indexes of inputs that produced outputs. First for notLockedOuts, then for lockedOuts
// - [outputs] Both locked and unlocked outputs
// - [txID] ID for transaction that produced this inputs and outputs
// Returns:
// - [updatedUTXOLockStates] locked state of produced utxos
// - [producedUTXOs] utxos with produced outputs
// Precondition: arguments must be syntacticly valid in conjunction
func (cs *lockedUTXOsChainStateImpl) ProduceUTXOsAndLockState(
	inputs []*avax.TransferableInput,
	inputIndexes []uint32,
	outputs []*avax.TransferableOutput,
	txID ids.ID,
) (
	map[ids.ID]utxoLockState, // updatedUTXOLockStates
	[]*avax.UTXO, // producedUTXOs
) {
	producedUTXOs := make([]*avax.UTXO, len(outputs))
	updatedUTXOLockStates := make(map[ids.ID]utxoLockState, len(outputs)*2)

	for outIndex, out := range outputs {
		input := inputs[inputIndexes[outIndex]]
		out := out.Output()
		consumedUTXOID := input.InputID()

		// produce new UTXO
		producedUTXO := &avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(outIndex),
			},
			Asset: avax.Asset{ID: input.AssetID()},
			Out:   out,
		}
		producedUTXOs[outIndex] = producedUTXO

		inLockState := LockStateUnlocked
		if lockedIn, ok := input.In.(*LockedIn); ok {
			inLockState = lockedIn.LockState
		}

		outLockState := LockStateUnlocked
		if lockedOut, ok := out.(*LockedOut); ok {
			outLockState = lockedOut.LockState
		}

		var depositTxID *ids.ID
		var bondTxID *ids.ID

		switch {
		case inLockState.isDeposited() && !outLockState.isDeposited():
			depositTxID = nil
		case inLockState.isDeposited() && outLockState.isDeposited():
			depositTxID = cs.lockedUTXOs[consumedUTXOID].DepositTxID
		case !inLockState.isDeposited() && outLockState.isDeposited():
			depositTxID = &txID
		}

		switch {
		case inLockState.isBonded() && !outLockState.isBonded():
			bondTxID = nil
		case inLockState.isBonded() && outLockState.isBonded():
			bondTxID = cs.lockedUTXOs[consumedUTXOID].BondTxID
		case !inLockState.isBonded() && outLockState.isBonded():
			bondTxID = &txID
		}

		updatedUTXOLockStates[producedUTXO.InputID()] = utxoLockState{
			BondTxID:    bondTxID,
			DepositTxID: depositTxID,
		}

		// removing consumed utxo from lock state
		updatedUTXOLockStates[consumedUTXOID] = utxoLockState{
			BondTxID:    nil,
			DepositTxID: nil,
		}
	}

	return updatedUTXOLockStates, producedUTXOs
}
