// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"bytes"
	"sort"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/database/versiondb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
)

var (
	genesisDepositOffer = deposit.Offer{
		ID:                    ids.ID{0},
		InterestRateNominator: 0,
		Start:                 uint64(time.Now().Add(-60 * time.Hour).Unix()),
		End:                   uint64(time.Now().Add(+60 * time.Hour).Unix()),
		MinAmount:             1,
		MinDuration:           60,
		MaxDuration:           60,
		UnlockPeriodDuration:  60,
	}

	newDepositOffer = deposit.Offer{
		ID:                    ids.ID{1},
		InterestRateNominator: 0,
		Start:                 uint64(time.Now().Add(-120 * time.Hour).Unix()),
		End:                   uint64(time.Now().Add(+120 * time.Hour).Unix()),
		MinAmount:             2,
		MinDuration:           120,
		MaxDuration:           120,
		UnlockPeriodDuration:  120,
	}
)

func TestGetDepositOffer(t *testing.T) {
	baseDBManager := manager.NewMemDB(version.CurrentDatabase)
	cs, err := newCaminoState(versiondb.New(baseDBManager.Current().Database), prometheus.NewRegistry())
	require.NoError(t, err)
	cs.AddDepositOffer(&genesisDepositOffer)

	tests := map[string]struct {
		expectedErr error
		offerID     ids.ID
	}{
		"Deposit offer does not exist": {
			offerID:     ids.GenerateTestID(),
			expectedErr: database.ErrNotFound,
		},
		"Happy path": {
			offerID:     ids.ID{0},
			expectedErr: nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := cs.GetDepositOffer(tt.offerID)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestGetAllDepositOffers(t *testing.T) {
	baseDBManager := manager.NewMemDB(version.CurrentDatabase)
	cs, err := newCaminoState(versiondb.New(baseDBManager.Current().Database), prometheus.NewRegistry())
	require.NoError(t, err)

	cs.AddDepositOffer(&newDepositOffer)
	cs.AddDepositOffer(&genesisDepositOffer)
	depositOffers, err := cs.GetAllDepositOffers()
	require.NoError(t, err)

	sort.Slice(depositOffers, func(i, j int) bool {
		return bytes.Compare(
			depositOffers[i].ID[:],
			depositOffers[j].ID[:]) == -1
	})

	require.Equal(t, depositOffers, []*deposit.Offer{&genesisDepositOffer, &newDepositOffer})
}

func TestLoadDepositOffers(t *testing.T) {
	baseDBManager := manager.NewMemDB(version.CurrentDatabase)
	cs, err := newCaminoState(versiondb.New(baseDBManager.Current().Database), prometheus.NewRegistry())
	require.NoError(t, err)

	cs.AddDepositOffer(&newDepositOffer)
	cs.AddDepositOffer(&genesisDepositOffer)
	err = cs.writeDepositOffers()
	require.NoError(t, err)
	err = cs.loadDepositOffers()
	require.NoError(t, err)

	expectedDepositOffers := map[ids.ID]*deposit.Offer{
		genesisDepositOffer.ID: &genesisDepositOffer,
		newDepositOffer.ID:     &newDepositOffer,
	}

	require.Equal(t, cs.depositOffers, expectedDepositOffers)
}
