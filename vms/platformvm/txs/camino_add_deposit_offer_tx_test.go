// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

func TestAddDepositOfferTxSyntacticVerify(t *testing.T) {
	ctx := defaultContext()
	owner1 := secp256k1fx.OutputOwners{Threshold: 1, Addrs: []ids.ShortID{{0, 0, 1}}}
	depositTxID := ids.ID{0, 1}
	creatorAddress := ids.ShortID{1}

	offer1 := &deposit.Offer{
		UpgradeVersionID: codec.UpgradeVersion1,
		End:              1,
		MinDuration:      1,
		MaxDuration:      1,
		MinAmount:        deposit.OfferMinDepositAmount,
		TotalMaxAmount:   1,
	}

	baseTx := BaseTx{BaseTx: avax.BaseTx{
		NetworkID:    ctx.NetworkID,
		BlockchainID: ctx.ChainID,
	}}

	tests := map[string]struct {
		tx          *AddDepositOfferTx
		expectedErr error
	}{
		"Nil tx": {
			expectedErr: ErrNilTx,
		},
		"Empty deposit offer creator address": {
			tx: &AddDepositOfferTx{
				BaseTx: baseTx,
			},
			expectedErr: errEmptyDepositOfferCreatorAddress,
		},
		"Non-zero RewardedAmount": {
			tx: &AddDepositOfferTx{
				BaseTx:                     baseTx,
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer: &deposit.Offer{
					UpgradeVersionID: codec.UpgradeVersion1,
					RewardedAmount:   1,
				},
			},
			expectedErr: errNotZeroDepositOfferAmounts,
		},
		"Non-zero DepositedAmount": {
			tx: &AddDepositOfferTx{
				BaseTx:                     baseTx,
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer: &deposit.Offer{
					UpgradeVersionID: codec.UpgradeVersion1,
					DepositedAmount:  1,
				},
			},
			expectedErr: errNotZeroDepositOfferAmounts,
		},
		"Zero TotalMaxAmount and TotalMaxRewardAmount": {
			tx: &AddDepositOfferTx{
				BaseTx:                     baseTx,
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer: &deposit.Offer{
					UpgradeVersionID: codec.UpgradeVersion1,
				},
			},
			expectedErr: errZeroDepositOfferLimits,
		},
		"Bad deposit offer": {
			tx: &AddDepositOfferTx{
				BaseTx:                     baseTx,
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer:               &deposit.Offer{UpgradeVersionID: codec.UpgradeVersion1, TotalMaxAmount: 1},
			},
			expectedErr: errBadDepositOffer,
		},
		"Bad deposit offer creator auth": {
			tx: &AddDepositOfferTx{
				BaseTx:                     baseTx,
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer:               offer1,
				DepositOfferCreatorAuth:    (*secp256k1fx.Input)(nil),
			},
			expectedErr: errBadDepositOfferCreatorAuth,
		},
		"Deposit offer version 0": {
			tx: &AddDepositOfferTx{
				BaseTx:                     baseTx,
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer: &deposit.Offer{
					End:         1,
					MinDuration: 1,
					MaxDuration: 1,
					MinAmount:   deposit.OfferMinDepositAmount,
				},
			},
			expectedErr: errWrongDepositOfferVersion,
		},
		"Locked base tx input": {
			tx: &AddDepositOfferTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    ctx.NetworkID,
					BlockchainID: ctx.ChainID,
					Ins: []*avax.TransferableInput{
						generateTestIn(ctx.AVAXAssetID, 1, depositTxID, ids.Empty, []uint32{0}),
					},
				}},
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer:               offer1,
				DepositOfferCreatorAuth:    &secp256k1fx.Input{SigIndices: []uint32{}},
			},
			expectedErr: locked.ErrWrongInType,
		},
		"Locked base tx output": {
			tx: &AddDepositOfferTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    ctx.NetworkID,
					BlockchainID: ctx.ChainID,
					Outs: []*avax.TransferableOutput{
						generateTestOut(ctx.AVAXAssetID, 1, owner1, depositTxID, ids.Empty),
					},
				}},
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer:               offer1,
				DepositOfferCreatorAuth:    &secp256k1fx.Input{SigIndices: []uint32{}},
			},
			expectedErr: locked.ErrWrongOutType,
		},
		"Stakable base tx input": {
			tx: &AddDepositOfferTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    ctx.NetworkID,
					BlockchainID: ctx.ChainID,
					Ins: []*avax.TransferableInput{
						generateTestStakeableIn(ctx.AVAXAssetID, 1, 1, []uint32{0}),
					},
				}},
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer:               offer1,
				DepositOfferCreatorAuth:    &secp256k1fx.Input{SigIndices: []uint32{}},
			},
			expectedErr: locked.ErrWrongInType,
		},
		"Stakable base tx output": {
			tx: &AddDepositOfferTx{
				BaseTx: BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    ctx.NetworkID,
					BlockchainID: ctx.ChainID,
					Outs: []*avax.TransferableOutput{
						generateTestStakeableOut(ctx.AVAXAssetID, 1, 1, owner1),
					},
				}},
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer:               offer1,
				DepositOfferCreatorAuth:    &secp256k1fx.Input{SigIndices: []uint32{}},
			},
			expectedErr: locked.ErrWrongOutType,
		},
		"OK: offer v1": {
			tx: &AddDepositOfferTx{
				BaseTx:                     baseTx,
				DepositOfferCreatorAddress: creatorAddress,
				DepositOffer:               offer1,
				DepositOfferCreatorAuth:    &secp256k1fx.Input{SigIndices: []uint32{}},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.tx.SyntacticVerify(ctx)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}