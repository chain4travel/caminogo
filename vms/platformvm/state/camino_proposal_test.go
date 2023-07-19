package state

import (
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/dac"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestGetProposal(t *testing.T) {
	proposalID := ids.ID{1}
	wrapper := &proposalStateWrapper{
		ProposalState: &dac.BaseFeeProposalState{
			Start:         100,
			End:           100,
			AllowedVoters: []ids.ShortID{{11}},
			SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
				Options: []dac.SimpleVoteOption[uint64]{{Value: 1234, Weight: 1}},
			},
			SuccessThreshold: 1,
		},
	}
	proposalBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, wrapper)
	require.NoError(t, err)
	testError := errors.New("test error")

	tests := map[string]struct {
		caminoState         func(*gomock.Controller) *caminoState
		proposalID          ids.ID
		expectedCaminoState func(*caminoState) *caminoState
		expectedProposal    dac.ProposalState
		expectedErr         error
	}{
		"Fail: proposal removed": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID: {Proposal: wrapper.ProposalState, removed: true},
						},
					},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID: {Proposal: wrapper.ProposalState, removed: true},
						},
					},
				}
			},
			proposalID:  proposalID,
			expectedErr: database.ErrNotFound,
		},
		"Fail: proposal in cache, but removed": {
			caminoState: func(c *gomock.Controller) *caminoState {
				cache := cache.NewMockCacher[ids.ID, dac.ProposalState](c)
				cache.EXPECT().Get(proposalID).Return(nil, true)
				return &caminoState{
					proposalsCache: cache,
					caminoDiff:     &caminoDiff{},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsCache: actualCaminoState.proposalsCache,
					caminoDiff:     &caminoDiff{},
				}
			},
			proposalID:  proposalID,
			expectedErr: database.ErrNotFound,
		},
		"OK: proposal added": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID: {Proposal: wrapper.ProposalState, added: true},
						},
					},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID: {Proposal: wrapper.ProposalState, added: true},
						},
					},
				}
			},
			proposalID:       proposalID,
			expectedProposal: wrapper.ProposalState,
		},
		"OK: proposal modified": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID: {Proposal: wrapper.ProposalState},
						},
					},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID: {Proposal: wrapper.ProposalState},
						},
					},
				}
			},
			proposalID:       proposalID,
			expectedProposal: wrapper.ProposalState,
		},
		"OK: proposal in cache": {
			caminoState: func(c *gomock.Controller) *caminoState {
				cache := cache.NewMockCacher[ids.ID, dac.ProposalState](c)
				cache.EXPECT().Get(proposalID).Return(wrapper.ProposalState, true)
				return &caminoState{
					proposalsCache: cache,
					caminoDiff:     &caminoDiff{},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsCache: actualCaminoState.proposalsCache,
					caminoDiff:     &caminoDiff{},
				}
			},
			proposalID:       proposalID,
			expectedProposal: wrapper.ProposalState,
		},
		"OK: proposal in db": {
			caminoState: func(c *gomock.Controller) *caminoState {
				cache := cache.NewMockCacher[ids.ID, dac.ProposalState](c)
				cache.EXPECT().Get(proposalID).Return(nil, false)
				cache.EXPECT().Put(proposalID, wrapper.ProposalState)
				db := database.NewMockDatabase(c)
				db.EXPECT().Get(proposalID[:]).Return(proposalBytes, nil)
				return &caminoState{
					proposalsDB:    db,
					proposalsCache: cache,
					caminoDiff:     &caminoDiff{},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsDB:    actualCaminoState.proposalsDB,
					proposalsCache: actualCaminoState.proposalsCache,
					caminoDiff:     &caminoDiff{},
				}
			},
			proposalID:       proposalID,
			expectedProposal: wrapper.ProposalState,
		},
		"Fail: db error": {
			caminoState: func(c *gomock.Controller) *caminoState {
				cache := cache.NewMockCacher[ids.ID, dac.ProposalState](c)
				cache.EXPECT().Get(proposalID).Return(nil, false)
				db := database.NewMockDatabase(c)
				db.EXPECT().Get(proposalID[:]).Return(nil, testError)
				return &caminoState{
					proposalsDB:    db,
					proposalsCache: cache,
					caminoDiff:     &caminoDiff{},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsDB:    actualCaminoState.proposalsDB,
					proposalsCache: actualCaminoState.proposalsCache,
					caminoDiff:     &caminoDiff{},
				}
			},
			proposalID:  proposalID,
			expectedErr: testError,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			caminoState := tt.caminoState(ctrl)
			actualProposal, err := caminoState.GetProposal(tt.proposalID)
			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expectedProposal, actualProposal)
			require.Equal(t, tt.expectedCaminoState(caminoState), caminoState)
		})
	}
}

