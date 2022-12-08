package state

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

const (
	ProposalContentLength = 1024
)

type ProposalType uint64

const (
	NOPProposal ProposalType = iota
)

type ProposalState uint64

const (
	Pending ProposalState = iota
	Active
	Concluded
)

type ProposalOutcome uint64

type ProposalStatus struct {
	State ProposalState `serialize:"true"`
}

type Proposal struct {
	TxID ids.ID

	Type  ProposalType  `serialize:"true"`
	State ProposalState `serialize:"true"`

	StartTime time.Time `serialize:"true"`
	EndTime   time.Time `serialize:"true"`

	Content [ProposalContentLength]byte `serialize:"true"`

	Votes map[ids.ID]*Vote `serialize:"true"`

	Priority txs.Priority `serialize:"true"`
}

type VoteType uint64

const (
	Accept VoteType = iota
	Reject
	Abstain
)

type Vote struct {
	TxID ids.ID

	Vote VoteType `serialize:"true"`
}

func (cs *caminoState) AddProposal(proposal *Proposal) {
	cs.modifiedProposals[proposal.TxID] = proposal
}

func (cs *caminoState) GetProposal(proposalID ids.ID) (*Proposal, error) {
	if propsal, ok := cs.modifiedProposals[proposalID]; ok {
		return propsal, nil
	}

	if proposal, ok := cs.proposals[proposalID]; ok {
		return proposal, nil
	}

	return nil, database.ErrNotFound
}

func (cs *caminoState) GetAllProposals() ([]*Proposal, error) {
	proposalMap := make(map[ids.ID]*Proposal)

	for k, v := range cs.proposals {
		proposalMap[k] = v
	}

	for k, v := range cs.modifiedProposals {
		proposalMap[k] = v
	}

	proposals := make([]*Proposal, len(proposalMap))

	i := 0
	for _, proposal := range proposalMap {
		proposals[i] = proposal
		i++
	}

	return proposals, nil
}

func (cs *caminoState) SetProposalState(proposalID ids.ID, state ProposalState) error {
	proposal, err := cs.GetProposal(proposalID)
	if err != nil {
		return err
	}

	proposal.State = state

	cs.modifiedProposals[proposalID] = proposal

	return nil

}

func (cs *caminoState) ArchiveProposal(proposalID ids.ID) error {
	proposal, err := cs.GetProposal(proposalID)
	if err != nil {
		return err
	}

	for k, _ := range proposal.Votes {
		delete(proposal.Votes, k)
	}

	cs.modifiedProposals[proposalID] = proposal

	return nil
}

func (cs *caminoState) AddVote(proposalID ids.ID, vote *Vote) error {

	proposal, err := cs.GetProposal(proposalID)
	if err != nil {
		return err
	}

	proposal.Votes[vote.TxID] = vote

	cs.modifiedProposals[proposalID] = proposal

	return nil
}

func (cs *caminoState) writeProposals() error {
	for proposalID, proposal := range cs.modifiedProposals {
		proposalBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, proposal)
		if err != nil {
			return fmt.Errorf("failed to serialize proposal: %v", err)
		}

		if err := cs.proposalList.Put(proposalID[:], proposalBytes); err != nil {
			return fmt.Errorf("failed to persit proposal: %v", err)
		}

		delete(cs.modifiedProposals, proposalID)
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
		proposal := &Proposal{
			TxID: proposalID,
		}
		_, err = blocks.GenesisCodec.Unmarshal(proposalBytes, proposal)
		if err != nil {
			return fmt.Errorf("failed to unmarshal proposal while loading from db: %v", err)
		}

		cs.proposals[proposalID] = proposal

	}
	return nil
}
