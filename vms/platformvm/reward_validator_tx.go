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
	"errors"
	"fmt"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/snow"
	"github.com/chain4travel/caminogo/utils/math"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

var (
	errShouldBeDSValidator     = errors.New("expected validator to be in the primary network")
	errWrongTxType             = errors.New("wrong transaction type")
	errTxBodyMissmatch         = errors.New("wrong system tx body")
	errToEarlyValidatorRemoval = errors.New("attempting to remove validator before their end time")
	errWrongValidatorRemoval   = errors.New("attempting to remove wrong validator")

	_ UnsignedProposalTx = &UnsignedRewardValidatorTx{}
)

// UnsignedRewardValidatorTx is a transaction that represents a proposal to
// remove a validator that is currently validating from the validator set.
//
// If this transaction is accepted and the next block accepted is a Commit
// block, the validator is removed and the address that the validator specified
// receives the staked AVAX as well as a validating reward.
//
// If this transaction is accepted and the next block accepted is an Abort
// block, the validator is removed and the address that the validator specified
// receives the staked AVAX but no reward.
type UnsignedRewardValidatorTx struct {
	avax.Metadata

	// The inputs to this transaction
	Ins []*avax.TransferableInput `serialize:"true" json:"inputs"`

	// The outputs of this transaction
	Outs []*avax.TransferableOutput `serialize:"true" json:"outputs"`

	// Input indexes that produced outputs (output[i] produced by inputs[inputIndexes[i]])
	InputIndexes []uint32 `serialize:"true" json:"inputIndexes"`

	// ID of the tx that created the validator being removed/rewarded
	ValidatorTxID ids.ID `serialize:"true" json:"validatorTxID"`

	// Marks if this validator should be rewarded according to this node.
	shouldPreferCommit bool

	// true if this transaction has already passed syntactic verification
	syntacticallyVerified bool
}

func (tx *UnsignedRewardValidatorTx) InitCtx(ctx *snow.Context) {
	for _, in := range tx.Ins {
		in.FxID = secp256k1fx.ID
	}
	for _, out := range tx.Outs {
		out.FxID = secp256k1fx.ID
		out.InitCtx(ctx)
	}
}

func (tx *UnsignedRewardValidatorTx) InputIDs() ids.Set {
	return nil
}

func (tx *UnsignedRewardValidatorTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return errNilTx
	case tx.ValidatorTxID == ids.Empty:
		return errInvalidID
	case tx.syntacticallyVerified: // already passed syntactic verification
		return nil
	}

	// cache that this is valid
	tx.syntacticallyVerified = true

	return nil
}

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedRewardValidatorTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	_, _, err := tx.Execute(vm, parentState, stx)
	return err
}

