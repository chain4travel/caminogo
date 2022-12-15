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
	DaoProposalBondAmountKey          = "dao-proposal-bond-amount"
	DaoProposalMaxDurrationKey        = "dao-proposal-max-durration"
	DaoProposalMinDurrationKey        = "dao-proposal-min-durration"
	DaoProposalMinPendingDurrationKey = "dao-proposal-min-pending-durration"
)

func addCaminoFlags(fs *flag.FlagSet) {
	// Bond amount required to place a DAO proposal on the Primary Network
	fs.Uint64(DaoProposalBondAmountKey, genesis.LocalParams.CaminoConfig.DaoProposalBondAmount, "Amount, in nAVAX, required to place a DAO proposal")
	fs.Duration(DaoProposalMaxDurrationKey, genesis.LocalParams.CaminoConfig.DaoProposalMaxDurration, "maximum time a proposal can be active")
	fs.Duration(DaoProposalMinDurrationKey, genesis.LocalParams.CaminoConfig.DaoProposalMinDurration, "min time a proposal must be active")
	fs.Duration(DaoProposalMinPendingDurrationKey, genesis.LocalParams.CaminoConfig.DaoProposalMinPendingDuration, "minimum time a proposal has to be pending before coming active")
}

func getCaminoPlatformConfig(v *viper.Viper) config.CaminoConfig {
	conf := config.CaminoConfig{
		DaoProposalBondAmount:         v.GetUint64(DaoProposalBondAmountKey),
		DaoProposalMaxDurration:       v.GetDuration(DaoProposalMaxDurrationKey),
		DaoProposalMinDurration:       v.GetDuration(DaoProposalMinDurrationKey),
		DaoProposalMinPendingDuration: v.GetDuration(DaoProposalMinPendingDurrationKey),
	}
	return conf
}
