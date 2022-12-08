package state

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	lastAcceptedID   = ids.GenerateTestID()
	twoProposalsMap  = generateXNumberOfProposalsWithDifferentIds(2)
	twoProposalsMap2 = generateXNumberOfProposalsWithDifferentIds(2)
	twoVotesMap      = generateXNumberOfVotesWithDifferentIds(2)
	twoVotesMap2     = generateXNumberOfVotesWithDifferentIds(2)
)

func TestGetAllProposals(t *testing.T) {
	tests := map[string]struct {
		d    Diff
		want []*Proposal
		err  error
	}{
		"Error missing parent state": {
			d: func() Diff {
				require := require.New(t)
				ctrl := gomock.NewController(t)
				state, _ := newInitializedState(require)
				versions := NewMockVersions(ctrl)
				versions.EXPECT().GetState(lastAcceptedID).Times(1).Return(state, true)
				versions.EXPECT().GetState(lastAcceptedID).Times(1).Return(state, false)

				diff, _ := NewDiff(lastAcceptedID, versions)
				return diff
			}(),
			err: ErrMissingParentState,
		},
		"Getting proposals only from parent state": {
			d: func() Diff {
				require := require.New(t)
				ctrl := gomock.NewController(t)
				s, _ := newInitializedState(require)
				versions := NewMockVersions(ctrl)
				versions.EXPECT().GetState(lastAcceptedID).AnyTimes().Return(s, true)

				s.(*state).caminoState.(*caminoState).proposals = twoProposalsMap
				diff, _ := NewDiff(lastAcceptedID, versions)
				return diff
			}(),
			want: func() []*Proposal {
				var v []*Proposal
				for _, value := range twoProposalsMap {
					v = append(v, value)
				}
				return v
			}(),
		},
		"Getting proposals from both current and parent state": {
			d: func() Diff {
				require := require.New(t)
				ctrl := gomock.NewController(t)
				s, _ := newInitializedState(require)
				versions := NewMockVersions(ctrl)
				versions.EXPECT().GetState(lastAcceptedID).AnyTimes().Return(s, true)

				s.(*state).caminoState.(*caminoState).proposals = twoProposalsMap
				d, _ := NewDiff(lastAcceptedID, versions)
				d.(*diff).caminoDiff.modifiedProposals = twoProposalsMap2
				return d
			}(),
			want: func() []*Proposal {
				var v []*Proposal
				for _, value := range twoProposalsMap {
					v = append(v, value)
				}
				for _, value := range twoProposalsMap2 {
					v = append(v, value)
				}
				return v
			}(),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.d.GetAllProposals()
			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestGetAllVotes(t *testing.T) {
	tests := map[string]struct {
		d    Diff
		want []*Vote
		err  error
	}{
		"Error missing parent state": {
			d: func() Diff {
				require := require.New(t)
				ctrl := gomock.NewController(t)
				state, _ := newInitializedState(require)
				versions := NewMockVersions(ctrl)
				versions.EXPECT().GetState(lastAcceptedID).Times(1).Return(state, true)
				versions.EXPECT().GetState(lastAcceptedID).Times(1).Return(state, false)

				diff, _ := NewDiff(lastAcceptedID, versions)
				return diff
			}(),
			err: ErrMissingParentState,
		},
		"Getting votes only from parent state": {
			d: func() Diff {
				require := require.New(t)
				ctrl := gomock.NewController(t)
				s, _ := newInitializedState(require)
				versions := NewMockVersions(ctrl)
				versions.EXPECT().GetState(lastAcceptedID).AnyTimes().Return(s, true)

				s.(*state).caminoState.(*caminoState).votes = twoVotesMap
				diff, _ := NewDiff(lastAcceptedID, versions)
				return diff
			}(),
			want: func() []*Vote {
				var v []*Vote
				for _, value := range twoVotesMap {
					v = append(v, value)
				}
				return v
			}(),
		},
		"Getting votes from both current and parent state": {
			d: func() Diff {
				require := require.New(t)
				ctrl := gomock.NewController(t)
				s, _ := newInitializedState(require)
				versions := NewMockVersions(ctrl)
				versions.EXPECT().GetState(lastAcceptedID).AnyTimes().Return(s, true)

				s.(*state).caminoState.(*caminoState).votes = twoVotesMap
				d, _ := NewDiff(lastAcceptedID, versions)
				d.(*diff).caminoDiff.modifiedVotes = twoVotesMap2
				return d
			}(),
			want: func() []*Vote {
				var v []*Vote
				for _, value := range twoVotesMap {
					v = append(v, value)
				}
				for _, value := range twoVotesMap2 {
					v = append(v, value)
				}
				return v
			}(),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.d.GetAllVotes()
			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, got)
		})
	}
}
