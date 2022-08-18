// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"errors"
	"fmt"
	"time"

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
	DaoProposal dao.Proposal `serialize:"true" json:"daoProposal"`

	// Where to send locked tokens when done voting
	// TODO @jax this needs to be changed to a PChainOut
	Bond []*avax.TransferableOutput `serialize:"true" json:"bond"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedDaoProposalTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedDaoProposalTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
	for _, bond := range tx.Bond {
		bond.InitCtx(ctx)
	}
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

	if len(tx.DaoProposal.Data) > dao.MaxDaoProposalBytes {
		return fmt.Errorf("maximum allowed proposal size exeeded: %d > %d",
			len(tx.DaoProposal.Data),
			dao.MaxDaoProposalBytes,
		)
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

	var totalTransferable uint64

	for _, bond := range tx.Bond {
		if bond.AssetID() == vm.ctx.AVAXAssetID { // only allow to use avax to bond
			totalTransferable += bond.Out.Amount()
		}
	}

	if totalTransferable < vm.DaoConfig.ProposalBondAmount {
		return nil, nil, fmt.Errorf("provided tx Inputs to dont have enough transferable AVAX to bond")
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

	outs := make([]*avax.TransferableOutput, len(tx.Outs)+len(tx.Bond))
	copy(outs, tx.Outs)
	copy(outs[len(tx.Outs):], tx.Bond)

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
		if daoProposals.GetProposalState(tx.DaoProposal.ID()) != dao.ProposalStateUnknown {
			return nil, nil, fmt.Errorf("%s proposal already exists", tx.DaoProposal.ID().Hex())
		}

		// Early exit if one tries to vote for an existing validator
		// this should be the verify block of the internal tx
		if tx.DaoProposal.ProposalType == dao.ProposalTypeAddValidator {
			nodeID, err := ids.ToShortID(tx.DaoProposal.Data)
			if err != nil {
				return nil, nil, fmt.Errorf("proposal -> nodId failed: %w", err)
			}
			if _, err := currentStakers.GetValidator(nodeID); err == nil {
				return nil, nil, errAlreadyValidator
			}
			if _, err := pendingStakers.GetValidator(nodeID); err == nil {
				return nil, nil, errAlreadyValidator
			}
		}

		if prop := daoProposals.Exists(tx); prop != nil {
			return nil, nil, fmt.Errorf("duplicate proposal found: %s", prop.ID().String())
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

	onCommitState := newVersionedStateWithNewDaoState(vm, parentState, newlydaoProposals)

	// Consume the UTXOS
	consumeInputs(onCommitState, tx.Ins)
	// Produce the UTXOS
	txID := tx.ID()
	produceOutputs(onCommitState, txID, vm.ctx.AVAXAssetID, tx.Outs)

	// Set up the state if this tx is aborted
	onAbortState := newVersionedStateWithNewDaoState(vm, parentState, daoProposals)
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

// func (tx *UnsignedDaoProposalTx) Weight() uint64       { return tx.DaoProposal.Weight() }

// NewDaoProposalTx returns a new UnsignedDaoProposalTx
func (vm *VM) newDaoProposalTx(
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
		DaoProposal: dao.Proposal{
			Thresh:       uint32(thresh),
			ProposalType: proposalType,
			Start:        startTime,
			End:          endTime,
			// Wght:         lockAmt,
			Data: proposal,
		},
		// Locks: lockedOuts,
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
