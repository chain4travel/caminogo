// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
)

// GetNextChainEventTime returns the next chain event time
// (stakers set changed / deposit expired / validator rewards distibution)
func GetNextChainEventTime(state state.Chain, stakerChangeTime time.Time) (time.Time, error) {
	cfg, err := state.Config()
	if err != nil {
		return time.Time{}, fmt.Errorf("couldn't get config: %w", err)
	}

	if cfg.CaminoConfig.ValidatorsRewardPeriod == 0 {
		return stakerChangeTime, nil
	}

	caminoConfig, err := state.CaminoConfig()
	if err != nil {
		return time.Time{}, fmt.Errorf("couldn't get camino config: %w", err)
	}

	validatorsRewardTime := getNextValidatorsRewardTime(
		uint64(state.GetTimestamp().Unix()),
		cfg.CaminoConfig.ValidatorsRewardPeriod,
		caminoConfig.ValidatorRewardsStartTime,
	)

	if stakerChangeTime.Before(validatorsRewardTime) {
		return stakerChangeTime, nil
	}

	return validatorsRewardTime, nil
}

func getNextValidatorsRewardTime(chainTime, rewardPeriod, rewardStartTime uint64) time.Time {
	return time.Unix(int64(math.Max(chainTime-chainTime%rewardPeriod+rewardPeriod, rewardStartTime)), 0)
}
