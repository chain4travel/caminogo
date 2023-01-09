// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/database/versiondb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/stretchr/testify/require"

	_ "embed"
)

var (
	defaultRewardConfig = reward.Config{
		MaxConsumptionRate: .12 * reward.PercentDenominator,
		MinConsumptionRate: .10 * reward.PercentDenominator,
		MintingPeriod:      365 * 24 * time.Hour,
		SupplyCap:          720 * units.MegaAvax,
	}
	assignedPAllocAmount   = uint64(4000000000000)
	unassignedPAllocAmount = uint64(10000000000000000)
	maxValidatorStake      = GetStakingConfig(constants.KopernikusID).MaxValidatorStake
	initialAdmin           = "X-kopernikus1g65uqn6t77p656w64023nh8nd9updzmxh8ttv3"
	expectedUtxoID1        = wrappers.IgnoreError(ids.FromString("23P43gnzKqawVt7UnoWBJpKakuja6jLJFJ6NbqqH4K7AzZC5f8")).(ids.ID)
	expectedUtxoID2        = wrappers.IgnoreError(ids.FromString("2Uz3NaWp8NieLSiAgaUKV9SJyyYHkRvRF4PPQeRJhchkTzFAAk")).(ids.ID)
	expectedUtxoID3        = wrappers.IgnoreError(ids.FromString("NxUtNF917PDfBkm6ZRvZE5qxrnirT4ucWktyYZzCMBTD9Y659")).(ids.ID)
	expectedBondTxID       = wrappers.IgnoreError(ids.FromString("2rpuZwaVeHj5Mov1eePjq2DW3XhtwCxW21dVXg2QYeWMFJ5Kq9")).(ids.ID)
)

