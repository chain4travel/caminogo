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

const (
	Undecided ProposalOutcome = iota
	Accepted
	Rejected
)

type Proposal struct {
	TxID     ids.ID
	Proposer ids.ID `serialize:"true"` // may be not needed due to tx id

	Type    ProposalType    `serialize:"true"`
	State   ProposalState   `serialize:"true"`
	Outcome ProposalOutcome `serialize:"true"`

	StartTime time.Time `serialize:"true"`
	EndTime   time.Time `serialize:"true"`

	Content [ProposalContentLength]byte `serialize:"true"`

	Priority txs.Priority `serialize:"true"`
}

type VoteType uint64

const (
	Accept VoteType = iota
	Reject
	Abstain
)

type Vote struct {
	TxID       ids.ID
	ProposalID ids.ID `serialize:"true"`

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
	// FIXME @Jax i think this is wrong and leads to duplicates, with not way of discernig the newest one
	// maybe by the fact that later ones are older ones, but that is implicit and not documented in the interface
	proposals := make([]*Proposal, len(cs.modifiedProposals)+len(cs.proposals))

	i := 0
	for _, proposal := range cs.proposals {
		proposals[i] = proposal
		i++
	}
	for _, proposal := range cs.modifiedProposals {
		proposals[i] = proposal
		i++
	}

	return proposals, nil
}

func (cs *caminoState) ConcludeProposal(proposalID ids.ID, outcome ProposalOutcome) error {
	proposal, err := cs.GetProposal(proposalID)
	if err != nil {
		return err
	}

	proposal.Outcome = outcome
	cs.modifiedProposals[proposalID] = proposal
	return nil
}

func (cs *caminoState) AddVote(vote *Vote) {
	cs.modifiedVotes[vote.TxID] = vote
}

func (cs *caminoState) GetVote(voteID ids.ID) (*Vote, error) {
	if vote, ok := cs.modifiedVotes[voteID]; ok {
		return vote, nil
	}

	if vote, ok := cs.votes[voteID]; ok {
		return vote, nil
	}

	return nil, database.ErrNotFound
}

func (cs *caminoState) GetAllVotes() ([]*Vote, error) {
	// FIXME @jax see GetAllProposals for details
	votes := make([]*Vote, len(cs.modifiedVotes)+len(cs.votes))

	i := 0
	for _, vote := range cs.votes {
		votes[i] = vote
		i++
	}

	for _, vote := range cs.modifiedVotes {
		votes[i] = vote
		i++
	}

	return votes, nil

}

func (cs *caminoState) writeVotes() error {
	for voteID, vote := range cs.modifiedVotes {
		voteBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, vote)
		if err != nil {
			return fmt.Errorf("failed to serialize vote: %v", err)
		}

		if err := cs.voteList.Put(voteID[:], voteBytes); err != nil {
			return fmt.Errorf("failed to persit vote: %v", err)
		}

		delete(cs.modifiedVotes, voteID)
	}
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

func (cs *caminoState) loadVotes() error {
	voteIt := cs.voteList.NewIterator()
	defer voteIt.Release()
	for voteIt.Next() {
		voteIDBytes := voteIt.Key()
		voteID, err := ids.ToID(voteIDBytes)
		if err != nil {
			return fmt.Errorf("failed to unmarshal voteID while loading from db: %v", err)
		}

		voteBytes := voteIt.Value()
		vote := &Vote{
			TxID: voteID,
		}
		_, err = blocks.GenesisCodec.Unmarshal(voteBytes, vote)
		if err != nil {
			return fmt.Errorf("failed to unmarshal vote while loading from db: %v", err)
		}

		cs.votes[voteID] = vote

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