func TestAddProposal(t *testing.T) {
	proposalID := ids.ID{1}
	proposal := &dac.BaseFeeProposalState{}

	tests := map[string]struct {
		caminoState         *caminoState
		proposalID          ids.ID
		proposal            dac.ProposalState
		expectedCaminoState *caminoState
	}{
		"OK": {
			caminoState: &caminoState{
				caminoDiff: &caminoDiff{modifiedProposals: map[ids.ID]*proposalDiff{}},
			},
			proposalID: proposalID,
			proposal:   proposal,
			expectedCaminoState: &caminoState{
				caminoDiff: &caminoDiff{
					modifiedProposals: map[ids.ID]*proposalDiff{
						proposalID: {Proposal: proposal, added: true},
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.caminoState.AddProposal(tt.proposalID, tt.proposal)
			require.Equal(t, tt.expectedCaminoState, tt.caminoState)
		})
	}
}

func TestModifyProposal(t *testing.T) {
	proposalID := ids.ID{1}
	proposal1 := &dac.BaseFeeProposalState{}

	tests := map[string]struct {
		caminoState         func(*gomock.Controller) *caminoState
		proposalID          ids.ID
		proposal            dac.ProposalState
		expectedCaminoState func(*caminoState) *caminoState
	}{
		"OK": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalsCache := cache.NewMockCacher[ids.ID, dac.ProposalState](c)
				proposalsCache.EXPECT().Evict(proposalID)
				return &caminoState{
					proposalsCache: proposalsCache,
					caminoDiff:     &caminoDiff{modifiedProposals: map[ids.ID]*proposalDiff{}},
				}
			},
			proposalID: proposalID,
			proposal:   proposal1,
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsCache: actualCaminoState.proposalsCache,
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID: {Proposal: proposal1},
						},
					},
				}
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			actualCaminoState := tt.caminoState(ctrl)
			actualCaminoState.ModifyProposal(tt.proposalID, tt.proposal)
			require.Equal(t, tt.expectedCaminoState(actualCaminoState), actualCaminoState)
		})
	}
}

func TestRemoveProposal(t *testing.T) {
	proposalID := ids.ID{1}
	proposal := &dac.BaseFeeProposalState{}

	tests := map[string]struct {
		caminoState         func(*gomock.Controller) *caminoState
		proposalID          ids.ID
		proposal            dac.ProposalState
		expectedCaminoState func(*caminoState) *caminoState
	}{
		"OK": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalsCache := cache.NewMockCacher[ids.ID, dac.ProposalState](c)
				proposalsCache.EXPECT().Evict(proposalID)
				return &caminoState{
					proposalsCache: proposalsCache,
					caminoDiff:     &caminoDiff{modifiedProposals: map[ids.ID]*proposalDiff{}},
				}
			},
			proposalID: proposalID,
			proposal:   proposal,
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsCache: actualCaminoState.proposalsCache,
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID: {Proposal: proposal, removed: true},
						},
					},
				}
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			actualCaminoState := tt.caminoState(ctrl)
			actualCaminoState.RemoveProposal(tt.proposalID, tt.proposal)
			require.Equal(t, tt.expectedCaminoState(actualCaminoState), actualCaminoState)
		})
	}
}

