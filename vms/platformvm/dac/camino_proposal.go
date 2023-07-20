// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

var (
	errEndNotAfterStart = errors.New("proposal end-time is not after start-time")
	ErrWrongVote        = errors.New("this proposal can't be voted with this vote")
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
	CreateProposalState(allowedVoters []ids.ShortID) ProposalState
	Visit(VerifierVisitor) error
}

type ProposalState interface {
	verify.Verifiable

	EndTime() time.Time
	IsActiveAt(time time.Time) bool
	CanBeVotedBy(voterAddr ids.ShortID) bool
	CanBeFinished() bool
	Visit(ExecutorVisitor) error
	// Will return modified ProposalState with added vote, original ProposalState will not be modified!
	AddVote(voterAddress ids.ShortID, vote Vote) (ProposalState, error)
}
