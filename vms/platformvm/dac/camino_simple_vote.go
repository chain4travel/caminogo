// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

var _ Vote = (*SimpleVote)(nil)

type SimpleVote struct {
	OptionIndex uint32 `serialize:"true"` // Index of voted option
}

func (v *SimpleVote) VotedOptions() any {
	return []uint32{v.OptionIndex}
}

func (*SimpleVote) Verify() error {
	return nil
}

type SimpleVoteOption[T any] struct {
	Value  T      `serialize:"true"` // Value that this option represents
	Weight uint32 `serialize:"true"` // How much this option was voted
}

type SimpleVoteOptions[T any] struct {
	Options              []SimpleVoteOption[T] `serialize:"true"`
	mostVotedWeight      uint32                // Weight of most voted option
	mostVotedOptionIndex uint32                // Index of most voted option
	unambiguous          bool                  // True, if there is an option with weight > then other options weight
}

func (vo SimpleVoteOptions[T]) GetMostVoted() (
	mostVotedWeight uint32,
	mostVotedIndex uint32,
	unambiguous bool,
) {
	if vo.mostVotedWeight != 0 {
		return vo.mostVotedWeight, vo.mostVotedOptionIndex, vo.unambiguous
	}

	unambiguous = true
	mostVotedIndexInt := 0
	weights := make([]int, len(vo.Options))
	for optionIndex := range vo.Options {
		weights[optionIndex] += int(vo.Options[optionIndex].Weight)
		if optionIndex != mostVotedIndexInt && weights[optionIndex] == weights[mostVotedIndexInt] {
			unambiguous = false
		} else if weights[optionIndex] > weights[mostVotedIndexInt] {
			mostVotedIndexInt = optionIndex
			unambiguous = true
		}
	}

	vo.mostVotedWeight = uint32(weights[mostVotedIndexInt])
	vo.mostVotedOptionIndex = uint32(mostVotedIndexInt)
	vo.unambiguous = unambiguous && vo.mostVotedWeight > 0

	return vo.mostVotedWeight, vo.mostVotedOptionIndex, vo.unambiguous
}

func (vo SimpleVoteOptions[T]) Voted() uint32 {
	voted := uint32(0)
	for i := range vo.Options {
		voted += vo.Options[i].Weight
	}
	return voted
}

// Will not modify original simple vote options
func (vo SimpleVoteOptions[T]) AddWeight(voteIntf Vote) (*SimpleVoteOptions[T], error) {
	vote, ok := voteIntf.(*SimpleVote)
	if !ok {
		return nil, ErrWrongVote
	}
	if int(vote.OptionIndex) >= len(vo.Options) {
		return nil, ErrWrongVote
	}

	updatedSimpleVoteOptions := &SimpleVoteOptions[T]{
		Options: make([]SimpleVoteOption[T], len(vo.Options)),
	}

	// we can't use the same slice, because we need to change its element
	copy(updatedSimpleVoteOptions.Options, vo.Options)
	updatedSimpleVoteOptions.Options[vote.OptionIndex].Weight++

	return updatedSimpleVoteOptions, nil
}
