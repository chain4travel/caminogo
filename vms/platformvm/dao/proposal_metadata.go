package dao

import (
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

type ProposalMetadata interface {
	verify.Verifiable
	AcceptVote(*Vote) error
}
