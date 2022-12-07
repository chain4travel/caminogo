// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
)

func TestValidateCaminoConfig(t *testing.T) {
	tests := map[string]struct {
		config Config
		err    error
	}{
		"deposit offer's end time before start time": {
			config: func() Config {
				tempOffers := make([]genesis.DepositOffer, len(CaminoConfig.Camino.DepositOffers))
				copy(tempOffers, CaminoConfig.Camino.DepositOffers)
				thisConfig := CaminoConfig
				thisConfig.Camino.DepositOffers = tempOffers

				offerStartTime := time.Now()
				offerEndTime := time.Now().Add(-1 * time.Hour)
				thisConfig.Camino.DepositOffers[0].Start = uint64(offerStartTime.Unix())
				thisConfig.Camino.DepositOffers[0].End = uint64(offerEndTime.Unix())
				return thisConfig
			}(),
			err: genesis.ErrOfferStartLessThanEnd,
		},
		"deposit offer's min duration greater than max duration": {
			config: func() Config {
				tempOffers := make([]genesis.DepositOffer, len(CaminoConfig.Camino.DepositOffers))
				copy(tempOffers, CaminoConfig.Camino.DepositOffers)
				thisConfig := CaminoConfig
				thisConfig.Camino.DepositOffers = tempOffers

				minDuration := 91
				thisConfig.Camino.DepositOffers[0].MinDuration = uint32(minDuration)
				return thisConfig
			}(),
			err: genesis.ErrOfferMinDurationGreaterThanMax,
		},
		"deposit offer's min duration less than half duration": {
			config: func() Config {
				tempOffers := make([]genesis.DepositOffer, len(CaminoConfig.Camino.DepositOffers))
				copy(tempOffers, CaminoConfig.Camino.DepositOffers)
				thisConfig := CaminoConfig
				thisConfig.Camino.DepositOffers = tempOffers

				minDuration := 29
				thisConfig.Camino.DepositOffers[0].MinDuration = uint32(minDuration)
				return thisConfig
			}(),
			err: genesis.ErrOfferMinDurationLessThanNoReward,
		},
		"deposit offer's min duration less than unlock period duration": {
			config: func() Config {
				tempOffers := make([]genesis.DepositOffer, len(CaminoConfig.Camino.DepositOffers))
				copy(tempOffers, CaminoConfig.Camino.DepositOffers)
				thisConfig := CaminoConfig
				thisConfig.Camino.DepositOffers = tempOffers

				minDuration := 59
				thisConfig.Camino.DepositOffers[0].MinDuration = uint32(minDuration)
				return thisConfig
			}(),
			err: genesis.ErrOfferMinDurationLessThanUnlock,
		},
		"deposit offer's min duration is zero": {
			config: func() Config {
				tempOffers := make([]genesis.DepositOffer, len(CaminoConfig.Camino.DepositOffers))
				copy(tempOffers, CaminoConfig.Camino.DepositOffers)
				thisConfig := CaminoConfig
				thisConfig.Camino.DepositOffers = tempOffers

				minDuration := 0
				thisConfig.Camino.DepositOffers[0].MinDuration = uint32(minDuration)
				return thisConfig
			}(),
			err: genesis.ErrOfferZerosMinDuration,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateCaminoConfig(&tt.config)
			require.ErrorIs(t, err, tt.err)
		})
	}
}
