// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	as "github.com/ava-labs/avalanchego/vms/platformvm/addrstate"
)

const (
	ExcludeMemberProposalMinDuration = uint64(time.Hour * 24 * 7 / time.Second)  // 7 days
	ExcludeMemberProposalMaxDuration = uint64(time.Hour * 24 * 30 / time.Second) // 30 days
)

var (
	_ Proposal      = (*ExcludeMemberProposal)(nil)
	_ ProposalState = (*ExcludeMemberProposalState)(nil)
)

type ExcludeMemberProposal struct {
	MemberAddress ids.ShortID `serialize:"true"`
	Start         uint64      `serialize:"true"`
	End           uint64      `serialize:"true"`
}

func (p *ExcludeMemberProposal) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *ExcludeMemberProposal) EndTime() time.Time {
	return time.Unix(int64(p.End), 0)
}

func (*ExcludeMemberProposal) GetOptions() any {
	return []bool{true, false}
}

func (p *ExcludeMemberProposal) GetData() any {
	return p.MemberAddress
}

func (*ExcludeMemberProposal) AdminProposer() as.AddressState {
	return as.AddressStateRoleConsortiumSecretary
}

func (p *ExcludeMemberProposal) Verify() error {
	switch {
	case p.Start >= p.End:
		return errEndNotAfterStart
	case p.End-p.Start < ExcludeMemberProposalMinDuration:
		return fmt.Errorf("%w (expected: minimum duration %d, actual: %d)", errWrongDuration, ExcludeMemberProposalMinDuration, p.End-p.Start)
	case p.End-p.Start > ExcludeMemberProposalMaxDuration:
		return fmt.Errorf("%w (expected: maximum duration %d, actual: %d)", errWrongDuration, ExcludeMemberProposalMaxDuration, p.End-p.Start)
	}
	return nil
}

func (p *ExcludeMemberProposal) CreateProposalState(allowedVoters []ids.ShortID) ProposalState {
	stateProposal := &ExcludeMemberProposalState{
		SimpleVoteOptions: SimpleVoteOptions[bool]{
			Options: []SimpleVoteOption[bool]{
				{Value: true},
				{Value: false},
			},
		},
		MemberAddress:      p.MemberAddress,
		Start:              p.Start,
		End:                p.End,
		AllowedVoters:      allowedVoters,
		TotalAllowedVoters: uint32(len(allowedVoters)),
	}
	return stateProposal
}

func (p *ExcludeMemberProposal) CreateFinishedProposalState(optionIndex uint32) (ProposalState, error) {
	if optionIndex >= 2 {
		return nil, fmt.Errorf("%w (expected: less than 2, actual: %d)", errWrongOptionIndex, optionIndex)
	}
	proposalState := p.CreateProposalState([]ids.ShortID{}).(*ExcludeMemberProposalState)
	proposalState.Options[optionIndex].Weight++
	return proposalState, nil
}

func (p *ExcludeMemberProposal) VerifyWith(verifier Verifier) error {
	return verifier.ExcludeMemberProposal(p)
}

type ExcludeMemberProposalState struct {
	SimpleVoteOptions[bool] `serialize:"true"`

	MemberAddress      ids.ShortID   `serialize:"true"`
	Start              uint64        `serialize:"true"`
	End                uint64        `serialize:"true"`
	AllowedVoters      []ids.ShortID `serialize:"true"`
	TotalAllowedVoters uint32        `serialize:"true"`
}

func (p *ExcludeMemberProposalState) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *ExcludeMemberProposalState) EndTime() time.Time {
	return time.Unix(int64(p.End), 0)
}

func (p *ExcludeMemberProposalState) IsActiveAt(time time.Time) bool {
	timestamp := uint64(time.Unix())
	return p.Start <= timestamp && timestamp <= p.End
}

func (p *ExcludeMemberProposalState) CanBeFinished() bool {
	mostVotedWeight, _, unambiguous := p.GetMostVoted()
	voted := p.Voted()
	// We don't check for 'no option can reach 50%+ of votes' for this proposal type, cause its impossible with just 2 options
	return voted == p.TotalAllowedVoters ||
		unambiguous && mostVotedWeight > p.TotalAllowedVoters/2
}

func (p *ExcludeMemberProposalState) IsSuccessful() bool {
	mostVotedWeight, _, unambiguous := p.GetMostVoted()
	voted := p.Voted()
	return unambiguous && voted > p.TotalAllowedVoters/2 && mostVotedWeight > voted/2
}

func (p *ExcludeMemberProposalState) Outcome() any {
	_, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	if !unambiguous {
		return -1
	}
	return mostVotedOptionIndex
}

func (p *ExcludeMemberProposalState) Result() (bool, uint32, bool) {
	mostVotedWeight, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	return p.Options[mostVotedOptionIndex].Value, mostVotedWeight, unambiguous
}

// Will return modified proposal with added vote, original proposal will not be modified!
func (p *ExcludeMemberProposalState) AddVote(voterAddress ids.ShortID, voteIntf Vote, isCairoPhase bool) (ProposalState, error) {
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
func (p *ExcludeMemberProposalState) ForceAddVote(voteIntf Vote, isCairoPhase bool) (ProposalState, error) {
	return p.addVote(voteIntf, isCairoPhase)
}

func (p *ExcludeMemberProposalState) addVote(voteIntf Vote, _ bool) (*ExcludeMemberProposalState, error) {
	simpleVoteOptions, err := p.AddWeight(voteIntf)
	if err != nil {
		return nil, err
	}

	return &ExcludeMemberProposalState{
		MemberAddress:      p.MemberAddress,
		Start:              p.Start,
		End:                p.End,
		AllowedVoters:      p.AllowedVoters,
		SimpleVoteOptions:  *simpleVoteOptions,
		TotalAllowedVoters: p.TotalAllowedVoters,
	}, nil
}

func (p *ExcludeMemberProposalState) ExecuteWith(executor Executor) error {
	return executor.ExcludeMemberProposal(p)
}

func (p *ExcludeMemberProposalState) GetBondTxIDsWith(bondTxIDsGetter BondTxIDsGetter) ([]ids.ID, error) {
	return bondTxIDsGetter.ExcludeMemberProposal(p)
}