func TestAddProposalIDToFinish(t *testing.T) {
	proposalID1 := ids.ID{1}
	proposalID2 := ids.ID{2}
	proposalID3 := ids.ID{3}

	tests := map[string]struct {
		caminoState         *caminoState
		proposalID          ids.ID
		expectedCaminoState *caminoState
	}{
		"OK": {
			proposalID: proposalID3,
			caminoState: &caminoState{
				caminoDiff: &caminoDiff{
					addedProposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
				},
			},
			expectedCaminoState: &caminoState{
				caminoDiff: &caminoDiff{
					addedProposalIDsToExecute: []ids.ID{proposalID1, proposalID2, proposalID3},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.caminoState.AddProposalIDToFinish(tt.proposalID)
			require.Equal(t, tt.expectedCaminoState, tt.caminoState)
		})
	}
}

func TestGetProposalIDsToFinish(t *testing.T) {
	proposalID1 := ids.ID{1}
	proposalID2 := ids.ID{2}
	proposalID3 := ids.ID{3}
	proposalID4 := ids.ID{4}

	tests := map[string]struct {
		caminoState                  *caminoState
		expectedCaminoState          *caminoState
		expectedProposalIDsToExecute []ids.ID
		expectedErr                  error
	}{
		"OK: no proposals to execute": {
			caminoState:         &caminoState{caminoDiff: &caminoDiff{}},
			expectedCaminoState: &caminoState{caminoDiff: &caminoDiff{}},
		},
		"OK: no new proposals to execute": {
			caminoState: &caminoState{
				caminoDiff:           &caminoDiff{},
				proposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
			},
			expectedCaminoState: &caminoState{
				caminoDiff:           &caminoDiff{},
				proposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
			},
			expectedProposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
		},
		"OK: only new proposals to execute": {
			caminoState: &caminoState{caminoDiff: &caminoDiff{
				addedProposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
			}},
			expectedCaminoState: &caminoState{caminoDiff: &caminoDiff{
				addedProposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
			}},
			expectedProposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
		},
		"OK": {
			caminoState: &caminoState{
				caminoDiff: &caminoDiff{
					addedProposalIDsToExecute: []ids.ID{proposalID3, proposalID4},
				},
				proposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
			},
			expectedCaminoState: &caminoState{
				caminoDiff: &caminoDiff{
					addedProposalIDsToExecute: []ids.ID{proposalID3, proposalID4},
				},
				proposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
			},
			expectedProposalIDsToExecute: []ids.ID{proposalID1, proposalID2, proposalID3, proposalID4},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			proposalIDsToExecute, err := tt.caminoState.GetProposalIDsToFinish()
			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expectedProposalIDsToExecute, proposalIDsToExecute)
			require.Equal(t, tt.expectedCaminoState, tt.caminoState)
		})
	}
}

func TestGetNextProposalExpirationTime(t *testing.T) {
	proposalID1 := ids.ID{1}
	proposalID2 := ids.ID{2}
	proposalID31 := ids.ID{3}
	proposalID32 := ids.ID{4}
	proposal2 := &dac.BaseFeeProposalState{
		End: 102,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 1}},
		},
	}
	proposal31 := &dac.BaseFeeProposalState{
		End: 103,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 2}},
		},
	}
	proposal32 := &dac.BaseFeeProposalState{
		End: 103,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 3}},
		},
	}
	proposal1Endtime := time.Unix(100, 0)

	tests := map[string]struct {
		caminoState                func(c *gomock.Controller) *caminoState
		removedProposalIDs         set.Set[ids.ID]
		expectedCaminoState        func(*caminoState) *caminoState
		expectedNextExpirationTime time.Time
		expectedErr                error
	}{
		"Fail: no proposals": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{}
			},
			expectedNextExpirationTime: mockable.MaxTime,
			expectedErr:                database.ErrNotFound,
		},
		"Fail: no proposals (all removed)": {
			caminoState: func(c *gomock.Controller) *caminoState {
				it := database.NewMockIterator(c)
				it.EXPECT().Next().Return(false)
				it.EXPECT().Error().Return(nil)
				it.EXPECT().Release()

				db := database.NewMockDatabase(c)
				db.EXPECT().NewIterator().Return(it)

				return &caminoState{
					proposalIDsByEndtimeDB:      db,
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			removedProposalIDs: set.Set[ids.ID]{proposalID1: struct{}{}},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalIDsByEndtimeDB:      actualCaminoState.proposalIDsByEndtimeDB,
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			expectedNextExpirationTime: mockable.MaxTime,
			expectedErr:                database.ErrNotFound,
		},
		"OK": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			expectedNextExpirationTime: proposal1Endtime,
		},
		"Ok: in-mem proposals removed, but db has some": {
			caminoState: func(c *gomock.Controller) *caminoState {
				it := database.NewMockIterator(c)
				it.EXPECT().Next().Return(true).Times(3)
				it.EXPECT().Key().Return(proposalToKey(proposalID2[:], proposal2))
				it.EXPECT().Key().Return(proposalToKey(proposalID31[:], proposal31))
				it.EXPECT().Key().Return(proposalToKey(proposalID32[:], proposal32))
				it.EXPECT().Next().Return(false)
				it.EXPECT().Error().Return(nil)
				it.EXPECT().Release()

				db := database.NewMockDatabase(c)
				db.EXPECT().NewIterator().Return(it)

				return &caminoState{
					proposalIDsByEndtimeDB:      db,
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			removedProposalIDs: set.Set[ids.ID]{
				proposalID1: struct{}{},
				proposalID2: struct{}{},
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalIDsByEndtimeDB:      actualCaminoState.proposalIDsByEndtimeDB,
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			expectedNextExpirationTime: proposal31.EndTime(),
		},
		"OK: some proposals removed": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1, proposalID2},
				}
			},
			removedProposalIDs: set.Set[ids.ID]{
				proposalID1: struct{}{},
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1, proposalID2},
				}
			},
			expectedNextExpirationTime: proposal1Endtime,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			caminoState := tt.caminoState(ctrl)
			nextExpirationTime, err := caminoState.GetNextProposalExpirationTime(tt.removedProposalIDs)
			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expectedNextExpirationTime, nextExpirationTime)
			require.Equal(t, tt.expectedCaminoState(caminoState), caminoState)
		})
	}
}

