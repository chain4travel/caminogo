// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"fmt"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/snow"
	"github.com/chain4travel/caminogo/vms/components/avax"
	"github.com/chain4travel/caminogo/vms/platformvm/dao"
)

var (
	_ UnsignedProposalTx = &UnsignedDaoConcludeProposalTx{}
)

// UnsignedDaoConcludeProposalTx is an unsigned daoVoteTx
type UnsignedDaoConcludeProposalTx struct {
	// Metadata
	avax.Metadata

	// The dap proposal to vote for
	TxID ids.ID `serialize:"true" json:"proposalID"` // TODO maybe this should be the tx id
}

// TODO @jax

/*
	baseline checks:
	- does proposal exists
	- is proposal concluded
		- has it enough yes votes
		- is it over e.g. is the timestamp after the endtime

	- if timestamp.after(endtime) && votes < threshold
		- set status to rejected
	- if votes >= treshold
		- execute wrapped tx
		- if wrapped tx failed to execute
			- deduct tx fees anyway
			- set proposal status to executionFailed

	- deduct tx fees
	- return bond
	- do archive logic -> we dont give a shit about retrys

*/

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedDaoConcludeProposalTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedDaoConcludeProposalTx) InitCtx(ctx *snow.Context) {
	// TODO needs investigation
}

func (tx *UnsignedDaoConcludeProposalTx) InputIDs() ids.Set {
	return nil
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *UnsignedDaoConcludeProposalTx) SyntacticVerify(ctx *snow.Context) error {
	if tx == nil {
		return errNilTx
	}

	// TODO does this rly need syntactic checks?
	return nil
}

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedDaoConcludeProposalTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	_, _, err := tx.Execute(vm, parentState, stx)
	return err
}

// Execute this transaction.
func (tx *UnsignedDaoConcludeProposalTx) Execute(
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

	proposalTxRaw, err := daoProposals.GetNextProposal()
	if err == database.ErrNotFound {
		return nil, nil, fmt.Errorf("failed to get next proposal: %v", err)
	} else if err != nil {
		return nil, nil, err
	}

	proposalTx, ok := proposalTxRaw.UnsignedTx.(*UnsignedDaoSubmitProposalTx)
	if !ok {
		panic("tx from proposal chain state was not a proposal")
	}

	proposalTxID := proposalTx.ID()
	if proposalTxID != tx.TxID {
		return nil, nil, fmt.Errorf(
			"attempting to concolude proposal: %s. Should be removing %s",
			tx.TxID,
			proposalTxID,
		)
	}

	var innerOnCommitState, innerOnAbortState VersionedState
	var innerExecuteErr error
	var executed = false

	if vm.bootstrapped.GetValue() {
		currentTimestamp := parentState.GetTimestamp()

		proposalState := daoProposals.GetProposalState(proposalTxID)

		if currentTimestamp.Before(proposalTx.StartTime()) {
			return nil, nil, fmt.Errorf("proposal \"%s\" has not started yet", proposalTxID)
		}

		if currentTimestamp.After(proposalTx.StartTime()) && currentTimestamp.Before(proposalTx.EndTime()) && proposalState != dao.ProposalStateAccepted {
			return nil, nil, fmt.Errorf("proposal \"%s\" has not been accepted yet, but can still be voted on", proposalTxID)
		}

		if proposalState == dao.ProposalStateConcluded {
			return nil, nil, fmt.Errorf("proposal \"%s\" was already concluded", proposalTxID)
		}

		if proposalState == dao.ProposalStateAccepted {
			innerOnCommitState, innerOnAbortState, innerExecuteErr = proposalTx.ProposedTx.Execute(vm, parentState, stx)
			executed = true
		}

		if currentTimestamp.Equal(proposalTx.EndTime()) && proposalState != dao.ProposalStateAccepted {
			vm.Logger().Info("Proposal \"%s\" did not receive enough votes to be executed")
		}

		// maybe equals is a better idea to figure out if we are at the end of the voting period as
		// advance time should only bring us to this exact moment
	}

	// delete the current proposal from state
	newlydaoProposals, err := daoProposals.ArchiveNextProposals(1)
	if err != nil {
		return nil, nil, err
	}

	var onCommitState VersionedState
	var onAbortState VersionedState
	// if the proposal was not executed or the execution failed build of from parent state
	if !executed || innerExecuteErr != nil {
		onCommitState = newVersionedStateWithNewDaoState(vm, parentState, newlydaoProposals)
		onAbortState = newVersionedStateWithNewDaoState(vm, parentState, daoProposals)
	} else { // otherwise use the resulting states of the execution of the proposedTx
		onCommitState = newVersionedStateWithNewDaoState(vm, innerOnCommitState, newlydaoProposals)
		onAbortState = newVersionedStateWithNewDaoState(vm, innerOnAbortState, daoProposals)
	}

	// TODO unbond proposal bond
	// // Consume the UTXOS

	// consumeInputs(onCommitState, tx.Ins)

	// Set up the state if this tx is aborted
	// // Consume the UTXOS
	// consumeInputs(onAbortState, tx.Ins)

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if this node thinks the vote
// should be inserted.
func (tx *UnsignedDaoConcludeProposalTx) InitiallyPrefersCommit(vm *VM) bool {
	return true
}

// newDaoVoteTx returns a new UnsignedDaoConcludeProposalTx
func (vm *VM) newConcludeProposalTx(
	proposalID ids.ID, // The dao proposal we vote for
) (*Tx, error) {
	// Create the tx
	utx := &UnsignedDaoConcludeProposalTx{
		TxID: proposalID,
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, nil); err != nil {
		return nil, err
	}
	return tx, utx.SyntacticVerify(vm.ctx)
}
