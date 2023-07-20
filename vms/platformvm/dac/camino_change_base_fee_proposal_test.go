package dac

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

func TestBaseFeeProposalCreateProposalState(t *testing.T) {
	tests := map[string]struct {
		proposal              *BaseFeeProposal
		allowedVoters         []ids.ShortID
		expectedProposalState ProposalState
		expectedProposal      *BaseFeeProposal
	}{
		"OK: even number of allowed voters": {
			proposal: &BaseFeeProposal{
				Start:   100,
				End:     101,
				Options: []uint64{123, 555, 7},
			},
			allowedVoters: []ids.ShortID{{1}, {2}, {3}, {4}},
			expectedProposalState: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{{1}, {2}, {3}, {4}},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 123},
						{Value: 555},
						{Value: 7},
					},
				},
				SuccessThreshold: 2,
			},
			expectedProposal: &BaseFeeProposal{
				Start:   100,
				End:     101,
				Options: []uint64{123, 555, 7},
			},
		},
		"OK: odd number of allowed voters": {
			proposal: &BaseFeeProposal{
				Start:   100,
				End:     101,
				Options: []uint64{123, 555, 7},
			},
			allowedVoters: []ids.ShortID{{1}, {2}, {3}, {4}, {5}},
			expectedProposalState: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{{1}, {2}, {3}, {4}, {5}},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 123},
						{Value: 555},
						{Value: 7},
					},
				},
				SuccessThreshold: 2,
			},
			expectedProposal: &BaseFeeProposal{
				Start:   100,
				End:     101,
				Options: []uint64{123, 555, 7},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			proposalState := tt.proposal.CreateProposalState(tt.allowedVoters)
			require.Equal(t, tt.expectedProposal, tt.proposal)
			require.Equal(t, tt.expectedProposalState, proposalState)
		})
	}
}

func TestBaseFeeProposalStateAddVote(t *testing.T) {
	voterAddr := ids.ShortID{1}

	tests := map[string]struct {
		proposal                 *BaseFeeProposalState
		voterAddr                ids.ShortID
		vote                     Vote
		expectedUpdatedProposal  ProposalState
		expectedOriginalProposal *BaseFeeProposalState
		expectedErr              error
	}{
		"Wrong vote type": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{voterAddr},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 1}, // 2
					},
					mostVotedWeight:      2,
					mostVotedOptionIndex: 0,
					unambiguous:          false,
				},
			},
			voterAddr: voterAddr,
			vote:      &BaseFeeProposalState{}, // not *SimpleVote
			expectedOriginalProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{voterAddr},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 1}, // 2
					},
					mostVotedWeight:      2,
					mostVotedOptionIndex: 0,
					unambiguous:          false,
				},
			},
			expectedErr: ErrWrongVote,
		},
		"Wrong vote option index": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{voterAddr},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 1}, // 2
					},
					mostVotedWeight:      2,
					mostVotedOptionIndex: 0,
					unambiguous:          false,
				},
			},
			voterAddr: ids.ShortID{3},
			vote:      &SimpleVote{OptionIndex: 3},
			expectedOriginalProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{voterAddr},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 1}, // 2
					},
					mostVotedWeight:      2,
					mostVotedOptionIndex: 0,
					unambiguous:          false,
				},
			},
			expectedErr: ErrWrongVote,
		},
		"OK: adding vote to not voted option": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{voterAddr},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 1}, // 2
					},
					mostVotedWeight:      2,
					mostVotedOptionIndex: 0,
					unambiguous:          false,
				},
			},
			voterAddr: voterAddr,
			vote:      &SimpleVote{OptionIndex: 1},
			expectedUpdatedProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 1}, // 1
						{Value: 30, Weight: 1}, // 2
					},
				},
			},
			expectedOriginalProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{voterAddr},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 1}, // 2
					},
					mostVotedWeight:      2,
					mostVotedOptionIndex: 0,
					unambiguous:          false,
				},
			},
		},
		"OK: adding vote to already voted option": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{voterAddr},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 1}, // 2
					},
					mostVotedWeight:      2,
					mostVotedOptionIndex: 0,
					unambiguous:          false,
				},
			},
			voterAddr: voterAddr,
			vote:      &SimpleVote{OptionIndex: 2},
			expectedUpdatedProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 2}, // 2
					},
				},
			},
			expectedOriginalProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{voterAddr},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 10, Weight: 2}, // 0
						{Value: 20, Weight: 0}, // 1
						{Value: 30, Weight: 1}, // 2
					},
					mostVotedWeight:      2,
					mostVotedOptionIndex: 0,
					unambiguous:          false,
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			updatedProposal, err := tt.proposal.AddVote(tt.voterAddr, tt.vote)
			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expectedUpdatedProposal, updatedProposal)
			require.Equal(t, tt.expectedOriginalProposal, tt.proposal)
		})
	}
}

