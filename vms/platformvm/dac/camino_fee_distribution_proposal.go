// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	as "github.com/ava-labs/avalanchego/vms/platformvm/addrstate"
)

const (
	feeDistributionProposalMaxOptionsCount = 3
	FeeDistributionFractionsCount          = 3
)

var (
	_ Proposal      = (*FeeDistributionProposal)(nil)
	_ ProposalState = (*FeeDistributionProposalState)(nil)

	errWrongFeeDistribution = errors.New("fee distribution sum is not equal to 100%")
)

type FeeDistributionProposal struct {
	// New fee distribution options. Fee is distributed between validators, grant program and burning.
	// Each uint64 value is nominator that represents fraction of fee distribution.
	Options [][FeeDistributionFractionsCount]uint64 `serialize:"true"`
	Start   uint64                                  `serialize:"true"` // Start time of proposal
	End     uint64                                  `serialize:"true"` // End time of proposal
}

func (p *FeeDistributionProposal) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *FeeDistributionProposal) EndTime() time.Time {
	return time.Unix(int64(p.End), 0)
}

func (p *FeeDistributionProposal) GetOptions() any {
	return p.Options
}

// This proposal type cannot be admin proposal
func (*FeeDistributionProposal) AdminProposer() as.AddressState {
	return as.AddressStateEmpty
}

func (p *FeeDistributionProposal) Verify() error {
	switch {
	case len(p.Options) == 0:
		return errNoOptions
	case len(p.Options) > feeDistributionProposalMaxOptionsCount:
		return fmt.Errorf("%w (expected: no more than %d, actual: %d)", errWrongOptionsCount, feeDistributionProposalMaxOptionsCount, len(p.Options))
	case p.Start >= p.End:
		return errEndNotAfterStart
	}
	unique := set.NewSet[[FeeDistributionFractionsCount]uint64](len(p.Options))
	for _, option := range p.Options {
		sum, err := math.Add64(option[0], option[1])
		if err != nil {
			return fmt.Errorf("%w: %s", errWrongFeeDistribution, err)
		}
		sum, err = math.Add64(sum, option[2])
		if err != nil {
			return fmt.Errorf("%w: %s", errWrongFeeDistribution, err)
		}
		if sum != FractionDenominator {
			return errWrongFeeDistribution
		}
		if unique.Contains(option) {
			return errNotUniqueOption
		}
		unique.Add(option)
	}
	return nil
}

func (p *FeeDistributionProposal) CreateProposalState(allowedVoters []ids.ShortID) ProposalState {
	stateProposal := &FeeDistributionProposalState{
		SimpleVoteOptions: SimpleVoteOptions[[FeeDistributionFractionsCount]uint64]{
			Options: make([]SimpleVoteOption[[FeeDistributionFractionsCount]uint64], len(p.Options)),
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

func (p *FeeDistributionProposal) CreateFinishedProposalState(optionIndex uint32) (ProposalState, error) {
	if optionIndex >= uint32(len(p.Options)) {
		return nil, fmt.Errorf("%w (expected: less than %d, actual: %d)", errWrongOptionIndex, len(p.Options), optionIndex)
	}
	proposalState := p.CreateProposalState([]ids.ShortID{}).(*FeeDistributionProposalState)
	proposalState.Options[optionIndex].Weight++
	return proposalState, nil
}

func (p *FeeDistributionProposal) VerifyWith(verifier Verifier) error {
	return verifier.FeeDistributionProposal(p)
}

type FeeDistributionProposalState struct {
	SimpleVoteOptions[[FeeDistributionFractionsCount]uint64] `serialize:"true"` // New base fee options
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

func (p *FeeDistributionProposalState) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *FeeDistributionProposalState) EndTime() time.Time {
	return time.Unix(int64(p.End), 0)
}

func (p *FeeDistributionProposalState) IsActiveAt(time time.Time) bool {
	timestamp := uint64(time.Unix())
	return p.Start <= timestamp && timestamp <= p.End
}

func (p *FeeDistributionProposalState) CanBeFinished() bool {
	mostVotedWeight, _, unambiguous := p.GetMostVoted()
	voted := p.Voted()
	return p.TotalAllowedVoters-voted+mostVotedWeight < voted/2+1 ||
		voted == p.TotalAllowedVoters ||
		unambiguous && mostVotedWeight > p.TotalAllowedVoters/2
}

func (p *FeeDistributionProposalState) IsSuccessful() bool {
	mostVotedWeight, _, unambiguous := p.GetMostVoted()
	voted := p.Voted()
	return unambiguous && voted > p.TotalAllowedVoters/2 && mostVotedWeight > voted/2
}

func (p *FeeDistributionProposalState) Outcome() any {
	_, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	if !unambiguous {
		return -1
	}
	return mostVotedOptionIndex
}

func (p *FeeDistributionProposalState) Result() ([FeeDistributionFractionsCount]uint64, uint32, bool) {
	mostVotedWeight, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	return p.Options[mostVotedOptionIndex].Value, mostVotedWeight, unambiguous
}

// Will return modified proposal with added vote, original proposal will not be modified!
func (p *FeeDistributionProposalState) AddVote(voterAddress ids.ShortID, voteIntf Vote, isCairoPhase bool) (ProposalState, error) {
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
func (p *FeeDistributionProposalState) ForceAddVote(voteIntf Vote, isCairoPhase bool) (ProposalState, error) {
	return p.addVote(voteIntf, isCairoPhase)
}

func (p *FeeDistributionProposalState) addVote(voteIntf Vote, _ bool) (*FeeDistributionProposalState, error) {
	simpleVoteOptions, err := p.AddWeight(voteIntf)
	if err != nil {
		return nil, err
	}

	return &FeeDistributionProposalState{
		Start:              p.Start,
		End:                p.End,
		AllowedVoters:      p.AllowedVoters,
		SimpleVoteOptions:  *simpleVoteOptions,
		TotalAllowedVoters: p.TotalAllowedVoters,
	}, nil
}

func (p *FeeDistributionProposalState) ExecuteWith(executor Executor) error {
	return executor.FeeDistributionProposal(p)
}

func (p *FeeDistributionProposalState) GetBondTxIDsWith(bondTxIDsGetter BondTxIDsGetter) ([]ids.ID, error) {
	return bondTxIDsGetter.FeeDistributionProposal(p)
}
