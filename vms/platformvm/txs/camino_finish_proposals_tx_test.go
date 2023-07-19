// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/stretchr/testify/require"
)

func TestFinishProposalsTxSyntacticVerify(t *testing.T) {
	ctx := defaultContext()
	owner1 := secp256k1fx.OutputOwners{Threshold: 1, Addrs: []ids.ShortID{{0, 0, 1}}}

	proposalID1 := ids.ID{1}
	proposalID2 := ids.ID{2}

	baseTx := BaseTx{BaseTx: avax.BaseTx{
		NetworkID:    ctx.NetworkID,
		BlockchainID: ctx.ChainID,
	}}

	tests := map[string]struct {
		tx          *FinishProposalsTx
		expectedErr error
	}{
		"Nil tx": {
			expectedErr: ErrNilTx,
		},
		"No proposals": {
			tx: &FinishProposalsTx{
				BaseTx: baseTx,
			},
			expectedErr: errNoFinishedProposals,
		},
		"Not unique proposals": {
			tx: &FinishProposalsTx{
				BaseTx:              baseTx,
				FinishedProposalIDs: []ids.ID{proposalID1, proposalID1},
			},
			expectedErr: errNotUniqueProposalID,
		},
		"Stakable base tx input": {
			tx: &FinishProposalsTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    ctx.NetworkID,
					BlockchainID: ctx.ChainID,
					Ins: []*avax.TransferableInput{
						generateTestStakeableIn(ctx.AVAXAssetID, 1, 1, []uint32{0}),
					},
				}},
				FinishedProposalIDs: []ids.ID{proposalID1},
			},
			expectedErr: locked.ErrWrongInType,
		},
		"Stakable base tx output": {
			tx: &FinishProposalsTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    ctx.NetworkID,
					BlockchainID: ctx.ChainID,
					Outs: []*avax.TransferableOutput{
						generateTestStakeableOut(ctx.AVAXAssetID, 1, 1, owner1),
					},
				}},
				FinishedProposalIDs: []ids.ID{proposalID1},
			},
			expectedErr: locked.ErrWrongOutType,
		},
		"OK": {
			tx: &FinishProposalsTx{
				BaseTx:              baseTx,
				FinishedProposalIDs: []ids.ID{proposalID1, proposalID2},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, tt.tx.SyntacticVerify(ctx), tt.expectedErr)
		})
	}
}