func TestBaseFeeProposalStateGetMostVoted(t *testing.T) {
	tests := map[string]struct {
		proposal                *BaseFeeProposalState
		expectedProposal        *BaseFeeProposalState
		expectedMostVotedWeight uint32
		expectedMostVotedIndex  uint32
		expectedUnambiguous     bool
	}{
		"OK: 3 different weights, no cache": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 1, Weight: 4}, // 0
						{Value: 2, Weight: 7}, // 1
						{Value: 3, Weight: 5}, // 2
					},
				},
			},
			expectedProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 1, Weight: 4}, // 0
						{Value: 2, Weight: 7}, // 1
						{Value: 3, Weight: 5}, // 2
					},
				},
			},
			expectedMostVotedWeight: 7,
			expectedMostVotedIndex:  1,
			expectedUnambiguous:     true,
		},
		"OK: 2 equal and 1 higher weight, no cache": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 1, Weight: 5}, // 0
						{Value: 2, Weight: 7}, // 1
						{Value: 3, Weight: 5}, // 2
					},
				},
			},
			expectedProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 1, Weight: 5}, // 0
						{Value: 2, Weight: 7}, // 1
						{Value: 3, Weight: 5}, // 2
					},
				},
			},
			expectedMostVotedWeight: 7,
			expectedMostVotedIndex:  1,
			expectedUnambiguous:     true,
		},
		"OK: 2 equal and 1 lower weight, no cache": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 1, Weight: 4}, // 0
						{Value: 2, Weight: 5}, // 1
						{Value: 3, Weight: 5}, // 2
					},
				},
			},
			expectedProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 1, Weight: 4}, // 0
						{Value: 2, Weight: 5}, // 1
						{Value: 3, Weight: 5}, // 2
					},
				},
			},
			expectedMostVotedWeight: 5,
			expectedMostVotedIndex:  1,
			expectedUnambiguous:     false,
		},
		"OK: 3 equal weights, no cache": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 1, Weight: 5}, // 0
						{Value: 2, Weight: 5}, // 1
						{Value: 3, Weight: 5}, // 2
					},
				},
			},
			expectedProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					Options: []SimpleVoteOption[uint64]{
						{Value: 1, Weight: 5}, // 0
						{Value: 2, Weight: 5}, // 1
						{Value: 3, Weight: 5}, // 2
					},
				},
			},
			expectedMostVotedWeight: 5,
			expectedMostVotedIndex:  0,
			expectedUnambiguous:     false,
		},
		"OK: cached result": {
			proposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					mostVotedWeight:      5,
					mostVotedOptionIndex: 5,
					unambiguous:          false,
				},
			},
			expectedProposal: &BaseFeeProposalState{
				Start:         100,
				End:           101,
				AllowedVoters: []ids.ShortID{},
				SimpleVoteOptions: SimpleVoteOptions[uint64]{
					mostVotedWeight:      5,
					mostVotedOptionIndex: 5,
					unambiguous:          false,
				},
			},
			expectedMostVotedWeight: 5,
			expectedMostVotedIndex:  5,
			expectedUnambiguous:     false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mostVotedWeight, mostVotedIndex, unambiguous := tt.proposal.GetMostVoted()
			require.Equal(t, tt.expectedProposal, tt.proposal)
			require.Equal(t, tt.expectedMostVotedWeight, mostVotedWeight)
			require.Equal(t, tt.expectedMostVotedIndex, mostVotedIndex)
			require.Equal(t, tt.expectedUnambiguous, unambiguous)
		})
	}
}

func TestBaseFeeProposalStateCanBeVotedBy(t *testing.T) {
	tests := map[string]struct {
		proposal         *BaseFeeProposalState
		voterAddr        ids.ShortID
		expectedProposal *BaseFeeProposalState
		expectedResult   bool
	}{
		"Not allowed": {
			proposal: &BaseFeeProposalState{
				AllowedVoters: []ids.ShortID{{1}, {2}},
			},
			voterAddr: ids.ShortID{3},
			expectedProposal: &BaseFeeProposalState{
				AllowedVoters: []ids.ShortID{{1}, {2}},
			},
			expectedResult: false,
		},
		"OK": {
			proposal: &BaseFeeProposalState{
				AllowedVoters: []ids.ShortID{{1}, {2}},
			},
			voterAddr: ids.ShortID{1},
			expectedProposal: &BaseFeeProposalState{
				AllowedVoters: []ids.ShortID{{1}, {2}},
			},
			expectedResult: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := tt.proposal.CanBeVotedBy(tt.voterAddr)
			require.Equal(t, tt.expectedResult, result)
			require.Equal(t, tt.expectedProposal, tt.proposal)
		})
	}
}
