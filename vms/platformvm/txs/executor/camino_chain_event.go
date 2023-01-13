// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
)

// GetNextChainEventTime returns the next chain event time
// (stakers set changed / deposit expired / validator rewards distibution)
func GetNextChainEventTime(state state.Chain, stakerChangeTime time.Time) (time.Time, error) {
	cfg, err := state.Config()
	if err != nil {
		return time.Time{}, fmt.Errorf("couldn't get config: %w", err)
	}

	nextDeferredStakerEndTime, err := GetNextDeferredStakerEndTime(state)
	if err == nil && nextDeferredStakerEndTime.Before(stakerChangeTime) {
		stakerChangeTime = nextDeferredStakerEndTime
	}

	if cfg.CaminoConfig.ValidatorsRewardPeriod == 0 {
		return stakerChangeTime, nil
	}

	validatorsRewardTime := getNextValidatorsRewardTime(
		uint64(state.GetTimestamp().Unix()),
		cfg.CaminoConfig.ValidatorsRewardPeriod,
	)

	if stakerChangeTime.Before(validatorsRewardTime) {
		return stakerChangeTime, nil
	}

	return validatorsRewardTime, nil
}

func getNextValidatorsRewardTime(chainTime uint64, validatorsRewardPeriod uint64) time.Time {
	return time.Unix(int64(chainTime-chainTime%validatorsRewardPeriod+validatorsRewardPeriod), 0)
}

func GetNextDeferredStakerEndTime(state state.Chain) (time.Time, error) {
	deferredStakerIterator, err := state.GetDeferredStakerIterator()
	if err != nil {
		return time.Time{}, err
	}
	defer deferredStakerIterator.Release()
	if deferredStakerIterator.Next() {
		return deferredStakerIterator.Value().NextTime, nil
	}
	return time.Time{}, database.ErrNotFound
}
