package dao

import (
	"time"
)

const (
	ProposalContentLength = 1024
)

type ProposalType uint64

const (
	ProposalTypeNOP ProposalType = iota // no action on chain (NO OPERATION)
)

type ProposalState uint64

const (
	ProposalStatePending ProposalState = iota
	ProposalStateActive
	ProposalStateConcluded
)

type Proposal struct {
	Type ProposalType `serialize:"true"`

	StartTime time.Time `serialize:"true"`
	EndTime   time.Time `serialize:"true"`

	Content []byte `serialize:"true"`
}
