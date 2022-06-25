// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package lock

import (
	"time"
)

type Config struct {
	// MinLockDuration is the minimum amount of lock duration
	MinLockDuration time.Duration `json:"minLockDuration"`
	// MaxLockDuration is the maximum amount of lock duration
	MaxLockDuration time.Duration `json:"maxLockDuration"`
	// MinLockAmount, in nAVAX, is the minimum amount of tokens that can be locked
	MinLockAmount uint64 `json:"minLockAmount"`
}
