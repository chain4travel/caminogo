// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"bytes"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"golang.org/x/exp/slices"
)

const MaxProposalSize = 2048 // TODO@

var (
	errEndNotAfterStart               = errors.New("proposal end-time is not after start-time")
	ErrWrongVote                      = errors.New("this proposal can't be voted with this vote")
	ErrNotAllowedToVoteOnThisProposal = errors.New("this address has already voted or not allowed to vote on this proposal")
)

type VerifierVisitor interface {
	BaseFeeProposal(*BaseFeeProposal) error
}

type ExecutorVisitor interface {
	BaseFeeProposal(*BaseFeeProposalState) error
}

type Proposal interface {
	verify.Verifiable

	StartTime() time.Time
	// EndTime() time.Time
	// IsActiveAt(time time.Time) bool

	CreateProposalState(allowedVoters []ids.ShortID) ProposalState
	Visit(VerifierVisitor) error
}

type ProposalState interface {
	verify.Verifiable

	// StartTime() time.Time
	EndTime() time.Time
	IsActiveAt(time time.Time) bool

	VerifyCanVote(voterAddr ids.ShortID) error
	CanBeFinished() bool
	Visit(ExecutorVisitor) error
	// Will return modified proposal with added vote, original proposal will not be modified!
	AddVote(voterAddress ids.ShortID, vote Vote) (ProposalState, error)
}

func verifyCanVote(allowedVoters []ids.ShortID, voterAddr ids.ShortID) error {
	_, allowedToVote := slices.BinarySearchFunc(allowedVoters, voterAddr, func(id, other ids.ShortID) int {
		return bytes.Compare(id[:], other[:])
	})
	if !allowedToVote {
		return ErrNotAllowedToVoteOnThisProposal
	}
	return nil
}