func TestGetNextToExpireProposalIDsAndTime(t *testing.T) {
	proposalID1 := ids.ID{1}
	proposalID2 := ids.ID{2}
	proposalID31 := ids.ID{3}
	proposalID32 := ids.ID{4}
	proposal2 := &dac.BaseFeeProposalState{
		End: 102,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 1}},
		},
	}
	proposal31 := &dac.BaseFeeProposalState{
		End: 103,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 2}},
		},
	}
	proposal32 := &dac.BaseFeeProposalState{
		End: 103,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 3}},
		},
	}
	proposal1Endtime := time.Unix(100, 0)

	tests := map[string]struct {
		caminoState                func(c *gomock.Controller) *caminoState
		removedProposalIDs         set.Set[ids.ID]
		expectedCaminoState        func(*caminoState) *caminoState
		expectedNextExpirationTime time.Time
		expectedNextToExpireIDs    []ids.ID
		expectedErr                error
	}{
		"Fail: no proposals": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{}
			},
			expectedNextExpirationTime: mockable.MaxTime,
			expectedErr:                database.ErrNotFound,
		},
		"Fail: no proposals (all removed)": {
			caminoState: func(c *gomock.Controller) *caminoState {
				it := database.NewMockIterator(c)
				it.EXPECT().Next().Return(false)
				it.EXPECT().Error().Return(nil)
				it.EXPECT().Release()

				db := database.NewMockDatabase(c)
				db.EXPECT().NewIterator().Return(it)

				return &caminoState{
					proposalIDsByEndtimeDB:      db,
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			removedProposalIDs: set.Set[ids.ID]{proposalID1: struct{}{}},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalIDsByEndtimeDB:      actualCaminoState.proposalIDsByEndtimeDB,
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			expectedNextExpirationTime: mockable.MaxTime,
			expectedErr:                database.ErrNotFound,
		},
		"OK": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			expectedNextExpirationTime: proposal1Endtime,
			expectedNextToExpireIDs:    []ids.ID{proposalID1},
		},
		"Ok: in-mem proposals removed, but db has some": {
			caminoState: func(c *gomock.Controller) *caminoState {
				it := database.NewMockIterator(c)
				it.EXPECT().Next().Return(true).Times(3)
				it.EXPECT().Key().Return(proposalToKey(proposalID2[:], proposal2))
				it.EXPECT().Key().Return(proposalToKey(proposalID31[:], proposal31))
				it.EXPECT().Key().Return(proposalToKey(proposalID32[:], proposal32))
				it.EXPECT().Next().Return(false)
				it.EXPECT().Error().Return(nil)
				it.EXPECT().Release()

				db := database.NewMockDatabase(c)
				db.EXPECT().NewIterator().Return(it)

				return &caminoState{
					proposalIDsByEndtimeDB:      db,
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			removedProposalIDs: set.Set[ids.ID]{
				proposalID1: struct{}{},
				proposalID2: struct{}{},
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalIDsByEndtimeDB:      actualCaminoState.proposalIDsByEndtimeDB,
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
			expectedNextExpirationTime: proposal31.EndTime(),
			expectedNextToExpireIDs:    []ids.ID{proposalID31, proposalID32},
		},
		"OK: some proposals removed": {
			caminoState: func(c *gomock.Controller) *caminoState {
				return &caminoState{
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1, proposalID2},
				}
			},
			removedProposalIDs: set.Set[ids.ID]{
				proposalID1: struct{}{},
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalsNextExpirationTime: &proposal1Endtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1, proposalID2},
				}
			},
			expectedNextExpirationTime: proposal1Endtime,
			expectedNextToExpireIDs:    []ids.ID{proposalID2},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			caminoState := tt.caminoState(ctrl)
			nextToExpireIDs, nextExpirationTime, err := caminoState.GetNextToExpireProposalIDsAndTime(tt.removedProposalIDs)
			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expectedNextExpirationTime, nextExpirationTime)
			require.Equal(t, tt.expectedNextToExpireIDs, nextToExpireIDs)
			require.Equal(t, tt.expectedCaminoState(caminoState), caminoState)
		})
	}
}

