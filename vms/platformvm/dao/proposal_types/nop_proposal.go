package proposaltypes

import (
	"fmt"

	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
)

const (
	MaxDummyDataLength = 64
)

var (
	errNOPProposalAbstainNotAllowed = fmt.Errorf("abstain votes are not allowed for NOP Proposals")
)

type NOPProposalMetadata interface {
	dao.ProposalMetadata
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

func (nop nopProposalMetadata) AcceptVote(vote *dao.Vote) error {
	if vote.Vote == dao.Abstain {
		return errNOPProposalAbstainNotAllowed
	}

	return nil
}

func (nop nopProposalMetadata) GetDummyData() []byte {
	return nop.dummyData
}
