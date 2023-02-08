// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

type CaminoConfig struct {
	DaoProposalBondAmount  uint64
	ValidatorsRewardPeriod uint64
}

type CaminoGenesisConfig struct {
	VerifyNodeSignature       bool   `serialize:"true"`
	LockModeBondDeposit       bool   `serialize:"true"`
	ValidatorRewardsStartTime uint64 `serialize:"true"`
}
