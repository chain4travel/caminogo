// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	blocks "github.com/ava-labs/avalanchego/vms/platformvm/blocks"
)

type Deposit struct {
	DepositOfferID      ids.ID `serialize:"true"`
	UnlockedAmount      uint64 `serialize:"true"`
	ClaimedRewardAmount uint64 `serialize:"true"`
	Start               uint64 `serialize:"true"`
}

func (d *Deposit) StartTime() time.Time {
	return time.Unix(int64(d.Start), 0)
}

func (d *Deposit) IsExpired(
	depositOffer *DepositOffer,
	timestamp time.Time,
	gradualUnlockHalfDuration time.Duration,
) bool {
	endtime := d.StartTime().Add(time.Duration(depositOffer.Duration))
	if depositOffer.GradualUnlock {
		endtime.Add(gradualUnlockHalfDuration)
	}
	return !endtime.After(timestamp)
}

// TODO@ move out of type?
func (d *Deposit) UnlockableAmount(depositOffer *DepositOffer, depositAmount, unlockTime uint64) uint64 {
	// TODO@ normal safe math
	unlockPercent := unlockTime / (d.Start + depositOffer.Duration)
	totalUnlockableAmount := depositAmount * unlockPercent
	return totalUnlockableAmount - d.UnlockedAmount
}

func (cs *caminoState) UpdateDeposit(depositTxID ids.ID, deposit *Deposit) {
	cs.modifiedDeposits[depositTxID] = deposit
}

func (cs *caminoState) GetDeposit(depositTxID ids.ID) (*Deposit, error) {
	// Try to get from modified state
	deposit, ok := cs.modifiedDeposits[depositTxID]
	// Try to get it from cache
	if !ok {
		var depositIntf interface{}
		if depositIntf, ok = cs.depositsCache.Get(depositTxID); ok {
			deposit = depositIntf.(*Deposit)
		}
	}
	// Try to get it from database
	if !ok {
		depositBytes, err := cs.depositsDB.Get(depositTxID[:])
		if err != nil {
			return nil, err
		}

		deposit = &Deposit{}
		if _, err := blocks.GenesisCodec.Unmarshal(depositBytes, deposit); err != nil {
			return nil, err
		}

		cs.depositsCache.Put(depositTxID, deposit)
	}

	return deposit, nil
}

func (cs *caminoState) writeDeposits() error {
	for depositTxID, deposit := range cs.modifiedDeposits {
		delete(cs.modifiedDeposits, depositTxID)

		depositBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, deposit)
		if err != nil {
			return fmt.Errorf("failed to serialize deposit: %w", err)
		}

		if err := cs.depositsDB.Put(depositTxID[:], depositBytes); err != nil {
			return err
		}
	}
	return nil
}
