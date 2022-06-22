// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************

// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"fmt"
	"time"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/timer/mockable"
)

type LocksState interface {
	CurrentLocksChainState() currentLocksChainState
	// PendingStakerChainState() pendingStakerChainState
}

// getNextLockChangeTime returns the next time that a locked tokens should unlock
func getNextLockChangeTime(locksState LocksState) (time.Time, error) {
	earliest := mockable.MaxTime
	currentLocksState := locksState.CurrentLocksChainState()
	if currentLocks := currentLocksState.Locks(); len(currentLocks) > 0 {
		nextLock := currentLocks[0]
		lockTx, ok := nextLock.UnsignedTx.(TimedTx)
		if !ok {
			return time.Time{}, errWrongTxType
		}
		endTime := lockTx.EndTime()
		if endTime.Before(earliest) {
			earliest = endTime
		}
	}
	return earliest, nil
}

// getLockToReward return the staker txID to remove from the primary network
// staking set, if one exists.
func getLockToReward(preferredState MutableState) (ids.ID, bool, error) {
	currentChainTimestamp := preferredState.GetTimestamp()
	if !currentChainTimestamp.Before(mockable.MaxTime) {
		return ids.Empty, false, errEndOfTime
	}

	currentLocksState := preferredState.CurrentLocksChainState()
	tx, _, err := currentLocksState.GetNextLock()
	if err != nil {
		return ids.Empty, false, err
	}

	lockTx, ok := tx.UnsignedTx.(TimedTx)
	if !ok {
		return ids.Empty, false, fmt.Errorf("expected lock tx to be TimedTx but got %T", tx.UnsignedTx)
	}
	return tx.ID(), currentChainTimestamp.Equal(lockTx.EndTime()), nil
}
