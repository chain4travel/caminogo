// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dao

import "time"

type Config struct {
	// Fee that must be burned by every AddDaoProposal transaction
	ProposalTxFee uint64 `json:"proposalTxFee"`
	// Fee that must be burned by every VoteProposal transaction
	VoteTxFee uint64 `json:"voteTxFee"`
	// How much Bond needs to be put up, to facilitate a Vote on a proposal
	ProposalBondAmount uint64 `json:"proposalBondAmount"`
	// Minimum duration a voting period has to be active
	MinProposalDuration time.Duration `json:"minProposalDuration"`
	// Maximum duration a voting period can be active
	MaxProposalDuration time.Duration `json:"maxProposalDuration"`
}
