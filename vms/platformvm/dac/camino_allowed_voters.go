// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"bytes"

	"github.com/ava-labs/avalanchego/ids"
	"golang.org/x/exp/slices"
)

func excludeFromAllowedVoters(allowedVoters []ids.ShortID, voterAddress ids.ShortID) ([]ids.ShortID, error) {
	voterAddrPos, allowedToVote := slices.BinarySearchFunc(allowedVoters, voterAddress, func(id, other ids.ShortID) int {
		return bytes.Compare(id[:], other[:])
	})
	if !allowedToVote {
		return nil, ErrNotAllowedToVoteOnProposal
	}

	// we can't use the same slice, cause we need to change its elements
	updatedAllowedVoters := make([]ids.ShortID, len(allowedVoters)-1)
	copy(updatedAllowedVoters, allowedVoters[:voterAddrPos])
	updatedAllowedVoters = append(updatedAllowedVoters[:voterAddrPos], allowedVoters[voterAddrPos+1:]...)

	return updatedAllowedVoters, nil
}
