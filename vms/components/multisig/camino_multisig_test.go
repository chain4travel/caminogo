// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package multisig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/components/avax"
)

func TestVerify(t *testing.T) {
	tests := map[string]struct {
		alias       Alias
		message     string
		expectedErr error
	}{
		"MemoSizeShouldBeLowerThanMaxMemoSize": {
			alias: Alias{
				Owners: &avax.TestState{},
				Memo:   make([]byte, avax.MaxMemoSize+1),
				ID:     hashing.ComputeHash160Array(ids.Empty[:]),
			},
			message:     "memo size should be lower than max memo size",
			expectedErr: errMemoIsToBig,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.alias.Verify()
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
