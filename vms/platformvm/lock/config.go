// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package lock

import (
	"math/big"
	"time"
)

// PercentDenominator is the denominator used to calculate percentages
const PercentDenominator = 1_000_000

// consumptionRateDenominator is the magnitude offset used to emulate
// floating point fractions.
var bigPercentDenominator = new(big.Int).SetUint64(PercentDenominator)

type Config struct {
	// MinLockDuration is the minimum amount of lock duration
	MinLockDuration time.Duration `json:"minLockDuration"`
	// MaxLockDuration is the maximum amount of lock duration
	MaxLockDuration time.Duration `json:"maxLockDuration"`
	// MinLockAmount, in nAVAX, is the minimum amount of tokens that can be locked
	MinLockAmount uint64 `json:"minLockAmount"`
	// RewardRate is the fraction of the locked token tokens that is allocated to the owner after unlock multiplied by some PercentDenominator // TODO@evlekht fix descr
	RewardRate uint64 `json:"rewardFraction"`
}