func TestValidateCaminoConfig(t *testing.T) {
	type args struct {
		config *Config
	}
	tests := map[string]struct {
		args args
		err  error
	}{
		"non-camino allocations / LockModeBondDeposit=false": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.LockModeBondDeposit = false
					thisConfig.Allocations = []Allocation{{AVAXAddr: ids.GenerateTestShortID()}}
					return &thisConfig
				}(),
			},
		},
		"non-camino allocations": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Allocations = []Allocation{{AVAXAddr: ids.GenerateTestShortID()}}
					return &thisConfig
				}(),
			},
			err: errors.New("config.Allocations != 0"),
		},
		"invalid deposit offer / start date after before date": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					offer := KopernikusConfig.Camino.DepositOffers[0]
					offer.Start = 2
					offer.End = 1
					offers := []genesis.DepositOffer{offer}
					thisConfig.Camino.DepositOffers = offers
					return &thisConfig
				}(),
			},
			err: fmt.Errorf("%w: starttime %d, endtime %d", genesis.ErrOfferStartNotBeforeEnd, 2, 1),
		},
		"invalid deposit offer duplicate": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.DepositOffers = append(thisConfig.Camino.DepositOffers, thisConfig.Camino.DepositOffers[0])
					return &thisConfig
				}(),
			},
			err: errors.New("deposit offer duplicate"),
		},
		"allocation duplicate": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{thisConfig.Camino.Allocations[0]} // adding duplicate of 1st allocation into the array
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, KopernikusConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("allocation duplicate"),
		},
		"platform allocation duplicate": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					a := thisConfig.Camino.Allocations[0]
					a.PlatformAllocations = []PlatformAllocation{KopernikusConfig.Camino.Allocations[0].PlatformAllocations[0]}
					a.PlatformAllocations = append(a.PlatformAllocations, KopernikusConfig.Camino.Allocations[0].PlatformAllocations...)
					thisConfig.Camino.Allocations = []CaminoAllocation{a}
					return &thisConfig
				}(),
			},
			err: errors.New("platform allocation duplicate"),
		},
		"allocation / deposit offer mismatch": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{PlatformAllocations: []PlatformAllocation{{DepositOfferID: ids.GenerateTestID()}}}}
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, KopernikusConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("allocation deposit offer id doesn't match any offer"),
		},
		"staker ins't consortium member": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{PlatformAllocations: []PlatformAllocation{{NodeID: ids.GenerateTestNodeID()}}}}
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, KopernikusConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("staker ins't consortium member"),
		},
		"consortium member not kyc verified": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{AddressStates: AddressStates{ConsortiumMember: true}, PlatformAllocations: []PlatformAllocation{{NodeID: ids.GenerateTestNodeID()}}}}
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, KopernikusConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("consortium member not kyc verified"),
		},
		"wrong validator duration": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{AddressStates: AddressStates{ConsortiumMember: true, KYCVerified: true}, PlatformAllocations: []PlatformAllocation{{NodeID: ids.GenerateTestNodeID(), ValidatorDuration: 0}}}}
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, KopernikusConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("wrong validator duration"),
		},
		"repeated staker allocation": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{AddressStates: AddressStates{ConsortiumMember: true, KYCVerified: true}, PlatformAllocations: []PlatformAllocation{{NodeID: KopernikusConfig.Camino.Allocations[0].PlatformAllocations[0].NodeID, ValidatorDuration: 1}}}}
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, KopernikusConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("repeated staker allocation"),
		},
		"no staker allocations": {
			args: args{
				config: func() *Config {
					thisConfig := KopernikusConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{PlatformAllocations: []PlatformAllocation{}}}
					return &thisConfig
				}(),
			},
			err: errors.New("no staker allocations"),
		},
		"valid": {
			args: args{
				config: &KopernikusConfig,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateCaminoConfig(tt.args.config)

			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestBuildCaminoGenesis(t *testing.T) {
	ctx := snow.DefaultContextTest()
	rewards := reward.NewCalculator(defaultRewardConfig)
	ctx.NetworkID = constants.KopernikusID
	baseDB := memdb.New()
	db := versiondb.New(baseDB)
	cfg := defaultConfig()

	addrs := set.Set[ids.ShortID]{}
	_, _, avaxAddrBytes, err := address.Parse(initialAdmin)
	require.NoError(t, err)
	avaxAddr, err := ids.ToShortID(avaxAddrBytes)
	require.NoError(t, err)
	addrs.Add(avaxAddr)
	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{avaxAddr},
	}
	type args struct {
		config *Config
		hrp    string
	}
	tests := map[string]struct {
		args          args
		expectedUtxos map[ids.ID]*avax.UTXO
		err           error
	}{
		"success - kopernikus": {
			args: args{
				config: &KopernikusConfig,
				hrp:    constants.KopernikusHRP,
			},
			expectedUtxos: map[ids.ID]*avax.UTXO{
				expectedUtxoID1: generateTestUTXO(expectedUtxoID1, ctx.AVAXAssetID, assignedPAllocAmount-maxValidatorStake, outputOwners, expectedUtxoID1, ids.Empty),
				expectedUtxoID2: generateTestUTXO(expectedUtxoID2, ctx.AVAXAssetID, maxValidatorStake, outputOwners, expectedUtxoID2, expectedBondTxID),
				expectedUtxoID3: generateTestUTXO(expectedUtxoID3, ctx.AVAXAssetID, unassignedPAllocAmount, outputOwners, expectedUtxoID3, ids.Empty),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			genesisBytes, _, err := buildCaminoGenesis(tt.args.config, tt.args.hrp)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			s := defaultState(genesisBytes, cfg, ctx, db, rewards)

			utxos, err := avax.GetAllUTXOs(s, addrs)
			require.NoError(t, err)
			require.Equal(t, len(tt.expectedUtxos), len(utxos))
			for _, utxo := range utxos {
				require.Equal(t, tt.expectedUtxos[utxo.TxID].TxID, utxo.TxID)
				require.Equal(t, tt.expectedUtxos[utxo.TxID].Out, utxo.Out)
			}

			_, err = s.GetAddressStates(avaxAddr)
			require.NoError(t, err)
			offers, err := s.GetAllDepositOffers()
			require.NoError(t, err)

			genesisOffers := tt.args.config.Camino.DepositOffers

			for _, offer := range offers {
				require.Equal(t, getGenesisOffer(offer.ID, genesisOffers).Start, offer.Start)
				require.Equal(t, getGenesisOffer(offer.ID, genesisOffers).End, offer.End)
				require.Equal(t, uint64(1), offer.MinAmount)
				require.GreaterOrEqual(t, wrappers.IgnoreError(math.Sub(offer.End, offer.Start)), uint64(offer.MinDuration))
				require.Equal(t, offer.MinDuration, offer.MaxDuration)
				require.Equal(t, offer.NoRewardsPeriodDuration*2, offer.UnlockPeriodDuration)
				require.Equal(t, deposit.OfferFlagLocked, offer.Flags)
			}
		})
	}
}

func getGenesisOffer(id ids.ID, offers []genesis.DepositOffer) genesis.DepositOffer {
	for _, do := range offers {
		doID, err := do.ID()
		if err != nil {
			panic(err)
		}
		if doID == id {
			return do
		}
	}
	return genesis.DepositOffer{}
}