// Execute this transaction.
//
// The current validating set must have at least one member.
// The next validator to be removed must be the validator specified in this block.
// The next validator to be removed must be have an end time equal to the current
//
//	chain timestamp.
func (tx *UnsignedRewardValidatorTx) Execute(
	vm *VM,
	parentState MutableState,
	stx *Tx,
) (
	VersionedState,
	VersionedState,
	error,
) {
	// Verify the tx is well-formed
	if err := tx.SyntacticVerify(vm.ctx); err != nil {
		return nil, nil, err
	}

	if len(stx.Creds) != 0 {
		return nil, nil, errWrongNumberOfCredentials
	}

	currentStakers := parentState.CurrentStakerChainState()
	validatorTx, stakerReward, err := currentStakers.GetNextStaker()
	if err == database.ErrNotFound {
		return nil, nil, fmt.Errorf("failed to get next staker stop time: %w", err)
	}
	if err != nil {
		return nil, nil, err
	}

	validatorTxID := validatorTx.ID()
	if validatorTxID != tx.ValidatorTxID {
		return nil, nil, fmt.Errorf("removing validator (%s) instead of (%s): %w",
			tx.ValidatorTxID,
			validatorTxID,
			errWrongValidatorRemoval,
		)
	}

	addValidatorTx, ok := validatorTx.UnsignedTx.(*UnsignedAddValidatorTx)
	if !ok {
		return nil, nil, errShouldBeDSValidator
	}

	// Verify that the chain's timestamp is the validator's end time
	currentTime := parentState.GetTimestamp()
	if endTime := addValidatorTx.EndTime(); !endTime.Equal(currentTime) {
		return nil, nil, fmt.Errorf("removing validator (%s) at %s: %w",
			validatorTxID,
			endTime,
			errToEarlyValidatorRemoval,
		)
	}

	lockedUTXOsState := parentState.LockedUTXOsChainState()

	// Verify that tx body is valid

	ins, outs, inputIndexes, err := vm.unlock(parentState, []ids.ID{validatorTxID}, LockStateBonded)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}

	var expectedTx UnsignedTx = &UnsignedRewardValidatorTx{
		Ins:           ins,
		Outs:          outs,
		InputIndexes:  inputIndexes,
		ValidatorTxID: validatorTxID,
	}
	expectedBytes, err := Codec.Marshal(CodecVersion, &expectedTx)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't marshal UnsignedTx: %w", err)
	}

	if !bytes.Equal(tx.UnsignedBytes(), expectedBytes) {
		return nil, nil, errTxBodyMissmatch
	}

	// Set up the state if this tx is committed

	rewardValidatorTxID := tx.ID()

	updatedUTXOLockStates, utxos := lockedUTXOsState.ProduceUTXOsAndLockState(
		tx.Ins,
		tx.InputIndexes,
		tx.Outs,
		rewardValidatorTxID,
	)

	newlyLockedUTXOsState, err := lockedUTXOsState.UpdateLockState(updatedUTXOLockStates)
	if err != nil {
		return nil, nil, err
	}

	newlyCurrentStakers, err := currentStakers.DeleteNextStaker()
	if err != nil {
		return nil, nil, err
	}

	pendingStakers := parentState.PendingStakerChainState()

	onCommitState := newVersionedState(parentState, newlyCurrentStakers, pendingStakers, newlyLockedUTXOsState)

	// Consume the UTXOS
	consumeInputs(onCommitState, tx.Ins)
	// Produce the UTXOS
	for _, utxo := range utxos {
		onCommitState.AddUTXO(utxo)
	}

	// Provide the reward here
	if stakerReward > 0 {
		outIntf, err := vm.fx.CreateOutput(stakerReward, addValidatorTx.RewardsOwner)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create output: %w", err)
		}
		out, ok := outIntf.(verify.State)
		if !ok {
			return nil, nil, errInvalidState
		}

		utxo := &avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        rewardValidatorTxID,
				OutputIndex: uint32(len(addValidatorTx.Outs)),
			},
			Asset: avax.Asset{ID: vm.ctx.AVAXAssetID},
			Out:   out,
		}

		onCommitState.AddUTXO(utxo)
		onCommitState.AddRewardUTXO(rewardValidatorTxID, utxo)
	}

	onAbortState := newVersionedState(parentState, newlyCurrentStakers, pendingStakers, newlyLockedUTXOsState)

	// Consume the UTXOS
	consumeInputs(onAbortState, tx.Ins)
	// Produce the UTXOS
	for _, utxo := range utxos {
		onAbortState.AddUTXO(utxo)
	}

	// If the reward is aborted, then the current supply should be decreased.
	currentSupply := onAbortState.GetCurrentSupply()
	newSupply, err := math.Sub64(currentSupply, stakerReward)
	if err != nil {
		return nil, nil, err
	}
	onAbortState.SetCurrentSupply(newSupply)

	// Handle reward preferences
	nodeID := addValidatorTx.Validator.ID()
	startTime := addValidatorTx.StartTime()

	uptime, err := vm.uptimeManager.CalculateUptimePercentFrom(nodeID, startTime)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate uptime: %w", err)
	}
	tx.shouldPreferCommit = uptime >= vm.UptimePercentage

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if this node thinks the validator
// should receive a staking reward.
//
// TODO: A validator should receive a reward only if they are sufficiently
// responsive and correct during the time they are validating.
// Right now they receive a reward if they're up (but not necessarily
// correct and responsive) for a sufficient amount of time
func (tx *UnsignedRewardValidatorTx) InitiallyPrefersCommit(*VM) bool {
	return tx.shouldPreferCommit
}

// RewardStakerTx creates a new transaction that proposes to remove the staker
// [validatorID] from the default validator set.
func (vm *VM) newRewardValidatorTx(validatorTxID ids.ID) (*Tx, error) {
	ins, outs, inputIndexes, err := vm.unlock(vm.internalState, []ids.ID{validatorTxID}, LockStateBonded)
	if err != nil {
		return nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}

	utx := &UnsignedRewardValidatorTx{
		Ins:           ins,
		Outs:          outs,
		InputIndexes:  inputIndexes,
		ValidatorTxID: validatorTxID,
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, nil); err != nil {
		return nil, err
	}
	return tx, utx.SyntacticVerify(vm.ctx)
}
