package dao

import (
	"fmt"
)

const (
	MaxDummyDataLength = 64
)

var (
	errNOPProposalAbstainNotAllowed = fmt.Errorf("abstain votes are not allowed for NOP Proposals")
)

type NOPProposalMetadata interface {
	ProposalMetadata
	GetDummyData() []byte
}

type nopProposalMetadata struct {
	dummyData []byte `serialize:"true"`
}

func (nop nopProposalMetadata) Verify() error {
	if len(nop.dummyData) > MaxDummyDataLength {
		return fmt.Errorf("dummyData is too long")
	}
	return nil
}

func (nop nopProposalMetadata) AcceptVote(vote *Vote) error {
	if vote.Vote == Abstain {
		return errNOPProposalAbstainNotAllowed
	}

	return nil
}

func (nop nopProposalMetadata) GetDummyData() []byte {
	return nop.dummyData
}
