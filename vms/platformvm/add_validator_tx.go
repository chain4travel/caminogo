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
	"github.com/chain4travel/caminogo/utils/math"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/components/verify"
	"github.com/chain4travel/caminogo/vms/secp256k1fx"
)

var (
	errNilTx                     = errors.New("tx is nil")
	errWeightTooSmall            = errors.New("weight of this validator is too low")
	errStakeTooShort             = errors.New("staking period is too short")
	errStakeTooLong              = errors.New("staking period is too long")
	errWrongBondAmount           = errors.New("wrong bond amount for this validator")
	errFutureStakeTime           = fmt.Errorf("staker is attempting to start staking more than %s ahead of the current chain time", maxFutureStartTime)
	errNodeSigVerificationFailed = errors.New("node signature verification failed")

	_ UnsignedProposalTx = &UnsignedAddValidatorTx{}
	_ TimedTx            = &UnsignedAddValidatorTx{}
)

// UnsignedAddValidatorTx is an unsigned addValidatorTx
type UnsignedAddValidatorTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	// Describes the validator
	Validator Validator `serialize:"true" json:"validator"`

	// Where to send staking rewards when done validating
	RewardsOwner Owner `serialize:"true" json:"rewardsOwner"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedAddValidatorTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedAddValidatorTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
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

func (tx *UnsignedAddValidatorTx) Bond() []*avax.TransferableOutput {
	var bond []*avax.TransferableOutput
	for _, output := range tx.Outs {
		if out, ok := output.Out.(*LockedOut); ok && out.LockState.isBonded() {
			bond = append(bond, output)
		}
	}
	return bond
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
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}
	if err := verify.All(&tx.Validator, tx.RewardsOwner); err != nil {
		return fmt.Errorf("failed to verify validator or rewards owner: %w", err)
	}

	totalBond := uint64(0)
	for _, out := range tx.Outs {
		if lockedOut, ok := out.Out.(*LockedOut); ok && lockedOut.LockState.isBonded() {
			newTotalBond, err := math.Add64(totalBond, lockedOut.Amount())
			if err != nil {
				return err
			}
			totalBond = newTotalBond
		}
	}

	if totalBond != tx.Validator.Wght {
		return fmt.Errorf("validator weight %d is not equal to total bond amount %d", tx.Validator.Wght, totalBond)
	}

	// TODO@
	// if err := syntacticVerifyLock(tx.Ins, tx.Outs, LockStateBonded, true); err != nil {
	// 	return err
	// }

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

	if tx.Validator.Wght != parentState.GetValidatorBondAmount() {
		return nil, nil, errWrongBondAmount
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
		if _, err := currentStakers.GetValidator(tx.Validator.NodeID); err == nil {
			return nil, nil, fmt.Errorf(
				"%s is already a primary network validator",
				tx.Validator.NodeID.PrefixedString(constants.NodeIDPrefix),
			)
		} else if err != database.ErrNotFound {
			return nil, nil, fmt.Errorf(
				"failed to find whether %s is a validator: %w",
				tx.Validator.NodeID.PrefixedString(constants.NodeIDPrefix),
				err,
			)
		}

		// Ensure this validator isn't about to become a validator.
		if _, err := pendingStakers.GetValidatorTx(tx.Validator.NodeID); err == nil {
			return nil, nil, fmt.Errorf(
				"%s is about to become a primary network validator",
				tx.Validator.NodeID.PrefixedString(constants.NodeIDPrefix),
			)
		} else if err != database.ErrNotFound {
			return nil, nil, fmt.Errorf(
				"failed to find whether %s is about to become a validator: %w",
				tx.Validator.NodeID.PrefixedString(constants.NodeIDPrefix),
				err,
			)
		}

		baseTxCredsLen := len(stx.Creds) - 1

		// Verify the flowcheck
		if err := vm.semanticVerifySpend(parentState, tx, tx.Ins, tx.Outs, stx.Creds[:baseTxCredsLen], vm.AddStakerTxFee, vm.ctx.AVAXAssetID); err != nil {
			return nil, nil, fmt.Errorf("failed semanticVerifySpend: %w", err)
		}

		// Verify that nodeId signature is present
		if err := vm.fx.VerifyPermission(tx,
			&secp256k1fx.Input{SigIndices: []uint32{0}},
			stx.Creds[baseTxCredsLen],
			&secp256k1fx.OutputOwners{Threshold: 1, Addrs: []ids.ShortID{tx.Validator.NodeID}}); err != nil {
			return nil, nil, errNodeSigVerificationFailed
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

	utxoLockStates, utxos, err := lockedUTXOsState.ProduceUTXOsAndLockState(
		parentState,
		tx.Ins,
		tx.Outs,
		LockStateBonded,
		txID,
	)
	if err != nil {
		return nil, nil, err
	}

	newlyLockedUTXOsState, err := lockedUTXOsState.UpdateLockState(utxoLockStates)
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

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if the proposed validators start time is
// after the current wall clock time,
func (tx *UnsignedAddValidatorTx) InitiallyPrefersCommit(vm *VM) bool {
	return tx.StartTime().After(vm.clock.Time())
}

// NewAddValidatorTx returns a new addValidatorTx
func (vm *VM) newAddValidatorTx(
	startTime, // Unix time they start validating
	endTime uint64, // Unix time they stop validating
	nodeID ids.ShortID, // ID of the node we want to validate with
	rewardAddress ids.ShortID, // Address to send reward to, if applicable
	keys []*crypto.PrivateKeySECP256K1R, // Keys providing the staked tokens
) (*Tx, error) {
	bondAmount := vm.internalState.GetValidatorBondAmount()
	ins, outs, signers, err := vm.spend(keys, bondAmount, vm.AddStakerTxFee, LockStateBonded)
	if err != nil {
		return nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}

	// Add nodeID signer at the end of input signers
	kc := secp256k1fx.NewKeychain(keys...)
	nodeIDSigner := make([]*crypto.PrivateKeySECP256K1R, 0, 1)
	if key, found := kc.Get(nodeID); found {
		nodeIDSigner = append(nodeIDSigner, key)
	}
	signers = append(signers, nodeIDSigner)

	// Create the tx
	utx := &UnsignedAddValidatorTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		Validator: Validator{
			NodeID: nodeID,
			Start:  startTime,
			End:    endTime,
			Wght:   bondAmount,
		},
		RewardsOwner: &secp256k1fx.OutputOwners{
			Locktime:  0,
			Threshold: 1,
			Addrs:     []ids.ShortID{rewardAddress},
		},
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, signers); err != nil {
		return nil, err
	}
	return tx, utx.SyntacticVerify(vm.ctx)
}
