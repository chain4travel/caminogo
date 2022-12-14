package state

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
)

var (
	errSerializingProposal = errors.New("failed to serialize proposal")
	errPersistingProposal  = errors.New("failed to persist proposal")
)

type ProposalLookup struct {
	Proposal *dao.Proposal        `serialize:"true"`
	Votes    map[ids.ID]*dao.Vote `json:"votes"`
	State    dao.ProposalState    `serialize:"true"`
}

func (cs *caminoState) AddProposal(propsalID ids.ID, proposal *dao.Proposal, state dao.ProposalState) {
	cs.modifiedProposalLookups[propsalID] = &ProposalLookup{
		proposal, make(map[ids.ID]*dao.Vote), state,
	}
}

func (cs *caminoState) AddProposalLookup(propsalID ids.ID, lookup *ProposalLookup) {
	cs.modifiedProposalLookups[propsalID] = lookup
}

func (cs *caminoState) GetProposalLookup(proposalID ids.ID) (*ProposalLookup, error) {
	if propsal, ok := cs.modifiedProposalLookups[proposalID]; ok {
		return propsal, nil
	}

	if proposal, ok := cs.proposals[proposalID]; ok {
		return proposal, nil
	}

	return nil, database.ErrNotFound
}

func (cs *caminoState) GetAllProposals() ([]*ProposalLookup, error) {
	proposalMap := make(map[ids.ID]*ProposalLookup)

	for k, v := range cs.proposals {
		proposalMap[k] = v
	}

	for k, v := range cs.modifiedProposalLookups {
		proposalMap[k] = v
	}

	proposals := make([]*ProposalLookup, len(proposalMap))

	i := 0
	for _, proposal := range proposalMap {
		proposals[i] = proposal
		i++
	}

	return proposals, nil
}

func (cs *caminoState) SetProposalState(proposalID ids.ID, state dao.ProposalState) error {
	proposal, err := cs.GetProposalLookup(proposalID)
	if err != nil {
		return err
	}

	proposal.State = state

	cs.modifiedProposalLookups[proposalID] = proposal

	return nil
}

func (cs *caminoState) ArchiveProposal(proposalID ids.ID) error {
	proposal, err := cs.GetProposalLookup(proposalID)
	if err != nil {
		return err
	}

	for k := range proposal.Votes {
		delete(proposal.Votes, k)
	}

	cs.modifiedProposalLookups[proposalID] = proposal

	return nil
}

func (cs *caminoState) AddVote(proposalID ids.ID, voteID ids.ID, vote *dao.Vote) error {
	proposal, err := cs.GetProposalLookup(proposalID)
	if err != nil {
		return err
	}

	proposal.Votes[voteID] = vote

	cs.modifiedProposalLookups[proposalID] = proposal

	return nil
}

func (cs *caminoState) writeProposals() error {
	for proposalID, proposal := range cs.modifiedProposalLookups {
		proposalBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, proposal)
		if err != nil {
			return fmt.Errorf("%w: %v", errSerializingProposal, err)
		}
		proposalID := proposalID
		if err := cs.proposalList.Put(proposalID[:], proposalBytes); err != nil {
			return fmt.Errorf("%w: %v", errPersistingProposal, err)
		}

		delete(cs.modifiedProposalLookups, proposalID)
	}
	return nil
}

func (cs *caminoState) loadProposals() error {
	proposalIt := cs.proposalList.NewIterator()
	defer proposalIt.Release()
	for proposalIt.Next() {
		proposalIDBytes := proposalIt.Key()
		proposalID, err := ids.ToID(proposalIDBytes)
		if err != nil {
			return fmt.Errorf("failed to unmarshal proposalID while loading from db: %v", err)
		}

		proposalBytes := proposalIt.Value()
		proposal := &ProposalLookup{}
		_, err = blocks.GenesisCodec.Unmarshal(proposalBytes, proposal)
		if err != nil {
			return fmt.Errorf("failed to unmarshal proposal while loading from db: %v", err)
		}

		cs.proposals[proposalID] = proposal
	}
	return nil
}
