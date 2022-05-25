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
	"github.com/chain4travel/caminogo/utils/timer/mockable"
	"github.com/chain4travel/caminogo/vms/components/avax"
)

var (
	errNotAddValidator = errors.New("caller is not validator staker")

	_ UnsignedProposalTx = &UnsignedDaoVoteTx{}
)

// UnsignedDaoVoteTx is an unsigned daoVoteTx
type UnsignedDaoVoteTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	// The nodeID this vote counts for
	NodeID ids.ShortID `serialize:"true" json:"nodeID"`

	// The dap proposal to vote for
	ProposalID ids.ID `serialize:"true" json:"proposalID"`

	// Start time this TX will be placed, must be syncBound after now
	Start int64 `serialize:"true" json:"startTime"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedDaoVoteTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedDaoVoteTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *UnsignedDaoVoteTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return errNilTx
	case tx.syntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}

	// cache that this is valid
	tx.syntacticallyVerified = true
	return nil
}

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedDaoVoteTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	_, _, err := tx.Execute(vm, parentState, stx)
	return err
}

// Execute this transaction.
func (tx *UnsignedDaoVoteTx) Execute(
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

	daoProposals := parentState.DaoProposalChainState()
	currentStakers := parentState.CurrentStakerChainState()

	if vm.bootstrapped.GetValue() {
		currentTimestamp := parentState.GetTimestamp()
		// Ensure the proposed validator starts after the current time
		startTime := tx.StartTime()
		if !currentTimestamp.Before(startTime) {
			return nil, nil, fmt.Errorf(
				"daoVote start time (%s) at or before current timestamp (%s)",
				startTime,
				currentTimestamp,
			)
		}

		// verify that the proposal is active and that we have not voted so far
		daoProposal, err := daoProposals.GetActiveProposal(tx.ProposalID)
		switch {
		case err != nil:
			return nil, nil, err
		case !currentTimestamp.Before(daoProposal.DaoProposalTx().EndTime()):
			return nil, nil, fmt.Errorf("proposal: %s is not active anymore", tx.ProposalID.String())
		case len(daoProposal.Votes()) >= int(daoProposal.DaoProposalTx().DaoProposal.Thresh):
			return nil, nil, fmt.Errorf("proposal: %s already accepted", tx.ProposalID.String())
		case daoProposal.Voted(tx.NodeID):
			return nil, nil, fmt.Errorf("node %s has already voted on proposal: %s", tx.NodeID.String(), tx.ProposalID.String())
		}

		// now verify that the caller is the addValidator of tx.nodeID
		validator, err := currentStakers.GetValidator(tx.NodeID)
		if err != nil {
			return nil, nil, err
		}

		ok, err := validator.VerifyCredsIntersection(vm, stx)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, errNotAddValidator
		}

		// Verify the flowcheck
		if err := vm.semanticVerifySpend(
			parentState,
			tx,
			tx.Ins,
			[]*avax.TransferableOutput{},
			stx.Creds,
			vm.DaoConfig.VoteTxFee,
			vm.ctx.AVAXAssetID,
		); err != nil {
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

	pendingStakers := parentState.PendingStakerChainState()

	// Set up the state if this tx is committed
	newlydaoProposals := daoProposals.AddVote(stx)

	onCommitState := newVersionedState(vm, parentState, currentStakers, pendingStakers, newlydaoProposals)

	// Consume the UTXOS
	consumeInputs(onCommitState, tx.Ins)

	// Set up the state if this tx is aborted
	onAbortState := newVersionedState(vm, parentState, currentStakers, pendingStakers, daoProposals)
	// Consume the UTXOS
	consumeInputs(onAbortState, tx.Ins)

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if this node thinks the vote
// should be inserted.
func (tx *UnsignedDaoVoteTx) InitiallyPrefersCommit(vm *VM) bool {
	return true
}

// TimedTx Interface
func (tx *UnsignedDaoVoteTx) StartTime() time.Time { return time.Unix(tx.Start, 0) }
func (tx *UnsignedDaoVoteTx) EndTime() time.Time   { return mockable.MaxTime }
func (tx *UnsignedDaoVoteTx) Weight() uint64       { return 0 }

// newDaoVoteTx returns a new UnsignedDaoVoteTx
func (vm *VM) newDaoVoteTx(
	nodeID ids.ShortID, // The nodeID placing the proposal
	keys []*crypto.PrivateKeySECP256K1R, // Keys providing the staked tokens
	changeAddr ids.ShortID, // Address to send change to, if there is any
	proposalID ids.ID, // The dao proposal we vote for
) (*Tx, error) {
	ins, unlockedOuts, _, signers, err := vm.stake(keys, 0, vm.DaoConfig.VoteTxFee, changeAddr)
	if err != nil {
		return nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}

	// Create the tx
	utx := &UnsignedDaoVoteTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         unlockedOuts,
		}},
		NodeID:     nodeID,
		ProposalID: proposalID,
		Start:      vm.Clock().Time().Add(2 * syncBound).Unix(),
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, signers); err != nil {
		return nil, err
	}
	return tx, utx.SyntacticVerify(vm.ctx)
}
