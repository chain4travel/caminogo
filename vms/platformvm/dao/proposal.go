package dao

import (
	"fmt"
	"time"
)

const (
	ProposalMaxContentLength = 1024
)

var (
	errContentTooLong        = fmt.Errorf("content is longer than %d bytes", ProposalMaxContentLength)
	errInvalidProposalType   = fmt.Errorf("invalid proposalType")
	errProposalTypeMissmatch = fmt.Errorf("proposalType and proposalMetadata do not match")
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

	Metadata ProposalMetadata `serialize:"true"` // used for additional information that are used for the syntactic evaluation of a proposal type (multiple options, thresholds, etc.)
	Content  []byte           `serialize:"true"` // used for an IPFS link that contains metadata about the semantics of the proposal (links, images, html, rich text etc.)
}

// utility functions
func (p Proposal) Duration() time.Duration {
	return p.EndTime.Sub(p.StartTime)
}

func (p Proposal) Verify() error {

	if len(p.Content) > ProposalMaxContentLength {
		return errContentTooLong
	}

	if err := p.Metadata.Verify(); err != nil {
		return err
	}

	// verify metadata
	switch p.Type {
	case ProposalTypeNOP:
		metadata, ok := p.Metadata.(NOPProposalMetadata)
		if !ok {
			return errProposalTypeMissmatch
		}
		if err := metadata.Verify(); err != nil {
			return err
		}

	default:
		return errInvalidProposalType
	}

	return nil
}
