// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

var (
	ErrOfferStartLessThanEnd            = errors.New("deposit offer's end time is before start time")
	ErrOfferMinDurationGreaterThanMax   = errors.New("deposit offer's minimum duration is greater than maximum duration")
	ErrOfferMinDurationLessThanUnlock   = errors.New("deposit offer's minimum duration is less than unlock period duration")
	ErrOfferMinDurationLessThanNoReward = errors.New("deposit offer minimum duration is less than no-rewards period duration")
	ErrOfferZerosMinDuration            = errors.New("deposit offer has zero minimum duration")
)

// Camino genesis args
type Camino struct {
	VerifyNodeSignature bool           `serialize:"true" json:"verifyNodeSignature"`
	LockModeBondDeposit bool           `serialize:"true" json:"lockModeBondDeposit"`
	InitialAdmin        ids.ShortID    `serialize:"true" json:"initialAdmin"`
	DepositOffers       []DepositOffer `serialize:"true" json:"depositOffers"`
	Deposits            []*txs.Tx      `serialize:"true" json:"deposits"`
}

type DepositOffer struct {
	InterestRateNominator   uint64 `serialize:"true" json:"interestRateNominator"`
	Start                   uint64 `serialize:"true" json:"start"`
	End                     uint64 `serialize:"true" json:"end"`
	MinAmount               uint64 `serialize:"true" json:"minAmount"`
	MinDuration             uint32 `serialize:"true" json:"minDuration"`
	MaxDuration             uint32 `serialize:"true" json:"maxDuration"`
	UnlockPeriodDuration    uint32 `serialize:"true" json:"unlockPeriodDuration"`
	NoRewardsPeriodDuration uint32 `serialize:"true" json:"noRewardsPeriodDuration"`
	Flags                   uint64 `serialize:"true" json:"flags"`
}

// Gets offer id from its bytes hash
func (offer DepositOffer) ID() (ids.ID, error) {
	bytes, err := blocks.GenesisCodec.Marshal(blocks.Version, offer)
	if err != nil {
		return ids.Empty, err
	}
	return hashing.ComputeHash256Array(bytes), nil
}

func (offer DepositOffer) Verify() error {
	if offer.Start >= offer.End {
		return fmt.Errorf(
			"%w, starttime (%v) endtime (%v)",
			ErrOfferStartLessThanEnd,
			offer.Start,
			offer.End,
		)
	}

	if offer.MinDuration > offer.MaxDuration {
		return ErrOfferMinDurationGreaterThanMax
	}

	if offer.MinDuration == 0 {
		return ErrOfferZerosMinDuration
	}

	if offer.MinDuration < offer.NoRewardsPeriodDuration {
		return fmt.Errorf(
			"%w, minimum duration (%v) no-rewards period duration (%v)",
			ErrOfferMinDurationLessThanNoReward,
			offer.MinDuration,
			offer.NoRewardsPeriodDuration,
		)
	}

	if offer.MinDuration < offer.UnlockPeriodDuration {
		return fmt.Errorf(
			"%w, minimum duration (%v) unlock period duration (%v)",
			ErrOfferMinDurationLessThanUnlock,
			offer.MinDuration,
			offer.UnlockPeriodDuration,
		)
	}

	return nil
}