func TestWriteProposals(t *testing.T) {
	testError := errors.New("test error")
	proposalID1 := ids.ID{1}
	proposalID2 := ids.ID{2}
	proposalID3 := ids.ID{3}

	proposalWrapper1 := &proposalStateWrapper{ProposalState: &dac.BaseFeeProposalState{
		End: 10,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 1}},
		},
	}}
	proposalWrapper2 := &proposalStateWrapper{ProposalState: &dac.BaseFeeProposalState{
		End: 10,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 2}},
		},
	}}
	proposalWrapper3 := &proposalStateWrapper{ProposalState: &dac.BaseFeeProposalState{
		End: 11,
		SimpleVoteOptions: dac.SimpleVoteOptions[uint64]{
			Options: []dac.SimpleVoteOption[uint64]{{Value: 3}},
		},
	}}

	proposalEndtime := proposalWrapper2.EndTime()
	proposal1Bytes, err := blocks.GenesisCodec.Marshal(blocks.Version, proposalWrapper1)
	require.NoError(t, err)
	proposal2Bytes, err := blocks.GenesisCodec.Marshal(blocks.Version, proposalWrapper2)
	require.NoError(t, err)

	tests := map[string]struct {
		caminoState         func(*gomock.Controller) *caminoState
		expectedCaminoState func(*caminoState) *caminoState
		expectedErr         error
	}{
		"Fail: db errored on modified proposal Put": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalsDB := database.NewMockDatabase(c)
				proposalsDB.EXPECT().Put(proposalID1[:], proposal1Bytes).Return(testError)
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID1: {Proposal: proposalWrapper1.ProposalState},
						},
					},
					proposalsDB: proposalsDB,
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{},
					},
					proposalsDB: actualCaminoState.proposalsDB,
				}
			},
			expectedErr: testError,
		},
		"Fail: db errored on added proposal Put": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalsDB := database.NewMockDatabase(c)
				proposalsDB.EXPECT().Put(proposalID1[:], proposal1Bytes).Return(testError)
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID1: {Proposal: proposalWrapper1.ProposalState, added: true},
						},
					},
					proposalsDB: proposalsDB,
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{},
					},
					proposalsDB: actualCaminoState.proposalsDB,
				}
			},
			expectedErr: testError,
		},
		"Fail: db errored on removed proposal Delete": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalsDB := database.NewMockDatabase(c)
				proposalsDB.EXPECT().Delete(proposalID1[:]).Return(testError)
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID1: {Proposal: proposalWrapper1.ProposalState, removed: true},
						},
					},
					proposalsDB: proposalsDB,
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{},
					},
					proposalsDB: actualCaminoState.proposalsDB,
				}
			},
			expectedErr: testError,
		},
		"OK: add proposals-to-execute": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalIDsToExecuteDB := database.NewMockDatabase(c)
				proposalIDsToExecuteDB.EXPECT().Put(proposalID1[:], nil).Return(nil)
				proposalIDsToExecuteDB.EXPECT().Put(proposalID2[:], nil).Return(nil)

				proposalsIterator := database.NewMockIterator(c)
				proposalsIterator.EXPECT().Next().Return(false)
				proposalsIterator.EXPECT().Error().Return(nil)
				proposalsIterator.EXPECT().Release()

				proposalIDsByEndtimeDB := database.NewMockDatabase(c)
				proposalIDsByEndtimeDB.EXPECT().NewIterator().Return(proposalsIterator)

				return &caminoState{
					proposalIDsToExecuteDB: proposalIDsToExecuteDB,
					proposalIDsByEndtimeDB: proposalIDsByEndtimeDB,
					caminoDiff: &caminoDiff{
						addedProposalIDsToExecute: []ids.ID{proposalID1, proposalID2},
					},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalIDsToExecuteDB: actualCaminoState.proposalIDsToExecuteDB,
					proposalIDsByEndtimeDB: actualCaminoState.proposalIDsByEndtimeDB,
					caminoDiff:             &caminoDiff{},
					proposalIDsToExecute:   []ids.ID{proposalID1, proposalID2},
				}
			},
		},
		"OK: add, modify and delete; nextExpiration partial removal, added new": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalsDB := database.NewMockDatabase(c)
				proposalsDB.EXPECT().Put(proposalID1[:], proposal1Bytes).Return(nil)
				proposalsDB.EXPECT().Put(proposalID2[:], proposal2Bytes).Return(nil)
				proposalsDB.EXPECT().Delete(proposalID3[:]).Return(nil)

				proposalIDsByEndtimeDB := database.NewMockDatabase(c)
				proposalIDsByEndtimeDB.EXPECT().Put(proposalToKey(proposalID1[:], proposalWrapper1), nil).Return(nil)
				proposalIDsByEndtimeDB.EXPECT().Delete(proposalToKey(proposalID3[:], proposalWrapper3)).Return(nil)

				return &caminoState{
					proposalIDsByEndtimeDB: proposalIDsByEndtimeDB,
					proposalsDB:            proposalsDB,
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID1: {Proposal: proposalWrapper1.ProposalState, added: true},
							proposalID2: {Proposal: proposalWrapper2.ProposalState},
							proposalID3: {Proposal: proposalWrapper3.ProposalState, removed: true},
						},
					},
					proposalsNextExpirationTime: &proposalEndtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID2, proposalID3},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalIDsByEndtimeDB: actualCaminoState.proposalIDsByEndtimeDB,
					proposalsDB:            actualCaminoState.proposalsDB,
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{},
					},
					proposalsNextExpirationTime: &proposalEndtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1, proposalID2},
				}
			},
		},
		"OK: nextExpiration full removal, can't add new, peek into db": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalsDB := database.NewMockDatabase(c)
				proposalsDB.EXPECT().Put(proposalID1[:], proposal1Bytes).Return(nil)
				proposalsDB.EXPECT().Delete(proposalID2[:]).Return(nil)

				proposalsIterator := database.NewMockIterator(c)
				proposalsIterator.EXPECT().Next().Return(true)
				proposalsIterator.EXPECT().Key().Return(proposalToKey(proposalID1[:], proposalWrapper1))
				proposalsIterator.EXPECT().Next().Return(false)
				proposalsIterator.EXPECT().Error().Return(nil)
				proposalsIterator.EXPECT().Release()

				proposalIDsByEndtimeDB := database.NewMockDatabase(c)
				proposalIDsByEndtimeDB.EXPECT().Put(proposalToKey(proposalID1[:], proposalWrapper1), nil).Return(nil)
				proposalIDsByEndtimeDB.EXPECT().Delete(proposalToKey(proposalID2[:], proposalWrapper2)).Return(nil)
				proposalIDsByEndtimeDB.EXPECT().NewIterator().Return(proposalsIterator)

				return &caminoState{
					proposalIDsByEndtimeDB: proposalIDsByEndtimeDB,
					proposalsDB:            proposalsDB,
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{
							proposalID1: {Proposal: proposalWrapper1.ProposalState, added: true},
							proposalID2: {Proposal: proposalWrapper2.ProposalState, removed: true},
						},
					},
					proposalsNextExpirationTime: &proposalEndtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID2},
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalIDsByEndtimeDB: actualCaminoState.proposalIDsByEndtimeDB,
					proposalsDB:            actualCaminoState.proposalsDB,
					caminoDiff: &caminoDiff{
						modifiedProposals: map[ids.ID]*proposalDiff{},
					},
					proposalsNextExpirationTime: &proposalEndtime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1},
				}
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			actualCaminoState := tt.caminoState(ctrl)
			require.ErrorIs(t, actualCaminoState.writeProposals(), tt.expectedErr)
			require.Equal(t, tt.expectedCaminoState(actualCaminoState), actualCaminoState)
		})
	}
}

