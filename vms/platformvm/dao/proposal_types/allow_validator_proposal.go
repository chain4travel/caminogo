package proposaltypes

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
)

const ()

var ()

type ValidatorProposalAction uint64

const (
	ValidatorProposalActionRemove ValidatorProposalAction = iota
	ValidatorProposalActionAllow
)

type ValidatorProposalMetadata interface {
	dao.ProposalMetadata
	Action() ValidatorProposalAction
	Address() ids.ShortID
}

type validatorProposalMetadata struct {
	ProposalAction ValidatorProposalAction `serialize:"true"`
	TargetAddress  ids.ShortID             `serialize:"true"`
}

func (vpm validatorProposalMetadata) Verify() error {
	return nil
}

func (vpm validatorProposalMetadata) AcceptVote(vote *dao.Vote) error {
	if vote.Vote == dao.Abstain {
		return errNOPProposalAbstainNotAllowed
	}

	return nil
}

func (vpm validatorProposalMetadata) Action() ValidatorProposalAction { return vpm.ProposalAction }
func (vpm validatorProposalMetadata) Address() ids.ShortID            { return vpm.TargetAddress }
