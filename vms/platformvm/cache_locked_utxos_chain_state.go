// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"errors"
	"fmt"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/hashing"
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
		utxoDB UTXOGetter,
		inputs []*avax.TransferableInput,
		outputs []*avax.TransferableOutput,
		lockState LockState,
		txID ids.ID,
	) (map[ids.ID]utxoLockState, []*avax.UTXO, error)
	UpdateLockState(updatedUTXOStates map[ids.ID]utxoLockState) (lockedUTXOsChainState, error)
	Apply(InternalState)
}

// lockedUTXOsChainStateImpl is a copy on write implementation for versioning
// the lock state of utxos. None of the slices, maps, or pointers should be modified
// after initialization.
type lockedUTXOsChainStateImpl struct {
	lockedUTXOs map[ids.ID]utxoLockState // lockedUTXO.ID -> { bondTx.ID, depositTx.ID }

	updatedUTXOs map[ids.ID]utxoLockState // utxo.ID -> { bondTx.ID, depositTx.ID }
}

func (cs *lockedUTXOsChainStateImpl) GetUTXOLockState(utxoID ids.ID) utxoLockState {
	return cs.lockedUTXOs[utxoID]
}

func (cs *lockedUTXOsChainStateImpl) GetBondedUTXOIDs(bondTxID ids.ID) ids.Set {
	utxoIDs := ids.Set{}
	for utxoID, lockState := range cs.lockedUTXOs {
		if lockState.BondTxID != nil && *lockState.BondTxID == bondTxID {
			utxoIDs.Add(utxoID)
		}
	}
	// TODO@ updatedUTXOs ?
	return utxoIDs
}

func (cs *lockedUTXOsChainStateImpl) GetDepositedUTXOIDs(depositTxID ids.ID) ids.Set {
	utxoIDs := ids.Set{}
	for utxoID, lockState := range cs.lockedUTXOs {
		if lockState.DepositTxID != nil && *lockState.DepositTxID == depositTxID {
			utxoIDs.Add(utxoID)
		}
	}
	// TODO@ updatedUTXOs ?
	return utxoIDs
}

// Creates the updated locked chain state
// Arguments:
// - [updatedUTXOLockStates] locked state of produced utxos
// Returns:
// - [newlyLockedUTXOsState] updated locked UTXOs chain state
func (cs *lockedUTXOsChainStateImpl) UpdateLockState(updatedUTXOStates map[ids.ID]utxoLockState) (lockedUTXOsChainState, error) {
	newCS := &lockedUTXOsChainStateImpl{
		lockedUTXOs:  make(map[ids.ID]utxoLockState, len(cs.lockedUTXOs)),
		updatedUTXOs: updatedUTXOStates,
	}

	for utxoID, lockIDs := range cs.lockedUTXOs {
		newCS.lockedUTXOs[utxoID] = lockIDs
	}

	for utxoID, lockState := range updatedUTXOStates {
		if lockState.BondTxID == nil && lockState.DepositTxID == nil {
			delete(newCS.lockedUTXOs, utxoID)
		} else {
			newCS.lockedUTXOs[utxoID] = utxoLockState{
				BondTxID:    lockState.BondTxID,
				DepositTxID: lockState.DepositTxID,
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
	utxoDB UTXOGetter,
	inputs []*avax.TransferableInput,
	outputs []*avax.TransferableOutput,
	lockState LockState,
	txID ids.ID,
) (
	map[ids.ID]utxoLockState, // updatedUTXOLockStates
	[]*avax.UTXO, // producedUTXOs
	error,
) {
	if lockState != LockStateBonded && lockState != LockStateDeposited {
		return nil, nil, errInvalidTargetLockState
	}

	producedUTXOs := make([]*avax.UTXO, len(outputs))
	updatedUTXOLockStates := make(map[ids.ID]utxoLockState, len(outputs)*2)

	insOutIndexes, err := GetInsOuts(utxoDB, inputs, outputs, lockState)
	if err != nil {
		return nil, nil, err
	}

	for inputIndex, outIndexes := range insOutIndexes {
		input := inputs[inputIndex]
		for _, outIndex := range outIndexes {
			output := outputs[outIndex]
			consumedUTXOID := input.InputID()

			// produce new UTXO
			producedUTXO := &avax.UTXO{
				UTXOID: avax.UTXOID{
					TxID:        txID,
					OutputIndex: uint32(inputIndex),
				},
				Asset: avax.Asset{ID: input.AssetID()},
				Out:   output.Out,
			}
			producedUTXOs[inputIndex] = producedUTXO

			inLockState := LockStateUnlocked
			if lockedIn, ok := input.In.(*LockedIn); ok {
				inLockState = lockedIn.LockState
			}

			outLockState := LockStateUnlocked
			if lockedOut, ok := output.Out.(*LockedOut); ok {
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
	}

	return updatedUTXOLockStates, producedUTXOs, nil
}

func GetInsOuts(
	utxoDB UTXOGetter,
	inputs []*avax.TransferableInput,
	outputs []*avax.TransferableOutput,
	lockState LockState,
) (
	outs [][]int,
	err error,
) {
	if lockState != LockStateBonded && lockState != LockStateDeposited {
		return nil, errInvalidTargetLockState
	}

	inputOuts := make([][]int, len(inputs))
	trackedOuts := make(map[int]struct{}, len(outputs))

	for inputIndex, in := range inputs {
		utxo, err := utxoDB.GetUTXO(in.InputID())
		if err != nil {
			return nil, err
		}
		out := utxo.Out
		if lockedOut, ok := utxo.Out.(*LockedOut); ok {
			out = lockedOut.TransferableOut
		}
		consumedOwnerID, err := getOwnerID(out)
		if err != nil {
			return nil, err
		}

		inputLockState := LockStateUnlocked
		if lockedIn, ok := in.In.(*LockedIn); ok {
			inputLockState = lockedIn.LockState
		}

		trackedAmount := uint64(0)

		for outIndex, output := range outputs {
			if _, ok := trackedOuts[outIndex]; ok {
				continue
			}

			out := output.Out

			sum := trackedAmount + out.Amount()
			if trackedAmount != 0 && sum != in.In.Amount() {
				continue
			}

			outputLockState := LockStateUnlocked
			if lockedOut, ok := out.(*LockedOut); ok {
				outputLockState = lockedOut.LockState
				out = lockedOut.TransferableOut
			}

			if inputLockState|lockState != outputLockState {
				continue
			}

			ownerID, err := getOwnerID(out)
			if err != nil {
				return nil, err
			}

			if ownerID != consumedOwnerID {
				continue
			}

			inputOuts[inputIndex] = append(inputOuts[inputIndex], outIndex)
			trackedOuts[outIndex] = struct{}{}
			trackedAmount = sum

			if sum == in.In.Amount() {
				break
			}
		}
		if trackedAmount != in.In.Amount() {
			return nil, errors.New("error") // TODO@ err
		}
	}
	return inputOuts, nil
}

func getOwnerID(out interface{}) (ids.ID, error) {
	owned, ok := out.(Owned)
	if !ok {
		return ids.Empty, errUnknownOwnersType
	}
	owner := owned.Owners()
	ownerBytes, err := Codec.Marshal(CodecVersion, owner)
	if err != nil {
		return ids.Empty, fmt.Errorf("couldn't marshal owner: %w", err)
	}

	return hashing.ComputeHash256Array(ownerBytes), nil
}
