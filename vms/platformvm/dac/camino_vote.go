package dac

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

const MaxVoteSize = 2048 // TODO@

var (
	_ Vote = (*SimpleVote)(nil)

	errNoOptions       = errors.New("no options")
	errNotUniqueOption = errors.New("not unique option")
)

type Vote interface {
	verify.Verifiable
}
type VoteWithAddr struct {
	Vote         `serialize:"true"`
	VoterAddress ids.ShortID `serialize:"true"`
}

type VoteWrapper struct {
	Vote `serialize:"true"`
}

type SimpleVote struct {
	OptionIndex uint32 `serialize:"true"`
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
	p.unambiguous = unambiguous

	return p.mostVotedWeight, p.mostVotedOptionIndex, p.unambiguous
}
