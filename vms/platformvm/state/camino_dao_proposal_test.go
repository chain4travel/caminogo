package state

import (
	"errors"
	"testing"

	"github.com/ava-labs/avalanchego/database/linkeddb"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/stretchr/testify/require"
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
			err: errors.New("failed to serialize proposal"),
		},
		"error persisting proposal - closed db connection": {
			cs: caminoState{caminoDiff: &caminoDiff{modifiedProposalLookups: twoProposalsMap}},
			prepare: func(cs *caminoState) {
				closedDB := memdb.New()
				closedDB.Close()
				cs.proposalList = linkeddb.NewDefault(closedDB)
			},
			err: errors.New("failed to persist proposal"),
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
				require.ErrorContains(t, err, tt.err.Error())
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
