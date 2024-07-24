// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************
// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package version

import (
	"encoding/json"
	"time"

	_ "embed"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
)

const (
	Client = "avalanchego"
	// RPCChainVMProtocol should be bumped anytime changes are made which
	// require the plugin vm to upgrade to latest avalanchego release to be
	// compatible.
	RPCChainVMProtocol uint = 31
)

// These are globals that describe network upgrades and node versions
var (
	// GitCommit and GitVersion will be set with -X during build step
	GitCommit  string = "unknown"
	GitVersion string = "unknown"

	Current = &Semantic{
		Major: 1,
		Minor: 1,
		Patch: 18,
	}
	CurrentApp = &Application{
		Name:  Client,
		Major: Current.Major,
		Minor: Current.Minor,
		Patch: Current.Patch,
	}
	MinimumCompatibleVersion = &Application{
		Name:  Client,
		Major: 1,
		Minor: 1,
		Patch: 0,
	}
	PrevMinimumCompatibleVersion = &Application{
		Name:  Client,
		Major: 1,
		Minor: 0,
		Patch: 0,
	}

	CurrentDatabase = DatabaseVersion1_4_5
	PrevDatabase    = DatabaseVersion1_0_0

	DatabaseVersion1_4_5 = &Semantic{
		Major: 1,
		Minor: 4,
		Patch: 5,
	}
	DatabaseVersion1_0_0 = &Semantic{
		Major: 1,
		Minor: 0,
		Patch: 0,
	}

	//go:embed compatibility.json
	rpcChainVMProtocolCompatibilityBytes []byte
	// RPCChainVMProtocolCompatibility maps RPCChainVMProtocol versions to the
	// set of avalanchego versions that supported that version. This is not used
	// by avalanchego, but is useful for downstream libraries.
	RPCChainVMProtocolCompatibility map[uint][]*Semantic

	DefaultUpgradeTime = time.Date(2020, time.December, 5, 5, 0, 0, 0, time.UTC)

	ApricotPhase3Times = map[uint32]time.Time{
		constants.MainnetID: time.Date(2021, time.August, 24, 14, 0, 0, 0, time.UTC),
		constants.FujiID:    time.Date(2021, time.August, 16, 19, 0, 0, 0, time.UTC),
	}

	ApricotPhase4Times = map[uint32]time.Time{
		constants.MainnetID: time.Date(2021, time.September, 22, 21, 0, 0, 0, time.UTC),
		constants.FujiID:    time.Date(2021, time.September, 16, 21, 0, 0, 0, time.UTC),
	}
	ApricotPhase4MinPChainHeight = map[uint32]uint64{
		constants.MainnetID: 793005,
		constants.FujiID:    47437,
	}

	ApricotPhase5Times = map[uint32]time.Time{
		constants.MainnetID: time.Date(2021, time.December, 2, 18, 0, 0, 0, time.UTC),
		constants.FujiID:    time.Date(2021, time.November, 24, 15, 0, 0, 0, time.UTC),
	}

	SunrisePhase0Times       = map[uint32]time.Time{}
	SunrisePhase0DefaultTime = time.Date(2022, time.May, 16, 8, 0, 0, 0, time.UTC)

	ApricotPhase6Times = map[uint32]time.Time{
		constants.MainnetID: time.Date(2022, time.September, 6, 20, 0, 0, 0, time.UTC),
		constants.FujiID:    time.Date(2022, time.September, 6, 20, 0, 0, 0, time.UTC),
	}

	BanffTimes = map[uint32]time.Time{
		constants.MainnetID: time.Date(2022, time.October, 18, 16, 0, 0, 0, time.UTC),
		constants.FujiID:    time.Date(2022, time.October, 3, 14, 0, 0, 0, time.UTC),
	}

	AthensPhaseTimes = map[uint32]time.Time{
		constants.KopernikusID: time.Date(2023, time.July, 4, 13, 0, 0, 0, time.UTC),
		constants.ColumbusID:   time.Date(2023, time.July, 7, 8, 0, 0, 0, time.UTC),
		constants.CaminoID:     time.Date(2023, time.July, 17, 8, 0, 0, 0, time.UTC),
	}
	AthensPhaseDefaultTime = time.Date(2023, time.July, 1, 8, 0, 0, 0, time.UTC)

	// TODO: update this before release
	BerlinPhaseTimes = map[uint32]time.Time{
		constants.KopernikusID: time.Date(2023, time.July, 4, 13, 0, 0, 0, time.UTC),
		constants.ColumbusID:   time.Date(2023, time.July, 7, 8, 0, 0, 0, time.UTC),
		constants.CaminoID:     time.Date(2023, time.July, 17, 8, 0, 0, 0, time.UTC),
	}
	BerlinPhaseDefaultTime = time.Date(2023, time.July, 1, 8, 0, 0, 0, time.UTC)

	// TODO: update this before release, must be exactly the same as Berlin
	CortinaTimes = map[uint32]time.Time{
		constants.MainnetID: time.Date(2023, time.April, 25, 15, 0, 0, 0, time.UTC),
		constants.FujiID:    time.Date(2023, time.April, 6, 15, 0, 0, 0, time.UTC),
	}
	CortinaXChainStopVertexID map[uint32]ids.ID

	// TODO: update this before release
	DurangoTimes = map[uint32]time.Time{
		constants.MainnetID: time.Date(10000, time.December, 1, 0, 0, 0, 0, time.UTC),
		constants.FujiID:    time.Date(10000, time.December, 1, 0, 0, 0, 0, time.UTC),
	}
)

