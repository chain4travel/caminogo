// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"math"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
)

func (s *state) LockedUTXOs(txIDs ids.Set, addresses ids.ShortSet, lockState locked.State) ([]*avax.UTXO, error) {
	retUtxos := []*avax.UTXO{}
	for address := range addresses {
		utxoIDs, err := s.UTXOIDs(address.Bytes(), ids.ID{}, math.MaxInt)
		if err != nil {
			return nil, err
		}
		for _, utxoID := range utxoIDs {
			utxo, err := s.GetUTXO(utxoID)
			if err != nil {
				return nil, err
			}
			if utxo == nil {
				continue
			}
			if lockedOut, ok := utxo.Out.(*locked.Out); ok && lockedOut.IDs.Match(lockState, txIDs) {
				retUtxos = append(retUtxos, utxo)
			}
		}
	}
	return retUtxos, nil
}

func (s *state) CaminoGenesisState() (*genesis.Camino, error) {
	return s.caminoState.GenesisState(), nil
}

func (s *state) SetAddressStates(address ids.ShortID, states uint64) {
	s.caminoState.SetAddressStates(address, states)
}

func (s *state) GetAddressStates(address ids.ShortID) (uint64, error) {
	return s.caminoState.GetAddressStates(address)
}

func (s *state) AddDepositOffer(offer *deposit.Offer) {
	s.caminoState.AddDepositOffer(offer)
}

func (s *state) GetDepositOffer(offerID ids.ID) (*deposit.Offer, error) {
	return s.caminoState.GetDepositOffer(offerID)
}

func (s *state) GetAllDepositOffers() ([]*deposit.Offer, error) {
	return s.caminoState.GetAllDepositOffers()
}

func (s *state) UpdateDeposit(depositTxID ids.ID, deposit *deposit.Deposit) {
	s.caminoState.UpdateDeposit(depositTxID, deposit)
}

func (s *state) GetDeposit(depositTxID ids.ID) (*deposit.Deposit, error) {
	return s.caminoState.GetDeposit(depositTxID)
}

func (s *state) AddProposal(proposalID ids.ID, proposal *dao.Proposal, state dao.ProposalState) {
	s.caminoState.AddProposal(proposalID, proposal, state)
}

func (s *state) AddProposalLookup(proposalID ids.ID, lookup *ProposalLookup) {
	s.caminoState.AddProposalLookup(proposalID, lookup)
}

func (s *state) GetProposalLookup(proposalID ids.ID) (*ProposalLookup, error) {
	return s.caminoState.GetProposalLookup(proposalID)
}

func (s *state) GetAllProposals() ([]*ProposalLookup, error) {
	return s.caminoState.GetAllProposals()
}

func (s *state) ArchiveProposal(proposalID ids.ID) error {
	return s.caminoState.ArchiveProposal(proposalID)
}

func (s *state) SetProposalState(proposalID ids.ID, state dao.ProposalState) error {
	return s.caminoState.SetProposalState(proposalID, state)
}

func (s *state) AddVote(proposalID ids.ID, voteID ids.ID, vote *dao.Vote) error {
	return s.caminoState.AddVote(proposalID, voteID, vote)
}
