// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dao

import "time"

type Config struct {
	// Fee that must be burned by every AddDaoProposal transaction
	ProposalTxFee uint64 `json:"proposalTxFee"`
	// Minimal lock amount to ba able to add a proposal
	MinProposalLock uint64 `json:"minProposalLock"`
	// Minimum duration a voting period has to be active
	MinProposalDuration time.Duration `json:"minProposalDuration"`
	// Maximum duration a voting period can be active
	MaxProposalDuration time.Duration `json:"maxProposalDuration"`
}
