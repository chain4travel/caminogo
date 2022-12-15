package dao

import (
	"fmt"
	"time"
)

const (
	ProposalMaxContentLength = 1024
)

var (
<<<<<<< HEAD
	errContentTooLong        = fmt.Errorf("content is longer than %d bytes", ProposalMaxContentLength)
	errInvalidProposalType   = fmt.Errorf("invalid proposalType")
	errProposalTypeMissmatch = fmt.Errorf("proposalType and proposalMetadata do not match")
	errProposalEnded         = fmt.Errorf("proposal has ended")
	errProposalNotStarted    = fmt.Errorf("proposas has not started yet")
=======
	errContentTooLong     = fmt.Errorf("content is longer than %d bytes", ProposalMaxContentLength)
	errProposalEnded      = fmt.Errorf("proposal has ended")
	errProposalNotStarted = fmt.Errorf("proposas has not started yet")
>>>>>>> 36e94f777 (intermediate)
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
	StartTime time.Time `serialize:"true", json:"startTime"`
	EndTime   time.Time `serialize:"true", json:"endTime"`

	Metadata ProposalMetadata `serialize:"true", json:"metadata"` // used for additional information that are used for the syntactic evaluation of a proposal type (multiple options, thresholds, etc.)
	Content  []byte           `serialize:"true", json:"content"`  // used for an IPFS link that contains metadata about the semantics of the proposal (links, images, html, rich text etc.)
}

// utility functions
func (p Proposal) Duration() time.Duration {
	return p.EndTime.Sub(p.StartTime)
}

func (p Proposal) IsActive(currentTime time.Time) error {
	if currentTime.After(p.EndTime) {
		return errProposalEnded
	}
	if currentTime.Before(p.StartTime) {
		return errProposalNotStarted
	}
	return nil
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
