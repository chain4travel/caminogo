// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

var (
	_ Vote = (*SimpleVote)(nil)

	errNoOptions       = errors.New("no options")
	errNotUniqueOption = errors.New("not unique option")
)

type Vote interface {
	verify.Verifiable

	VotedOptions() any
}
type VoteWithAddr struct {
	Vote         `serialize:"true"`
	VoterAddress ids.ShortID `serialize:"true"`
}

type SimpleVote struct {
	OptionIndex uint32 `serialize:"true"`
}

func (v *SimpleVote) VotedOptions() any {
	return []uint32{v.OptionIndex}
}

func (*SimpleVote) Verify() error {
	return nil
}

type SimpleVoteOption[T any] struct {
	Value  T      `serialize:"true"`
	Weight uint32 `serialize:"true"`
}

type SimpleVoteOptions[T comparable] struct {
	Options              []SimpleVoteOption[T] `serialize:"true"`
	mostVotedWeight      uint32
	mostVotedOptionIndex uint32
	unambiguous          bool
}

func (p *SimpleVoteOptions[T]) Verify() error {
	if len(p.Options) == 0 {
		return errNoOptions
	}
	unique := set.NewSet[T](len(p.Options))
	for _, option := range p.Options {
		if unique.Contains(option.Value) {
			return errNotUniqueOption
		}
		unique.Add(option.Value)
	}
	return nil
}

func (p SimpleVoteOptions[T]) GetMostVoted() (
	mostVotedWeight uint32,
	mostVotedIndex uint32,
	unambiguous bool,
) {
	if p.mostVotedWeight != 0 {
		return p.mostVotedWeight, p.mostVotedOptionIndex, p.unambiguous
	}

	unambiguous = true
	mostVotedIndexInt := 0
	weights := make([]int, len(p.Options))
	for optionIndex := range p.Options {
		weights[optionIndex] += int(p.Options[optionIndex].Weight)
		if optionIndex != mostVotedIndexInt && weights[optionIndex] == weights[mostVotedIndexInt] {
			unambiguous = false
		} else if weights[optionIndex] > weights[mostVotedIndexInt] {
			mostVotedIndexInt = optionIndex
			unambiguous = true
		}
	}

	p.mostVotedWeight = uint32(weights[mostVotedIndexInt])
	p.mostVotedOptionIndex = uint32(mostVotedIndexInt)
	p.unambiguous = unambiguous && p.mostVotedWeight > 0

	return p.mostVotedWeight, p.mostVotedOptionIndex, p.unambiguous
}

var _ Vote = (*DummyVote)(nil)

type DummyVote struct {
	ErrorStr string `serialize:"true"`
}

func (v *DummyVote) Verify() error {
	if v.ErrorStr != "" {
		return errors.New(v.ErrorStr)
	}
	return nil
}
func (*DummyVote) VotedOptions() any { return nil }
