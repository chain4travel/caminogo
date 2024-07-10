// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package info

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/logging"
)

func TestGetGenesisBytes(t *testing.T) {
	service := Info{log: logging.NoLog{}}
	service.GenesisBytes = []byte("some random bytes")
	reply := GetGenesisBytesReply{}
	require.NoError(t, service.GetGenesisBytes(nil, nil, &reply))
	require.Equal(t, GetGenesisBytesReply{GenesisBytes: service.GenesisBytes}, reply)
}
