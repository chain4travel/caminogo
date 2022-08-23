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
	"errors"
	"fmt"
	"time"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/snow"
	"github.com/chain4travel/caminogo/utils/constants"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
	"github.com/chain4travel/caminogo/vms/platformvm/reward"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"

	safemath "github.com/chain4travel/caminogo/utils/math"
)

var (
	errNilTx                     = errors.New("tx is nil")
	errWeightTooSmall            = errors.New("weight of this validator is too low")
	errWeightTooLarge            = errors.New("weight of this validator is too large")
	errStakeTooShort             = errors.New("staking period is too short")
	errStakeTooLong              = errors.New("staking period is too long")
	errInsufficientDelegationFee = errors.New("staker charges an insufficient delegation fee")
	errFutureStakeTime           = fmt.Errorf("staker is attempting to start staking more than %s ahead of the current chain time", maxFutureStartTime)
	errTooManyShares             = fmt.Errorf("a staker can only require at most %d shares from delegators", reward.PercentDenominator)

	_ UnsignedProposalTx = &UnsignedAddValidatorTx{}
	_ TimedTx            = &UnsignedAddValidatorTx{}
)

// UnsignedAddValidatorTx is an unsigned addValidatorTx
type UnsignedAddValidatorTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// Describes the delegatee
	Validator Validator `serialize:"true" json:"validator"`
	// Where to send bonded tokens when done validating
	Bond         []*avax.TransferableOutput `serialize:"true" json:"stake"`
	InputIndexes []uint8                    `serialize:"true" json:"bondInputIndexes"`
	// Where to send staking rewards when done validating
	RewardsOwner Owner `serialize:"true" json:"rewardsOwner"`
	// Fee this validator charges delegators as a percentage, times 10,000
	// For example, if this validator has Shares=300,000 then they take 30% of rewards from delegators
	Shares uint32 `serialize:"true" json:"shares"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedAddValidatorTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedAddValidatorTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
	for _, out := range tx.Bond {
		out.FxID = secp256k1fx.ID
		out.InitCtx(ctx)
	}
	tx.RewardsOwner.InitCtx(ctx)
}

// StartTime of this validator
func (tx *UnsignedAddValidatorTx) StartTime() time.Time {
	return tx.Validator.StartTime()
}

// EndTime of this validator
func (tx *UnsignedAddValidatorTx) EndTime() time.Time {
	return tx.Validator.EndTime()
}

// Weight of this validator
func (tx *UnsignedAddValidatorTx) Weight() uint64 {
	return tx.Validator.Weight()
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *UnsignedAddValidatorTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return errNilTx
	case tx.syntacticallyVerified: // already passed syntactic verification
		return nil
	case tx.Shares > reward.PercentDenominator: // Ensure delegators shares are in the allowed amount
		return errTooManyShares
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}
	if err := verify.All(&tx.Validator, tx.RewardsOwner); err != nil {
		return fmt.Errorf("failed to verify validator or rewards owner: %w", err)
	}

	totalBond := uint64(0)
	for _, out := range tx.Bond {
		if err := out.Verify(); err != nil {
			return fmt.Errorf("failed to verify output: %w", err)
		}
		newTotalBond, err := safemath.Add64(totalBond, out.Output().Amount())
		if err != nil {
			return err
		}
		totalBond = newTotalBond
	}

	switch {
	case !avax.IsSortedTransferableOutputs(tx.Bond, Codec):
		return errOutputsNotSorted
	case totalBond != tx.Validator.Wght:
		return fmt.Errorf("validator weight %d is not equal to total bond amount %d", tx.Validator.Wght, totalBond)
	}

	outs := make([]*avax.TransferableOutput, len(tx.Outs)+len(tx.Bond))
	copy(outs, tx.Outs)
	copy(outs[len(tx.Outs):], tx.Bond)
	if err := syntacticVerifyInputIndexes(tx.Ins, tx.InputIndexes, outs); err != nil {
		return err
	}

	// cache that this is valid
	tx.syntacticallyVerified = true
	return nil
}

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedAddValidatorTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	startTime := tx.StartTime()
	maxLocalStartTime := vm.clock.Time().Add(maxFutureStartTime)
	if startTime.After(maxLocalStartTime) {
		return errFutureStakeTime
	}

	_, _, err := tx.Execute(vm, parentState, stx)
	// We ignore [errFutureStakeTime] here because an advanceTimeTx will be
	// issued before this transaction is issued.
	if errors.Is(err, errFutureStakeTime) {
		return nil
	}
	return err
}

// Execute this transaction.
func (tx *UnsignedAddValidatorTx) Execute(
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

	switch {
	case tx.Validator.Wght < vm.MinValidatorStake: // Ensure validator is staking at least the minimum amount
		return nil, nil, errWeightTooSmall
	case tx.Validator.Wght > vm.MaxValidatorStake: // Ensure validator isn't staking too much
		return nil, nil, errWeightTooLarge
	case tx.Shares < vm.MinDelegationFee:
		return nil, nil, errInsufficientDelegationFee
	}

	duration := tx.Validator.Duration()
	switch {
	case duration < vm.MinStakeDuration: // Ensure staking length is not too short
		return nil, nil, errStakeTooShort
	case duration > vm.MaxStakeDuration: // Ensure staking length is not too long
		return nil, nil, errStakeTooLong
	}

	currentStakers := parentState.CurrentStakerChainState()
	pendingStakers := parentState.PendingStakerChainState()
	lockedUTXOsState := parentState.LockedUTXOsChainState()

	outs := make([]*avax.TransferableOutput, len(tx.Outs)+len(tx.Bond))
	copy(outs, tx.Outs)
	copy(outs[len(tx.Outs):], tx.Bond)

	if vm.bootstrapped.GetValue() {
		currentTimestamp := parentState.GetTimestamp()
		// Ensure the proposed validator starts after the current time
		startTime := tx.StartTime()
		if !currentTimestamp.Before(startTime) {
			return nil, nil, fmt.Errorf(
				"validator's start time (%s) at or before current timestamp (%s)",
				startTime,
				currentTimestamp,
			)
		}

		// Ensure this validator isn't currently a validator.
		_, err := currentStakers.GetValidator(tx.Validator.NodeID)
		if err == nil {
			return nil, nil, fmt.Errorf(
				"%s is already a primary network validator",
				tx.Validator.NodeID.PrefixedString(constants.NodeIDPrefix),
			)
		}
		if err != database.ErrNotFound {
			return nil, nil, fmt.Errorf(
				"failed to find whether %s is a validator: %w",
				tx.Validator.NodeID.PrefixedString(constants.NodeIDPrefix),
				err,
			)
		}

		// Ensure this validator isn't about to become a validator.
		_, err = pendingStakers.GetValidatorTx(tx.Validator.NodeID)
		if err == nil {
			return nil, nil, fmt.Errorf(
				"%s is about to become a primary network validator",
				tx.Validator.NodeID.PrefixedString(constants.NodeIDPrefix),
			)
		}
		if err != database.ErrNotFound {
			return nil, nil, fmt.Errorf(
				"failed to find whether %s is about to become a validator: %w",
				tx.Validator.NodeID.PrefixedString(constants.NodeIDPrefix),
				err,
			)
		}

		// Verify the flowcheck
		if err := vm.semanticVerifySpend(parentState, tx, tx.Ins, outs, stx.Creds, vm.AddStakerTxFee, vm.ctx.AVAXAssetID); err != nil {
			return nil, nil, fmt.Errorf("failed semanticVerifySpend: %w", err)
		}

		if err := lockedUTXOsState.SemanticVerifyLockInputs(tx.Ins, true); err != nil {
			return nil, nil, fmt.Errorf("failed semanticVerifyLock: %w", err)
		}

		// Make sure the tx doesn't start too far in the future. This is done
		// last to allow SemanticVerification to explicitly check for this
		// error.
		maxStartTime := currentTimestamp.Add(maxFutureStartTime)
		if startTime.After(maxStartTime) {
			return nil, nil, errFutureStakeTime
		}
	}

	// Set up the state if this tx is committed

	txID := tx.ID()

	newlyPendingStakers := pendingStakers.AddStaker(stx)

	newlyLockedUTXOsState, utxos, err := lockedUTXOsState.UpdateAndProduceUTXOs(
		tx.Ins,
		tx.InputIndexes,
		tx.Outs,
		tx.Bond,
		txID,
		true,
	)
	if err != nil {
		return nil, nil, err
	}

	onCommitState := newVersionedState(parentState, currentStakers, newlyPendingStakers, newlyLockedUTXOsState)

	// Consume the UTXOS
	consumeInputs(onCommitState, tx.Ins)
	// Produce the UTXOS
	for _, utxo := range utxos {
		onCommitState.AddUTXO(utxo)
	}

	// Set up the state if this tx is aborted
	onAbortState := newVersionedState(parentState, currentStakers, pendingStakers, lockedUTXOsState)
	// Consume the UTXOS
	consumeInputs(onAbortState, tx.Ins)
	// Produce the UTXOS
	produceOutputs(onAbortState, txID, vm.ctx.AVAXAssetID, outs)

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if the proposed validators start time is
// after the current wall clock time,
func (tx *UnsignedAddValidatorTx) InitiallyPrefersCommit(vm *VM) bool {
	return tx.StartTime().After(vm.clock.Time())
}

// NewAddValidatorTx returns a new addValidatorTx
func (vm *VM) newAddValidatorTx(
	bondAmt, // Amount the validator bonds
	startTime, // Unix time they start validating
	endTime uint64, // Unix time they stop validating
	nodeID ids.ShortID, // ID of the node we want to validate with
	rewardAddress ids.ShortID, // Address to send reward to, if applicable
	shares uint32, // 10,000 times percentage of reward taken from delegators
	keys []*crypto.PrivateKeySECP256K1R, // Keys providing the staked tokens
	changeAddr ids.ShortID, // Address to send change to, if there is any
) (*Tx, error) {
	ins, notBondedOuts, bondedOuts, inputIndexes, signers, err := vm.spend(keys, bondAmt, vm.AddStakerTxFee, changeAddr, spendModeBond)
	if err != nil {
		return nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}
	// Create the tx
	utx := &UnsignedAddValidatorTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         notBondedOuts,
		}},
		Validator: Validator{
			NodeID: nodeID,
			Start:  startTime,
			End:    endTime,
			Wght:   bondAmt,
		},
		Bond:         bondedOuts,
		InputIndexes: inputIndexes,
		RewardsOwner: &secp256k1fx.OutputOwners{
			Locktime:  0,
			Threshold: 1,
			Addrs:     []ids.ShortID{rewardAddress},
		},
		Shares: shares,
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, signers); err != nil {
		return nil, err
	}
	return tx, utx.SyntacticVerify(vm.ctx)
}
