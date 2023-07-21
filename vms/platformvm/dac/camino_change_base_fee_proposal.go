package dac

import (
	"bytes"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"golang.org/x/exp/slices"
)

var (
	_ Proposal      = (*BaseFeeProposal)(nil)
	_ ProposalState = (*BaseFeeProposalState)(nil)

	errZeroFee           = errors.New("base-fee option is zero")
	errWrongOptionsCount = errors.New("wrong options count")
)

type BaseFeeProposal struct {
	Options []uint64 `serialize:"true"`
	Start   uint64   `serialize:"true"`
	End     uint64   `serialize:"true"`
}

func (p *BaseFeeProposal) StartTime() time.Time {
	return time.Unix(int64(p.Start), 0)
}

func (p *BaseFeeProposal) Verify() error {
	switch {
	case len(p.Options) > 3:
		return errWrongOptionsCount
	case p.Start >= p.End:
		return errEndNotAfterStart
	}
	for _, fee := range p.Options {
		if fee == 0 {
			return errZeroFee
		}
	}
	return nil
}

// Will return modified proposal allowed voters set, original proposal will not be modified!
func (p *BaseFeeProposal) CreateProposalState(allowedVoters []ids.ShortID) ProposalState {
	stateProposal := &BaseFeeProposalState{
		SimpleVoteOptions: SimpleVoteOptions[uint64]{
			Options: make([]SimpleVoteOption[uint64], len(p.Options)),
		},
		Start:            p.Start,
		End:              p.End,
		AllowedVoters:    allowedVoters,
		SuccessThreshold: uint32(len(allowedVoters) / 2),
		FinishThreshold:  uint32(len(allowedVoters)),
	}
	for i := range p.Options {
		stateProposal.Options[i].Value = p.Options[i]
	}
	return stateProposal
}

func (p *BaseFeeProposal) Visit(visitor VerifierVisitor) error {
	return visitor.BaseFeeProposal(p)
}

type BaseFeeProposalState struct {
	SimpleVoteOptions[uint64] `serialize:"true"`

	Start            uint64        `serialize:"true"`
	End              uint64        `serialize:"true"`
	AllowedVoters    []ids.ShortID `serialize:"true"`
	SuccessThreshold uint32        `serialize:"true"`
	FinishThreshold  uint32        `serialize:"true"`
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
	voted := uint32(0)
	for i := range p.Options {
		voted += p.Options[i].Weight
	}
	return voted == p.FinishThreshold || (unambiguous && mostVotedWeight > p.SuccessThreshold)
}

// Votes must be valid for this proposal, could panic otherwise.
func (p *BaseFeeProposalState) Result() (uint64, uint32, bool) {
	mostVotedWeight, mostVotedOptionIndex, unambiguous := p.GetMostVoted()
	return p.Options[mostVotedOptionIndex].Value, mostVotedWeight, unambiguous
}

// Will return modified proposal with added vote, original proposal will not be modified!
func (p *BaseFeeProposalState) AddVote(voterAddress ids.ShortID, voteIntf Vote) (ProposalState, error) {
	vote, ok := voteIntf.(*SimpleVote)
	if !ok {
		return nil, ErrWrongVote
	}
	if int(vote.OptionIndex) >= len(p.Options) {
		return nil, ErrWrongVote
	}

	voterAddrPos, allowedToVote := slices.BinarySearchFunc(p.AllowedVoters, voterAddress, func(id, other ids.ShortID) int {
		return bytes.Compare(id[:], other[:])
	})
	if !allowedToVote {
		return nil, ErrNotAllowedToVoteOnProposal
	}

	updatedProposal := &BaseFeeProposalState{
		Start:         p.Start,
		End:           p.End,
		AllowedVoters: make([]ids.ShortID, len(p.AllowedVoters)-1),
		SimpleVoteOptions: SimpleVoteOptions[uint64]{
			Options: make([]SimpleVoteOption[uint64], len(p.Options)),
		},
		SuccessThreshold: p.SuccessThreshold,
	}
	// we can't use the same slice, cause we need to change its elements
	copy(updatedProposal.AllowedVoters, p.AllowedVoters[:voterAddrPos])
	updatedProposal.AllowedVoters = append(updatedProposal.AllowedVoters[:voterAddrPos], p.AllowedVoters[voterAddrPos+1:]...)
	// we can't use the same slice, cause we need to change its element
	copy(updatedProposal.Options, p.Options)
	updatedProposal.Options[vote.OptionIndex].Weight++
	return updatedProposal, nil
}

func (p *BaseFeeProposalState) Visit(visitor ExecutorVisitor) error {
	return visitor.BaseFeeProposal(p)
}
