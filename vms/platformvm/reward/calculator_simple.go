// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package reward

import (
	"math/big"
	"time"
)

var _ CalculatorSimple = &calculator{}

type CalculatorSimple interface {
	CalculateReward(lockedDuration time.Duration, lockedAmount uint64) uint64
}

type calculator struct {
	rewardRateNominator      *big.Int
	mintingPeriodDenominator *big.Int
}

func NewCalculatorForLockReward(c LockRewardConfig) CalculatorSimple {
	return &calculator{
		// c.MaxLockDuration * percentDenominator
		mintingPeriodDenominator: new(big.Int).Mul(new(big.Int).SetUint64(uint64(c.MaxLockDuration)), bigPercentDenominator),
		rewardRateNominator:      new(big.Int).SetUint64(c.RewardRate),
	}
}

// CalculateReward returns the amount of tokens to reward the owner of locked tokens with.
//
// rewardRateNominator = rewardFraction * percentDenominator = c.RewardRate
// mintingPeriodDenominator = maxLockDuration * percentDenominator
// Reward = lockedAmount * lockedDuration * rewardRateNominator / mintingPeriodDenominator
func (c *calculator) CalculateReward(lockedDuration time.Duration, lockedAmount uint64) uint64 {
	reward := new(big.Int).SetUint64(lockedAmount)
	reward.Mul(reward, new(big.Int).SetUint64(uint64(lockedDuration)))
	reward.Mul(reward, c.rewardRateNominator)
	reward.Div(reward, c.mintingPeriodDenominator)
	return reward.Uint64()
}
