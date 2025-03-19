// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	as "github.com/ava-labs/avalanchego/vms/platformvm/addrstate"
)

const baseFeeProposalMaxOptionsCount = 3

var (
	_ Proposal      = (*BaseFeeProposal)(nil)
	_ ProposalState = (*BaseFeeProposalState)(nil)

	errZeroFee = errors.New("base-fee option is zero")
)

type BaseFeeProposal struct {
	Options []uint64 `serialize:"true"` // New base fee options
	Start   uint64   `serialize:"true"` // Start time of proposal
	End     uint64   `serialize:"true"` // End time of proposal
}

func (p *BaseFeeProposal) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *BaseFeeProposal) EndTime() time.Time {
	return time.Unix(int64(p.End), 0)
}

func (p *BaseFeeProposal) GetOptions() any {
	return p.Options
}

func (*BaseFeeProposal) AdminProposer() as.AddressState {
	return as.AddressStateEmpty // for now its forbidden, until we'll introduce dedicated role for it
}

func (p *BaseFeeProposal) Verify() error {
	switch {
	case len(p.Options) == 0:
		return errNoOptions
	case len(p.Options) > baseFeeProposalMaxOptionsCount:
		return fmt.Errorf("%w (expected: no more than %d, actual: %d)", errWrongOptionsCount, baseFeeProposalMaxOptionsCount, len(p.Options))
	case p.Start >= p.End:
		return errEndNotAfterStart
	}

	unique := set.NewSet[uint64](len(p.Options))
	for _, fee := range p.Options {
		if fee == 0 {
			return errZeroFee
		}
		if unique.Contains(fee) {
			return errNotUniqueOption
		}
		unique.Add(fee)
	}

	return nil
}

func (p *BaseFeeProposal) CreateProposalState(allowedVoters []ids.ShortID) ProposalState {
	stateProposal := &BaseFeeProposalState{
		SimpleVoteOptions: SimpleVoteOptions[uint64]{
			Options: make([]SimpleVoteOption[uint64], len(p.Options)),
		},
		Start:              p.Start,
		End:                p.End,
		AllowedVoters:      allowedVoters,
		TotalAllowedVoters: uint32(len(allowedVoters)),
	}
	for i := range p.Options {
		stateProposal.Options[i].Value = p.Options[i]
	}
	return stateProposal
}

func (p *BaseFeeProposal) CreateFinishedProposalState(optionIndex uint32) (ProposalState, error) {
	if optionIndex >= uint32(len(p.Options)) {
		return nil, fmt.Errorf("%w (expected: less than %d, actual: %d)", errWrongOptionIndex, len(p.Options), optionIndex)
	}
	proposalState := p.CreateProposalState([]ids.ShortID{}).(*BaseFeeProposalState)
	proposalState.Options[optionIndex].Weight++
	return proposalState, nil
}

func (p *BaseFeeProposal) VerifyWith(verifier Verifier) error {
	return verifier.BaseFeeProposal(p)
}

type BaseFeeProposalState struct {
	SimpleVoteOptions[uint64] `serialize:"true"` // New base fee options
	// Start time of proposal
	Start uint64 `serialize:"true"`
	// End time of proposal
	End uint64 `serialize:"true"`
	// Addresses that are allowed to vote for this proposal
	AllowedVoters []ids.ShortID `serialize:"true"`
	// Number of addresses that were initially allowed to vote for this proposal.
	// This is used to calculate thresholds like "half of total voters".
	TotalAllowedVoters uint32 `serialize:"true"`
}

func (p *BaseFeeProposalState) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *BaseFeeProposalState) EndTime() time.Time {
	return time.Unix(int64(p.End), 0)
}

func (p *BaseFeeProposalState) IsActiveAt(time time.Time) bool {
	timestamp := uint64(time.Unix())
	return p.Start <= timestamp && timestamp <= p.End
}

func (p *BaseFeeProposalState) CanBeFinished() bool {
	mostVotedWeight, _, unambiguous := p.GetMostVoted()
	voted := p.Voted()
	return p.TotalAllowedVoters-voted+mostVotedWeight < voted/2+1 ||
		voted == p.TotalAllowedVoters ||
		unambiguous && mostVotedWeight > p.TotalAllowedVoters/2
}

func (p *BaseFeeProposalState) IsSuccessful() bool {
	mostVotedWeight, _, unambiguous := p.GetMostVoted()
	voted := p.Voted()
	return unambiguous && voted > p.TotalAllowedVoters/2 && mostVotedWeight > voted/2
}

func (p *BaseFeeProposalState) Outcome() any {
	_, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	if !unambiguous {
		return -1
	}
	return mostVotedOptionIndex
}

func (p *BaseFeeProposalState) Result() (uint64, uint32, bool) {
	mostVotedWeight, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	return p.Options[mostVotedOptionIndex].Value, mostVotedWeight, unambiguous
}

// Will return modified proposal with added vote, original proposal will not be modified!
func (p *BaseFeeProposalState) AddVote(voterAddress ids.ShortID, voteIntf Vote, isCairoPhase bool) (ProposalState, error) {
	updatedProposal, err := p.addVote(voteIntf, isCairoPhase)
	if err != nil {
		return nil, err
	}
	updatedProposal.AllowedVoters, err = excludeFromAllowedVoters(p.AllowedVoters, voterAddress)
	if err != nil {
		return nil, err
	}
	return updatedProposal, nil
}

// Will return modified proposal with added vote ignoring allowed voters, original proposal will not be modified!
func (p *BaseFeeProposalState) ForceAddVote(voteIntf Vote, isCairoPhase bool) (ProposalState, error) {
	return p.addVote(voteIntf, isCairoPhase)
}

func (p *BaseFeeProposalState) addVote(voteIntf Vote, _ bool) (*BaseFeeProposalState, error) {
	simpleVoteOptions, err := p.AddWeight(voteIntf)
	if err != nil {
		return nil, err
	}

	return &BaseFeeProposalState{
		Start:              p.Start,
		End:                p.End,
		AllowedVoters:      p.AllowedVoters,
		SimpleVoteOptions:  *simpleVoteOptions,
		TotalAllowedVoters: p.TotalAllowedVoters,
	}, nil
}

func (p *BaseFeeProposalState) ExecuteWith(executor Executor) error {
	return executor.BaseFeeProposal(p)
}

func (p *BaseFeeProposalState) GetBondTxIDsWith(bondTxIDsGetter BondTxIDsGetter) ([]ids.ID, error) {
	return bondTxIDsGetter.BaseFeeProposal(p)
}
