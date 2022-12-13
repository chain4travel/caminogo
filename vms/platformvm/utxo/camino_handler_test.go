// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utxo

import (
	"testing"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/stretchr/testify/require"
)

var keys = crypto.BuildTestKeys()

func TestVerifySpendUTXOsWithLocked(t *testing.T) {
	fx := &secp256k1fx.Fx{}

	err := fx.InitializeVM(&secp256k1fx.TestVM{})
	require.NoError(t, err)

	err = fx.Bootstrapped()
	require.NoError(t, err)

	testHandler := &handler{
		ctx: snow.DefaultContextTest(),
		clk: &mockable.Clock{},
		utxosReader: avax.NewUTXOState(
			memdb.New(),
			txs.Codec,
		),
		fx: fx,
	}
	assetID := testHandler.ctx.AVAXAssetID

	tx := &dummyUnsignedTx{txs.BaseTx{}}
	tx.Initialize([]byte{0})

	outputOwners, cred1 := generateOwnersAndSig(tx)
	sigIndices := []uint32{0}
	lockTxID := ids.GenerateTestID()

	// Note that setting [chainTimestamp] also set's the VM's clock.
	// Adjust input/output locktimes accordingly.
	tests := map[string]struct {
		utxos           []*avax.UTXO
		ins             []*avax.TransferableInput
		outs            []*avax.TransferableOutput
		creds           []verify.Verifiable
		signs           []verify.State
		producedAmounts map[ids.ID]uint64
		expectErr       bool
	}{
		"ok": {
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{1}, assetID, 10, outputOwners, ids.Empty, ids.Empty),
			},
			signs: []verify.State{
				generateTestUTXO(ids.ID{1}, assetID, 10, outputOwners, ids.Empty, ids.Empty).Out,
			},
			ins: []*avax.TransferableInput{
				generateTestIn(assetID, 10, ids.Empty, ids.Empty, sigIndices),
			},
			outs: []*avax.TransferableOutput{
				generateTestOut(assetID, 10, outputOwners, ids.Empty, ids.Empty),
			},
			producedAmounts: map[ids.ID]uint64{},
			creds:           []verify.Verifiable{cred1},
			expectErr:       false,
		},
		"utxos have locked.Out": {
			utxos: []*avax.UTXO{
				generateTestUTXO(ids.ID{1}, assetID, 10, outputOwners, lockTxID, ids.Empty),
			},
			signs: []verify.State{
				generateTestUTXO(ids.ID{1}, assetID, 10, outputOwners, lockTxID, ids.Empty).Out,
			},
			ins: []*avax.TransferableInput{
				generateTestIn(assetID, 10, ids.Empty, ids.Empty, sigIndices),
			},
			outs: []*avax.TransferableOutput{
				generateTestOut(assetID, 10, outputOwners, ids.Empty, ids.Empty),
			},
			producedAmounts: map[ids.ID]uint64{},
			creds:           []verify.Verifiable{cred1},
			expectErr:       true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := testHandler.VerifySpendUTXOs(
				tx,
				test.utxos,
				test.ins,
				test.outs,
				test.creds,
				test.signs,
				test.producedAmounts,
			)
			require.True(t, test.expectErr == (err != nil))
		})
	}
}

