// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
)

func (b *builder) getNextDepositToUnlock(
	chainTimestamp time.Time,
	preferredState state.Chain,
) (ids.ID, bool, error) {
	if !chainTimestamp.Before(mockable.MaxTime) {
		return ids.Empty, false, errEndOfTime
	}

	// TODO@ add some sorted thing to state, like with stakers, so we can get next deposit

	return ids.Empty, false, nil
}
