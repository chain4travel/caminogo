// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/snowtest"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
)

func TestRewardsImportTxSyntacticVerify(t *testing.T) {
	ctx := snowtest.Context(t, snowtest.PChainID)

	tests := map[string]struct {
		tx          *RewardsImportTx
		expectedErr error
	}{
		"OK": {
			tx: &RewardsImportTx{BaseTx: BaseTx{BaseTx: avax.BaseTx{
				NetworkID:    ctx.NetworkID,
				BlockchainID: ctx.ChainID,
				Ins: []*avax.TransferableInput{
					generateTestIn(ctx.AVAXAssetID, 1, ids.Empty, ids.Empty, []uint32{}),
					generateTestIn(ctx.AVAXAssetID, 1, ids.Empty, ids.Empty, []uint32{}),
				},
			}}},
		},
		"Nil tx": {
			expectedErr: ErrNilTx,
		},
		"Input has wrong asset": {
			tx: &RewardsImportTx{BaseTx: BaseTx{BaseTx: avax.BaseTx{
				NetworkID:    ctx.NetworkID,
				BlockchainID: ctx.ChainID,
				Ins: []*avax.TransferableInput{
					generateTestIn(ctx.AVAXAssetID, 1, ids.Empty, ids.Empty, []uint32{}),
					generateTestIn(ids.GenerateTestID(), 1, ids.Empty, ids.Empty, []uint32{}),
				},
			}}},
			expectedErr: errNotAVAXAsset,
		},
		"Locked input": {
			tx: &RewardsImportTx{BaseTx: BaseTx{BaseTx: avax.BaseTx{
				NetworkID:    ctx.NetworkID,
				BlockchainID: ctx.ChainID,
				Ins: []*avax.TransferableInput{
					generateTestIn(ctx.AVAXAssetID, 1, ids.GenerateTestID(), ids.Empty, []uint32{}),
				},
			}}},
			expectedErr: locked.ErrWrongInType,
		},
		"Stakable input": {
			tx: &RewardsImportTx{BaseTx: BaseTx{BaseTx: avax.BaseTx{
				NetworkID:    ctx.NetworkID,
				BlockchainID: ctx.ChainID,
				Ins: []*avax.TransferableInput{
					generateTestStakeableIn(ctx.AVAXAssetID, 1, 1, []uint32{}),
				},
			}}},
			expectedErr: locked.ErrWrongInType,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if tt.tx != nil {
				avax.SortTransferableInputs(tt.tx.Ins)
			}
			err := tt.tx.SyntacticVerify(ctx)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
