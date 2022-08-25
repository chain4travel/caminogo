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
	errVotingTooShort = errors.New("voting period is too short")
	errVotingTooLong  = errors.New("voting period is too long")

	_ UnsignedProposalTx = &UnsignedDaoSubmitProposalTx{}
	_ TimedTx            = &UnsignedDaoSubmitProposalTx{}
)

// UnsignedDaoSubmitProposalTx is an unsigned daoProposalTx
type UnsignedDaoSubmitProposalTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	// Our Dao Proposal
	ProposalConfiguration dao.ProposalConfiguration `serialize:"true" json:"proposalConfiguration"`

	// ! @jax on the first iteration this will only contain one tx that is executed when the
	// ! proposal was accepted
	// Tx to exectue when proposal was accpeted
	ProposedTx UnsingedVoteableTx `serialize:"true" json:"proposedTx"` // TODO maybe this should be a generic tx and then be checked at runtime *shrug*

	// Where to send locked tokens when done voting
	// TODO @jax this needs to be changed to a PChainOut
	Bond []*avax.TransferableOutput `serialize:"true" json:"bond"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedDaoSubmitProposalTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedDaoSubmitProposalTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
	for _, bond := range tx.Bond {
		bond.InitCtx(ctx)
	}
}
func (tx *UnsignedDaoSubmitProposalTx) Weight() uint64 {
	return 0
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *UnsignedDaoSubmitProposalTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return errNilTx
	case tx.syntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}

	if err := tx.ProposalConfiguration.Verify(); err != nil {
		return fmt.Errorf("failed to verify ProposalConfiguration: %w", err)
	}

	// cache that this is valid
	tx.syntacticallyVerified = true
	return nil
}

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedDaoSubmitProposalTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {

	if _, status, err := parentState.GetTx(tx.ProposedTx.ID()); err == nil {
		return fmt.Errorf("[%s] ProposedTx with id %s was already executed with status %s", tx.ID(), tx.ProposedTx.ID(), status)
	}

	if err := tx.ProposedTx.VerifyWithProposalContext(parentState, tx.ProposalConfiguration); err != nil {
		return fmt.Errorf("[%s] ProposedTx did not accept the given proposal paramenters: %v", tx.ID(), err)
	}
	if innerErr := tx.ProposedTx.SemanticVerify(vm, parentState, stx); innerErr != nil {
		return fmt.Errorf("[%s] ProposedTx could not be executed: %v", tx.ID(), innerErr)
	}
	_, _, err := tx.Execute(vm, parentState, stx)
	return err
}

// Execute this transaction.
func (tx *UnsignedDaoSubmitProposalTx) Execute(
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

	duration := tx.ProposalConfiguration.Duration()
	switch {
	case duration < vm.DaoConfig.MinProposalDuration:
		return nil, nil, errVotingTooShort
	case duration > vm.DaoConfig.MaxProposalDuration: // Ensure staking length is not too long
		return nil, nil, errVotingTooLong
	}

	daoProposals := parentState.DaoProposalChainState()

	outs := make([]*avax.TransferableOutput, len(tx.Outs)+len(tx.Bond))
	copy(outs, tx.Outs)
	copy(outs[len(tx.Outs):], tx.Bond)

	if vm.bootstrapped.GetValue() {
		currentTimestamp := parentState.GetTimestamp()
		// Ensure the proposed validator starts after the current time
		startTime := tx.ProposalConfiguration.StartTime()
		if !currentTimestamp.Before(startTime) {
			return nil, nil, fmt.Errorf(
				"daoProposal start time (%s) at or before current timestamp (%s)",
				startTime,
				currentTimestamp,
			)
		}

		// Ensure this proposal isn't already inserted
		if daoProposals.GetProposalState(tx.ID()) != dao.ProposalStateUnknown {
			return nil, nil, fmt.Errorf("%s proposal already exists", tx.ID().Hex())
		}

		// We dont need to check for an already existing proposedTx as if its voted in a different proposal
		// its totaly fine

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
func (tx *UnsignedDaoSubmitProposalTx) InitiallyPrefersCommit(vm *VM) bool {
	return tx.ProposalConfiguration.StartTime().After(vm.clock.Time())
}

// TimedTx Interface
func (tx *UnsignedDaoSubmitProposalTx) StartTime() time.Time {
	return tx.ProposalConfiguration.StartTime()
}
func (tx *UnsignedDaoSubmitProposalTx) EndTime() time.Time {
	return tx.ProposalConfiguration.EndTime()
}

// NewDaoProposalTx returns a new UnsignedDaoSubmitProposalTx
func (vm *VM) newDaoSubmitProposalTx(
	keys []*crypto.PrivateKeySECP256K1R, // Keys providing the staked tokens
	changeAddr ids.ShortID,
	startTime, // The time voting starts
	endTime uint64, // The time voting ends
	proposedTx UnsingedVoteableTx, // this is the proposal message, stored in TX Metadata
) (*Tx, ids.ID, error) {

	ins, unlockedOuts, lockedOuts, signers, err := vm.stake(keys, vm.DaoConfig.ProposalBondAmount, vm.DaoConfig.ProposalTxFee, changeAddr)
	if err != nil {
		return nil, ids.ID{}, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}

	// 50% threshold based on current validators
	thresh := (len(vm.internalState.CurrentStakerChainState().Stakers()) + 1) / 2

	// Create the tx
	utx := &UnsignedDaoSubmitProposalTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         unlockedOuts,
		}},
		ProposedTx: proposedTx,
		ProposalConfiguration: dao.ProposalConfiguration{
			Thresh: uint32(thresh),
			Start:  startTime,
			End:    endTime,
		},
		Bond: lockedOuts,
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, signers); err != nil {
		return nil, ids.ID{}, err
	}
	return tx, utx.ID(), utx.SyntacticVerify(vm.ctx)
}
