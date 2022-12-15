package state

import (
	"bytes"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	"github.com/google/btree"
)

type ProposalLookup struct {
	TxID     ids.ID                    `serialize:"true"`
	Proposal *dao.Proposal             `serialize:"true"`
	Votes    map[ids.ShortID]*dao.Vote `serialize:"true"`
	// State    dao.ProposalState
}

func (pl *ProposalLookup) Less(thanIntf btree.Item) bool {
	than, ok := thanIntf.(*ProposalLookup)
	if !ok {
		panic("ProposalLookup::Less called with incompatible type")
	}
	switch dao.CompareProposals(*pl.Proposal, *than.Proposal) {
	case 1:
		return false
	case -1:
		return true
	default:
		return bytes.Compare(pl.TxID[:], than.TxID[:]) == -1
	}
}

type ProposalState interface {
	GetAllProposals() ([]*ProposalLookup, error)
	AddProposal(proposalID ids.ID, proposal *dao.Proposal)
	ArchiveProposal(proposalID ids.ID) error // just for now delete all votes from struct, they dominate potential memory usage

	GetProposalLookup(proposalID ids.ID) (*ProposalLookup, error)
	AddProposalLookup(proposalID ids.ID, lookup *ProposalLookup)

	// SetProposalState(proposalID ids.ID, state dao.ProposalState) error

	AddVote(proposalID ids.ID, address ids.ShortID, vote *dao.Vote) error
}

func (cs *caminoState) AddProposal(propsalID ids.ID, proposal *dao.Proposal) {
	cs.modifiedProposalLookups[propsalID] = &ProposalLookup{
		propsalID, proposal, make(map[ids.ShortID]*dao.Vote),
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

// func (cs *caminoState) SetProposalState(proposalID ids.ID, state dao.ProposalState) error {
// 	proposal, err := cs.GetProposalLookup(proposalID)
// 	if err != nil {
// 		return err
// 	}

// 	proposal.State = state

// 	cs.modifiedProposalLookups[proposalID] = proposal

// 	return nil

// }

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

func (cs *caminoState) AddVote(proposalID ids.ID, address ids.ShortID, vote *dao.Vote) error {

	proposal, err := cs.GetProposalLookup(proposalID)
	if err != nil {
		return err
	}

	proposal.Votes[address] = vote

	cs.modifiedProposalLookups[proposalID] = proposal

	return nil
}

func (cs *caminoState) writeProposals() error {
	for proposalID, proposal := range cs.modifiedProposalLookups {
		proposalBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, proposal)
		if err != nil {
			return fmt.Errorf("failed to serialize proposal: %v", err)
		}

		if err := cs.proposalList.Put(proposalID[:], proposalBytes); err != nil {
			return fmt.Errorf("failed to persit proposal: %v", err)
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
		proposal := &ProposalLookup{
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

// type baseProposalLookups struct {
// 	proposalLookupMap    map[ids.ID]*ProposalLookup
// 	proposalLookups      *btree.BTree
// 	propoosalLookupDiffs map[ids.ID]*ProposalLookup
// }

// type diffProposalLookups struct {
// 	proposalLookupModified bool
// 	proposalLookupDeleted  bool
// 	proposalLookup         *ProposalLookup
// }

// type ProposalLookupIterator GenericIterator[*ProposalLookup]

// func newBaseDeposits() *baseProposalLookups {
// 	return &baseProposalLookups{
// 		proposalLookupMap:    make(map[ids.ID]*ProposalLookup),
// 		proposalLookups:      btree.New(defaultTreeDegree),
// 		propoosalLookupDiffs: make(map[ids.ID]*ProposalLookup),
// 	}
// }

// func (bpl *baseProposalLookups) GetProposalLookup(proposalID ids.ID) (*ProposalLookup, error) {
// 	deposit, ok := bpl.proposalLookupMap[proposalID]
// 	if !ok {
// 		return nil, database.ErrNotFound
// 	}
// 	return deposit, nil
// }

// func (bpl *baseProposalLookups) PutProposalLookup(proposalLookup *ProposalLookup) {

// 	validatorDiff := v.getOrCreateValidatorDiff(staker.SubnetID, staker.NodeID)
// 	validatorDiff.validatorModified = true
// 	validatorDiff.validatorDeleted = false
// 	validatorDiff.validator = staker

// 	v.stakers.ReplaceOrInsert(staker)
// }