func TestLoadProposals(t *testing.T) {
	proposalID1 := ids.ID{1}
	proposalID2 := ids.ID{2}
	proposalID3 := ids.ID{3}
	proposalID4 := ids.ID{4}

	proposal1 := &dac.BaseFeeProposalState{End: 10}
	proposal2 := &dac.BaseFeeProposalState{End: 10}
	proposal3 := &dac.BaseFeeProposalState{End: 11}

	tests := map[string]struct {
		caminoState         func(*gomock.Controller) *caminoState
		expectedCaminoState func(*caminoState) *caminoState
		expectedErr         error
	}{
		"OK": {
			caminoState: func(c *gomock.Controller) *caminoState {
				expiredProposalsIterator := database.NewMockIterator(c)
				expiredProposalsIterator.EXPECT().Next().Return(true).Times(3)
				expiredProposalsIterator.EXPECT().Key().Return(proposalToKey(proposalID1[:], proposal1))
				expiredProposalsIterator.EXPECT().Key().Return(proposalToKey(proposalID2[:], proposal2))
				expiredProposalsIterator.EXPECT().Key().Return(proposalToKey(proposalID3[:], proposal3))
				expiredProposalsIterator.EXPECT().Error().Return(nil)
				expiredProposalsIterator.EXPECT().Release()

				proposalIDsByEndtimeDB := database.NewMockDatabase(c)
				proposalIDsByEndtimeDB.EXPECT().NewIterator().Return(expiredProposalsIterator)

				proposalsToExecuteIterator := database.NewMockIterator(c)
				proposalsToExecuteIterator.EXPECT().Next().Return(true).Times(2)
				proposalsToExecuteIterator.EXPECT().Key().Return(proposalID2[:])
				proposalsToExecuteIterator.EXPECT().Key().Return(proposalID4[:])
				proposalsToExecuteIterator.EXPECT().Next().Return(false)
				proposalsToExecuteIterator.EXPECT().Error().Return(nil)
				proposalsToExecuteIterator.EXPECT().Release()

				proposalIDsToExecuteDB := database.NewMockDatabase(c)
				proposalIDsToExecuteDB.EXPECT().NewIterator().Return(proposalsToExecuteIterator)

				return &caminoState{
					proposalIDsByEndtimeDB: proposalIDsByEndtimeDB,
					proposalIDsToExecuteDB: proposalIDsToExecuteDB,
				}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				nextTime := proposal1.EndTime()
				return &caminoState{
					proposalIDsByEndtimeDB:      actualCaminoState.proposalIDsByEndtimeDB,
					proposalIDsToExecuteDB:      actualCaminoState.proposalIDsToExecuteDB,
					proposalsNextExpirationTime: &nextTime,
					proposalsNextToExpireIDs:    []ids.ID{proposalID1, proposalID2},
					proposalIDsToExecute:        []ids.ID{proposalID2, proposalID4},
				}
			},
		},
		"OK: no proposals": {
			caminoState: func(c *gomock.Controller) *caminoState {
				proposalsIterator := database.NewMockIterator(c)
				proposalsIterator.EXPECT().Next().Return(false)
				proposalsIterator.EXPECT().Error().Return(nil)
				proposalsIterator.EXPECT().Release()
				proposalIDsByEndtimeDB := database.NewMockDatabase(c)
				proposalIDsByEndtimeDB.EXPECT().NewIterator().Return(proposalsIterator)
				return &caminoState{proposalIDsByEndtimeDB: proposalIDsByEndtimeDB}
			},
			expectedCaminoState: func(actualCaminoState *caminoState) *caminoState {
				return &caminoState{
					proposalIDsByEndtimeDB: actualCaminoState.proposalIDsByEndtimeDB,
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			actualCaminoState := tt.caminoState(ctrl)
			require.ErrorIs(t, actualCaminoState.loadProposals(), tt.expectedErr)
			require.Equal(t, tt.expectedCaminoState(actualCaminoState), actualCaminoState)
		})
	}
}
