package state

import (
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/linkeddb"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	"github.com/stretchr/testify/require"
)

var (
	proposalID        = ids.GenerateTestID()
	proposal          = &ProposalLookup{Proposal: &dao.Proposal{StartTime: time.Now()}, Votes: make(map[ids.ID]*dao.Vote), State: dao.ProposalStatePending}
	proposalWithVotes = &ProposalLookup{Proposal: &dao.Proposal{StartTime: time.Now()}, Votes: map[ids.ID]*dao.Vote{ids.GenerateTestID(): {Vote: dao.Accept}, ids.GenerateTestID(): {Vote: dao.Accept}}, State: dao.ProposalStateConcluded}
)

func TestLoadProposals(t *testing.T) {
	tests := map[string]struct {
		cs  caminoState
		err error
	}{
		"error while deserializing proposal": {
			cs:  caminoStateWithNProposals(2, false, false),
			err: errors.New("failed to unmarshal proposalID while loading from db"),
		},
		"invalid proposal content": {
			cs:  caminoStateWithNProposals(2, true, false),
			err: errors.New("failed to unmarshal proposal while loading from db"),
		},
		"success": {
			cs: caminoStateWithNProposals(2, true, true),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.cs.loadProposals()
			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)

			for _, p := range tt.cs.proposals {
				origProposal, _ := tt.cs.proposalList.Get(p.Proposal.Metadata)
				proposal := &ProposalLookup{}
				blocks.GenesisCodec.Unmarshal(origProposal, proposal) //nolint:errcheck
				require.Equal(t, proposal, p)
			}
		})
	}
}

func TestWriteProposals(t *testing.T) {
	tests := map[string]struct {
		cs      caminoState
		prepare func(cs *caminoState)
		err     error
	}{
		"error serializing proposal - empty interface": {
			cs: caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: map[ids.ID]*ProposalLookup{
				ids.GenerateTestID(): {},
			}}, proposalList: linkeddb.NewDefault(memdb.New())},
			err: errSerializingProposal,
		},
		"error persisting proposal - closed db connection": {
			cs: caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: twoProposalsMap}},
			prepare: func(cs *caminoState) {
				closedDB := memdb.New()
				closedDB.Close()
				cs.proposalList = linkeddb.NewDefault(closedDB)
			},
			err: errPersistingProposal,
		},
		"success": {
			cs: caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: twoProposalsMap}, proposalList: linkeddb.NewDefault(memdb.New())},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if tt.prepare != nil {
				tt.prepare(&tt.cs)
			}
			err := tt.cs.writeProposals()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Empty(t, tt.cs.modifiedProposalLookups)
			for id, p := range twoProposalsMap {
				savedP, err := tt.cs.proposalList.Get(id[:])
				require.NoError(t, err)
				require.Equal(t, p, savedP)
			}
		})
	}
}

func TestAddVote(t *testing.T) {
	type args struct {
		proposalID ids.ID
		voteID     ids.ID
		vote       *dao.Vote
	}

	tests := map[string]struct {
		cs   caminoState
		args args
		err  error
	}{
		"error proposal not found": {
			cs: caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: twoProposalsMap}},
			args: args{
				proposalID: ids.GenerateTestID(),
				voteID:     ids.GenerateTestID(),
				vote:       &dao.Vote{Vote: dao.Accept},
			},
			err: database.ErrNotFound,
		},
		"success": {
			cs: caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: map[ids.ID]*ProposalLookup{proposalID: proposal}}},
			args: args{
				proposalID: proposalID,
				voteID:     ids.GenerateTestID(),
				vote:       &dao.Vote{Vote: dao.Accept},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.cs.AddVote(tt.args.proposalID, tt.args.voteID, tt.args.vote)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.args.vote, tt.cs.modifiedProposalLookups[proposalID].Votes[tt.args.voteID])
		})
	}
}

func TestArchiveProposal(t *testing.T) {
	type args struct {
		proposalID ids.ID
	}
	tests := map[string]struct {
		cs   caminoState
		args args
		err  error
	}{
		"error": {
			cs:   caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: map[ids.ID]*ProposalLookup{}}},
			args: args{ids.GenerateTestID()},
			err:  database.ErrNotFound,
		},
		"success": {
			cs:   caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: map[ids.ID]*ProposalLookup{proposalID: proposalWithVotes}}},
			args: args{proposalID},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.cs.ArchiveProposal(tt.args.proposalID)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Empty(t, tt.cs.modifiedProposalLookups[proposalID].Votes)
		})
	}
}

func TestSetProposalState(t *testing.T) {
	type args struct {
		proposalID ids.ID
		state      dao.ProposalState
	}
	tests := map[string]struct {
		cs   caminoState
		args args
		err  error
	}{
		"error": {
			cs:   caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: map[ids.ID]*ProposalLookup{}}},
			args: args{ids.GenerateTestID(), dao.ProposalStatePending},
			err:  database.ErrNotFound,
		},
		"success": {
			cs:   caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: map[ids.ID]*ProposalLookup{proposalID: proposalWithVotes}}},
			args: args{proposalID, dao.ProposalStateConcluded},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.cs.SetProposalState(tt.args.proposalID, tt.args.state)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.args.state, tt.cs.modifiedProposalLookups[proposalID].State)
		})
	}
}
