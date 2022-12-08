package dao

import (
	"time"
)

const (
	ProposalMaxContentLength  = 1024
	ProposalMaxMetadataLength = 1024
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

	Metadata []byte `serialize:"true"` // used for additional information that are used for the syntactic evaluation of a proposal type (multiple options, thresholds, etc.)
	Content  []byte `serialize:"true"` // used for an IPFS link that contains metadata about the semantics of the proposal (links, images, html, rich text etc.)
}
