// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
)

func (d *diff) LockedUTXOs(txIDs ids.Set, addresses ids.ShortSet, lockState locked.State) ([]*avax.UTXO, error) {
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	retUtxos, err := parentState.LockedUTXOs(txIDs, addresses, lockState)
	if err != nil {
		return nil, err
	}

	// Apply modifiedUTXO's
	// Step 1: remove / update existing UTXOs
	remaining := ids.NewSet(len(d.modifiedUTXOs))
	for k := range d.modifiedUTXOs {
		remaining.Add(k)
	}
	for i := len(retUtxos) - 1; i >= 0; i-- {
		if utxo, exists := d.modifiedUTXOs[retUtxos[i].InputID()]; exists {
			if utxo.utxo == nil {
				retUtxos = append(retUtxos[:i], retUtxos[i+1:]...)
			} else {
				retUtxos[i] = utxo.utxo
			}
			delete(remaining, utxo.utxoID)
		}
	}

	// Step 2: Append new UTXOs
	for utxoID := range remaining {
		utxo := d.modifiedUTXOs[utxoID].utxo
		if utxo != nil {
			if lockedOut, ok := utxo.Out.(*locked.Out); ok && lockedOut.IDs.Match(lockState, txIDs) {
				retUtxos = append(retUtxos, utxo)
			}
		}
	}

	return retUtxos, nil
}

func (d *diff) CaminoGenesisState() (*genesis.Camino, error) {
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.CaminoGenesisState()
}

func (d *diff) SetAddressStates(address ids.ShortID, states uint64) {
	d.caminoDiff.modifiedAddressStates[address] = states
}

func (d *diff) GetAddressStates(address ids.ShortID) (uint64, error) {
	if states, ok := d.caminoDiff.modifiedAddressStates[address]; ok {
		return states, nil
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	return parentState.GetAddressStates(address)
}

func (d *diff) AddDepositOffer(offer *DepositOffer) {
	d.caminoDiff.modifiedDepositOffers[offer.id] = offer
}

func (d *diff) GetDepositOffer(offerID ids.ID) (*DepositOffer, error) {
	if offer, ok := d.caminoDiff.modifiedDepositOffers[offerID]; ok {
		return offer, nil
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	return parentState.GetDepositOffer(offerID)
}

func (d *diff) GetAllDepositOffers() ([]*DepositOffer, error) {
	offers := make([]*DepositOffer, len(d.caminoDiff.modifiedDepositOffers))

	for _, offer := range d.caminoDiff.modifiedDepositOffers {
		offers = append(offers, offer)
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	parentOffers, err := parentState.GetAllDepositOffers()
	if err != nil {
		return nil, err
	}

	return append(parentOffers, offers...), nil
}

// Voting
func (d *diff) GetAllProposals() ([]*Proposal, error) {
	proposals := make([]*Proposal, len(d.caminoDiff.modifiedProposals))
	i := 0
	for _, proposal := range d.caminoDiff.modifiedProposals {
		proposals[i] = proposal
		i++
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	parentProposals, err := parentState.GetAllProposals()
	if err != nil {
		return nil, err
	}

	return append(proposals, parentProposals...), nil

}

func (d *diff) GetProposal(proposalID ids.ID) (*Proposal, error) {
	if proposal, ok := d.caminoDiff.modifiedProposals[proposalID]; ok {
		return proposal, nil
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	return parentState.GetProposal(proposalID)

}

func (d *diff) AddProposal(proposal *Proposal) {
	d.caminoDiff.modifiedProposals[proposal.TxID] = proposal
}

func (d *diff) ArchiveProposal(proposalID ids.ID) error {
	proposal, err := d.GetProposal(proposalID)
	if err != nil {
		return err
	}
	for k, _ := range proposal.Votes {
		delete(proposal.Votes, k)
	}

	d.caminoDiff.modifiedProposals[proposalID] = proposal
	return nil

}

func (d *diff) SetProposalState(proposalID ids.ID, state ProposalState) error {
	proposal, err := d.GetProposal(proposalID)
	if err != nil {
		return err
	}

	proposal.State = state

	d.caminoDiff.modifiedProposals[proposalID] = proposal

	return nil
}

func (d *diff) AddVote(proposalID ids.ID, vote *Vote) error {

	proposal, err := d.GetProposal(proposalID)
	if err != nil {
		return err
	}

	proposal.Votes[vote.TxID] = vote

	d.caminoDiff.modifiedProposals[proposalID] = proposal
	return nil
}

// Finally apply all changes
func (d *diff) ApplyCaminoState(baseState State) {
	for k, v := range d.caminoDiff.modifiedAddressStates {
		baseState.SetAddressStates(k, v)
	}

	for _, v := range d.caminoDiff.modifiedDepositOffers {
		baseState.AddDepositOffer(v)
	}

	for _, v := range d.caminoDiff.modifiedProposals {
		baseState.AddProposal(v)
	}
}