func TestCaminoVerifySpendUTXOs(t *testing.T) {
	fx := &secp256k1fx.Fx{}

	err := fx.InitializeVM(&secp256k1fx.TestVM{})
	require.NoError(t, err)

	err = fx.Bootstrapped()
	require.NoError(t, err)

	h := &handler{
		ctx: snow.DefaultContextTest(),
		clk: &mockable.Clock{},
		utxosReader: avax.NewUTXOState(
			memdb.New(),
			txs.Codec,
		),
		fx: fx,
	}

	tx := &dummyUnsignedTx{txs.BaseTx{}}
	tx.Initialize([]byte{0})

	addresses, credentials := generateAddressesAndSignersWithKeys(tx, keys)

	tests := map[string]struct {
		utxos           []*avax.UTXO
		ins             []*avax.TransferableInput
		outs            []*avax.TransferableOutput
		creds           []verify.Verifiable
		signers         []verify.State
		producedAmounts map[ids.ID]uint64
		expectedErr     error
	}{
		"Wrong Number of Credentials": {
			utxos: []*avax.UTXO{{
				Asset: avax.Asset{ID: ids.GenerateTestID()},
				Out: &secp256k1fx.TransferOutput{
					Amt: 1,
				},
			}},
			ins: []*avax.TransferableInput{{
				Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: 1,
				},
			}},
			outs:    []*avax.TransferableOutput{},
			creds:   []verify.Verifiable{},
			signers: []verify.State{},
			producedAmounts: map[ids.ID]uint64{
				h.ctx.AVAXAssetID: 1,
			},
			expectedErr: errWrongCredentials,
		},
		"Wrong Number of UTXOs": {
			utxos: []*avax.UTXO{},
			ins: []*avax.TransferableInput{{
				Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
				In: &secp256k1fx.TransferInput{
					Amt: 1,
				},
			}},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{
					Sigs: [][crypto.SECP256K1RSigLen]byte{
						credentials[0].Sigs[0],
					},
				},
			},
			signers: []verify.State{
				&secp256k1fx.TransferOutput{
					Amt: 1,
					OutputOwners: secp256k1fx.OutputOwners{
						Addrs: []ids.ShortID{
							addresses[0],
						}, Threshold: 1,
					},
				},
			},
			producedAmounts: map[ids.ID]uint64{
				h.ctx.AVAXAssetID: 1,
			},
			expectedErr: ErrWrongUTXONumber,
		},
		"Zero Signers, Threshold=1": {
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
					},
				},
			},
			outs: []*avax.TransferableOutput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{},
			},
			signers: []verify.State{
				&secp256k1fx.TransferOutput{
					Amt: 1,
					OutputOwners: secp256k1fx.OutputOwners{
						Addrs: []ids.ShortID{
							addresses[0],
						}, Threshold: 1,
					},
				},
			},
			producedAmounts: map[ids.ID]uint64{},
			expectedErr:     secp256k1fx.ErrTooFewSigners,
		},
		"Two owners, Threshold=1, one signature": {
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
						Input: secp256k1fx.Input{
							SigIndices: []uint32{1},
						},
					},
				},
			},
			outs: []*avax.TransferableOutput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{
					Sigs: [][crypto.SECP256K1RSigLen]byte{
						credentials[1].Sigs[0],
					},
				},
			},
			signers: []verify.State{
				&secp256k1fx.TransferOutput{
					Amt: 1,
					OutputOwners: secp256k1fx.OutputOwners{
						Addrs: []ids.ShortID{
							addresses[0],
							addresses[1],
						}, Threshold: 1,
					},
				},
			},
			producedAmounts: map[ids.ID]uint64{},
			expectedErr:     nil,
		},
		"More signatures than is enough cause the error": {
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
						Input: secp256k1fx.Input{
							SigIndices: []uint32{0, 1},
						},
					},
				},
			},
			outs: []*avax.TransferableOutput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{
					Sigs: [][crypto.SECP256K1RSigLen]byte{
						credentials[0].Sigs[0],
						credentials[1].Sigs[0],
					},
				},
			},
			signers: []verify.State{
				&secp256k1fx.TransferOutput{
					Amt: 1,
					OutputOwners: secp256k1fx.OutputOwners{
						Addrs: []ids.ShortID{
							addresses[0],
							addresses[1],
						}, Threshold: 1,
					},
				},
			},
			producedAmounts: map[ids.ID]uint64{},
			expectedErr:     secp256k1fx.ErrTooManySigners,
		},
		"Three owners, Threshold=2, signed by 1st & 2nd": {
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
						Input: secp256k1fx.Input{
							SigIndices: []uint32{0, 1},
						},
					},
				},
			},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{
					Sigs: [][crypto.SECP256K1RSigLen]byte{
						credentials[0].Sigs[0],
						credentials[1].Sigs[0],
					},
				},
			},
			signers: []verify.State{
				&secp256k1fx.TransferOutput{
					Amt: 1,
					OutputOwners: secp256k1fx.OutputOwners{
						Addrs: []ids.ShortID{
							addresses[0],
							addresses[1],
							addresses[2],
						}, Threshold: 2,
					},
				},
			},
			producedAmounts: map[ids.ID]uint64{},
			expectedErr:     nil,
		},
		"Three owners, Threshold=2, signed by 2nd & 3rd": {
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
						Input: secp256k1fx.Input{
							SigIndices: []uint32{1, 2},
						},
					},
				},
			},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{
					Sigs: [][crypto.SECP256K1RSigLen]byte{
						credentials[1].Sigs[0],
						credentials[2].Sigs[0],
					},
				},
			},
			signers: []verify.State{
				&secp256k1fx.TransferOutput{
					Amt: 1,
					OutputOwners: secp256k1fx.OutputOwners{
						Addrs: []ids.ShortID{
							addresses[0],
							addresses[1],
							addresses[2],
						}, Threshold: 2,
					},
				},
			},
			producedAmounts: map[ids.ID]uint64{},
			expectedErr:     nil,
		},
		"Three owners, Threshold=2, signed by 1st & 3rd": {
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
						Input: secp256k1fx.Input{
							SigIndices: []uint32{0, 2},
						},
					},
				},
			},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{
					Sigs: [][crypto.SECP256K1RSigLen]byte{
						credentials[0].Sigs[0],
						credentials[2].Sigs[0],
					},
				},
			},
			signers: []verify.State{
				&secp256k1fx.TransferOutput{
					Amt: 1,
					OutputOwners: secp256k1fx.OutputOwners{
						Addrs: []ids.ShortID{
							addresses[0],
							addresses[1],
							addresses[2],
						}, Threshold: 2,
					},
				},
			},
			producedAmounts: map[ids.ID]uint64{},
			expectedErr:     nil,
		},
		"Three Signers, Threshold=3": {
			utxos: []*avax.UTXO{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt: 1,
					},
				},
			},
			ins: []*avax.TransferableInput{
				{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					In: &secp256k1fx.TransferInput{
						Amt: 1,
						Input: secp256k1fx.Input{
							SigIndices: []uint32{0, 1, 2},
						},
					},
				},
			},
			outs: []*avax.TransferableOutput{},
			creds: []verify.Verifiable{
				&secp256k1fx.Credential{
					Sigs: [][crypto.SECP256K1RSigLen]byte{
						credentials[0].Sigs[0],
						credentials[1].Sigs[0],
						credentials[2].Sigs[0],
					},
				},
			},
			signers: []verify.State{
				&secp256k1fx.TransferOutput{
					Amt: 1,
					OutputOwners: secp256k1fx.OutputOwners{
						Addrs: []ids.ShortID{
							addresses[0],
							addresses[1],
							addresses[2],
						}, Threshold: 3,
					},
				},
			},
			producedAmounts: map[ids.ID]uint64{},
			expectedErr:     nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := h.VerifySpendUTXOs(
				tx,
				tt.utxos,
				tt.ins,
				tt.outs,
				tt.creds,
				tt.signers,
				tt.producedAmounts,
			)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
