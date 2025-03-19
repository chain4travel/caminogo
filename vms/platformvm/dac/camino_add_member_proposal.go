// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	as "github.com/ava-labs/avalanchego/vms/platformvm/addrstate"
)

const AddMemberProposalDuration = uint64(time.Hour * 24 * 30 * 2 / time.Second) // 2 month

var (
	_ Proposal      = (*AddMemberProposal)(nil)
	_ ProposalState = (*AddMemberProposalState)(nil)
)

type AddMemberProposal struct {
	ApplicantAddress ids.ShortID `serialize:"true"`
	Start            uint64      `serialize:"true"`
	End              uint64      `serialize:"true"`
}

func (p *AddMemberProposal) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *AddMemberProposal) EndTime() time.Time {
	return time.Unix(int64(p.End), 0)
}

func (*AddMemberProposal) GetOptions() any {
	return []bool{true, false}
}

func (*AddMemberProposal) AdminProposer() as.AddressState {
	return as.AddressStateRoleConsortiumSecretary
}

func (p *AddMemberProposal) Verify() error {
	switch {
	case p.Start >= p.End:
		return errEndNotAfterStart
	case p.End-p.Start != AddMemberProposalDuration:
		return fmt.Errorf("%w (expected: %d, actual: %d)", errWrongDuration, AddMemberProposalDuration, p.End-p.Start)
	}
	return nil
}

func (p *AddMemberProposal) CreateProposalState(allowedVoters []ids.ShortID) ProposalState {
	stateProposal := &AddMemberProposalState{
		SimpleVoteOptions: SimpleVoteOptions[bool]{
			Options: []SimpleVoteOption[bool]{
				{Value: true},
				{Value: false},
			},
		},
		ApplicantAddress:   p.ApplicantAddress,
		Start:              p.Start,
		End:                p.End,
		AllowedVoters:      allowedVoters,
		TotalAllowedVoters: uint32(len(allowedVoters)),
	}
	return stateProposal
}

func (p *AddMemberProposal) CreateFinishedProposalState(optionIndex uint32) (ProposalState, error) {
	if optionIndex >= 2 {
		return nil, fmt.Errorf("%w (expected: less than 2, actual: %d)", errWrongOptionIndex, optionIndex)
	}
	proposalState := p.CreateProposalState([]ids.ShortID{}).(*AddMemberProposalState)
	proposalState.Options[optionIndex].Weight++
	return proposalState, nil
}

func (p *AddMemberProposal) VerifyWith(verifier Verifier) error {
	return verifier.AddMemberProposal(p)
}

type AddMemberProposalState struct {
	SimpleVoteOptions[bool] `serialize:"true"`

	ApplicantAddress   ids.ShortID   `serialize:"true"`
	Start              uint64        `serialize:"true"`
	End                uint64        `serialize:"true"`
	AllowedVoters      []ids.ShortID `serialize:"true"`
	TotalAllowedVoters uint32        `serialize:"true"`
}

func (p *AddMemberProposalState) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *AddMemberProposalState) EndTime() time.Time {
	return time.Unix(int64(p.End), 0)
}

func (p *AddMemberProposalState) IsActiveAt(time time.Time) bool {
	timestamp := uint64(time.Unix())
	return p.Start <= timestamp && timestamp <= p.End
}

func (p *AddMemberProposalState) CanBeFinished() bool {
	mostVotedWeight, _, unambiguous := p.GetMostVoted()
	voted := p.Voted()
	// We don't check for 'no option can reach 50%+ of votes' for this proposal type, cause its impossible with just 2 options
	return voted == p.TotalAllowedVoters ||
		unambiguous && mostVotedWeight > p.TotalAllowedVoters/2
}

func (p *AddMemberProposalState) IsSuccessful() bool {
	mostVotedWeight, _, unambiguous := p.GetMostVoted()
	voted := p.Voted()
	return unambiguous && voted > p.TotalAllowedVoters/2 && mostVotedWeight > voted/2
}

func (p *AddMemberProposalState) Outcome() any {
	_, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	if !unambiguous {
		return -1
	}
	return mostVotedOptionIndex
}

func (p *AddMemberProposalState) Result() (bool, uint32, bool) {
	mostVotedWeight, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	return p.Options[mostVotedOptionIndex].Value, mostVotedWeight, unambiguous
}

// Will return modified proposal with added vote, original proposal will not be modified!
func (p *AddMemberProposalState) AddVote(voterAddress ids.ShortID, voteIntf Vote, isCairoPhase bool) (ProposalState, error) {
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
func (p *AddMemberProposalState) ForceAddVote(voteIntf Vote, isCairoPhase bool) (ProposalState, error) {
	return p.addVote(voteIntf, isCairoPhase)
}

func (p *AddMemberProposalState) addVote(voteIntf Vote, _ bool) (*AddMemberProposalState, error) {
	simpleVoteOptions, err := p.AddWeight(voteIntf)
	if err != nil {
		return nil, err
	}

	return &AddMemberProposalState{
		ApplicantAddress:   p.ApplicantAddress,
		Start:              p.Start,
		End:                p.End,
		AllowedVoters:      p.AllowedVoters,
		SimpleVoteOptions:  *simpleVoteOptions,
		TotalAllowedVoters: p.TotalAllowedVoters,
	}, nil
}

func (p *AddMemberProposalState) ExecuteWith(executor Executor) error {
	return executor.AddMemberProposal(p)
}

func (p *AddMemberProposalState) GetBondTxIDsWith(bondTxIDsGetter BondTxIDsGetter) ([]ids.ID, error) {
	return bondTxIDsGetter.AddMemberProposal(p)
}
