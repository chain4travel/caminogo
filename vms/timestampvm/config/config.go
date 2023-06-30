// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
)

type Config struct {
	// Set of subnets that this node is validating
	TrackedSubnets set.Set[ids.ID]
}
