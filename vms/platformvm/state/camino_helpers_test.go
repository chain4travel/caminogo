package state

import "github.com/ava-labs/avalanchego/ids"

func generateXNumberOfProposalsWithDifferentIds(times int) map[ids.ID]*Proposal {
	proposals := make(map[ids.ID]*Proposal)
	for i := 0; i < times; i++ {
		proposals[ids.GenerateTestID()] = &Proposal{TxID: ids.GenerateTestID()}
	}
	return proposals
}

func generateXNumberOfVotesWithDifferentIds(times int) map[ids.ID]*Vote {
	votes := make(map[ids.ID]*Vote)
	for i := 0; i < times; i++ {
		votes[ids.GenerateTestID()] = &Vote{TxID: ids.GenerateTestID()}
	}
	return votes
}
