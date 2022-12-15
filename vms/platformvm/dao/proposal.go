package dao

import (
	"fmt"
	"time"
)

const (
	ProposalMaxContentLength = 1024
)

var (
	errContentTooLong     = fmt.Errorf("content is longer than %d bytes", ProposalMaxContentLength)
	errProposalEnded      = fmt.Errorf("proposal has ended")
	errProposalNotStarted = fmt.Errorf("proposas has not started yet")
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
	StartTime time.Time `serialize:"true" json:"startTime"`
	EndTime   time.Time `serialize:"true" json:"endTime"`

	Metadata ProposalMetadata `serialize:"true" json:"metadata"` // used for additional information that are used for the syntactic evaluation of a proposal type (multiple options, thresholds, etc.)
	Content  []byte           `serialize:"true" json:"content"`  // used for an IPFS link that contains metadata about the semantics of the proposal (links, images, html, rich text etc.)
}

// compares two proposals with respect to their start and end times
//
//	0 => a == b
//
// -1 => a < b
// +1 => a > b
func CompareProposals(a, b Proposal) int {
	if a.StartTime.Before(b.StartTime) {
		return -1
	}
	if a.StartTime.Equal(b.StartTime) {
		if a.EndTime.Before(b.EndTime) {
			return -1
		}
		if a.EndTime.Equal(b.EndTime) {
			return 0
		}
		return 1
	}
	return 1
}

// utility functions
func (p Proposal) Duration() time.Duration {
	return p.EndTime.Sub(p.StartTime)
}

func (p Proposal) StateAtTime(currentTime time.Time) ProposalState {
	switch {
	case currentTime.Before(p.StartTime):
		return ProposalStatePending
	case currentTime.After(p.StartTime) && currentTime.Before(p.EndTime):
		return ProposalStateActive
	default:
		return ProposalStateConcluded
	}
}

func (p Proposal) IsActive(currentTime time.Time) error {

	switch p.StateAtTime(currentTime) {
	case ProposalStateActive:
		return nil
	case ProposalStateConcluded:
		return errProposalEnded
	case ProposalStatePending:
		return errProposalNotStarted
	default:
		panic("unknown proposal state")
	}
}

func (p Proposal) Verify() error {

	if len(p.Content) > ProposalMaxContentLength {
		return errContentTooLong
	}

	// verify metadata
	if err := p.Metadata.Verify(); err != nil {
		return err
	}

	return nil
}
