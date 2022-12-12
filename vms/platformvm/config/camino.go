// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import "time"

type CaminoConfig struct {
	DaoProposalBondAmount         uint64
	DaoProposalMaxDurration       time.Duration
	DaoProposalMinDurration       time.Duration
	DaoProposalMinPendingDuration time.Duration
}
