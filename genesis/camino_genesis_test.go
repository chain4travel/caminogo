package genesis

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/stretchr/testify/require"

	_ "embed"
)

//go:embed genesis_camino_local.json
var customCaminoGenesisConfigJSON []byte

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
					thisConfig := CaminoLocalConfig
					thisConfig.Camino.LockModeBondDeposit = false
					thisConfig.Allocations = []Allocation{{AVAXAddr: ids.GenerateTestShortID()}}
					return &thisConfig
				}(),
			},
		},
		"non-camino allocations": {
			args: args{
				config: func() *Config {
					thisConfig := CaminoLocalConfig
					thisConfig.Allocations = []Allocation{{AVAXAddr: ids.GenerateTestShortID()}}
					return &thisConfig
				}(),
			},
			err: errors.New("config.Allocations != 0"),
		},
		"invalid deposit offer / start date after before date": {
			args: args{
				config: func() *Config {
					thisConfig := CaminoLocalConfig
					offer := CaminoLocalConfig.Camino.DepositOffers[0]
					offer.Start = 2
					offer.End = 1
					offers := []genesis.DepositOffer{offer}
					thisConfig.Camino.DepositOffers = offers
					return &thisConfig
				}(),
			},
			err: fmt.Errorf(
				"deposit offer starttime (%v) is not before its endtime (%v)",
				2,
				1,
			),
		},
		"invalid deposit offer duplicate": {
			args: args{
				config: func() *Config {
					thisConfig := CaminoLocalConfig
					thisConfig.Camino.DepositOffers = append(thisConfig.Camino.DepositOffers, thisConfig.Camino.DepositOffers[0])
					return &thisConfig
				}(),
			},
			err: errors.New("deposit offer duplicate"),
		},
		"allocation / deposit offer mismatch": {
			args: args{
				config: func() *Config {
					thisConfig := CaminoLocalConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{PlatformAllocations: []PlatformAllocation{{DepositOfferID: ids.GenerateTestID()}}}}
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, CaminoLocalConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("allocation deposit offer id doesn't match any offer"),
		},
		"wrong validator duration": {
			args: args{
				config: func() *Config {
					thisConfig := CaminoLocalConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{PlatformAllocations: []PlatformAllocation{{NodeID: ids.GenerateTestNodeID(), ValidatorDuration: 0}}}}
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, CaminoLocalConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("wrong validator duration"),
		},
		"repeated staker allocation": {
			args: args{
				config: func() *Config {
					thisConfig := CaminoLocalConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{PlatformAllocations: []PlatformAllocation{{NodeID: CaminoLocalConfig.Camino.Allocations[0].PlatformAllocations[0].NodeID, ValidatorDuration: 1}}}}
					thisConfig.Camino.Allocations = append(thisConfig.Camino.Allocations, CaminoLocalConfig.Camino.Allocations...)
					return &thisConfig
				}(),
			},
			err: errors.New("repeated staker allocation"),
		},
		"no staker allocations": {
			args: args{
				config: func() *Config {
					thisConfig := CaminoLocalConfig
					thisConfig.Camino.Allocations = []CaminoAllocation{{PlatformAllocations: []PlatformAllocation{}}}
					return &thisConfig
				}(),
			},
			err: errors.New("no staker allocations"),
		},
		"valid": {
			args: args{
				config: &CaminoLocalConfig,
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

func TestCaminoGenesisFromFile(t *testing.T) {
	tests := map[string]struct {
		networkID       uint32
		customConfig    []byte
		missingFilepath string
		err             string
		expected        string
	}{
		"camino local error": {
			networkID:    constants.CaminoLocalID,
			customConfig: customCaminoGenesisConfigJSON,
			err:          fmt.Sprintf("cannot override genesis config for standard network %s (%d)", constants.CaminoLocalName, constants.CaminoLocalID),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// test loading of genesis from file

			require := require.New(t)
			var customFile string
			if len(test.customConfig) > 0 {
				customFile = filepath.Join(t.TempDir(), "config.json")
				require.NoError(perms.WriteFile(customFile, test.customConfig, perms.ReadWrite))
			}

			if len(test.missingFilepath) > 0 {
				customFile = test.missingFilepath
			}

			genesisBytes, _, err := FromFile(test.networkID, customFile)
			if len(test.err) > 0 {
				require.Error(err)
				require.Contains(err.Error(), test.err)
				return
			}
			require.NoError(err)

			genesisHash := fmt.Sprintf("%x", hashing.ComputeHash256(genesisBytes))
			require.Equal(test.expected, genesisHash, "genesis hash mismatch")

			_, err = genesis.Parse(genesisBytes)
			require.NoError(err)
		})
	}
}

func TestCaminoGenesis(t *testing.T) {
	tests := []struct {
		networkID  uint32
		expectedID string
	}{
		{
			networkID:  constants.CaminoLocalID,
			expectedID: "2CtW9s71PCG6vNahuT16aXZo5RQ1MnD9EE8dyjmAC1K4BNabJV",
		},
	}
	for _, test := range tests {
		t.Run(constants.NetworkIDToNetworkName[test.networkID], func(t *testing.T) {
			require := require.New(t)

			config := GetConfig(test.networkID)
			genesisBytes, _, err := FromConfig(config)
			require.NoError(err)

			var genesisID ids.ID = hashing.ComputeHash256Array(genesisBytes)
			require.Equal(test.expectedID, genesisID.String())
		})
	}
}