func init() {
	var parsedRPCChainVMCompatibility map[uint][]string
	err := json.Unmarshal(rpcChainVMProtocolCompatibilityBytes, &parsedRPCChainVMCompatibility)
	if err != nil {
		panic(err)
	}

	RPCChainVMProtocolCompatibility = make(map[uint][]*Semantic)
	for rpcChainVMProtocol, versionStrings := range parsedRPCChainVMCompatibility {
		versions := make([]*Semantic, len(versionStrings))
		for i, versionString := range versionStrings {
			version, err := Parse(versionString)
			if err != nil {
				panic(err)
			}
			versions[i] = version
		}
		RPCChainVMProtocolCompatibility[rpcChainVMProtocol] = versions
	}

	// The mainnet stop vertex is well known. It can be verified on any fully
	// synced node by looking at the parentID of the genesis block.
	//
	// Ref: https://subnets.avax.network/x-chain/block/0
	mainnetXChainStopVertexID, err := ids.FromString("jrGWDh5Po9FMj54depyunNixpia5PN4aAYxfmNzU8n752Rjga")
	if err != nil {
		panic(err)
	}

	// The fuji stop vertex is well known. It can be verified on any fully
	// synced node by looking at the parentID of the genesis block.
	//
	// Ref: https://subnets-test.avax.network/x-chain/block/0
	fujiXChainStopVertexID, err := ids.FromString("2D1cmbiG36BqQMRyHt4kFhWarmatA1ighSpND3FeFgz3vFVtCZ")
	if err != nil {
		panic(err)
	}

	CortinaXChainStopVertexID = map[uint32]ids.ID{
		constants.MainnetID: mainnetXChainStopVertexID,
		constants.FujiID:    fujiXChainStopVertexID,
	}
}

func GetApricotPhase3Time(networkID uint32) time.Time {
	if upgradeTime, exists := ApricotPhase3Times[networkID]; exists {
		return upgradeTime
	}
	return DefaultUpgradeTime
}

func GetApricotPhase4Time(networkID uint32) time.Time {
	if upgradeTime, exists := ApricotPhase4Times[networkID]; exists {
		return upgradeTime
	}
	return DefaultUpgradeTime
}

func GetApricotPhase5Time(networkID uint32) time.Time {
	if upgradeTime, exists := ApricotPhase5Times[networkID]; exists {
		return upgradeTime
	}
	return DefaultUpgradeTime
}

func GetSunrisePhase0Time(networkID uint32) time.Time {
	if upgradeTime, exists := SunrisePhase0Times[networkID]; exists {
		return upgradeTime
	}
	return SunrisePhase0DefaultTime
}

func GetApricotPhase6Time(networkID uint32) time.Time {
	if upgradeTime, exists := ApricotPhase6Times[networkID]; exists {
		return upgradeTime
	}
	return DefaultUpgradeTime
}

func GetBanffTime(networkID uint32) time.Time {
	if upgradeTime, exists := BanffTimes[networkID]; exists {
		return upgradeTime
	}
	return DefaultUpgradeTime
}

func GetAthensPhaseTime(networkID uint32) time.Time {
	if upgradeTime, exists := AthensPhaseTimes[networkID]; exists {
		return upgradeTime
	}
	return AthensPhaseDefaultTime
}

func GetBerlinPhaseTime(networkID uint32) time.Time {
	if upgradeTime, exists := BerlinPhaseTimes[networkID]; exists {
		return upgradeTime
	}
	return BerlinPhaseDefaultTime
}

func GetCortinaTime(networkID uint32) time.Time {
	if upgradeTime, exists := CortinaTimes[networkID]; exists {
		return upgradeTime
	}
	return DefaultUpgradeTime
}

func GetDurangoTime(networkID uint32) time.Time {
	if upgradeTime, exists := DurangoTimes[networkID]; exists {
		return upgradeTime
	}
	return DefaultUpgradeTime
}

func GetCompatibility(networkID uint32) Compatibility {
	return NewCompatibility(
		CurrentApp,
		MinimumCompatibleVersion,
		GetBerlinPhaseTime(networkID).Add(-time.Minute),
		PrevMinimumCompatibleVersion,
	)
}
