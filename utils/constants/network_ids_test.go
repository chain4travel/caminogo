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

package constants

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetHRP(t *testing.T) {
	tests := []struct {
		id  uint32
		hrp string
	}{
		{
			id:  MainnetID,
			hrp: MainnetHRP,
		},
		{
			id:  TestnetID,
			hrp: ColumbusHRP,
		},
		{
			id:  FujiID,
			hrp: FujiHRP,
		},
		{
			id:  LocalID,
			hrp: LocalHRP,
		},
		{
			id:  4294967295,
			hrp: FallbackHRP,
		},
	}
	for _, test := range tests {
		t.Run(test.hrp, func(t *testing.T) {
			require.Equal(t, test.hrp, GetHRP(test.id))
		})
	}
}

func TestNetworkName(t *testing.T) {
	tests := []struct {
		id   uint32
		name string
	}{
		{
			id:   MainnetID,
			name: MainnetName,
		},
		{
			id:   TestnetID,
			name: ColumbusName,
		},
		{
			id:   FujiID,
			name: FujiName,
		},
		{
			id:   LocalID,
			name: LocalName,
		},
		{
			id:   4294967295,
			name: "network-4294967295",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.name, NetworkName(test.id))
		})
	}
}

func TestNetworkID(t *testing.T) {
	tests := []struct {
		name        string
		id          uint32
		expectedErr error
	}{
		{
			name: MainnetName,
			id:   MainnetID,
		},
		{
			name: "MaInNeT",
			id:   MainnetID,
		},
		{
			name: TestnetName,
			id:   TestnetID,
		},
		{
			name: FujiName,
			id:   FujiID,
		},
		{
			name: LocalName,
			id:   LocalID,
		},
		{
			name: "network-4294967295",
			id:   4294967295,
		},
		{
			name: "4294967295",
			id:   4294967295,
		},
		{
			name:        "networ-4294967295",
			expectedErr: ErrParseNetworkName,
		},
		{
			name:        "network-4294967295123123",
			expectedErr: ErrParseNetworkName,
		},
		{
			name:        "4294967295123123",
			expectedErr: ErrParseNetworkName,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)

			id, err := NetworkID(test.name)
			require.ErrorIs(err, test.expectedErr)
			require.Equal(test.id, id)
		})
	}
}
