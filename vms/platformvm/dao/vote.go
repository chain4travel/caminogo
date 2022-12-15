package dao

import (
	"fmt"
)

type VoteType uint64

const (
	Accept  VoteType = iota // I support this proposal
	Reject                  // I dont support this propsal
	Abstain                 // I want to remain neutral
)

var (
	errInvalidVoteType = fmt.Errorf("invalid voteType")
)

type Vote struct {
	Vote VoteType `serialize:"true" json:"voteType"`
}

func (v Vote) Verify() error {

	switch v.Vote {
	case Accept, Reject, Abstain:
		break
	default:
		return errInvalidVoteType
	}

	return nil
}
