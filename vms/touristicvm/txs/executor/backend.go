// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/touristicvm/config"
	"github.com/ava-labs/avalanchego/vms/touristicvm/fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/utxo"
)

type Backend struct {
	Config       *config.Config
	Ctx          *snow.Context
	Clk          *mockable.Clock
	Bootstrapped *utils.Atomic[bool]
	Fx           fx.Fx
	FlowChecker  utxo.Handler
}