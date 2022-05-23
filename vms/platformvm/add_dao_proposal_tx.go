// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"errors"
	"fmt"
	"time"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/snow"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/platformvm/dao"
)

var (
	errVotingTooShort   = errors.New("voting period is too short")
	errVotingTooLong    = errors.New("voting period is too long")
	errAlreadyValidator = errors.New("node is already a validator")

	_ UnsignedProposalTx = &UnsignedDaoProposalTx{}
)

// UnsignedDaoProposalTx is an unsigned daoProposalTx
type UnsignedDaoProposalTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	// Our Dao Proposal
	DaoProposal dao.DaoProposal `serialize:"true" json:"daoProposal"`

	// Where to send locked tokens when done voting
	Locks []*avax.TransferableOutput `serialize:"true" json:"locks"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedDaoProposalTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedDaoProposalTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *UnsignedDaoProposalTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return errNilTx
	case tx.syntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}

	if err := tx.DaoProposal.Verify(); err != nil {
		return fmt.Errorf("failed to verify DaoProposal: %w", err)
	}

	// cache that this is valid
	tx.syntacticallyVerified = true
	return nil
}

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedDaoProposalTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	_, _, err := tx.Execute(vm, parentState, stx)
	return err
}

// Execute this transaction.
func (tx *UnsignedDaoProposalTx) Execute(
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

	if tx.DaoProposal.Wght < vm.DaoConfig.MinProposalLock {
		return nil, nil, errWeightTooSmall
	}

	duration := tx.DaoProposal.Duration()
	switch {
	case duration < vm.DaoConfig.MinProposalDuration:
		return nil, nil, errVotingTooShort
	case duration > vm.DaoConfig.MaxProposalDuration: // Ensure staking length is not too long
		return nil, nil, errVotingTooLong
	}

	daoProposals := parentState.DaoProposalChainState()
	currentStakers := parentState.CurrentStakerChainState()
	pendingStakers := parentState.PendingStakerChainState()

	outs := make([]*avax.TransferableOutput, len(tx.Outs)+len(tx.Locks))
	copy(outs, tx.Outs)
	copy(outs[len(tx.Outs):], tx.Locks)

	if vm.bootstrapped.GetValue() {
		currentTimestamp := parentState.GetTimestamp()
		// Ensure the proposed validator starts after the current time
		startTime := tx.DaoProposal.StartTime()
		if !currentTimestamp.Before(startTime) {
			return nil, nil, fmt.Errorf(
				"daoProposal start time (%s) at or before current timestamp (%s)",
				startTime,
				currentTimestamp,
			)
		}

		// Ensure this proposal isn't already inserted
		_, err := daoProposals.GetProposal(tx.DaoProposal.ID())
		if err == nil {
			return nil, nil, fmt.Errorf("%s proposal already exists", tx.DaoProposal.ID().Hex())
		} else if err != database.ErrNotFound {
			return nil, nil, fmt.Errorf("cannot search for existing proposals: %w", err)
		}

		// Early exit if one tries to vote for an existing validator
		if tx.DaoProposal.ProposalType == dao.ProposalTypeAddValidator {
			if _, err := currentStakers.GetValidator(tx.DaoProposal.Proposer); err == nil {
				return nil, nil, errAlreadyValidator
			}
			if _, err := pendingStakers.GetValidator(tx.DaoProposal.Proposer); err == nil {
				return nil, nil, errAlreadyValidator
			}
		}

		// Verify the flowcheck
		if err := vm.semanticVerifySpend(parentState, tx, tx.Ins, outs, stx.Creds, vm.DaoConfig.ProposalTxFee, vm.ctx.AVAXAssetID); err != nil {
			return nil, nil, fmt.Errorf("failed semanticVerifySpend: %w", err)
		}

		// Make sure the tx doesn't start too far in the future. This is done
		// last to allow SemanticVerification to explicitly check for this
		// error.
		maxStartTime := vm.clock.Time().Add(maxFutureStartTime)
		if startTime.After(maxStartTime) {
			return nil, nil, errFutureStartTime
		}
	}

	// Set up the state if this tx is committed
	newlydaoProposals := daoProposals.AddProposal(stx)

	onCommitState := newVersionedState(vm, parentState, currentStakers, pendingStakers, newlydaoProposals)

	// Consume the UTXOS
	consumeInputs(onCommitState, tx.Ins)
	// Produce the UTXOS
	txID := tx.ID()
	produceOutputs(onCommitState, txID, vm.ctx.AVAXAssetID, tx.Outs)

	// Set up the state if this tx is aborted
	onAbortState := newVersionedState(vm, parentState, currentStakers, pendingStakers, daoProposals)
	// Consume the UTXOS
	consumeInputs(onAbortState, tx.Ins)
	// Produce the UTXOS
	produceOutputs(onAbortState, txID, vm.ctx.AVAXAssetID, outs)

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if this node thinks the vote
// should be inserted. This is currently true, if there are no duplicates
func (tx *UnsignedDaoProposalTx) InitiallyPrefersCommit(vm *VM) bool {
	return tx.DaoProposal.StartTime().After(vm.clock.Time())
}

// TimedTx Interface
func (tx *UnsignedDaoProposalTx) StartTime() time.Time { return tx.DaoProposal.StartTime() }
func (tx *UnsignedDaoProposalTx) EndTime() time.Time   { return tx.DaoProposal.EndTime() }
func (tx *UnsignedDaoProposalTx) Weight() uint64       { return tx.DaoProposal.Weight() }

// NewDaoProposalTx returns a new UnsignedDaoProposalTx
func (vm *VM) newDaoProposalTx(
	nodeID ids.ShortID, // The nodeID placing the proposal
	keys []*crypto.PrivateKeySECP256K1R, // Keys providing the staked tokens
	changeAddr ids.ShortID, // Address to send change to, if there is any
	proposalType, // The type of this proposal
	lockAmt, // The amount locked during voting period
	startTime, // The time voting starts
	endTime uint64, // The time voting ends
	proposal []byte, // this is the proposal message, stored in TX Metadata
) (*Tx, ids.ID, error) {
	ins, unlockedOuts, lockedOuts, signers, err := vm.stake(keys, lockAmt, vm.DaoConfig.ProposalTxFee, changeAddr)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}

	// 50% threshold based on current validators
	thresh := (len(vm.internalState.CurrentStakerChainState().Stakers()) + 1) / 2

	// Create the tx
	utx := &UnsignedDaoProposalTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         unlockedOuts,
		}},
		DaoProposal: dao.DaoProposal{
			Proposer:     nodeID,
			Thresh:       uint32(thresh),
			ProposalType: proposalType,
			Start:        startTime,
			End:          endTime,
			Wght:         lockAmt,
			Data:         proposal,
		},
		Locks: lockedOuts,
	}
	if err = utx.DaoProposal.InitializeID(); err != nil {
		return nil, ids.ID{}, err
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, signers); err != nil {
		return nil, ids.ID{}, err
	}
	return tx, utx.DaoProposal.ID(), utx.SyntacticVerify(vm.ctx)
}
