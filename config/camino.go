// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"flag"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/config"
	"github.com/spf13/viper"
)

const (
	DaoProposalBondAmountKey     = "dao-proposal-bond-amount"
	GradualUnlockHalfDurationKey = "gradual-unlock-half-duration"
)

func addCaminoFlags(fs *flag.FlagSet) {
	// Bond amount required to place a DAO proposal on the Primary Network
	fs.Uint64(DaoProposalBondAmountKey, genesis.LocalParams.CaminoConfig.DaoProposalBondAmount, "Amount, in nAVAX, required to place a DAO proposal")
	// TODO@ comment and flag text
	fs.Duration(DaoProposalBondAmountKey, genesis.LocalParams.CaminoConfig.GradualUnlockHalfDuration, "")
}

func getCaminoPlatformConfig(v *viper.Viper) config.CaminoConfig {
	conf := config.CaminoConfig{
		DaoProposalBondAmount:     v.GetUint64(DaoProposalBondAmountKey),
		GradualUnlockHalfDuration: v.GetDuration(GradualUnlockHalfDurationKey),
	}
	return conf
}
