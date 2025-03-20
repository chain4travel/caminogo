// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package utxo

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/multisig"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/platformvm/test"
	"github.com/ava-labs/avalanchego/vms/platformvm/test/expect"
	"github.com/ava-labs/avalanchego/vms/platformvm/test/generate"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestUnlockUTXOs(t *testing.T) {
	testHandler := defaultCaminoHandler(t)
	ctx := testHandler.ctx

	cryptFactory := secp256k1.Factory{}
	key, err := cryptFactory.NewPrivateKey()
	require.NoError(t, err)
	address := key.Address()
	outputOwners := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{address},
	}
	existingTxID := ids.GenerateTestID()

	type want struct {
		ins  []*avax.TransferableInput
		outs []*avax.TransferableOutput
	}
	tests := map[string]struct {
		lockState     locked.State
		utxos         []*avax.UTXO
		generateWant  func([]*avax.UTXO) want
		expectedError error
	}{
		"Unbond bonded UTXOs": {
			lockState: locked.StateBonded,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{9, 9}, ctx.AVAXAssetID, 5, outputOwners, ids.Empty, existingTxID, true),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generate.InFromUTXO(t, utxos[0], []uint32{}, false),
					},
					outs: []*avax.TransferableOutput{
						generate.Out(ctx.AVAXAssetID, 5, outputOwners, ids.Empty, ids.Empty),
					},
				}
			},
		},
		"Undeposit deposited UTXOs": {
			lockState: locked.StateDeposited,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{9, 9}, ctx.AVAXAssetID, 5, outputOwners, existingTxID, ids.Empty, true),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{
					ins: []*avax.TransferableInput{
						generate.InFromUTXO(t, utxos[0], []uint32{}, false),
					},
					outs: []*avax.TransferableOutput{
						generate.Out(ctx.AVAXAssetID, 5, outputOwners, ids.Empty, ids.Empty),
					},
				}
			},
		},
		"Unbond deposited UTXOs": {
			lockState: locked.StateBonded,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{9, 9}, ctx.AVAXAssetID, 5, outputOwners, existingTxID, ids.Empty, true),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{}
			},
			expectedError: errNotLockedUTXO,
		},
		"Undeposit bonded UTXOs": {
			lockState: locked.StateDeposited,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{9, 9}, ctx.AVAXAssetID, 5, outputOwners, ids.Empty, existingTxID, true),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{}
			},
			expectedError: errNotLockedUTXO,
		},
		"Unlock unlocked UTXOs": {
			lockState: locked.StateBonded,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{9, 9}, ctx.AVAXAssetID, 5, outputOwners, ids.Empty, ids.Empty, true),
			},
			generateWant: func(utxos []*avax.UTXO) want {
				return want{}
			},
			expectedError: errNotLockedUTXO,
		},
		"Wrong state, lockStateUnlocked": {
			lockState: locked.StateUnlocked,
			generateWant: func(utxos []*avax.UTXO) want {
				return want{}
			},
			expectedError: errInvalidTargetLockState,
		},
		"Wrong state, LockStateDepositedBonded": {
			lockState: locked.StateDepositedBonded,
			generateWant: func(utxos []*avax.UTXO) want {
				return want{}
			},
			expectedError: errInvalidTargetLockState,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			expected := tt.generateWant(tt.utxos)
			ins, outs, err := testHandler.unlockUTXOs(tt.utxos, tt.lockState)

			require.Equal(expected.ins, ins)
			require.Equal(expected.outs, outs)
			require.ErrorIs(tt.expectedError, err)
		})
	}
}

func TestLock(t *testing.T) {
	utxoOwner := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{test.Keys[0].Address()},
	}
	changeOwner := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{{1}},
	}
	recipientOwner := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{{2}},
	}

	existingTxID := ids.ID{1, 1}

	defaultState := func(
		t *testing.T,
		ctrl *gomock.Controller,
		utxos []*avax.UTXO,
		keys []*secp256k1.PrivateKey,
		to *secp256k1fx.OutputOwners,
		change *secp256k1fx.OutputOwners,
	) *state.MockState {
		t.Helper()
		state := state.NewMockState(ctrl)
		keychain := secp256k1fx.NewKeychain(keys...)
		utxoOwnerAddresses := keychain.Addresses().List()
		expect.StateVerifyMultisigOwner(t, state, to, nil, nil, true)
		expect.StateVerifyMultisigOwner(t, state, change, nil, nil, true)
		expect.StateGetAllUTXOs(t, state, utxoOwnerAddresses, [][]*avax.UTXO{utxos})
		for _, utxo := range utxos {
			expect.StateSpendMultisig(t, state, utxo)
		}
		return state
	}

	defaultStateWithEarlyErr := func(
		t *testing.T,
		ctrl *gomock.Controller,
		utxos []*avax.UTXO,
		keys []*secp256k1.PrivateKey,
		to *secp256k1fx.OutputOwners,
		change *secp256k1fx.OutputOwners,
	) *state.MockState {
		t.Helper()
		state := state.NewMockState(ctrl)
		keychain := secp256k1fx.NewKeychain(keys...)
		utxoOwnerAddresses := keychain.Addresses().List()
		expect.StateVerifyMultisigOwner(t, state, to, nil, nil, true)
		expect.StateVerifyMultisigOwner(t, state, change, nil, nil, true)
		expect.StateGetAllUTXOs(t, state, utxoOwnerAddresses, [][]*avax.UTXO{utxos})
		return state
	}

	noOpState := func(
		t *testing.T,
		ctrl *gomock.Controller,
		utxos []*avax.UTXO,
		keys []*secp256k1.PrivateKey,
		to *secp256k1fx.OutputOwners,
		change *secp256k1fx.OutputOwners,
	) *state.MockState {
		t.Helper()
		return state.NewMockState(ctrl)
	}

	tests := map[string]struct {
		state func(
			t *testing.T,
			ctrl *gomock.Controller,
			utxos []*avax.UTXO,
			keys []*secp256k1.PrivateKey,
			to *secp256k1fx.OutputOwners,
			change *secp256k1fx.OutputOwners,
		) *state.MockState

		utxos              []*avax.UTXO
		totalAmountToSpend uint64
		totalAmountToBurn  uint64
		appliedLockState   locked.State
		to                 *secp256k1fx.OutputOwners
		change             *secp256k1fx.OutputOwners
		keys               []*secp256k1.PrivateKey
		expectedIns        func([]*avax.UTXO) []*avax.TransferableInput
		expectedOuts       []*avax.TransferableOutput
		expectedOwners     []*secp256k1fx.OutputOwners
		expectedSigners    [][]*secp256k1.PrivateKey
		expectedErr        error
	}{
		"New bond owner": {
			state:            noOpState,
			appliedLockState: locked.StateBonded,
			to:               &recipientOwner,
			expectedErr:      errNewBondOwner,
		},
		"Syntactically invalid change owner": {
			state: noOpState,
			change: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{{100}},
				Threshold: 2,
			},
			expectedErr: errInvalidChangeOwner,
		},
		"Syntactically invalid to-owner": {
			state: noOpState,
			to: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{{100}},
				Threshold: 2,
			},
			expectedErr: errInvalidToOwner,
		},
		"Nested multisig change owner": {
			state: func(
				t *testing.T,
				ctrl *gomock.Controller,
				_ []*avax.UTXO,
				_ []*secp256k1.PrivateKey,
				_ *secp256k1fx.OutputOwners,
				change *secp256k1fx.OutputOwners,
			) *state.MockState {
				t.Helper()
				state := state.NewMockState(ctrl)
				expect.StateVerifyMultisigOwner(
					t, state, change,
					[]ids.ShortID{change.Addrs[0], {100}},
					[]*multisig.AliasWithNonce{
						{Alias: multisig.Alias{
							Owners: &secp256k1fx.OutputOwners{Addrs: []ids.ShortID{{100}}},
						}},
						{Alias: multisig.Alias{Owners: &secp256k1fx.OutputOwners{}}},
					},
					false,
				)
				return state
			},
			change:      &changeOwner,
			expectedErr: errInvalidChangeOwner,
		},
		"Composite multisig change owner": {
			state: func(
				t *testing.T,
				ctrl *gomock.Controller,
				_ []*avax.UTXO,
				_ []*secp256k1.PrivateKey,
				_ *secp256k1fx.OutputOwners,
				change *secp256k1fx.OutputOwners,
			) *state.MockState {
				t.Helper()
				state := state.NewMockState(ctrl)
				expect.StateVerifyMultisigOwner(
					t, state, change,
					[]ids.ShortID{{100}},
					[]*multisig.AliasWithNonce{{Alias: multisig.Alias{Owners: &secp256k1fx.OutputOwners{}}}},
					false,
				)
				return state
			},
			change: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{{100}, {101}},
				Threshold: 1,
			},
			expectedErr: errInvalidChangeOwner,
		},
		"Nested multisig to-owner": {
			state: func(
				t *testing.T,
				ctrl *gomock.Controller,
				_ []*avax.UTXO,
				_ []*secp256k1.PrivateKey,
				to *secp256k1fx.OutputOwners,
				_ *secp256k1fx.OutputOwners,
			) *state.MockState {
				t.Helper()
				state := state.NewMockState(ctrl)
				expect.StateVerifyMultisigOwner(
					t, state, to,
					[]ids.ShortID{to.Addrs[0], {100}},
					[]*multisig.AliasWithNonce{
						{Alias: multisig.Alias{
							Owners: &secp256k1fx.OutputOwners{Addrs: []ids.ShortID{{100}}},
						}},
						{Alias: multisig.Alias{Owners: &secp256k1fx.OutputOwners{}}},
					},
					false,
				)
				return state
			},
			to:          &recipientOwner,
			expectedErr: errInvalidToOwner,
		},
		"Composite multisig to-owner": {
			state: func(
				t *testing.T,
				ctrl *gomock.Controller,
				_ []*avax.UTXO,
				_ []*secp256k1.PrivateKey,
				to *secp256k1fx.OutputOwners,
				_ *secp256k1fx.OutputOwners,
			) *state.MockState {
				t.Helper()
				state := state.NewMockState(ctrl)
				expect.StateVerifyMultisigOwner(
					t, state, to,
					[]ids.ShortID{{100}},
					[]*multisig.AliasWithNonce{{Alias: multisig.Alias{Owners: &secp256k1fx.OutputOwners{}}}},
					false,
				)
				return state
			},
			to: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{{100}, {101}},
				Threshold: 1,
			},
			expectedErr: errInvalidToOwner,
		},
		"Bond unlocked utxo: not enough balance": {
			state:              defaultState,
			totalAmountToSpend: 9,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateBonded,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
			},
			expectedErr: errInsufficientBalance,
		},
		"Deposit unlocked utxo: not enough balance": {
			state:              defaultState,
			totalAmountToSpend: 9,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateDeposited,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
			},
			expectedErr: errInsufficientBalance,
		},
		"Bond bonded utxo": {
			state:              defaultStateWithEarlyErr,
			totalAmountToSpend: 9,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateBonded,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 10, utxoOwner, ids.Empty, existingTxID, true),
			},
			expectedErr: errInsufficientBalance,
		},
		"Deposit deposited utxo": {
			state:              defaultStateWithEarlyErr,
			totalAmountToSpend: 9,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateDeposited,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 1, utxoOwner, existingTxID, ids.Empty, true),
			},
			expectedErr: errInsufficientBalance,
		},
		"Deposit bonded for new owner: not enough balance": {
			state: func(
				t *testing.T,
				ctrl *gomock.Controller,
				utxos []*avax.UTXO,
				keys []*secp256k1.PrivateKey,
				toOwner *secp256k1fx.OutputOwners,
				_ *secp256k1fx.OutputOwners,
			) *state.MockState {
				t.Helper()
				state := state.NewMockState(ctrl)
				keychain := secp256k1fx.NewKeychain(keys...)
				utxoOwnerAddresses := keychain.Addresses().List()
				expect.StateGetAllUTXOs(t, state, utxoOwnerAddresses, [][]*avax.UTXO{utxos})
				expect.StateVerifyMultisigOwner(t, state, toOwner, nil, nil, true)
				expect.StateSpendMultisig(t, state, utxos[0])
				return state
			},
			totalAmountToSpend: 2,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateDeposited,
			to:                 &recipientOwner,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 2, utxoOwner, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{9, 9}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, existingTxID, true),
			},
			expectedErr: errInsufficientBalance,
		},
		"OK: burn, full transfer with change to other address": {
			state:              defaultState,
			totalAmountToSpend: 1,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateUnlocked,
			change:             &changeOwner,
			to:                 &recipientOwner,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
			},
			expectedIns: func(utxos []*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.InFromUTXO(t, utxos[0], []uint32{0}, false),
				}
			},
			expectedOuts: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 1, recipientOwner, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 3, changeOwner, ids.Empty, ids.Empty),
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{test.Keys[0]},
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&utxoOwner,
			},
		},
		"OK: burn, self transfer": {
			state:              defaultState,
			totalAmountToSpend: 1,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateUnlocked,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
			},
			expectedIns: func(utxos []*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.InFromUTXO(t, utxos[0], []uint32{0}, false),
				}
			},
			expectedOuts: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 4, utxoOwner, ids.Empty, ids.Empty),
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{test.Keys[0]},
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&utxoOwner,
			},
		},
		"OK: burn, self transfer with change to other address": {
			state:              defaultState,
			totalAmountToSpend: 1,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateUnlocked,
			change:             &changeOwner,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
			},
			expectedIns: func(utxos []*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.InFromUTXO(t, utxos[0], []uint32{0}, false),
				}
			},
			expectedOuts: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 1, utxoOwner, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 3, changeOwner, ids.Empty, ids.Empty),
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{test.Keys[0]},
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&utxoOwner,
			},
		},
		"OK: bond unlocked utxos": {
			state:              defaultState,
			totalAmountToSpend: 9,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateBonded,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{9, 9}, test.AVAXAssetID, 10, utxoOwner, ids.Empty, ids.Empty, true),
			},
			expectedIns: func(utxos []*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.InFromUTXO(t, utxos[0], []uint32{0}, false),
					generate.InFromUTXO(t, utxos[1], []uint32{0}, false),
				}
			},
			expectedOuts: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 9, utxoOwner, ids.Empty, locked.ThisTxID),
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{test.Keys[0]}, {test.Keys[0]},
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&utxoOwner, &utxoOwner,
			},
		},
		"OK: bond deposited utxos": {
			state:              defaultState,
			totalAmountToSpend: 9,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateBonded,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{9, 9}, test.AVAXAssetID, 10, utxoOwner, existingTxID, ids.Empty, true),
			},
			expectedIns: func(utxos []*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.InFromUTXO(t, utxos[0], []uint32{0}, false),
					generate.InFromUTXO(t, utxos[1], []uint32{0}, false),
				}
			},
			expectedOuts: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 4, utxoOwner, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 1, utxoOwner, existingTxID, ids.Empty),
				generate.Out(test.AVAXAssetID, 9, utxoOwner, existingTxID, locked.ThisTxID),
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{test.Keys[0]}, {test.Keys[0]},
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&utxoOwner, &utxoOwner,
			},
		},
		"OK: depositing": {
			state:              defaultState,
			totalAmountToSpend: 9,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateDeposited,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{9, 9}, test.AVAXAssetID, 10, utxoOwner, ids.Empty, ids.Empty, true),
			},
			expectedIns: func(utxos []*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.InFromUTXO(t, utxos[0], []uint32{0}, false),
					generate.InFromUTXO(t, utxos[1], []uint32{0}, false),
				}
			},
			expectedOuts: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 9, utxoOwner, locked.ThisTxID, ids.Empty),
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{test.Keys[0]}, {test.Keys[0]},
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&utxoOwner, &utxoOwner,
			},
		},
		"OK: depositing bonded amount": {
			state:              defaultState,
			totalAmountToSpend: 9,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateDeposited,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{9, 9}, test.AVAXAssetID, 10, utxoOwner, ids.Empty, existingTxID, true),
			},
			expectedIns: func(utxos []*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.InFromUTXO(t, utxos[0], []uint32{0}, false),
					generate.InFromUTXO(t, utxos[1], []uint32{0}, false),
				}
			},
			expectedOuts: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 4, utxoOwner, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 1, utxoOwner, ids.Empty, existingTxID),
				generate.Out(test.AVAXAssetID, 9, utxoOwner, locked.ThisTxID, existingTxID),
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{test.Keys[0]}, {test.Keys[0]},
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&utxoOwner, &utxoOwner,
			},
		},
		"OK: deposit for new owner with change": {
			state:              defaultState,
			totalAmountToSpend: 1,
			totalAmountToBurn:  1,
			appliedLockState:   locked.StateDeposited,
			to:                 &recipientOwner,
			change:             &changeOwner,
			keys:               []*secp256k1.PrivateKey{test.Keys[0]},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{8, 8}, test.AVAXAssetID, 5, utxoOwner, ids.Empty, ids.Empty, true),
			},
			expectedIns: func(utxos []*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.InFromUTXO(t, utxos[0], []uint32{0}, false),
				}
			},
			expectedOuts: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 3, changeOwner, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 1, recipientOwner, locked.ThisTxID, ids.Empty),
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{test.Keys[0]},
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&utxoOwner,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)

			ins, outs, signers, owners, err := defaultCaminoHandler(t).Lock(
				tt.state(t, ctrl, tt.utxos, tt.keys, tt.to, tt.change),
				tt.keys,
				tt.totalAmountToSpend,
				tt.totalAmountToBurn,
				tt.appliedLockState,
				tt.to,
				tt.change,
				0,
			)

			var expectedIns []*avax.TransferableInput
			if tt.expectedIns != nil {
				expectedIns = tt.expectedIns(tt.utxos)
			}

			require.ErrorIs(err, tt.expectedErr)
			require.Equal(expectedIns, ins)
			require.Equal(tt.expectedOuts, outs)
			require.Equal(tt.expectedSigners, signers)
			require.Equal(tt.expectedOwners, owners)
		})
	}
}

func TestVerifyLockUTXOs(t *testing.T) {
	fx := &secp256k1fx.Fx{}
	require.NoError(t, fx.InitializeVM(&secp256k1fx.TestVM{}))
	require.NoError(t, fx.Bootstrapped())
	tx := &dummyUnsignedTx{txs.BaseTx{}}
	tx.SetBytes([]byte{0})

	outputOwners1, cred1 := generateOwnersAndSig(t, test.Keys[0], tx)
	outputOwners2, cred2 := generateOwnersAndSig(t, test.Keys[1], tx)
	msigAddr := ids.ShortID{'m', 's', 'i', 'g'}
	msigOwner := secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{msigAddr},
	}
	invalidCycledMsigAlias := &multisig.AliasWithNonce{Alias: multisig.Alias{
		ID:     msigAddr,
		Owners: &msigOwner,
	}}

	depositTxID1 := ids.ID{0, 1}
	depositTxID2 := ids.ID{0, 2}

	noMsigState := func(c *gomock.Controller) *state.MockChain {
		s := state.NewMockChain(c)
		s.EXPECT().GetMultisigAlias(gomock.Any()).Return(nil, database.ErrNotFound).AnyTimes()
		return s
	}

	// Note that setting [chainTimestamp] also set's the VM's clock.
	// Adjust input/output locktimes accordingly.
	tests := map[string]struct {
		state            func(*gomock.Controller) *state.MockChain
		utxos            []*avax.UTXO
		ins              func(*testing.T, ...*avax.UTXO) []*avax.TransferableInput
		outs             []*avax.TransferableOutput
		creds            []verify.Verifiable
		mintedAmount     uint64
		burnedAmount     uint64
		appliedLockState locked.State
		expectedErr      error
	}{
		"Fail: Invalid appliedLockState": {
			state:            noMsigState,
			appliedLockState: locked.StateDepositedBonded,
			expectedErr:      errInvalidTargetLockState,
		},
		"Fail: Inputs length not equal credentials length": {
			state: noMsigState,
			ins: func(_ *testing.T, utxos ...*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 10, ids.Empty, ids.Empty, []uint32{0}),
				}
			},
			creds:            []verify.Verifiable{},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errInputsCredentialsMismatch,
		},
		"Fail: Inputs length not equal UTXOs length": {
			state: noMsigState,
			ins: func(_ *testing.T, utxos ...*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 10, ids.Empty, ids.Empty, []uint32{0}),
				}
			},
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errInputsUTXOsMismatch,
		},
		"Fail: Invalid credential": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
			},
			ins:              generate.InsFromUTXOs,
			creds:            []verify.Verifiable{(*secp256k1fx.Credential)(nil)},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errBadCredentials,
		},
		"Fail: Invalid utxo assetID": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.OtherAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
			},
			ins:              generate.InsFromUTXOs,
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errAssetIDMismatch,
		},
		"Fail: Invalid input assetID": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
			},
			ins: func(_ *testing.T, utxos ...*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.In(test.OtherAssetID, 10, ids.Empty, ids.Empty, []uint32{0}),
				}
			},
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errAssetIDMismatch,
		},
		"Fail: Invalid output assetID": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.StakeableOut(test.OtherAssetID, 10, 0, outputOwners1),
			},
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errAssetIDMismatch,
		},
		"Fail: Stakable utxo output": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.StakeableUTXO(ids.ID{1}, test.AVAXAssetID, 10, 0, outputOwners1),
			},
			ins:              generate.InsFromUTXOs,
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errWrongUTXOOutType,
		},
		"Fail: Stakable output": {
			state: noMsigState,
			outs: []*avax.TransferableOutput{
				generate.StakeableOut(test.AVAXAssetID, 10, 0, outputOwners1),
			},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errWrongOutType,
		},
		"Fail: Stakable input": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
			},
			ins: func(_ *testing.T, utxos ...*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.StakeableIn(test.AVAXAssetID, 10, 0, []uint32{0}),
				}
			},
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errWrongInType,
		},
		"Fail: UTXO is deposited, appliedLockState is unlocked": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty, true),
			},
			ins:              generate.InsFromUTXOs,
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateUnlocked,
			expectedErr:      errLockedUTXO,
		},
		"Fail: UTXO is deposited, appliedLockState is deposited": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty, true),
			},
			ins:              generate.InsFromUTXOs,
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateDeposited,
			expectedErr:      errLockingLockedUTXO,
		},
		"Fail: input lockIDs don't match utxo lockIDs": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty, true),
			},
			ins: func(_ *testing.T, utxos ...*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 10, depositTxID2, ids.Empty, []uint32{0}),
				}
			},
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateBonded,
			expectedErr:      errLockIDsMismatch,
		},
		"Fail: utxo is locked, but input is not": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty, true),
			},
			ins: func(_ *testing.T, utxos ...*avax.UTXO) []*avax.TransferableInput {
				return []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 10, ids.Empty, ids.Empty, []uint32{0}),
				}
			},
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateBonded,
			expectedErr:      errLockedFundsNotMarkedAsLocked,
		},
		"Fail: bond, produced output has invalid msig owner": {
			state: func(c *gomock.Controller) *state.MockChain {
				s := state.NewMockChain(c)
				s.EXPECT().GetMultisigAlias(outputOwners1.Addrs[0]).Return(nil, database.ErrNotFound)
				s.EXPECT().GetMultisigAlias(msigAddr).Return(invalidCycledMsigAlias, nil).Times(2)
				return s
			},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 2, outputOwners1, ids.Empty, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 1, msigOwner, ids.Empty, locked.ThisTxID),
			},
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateBonded,
			expectedErr:      secp256k1fx.ErrMsigCombination,
		},
		"Fail: bond, but no outs are actually bonded;  produced output has invalid msig owner": {
			state: func(c *gomock.Controller) *state.MockChain {
				s := state.NewMockChain(c)
				s.EXPECT().GetMultisigAlias(outputOwners1.Addrs[0]).Return(nil, database.ErrNotFound)
				s.EXPECT().GetMultisigAlias(msigAddr).Return(invalidCycledMsigAlias, nil).Times(2)
				return s
			},
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 1, outputOwners1, ids.Empty, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 1, msigOwner, ids.Empty, ids.Empty),
			},
			creds:            []verify.Verifiable{cred1},
			appliedLockState: locked.StateBonded,
			expectedErr:      secp256k1fx.ErrMsigCombination,
		},
		"Fail: bond, but no outs are actually bonded; produced + fee > consumed, owner1 has excess as locked": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 5, outputOwners1, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{2}, test.AVAXAssetID, 5, outputOwners1, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{4}, test.AVAXAssetID, 5, outputOwners2, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{5}, test.AVAXAssetID, 5, outputOwners2, depositTxID1, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 4, outputOwners1, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 6, outputOwners1, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 5, outputOwners2, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 5, outputOwners2, depositTxID1, ids.Empty),
			},
			creds:            []verify.Verifiable{cred1, cred1, cred2, cred2},
			appliedLockState: locked.StateBonded,
			expectedErr:      errWrongProducedAmount,
		},
		"Fail: bond, but no outs are actually bonded; produced + fee > consumed, owner2 has excess": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 5, outputOwners1, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{2}, test.AVAXAssetID, 5, outputOwners1, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{4}, test.AVAXAssetID, 5, outputOwners2, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{5}, test.AVAXAssetID, 5, outputOwners2, depositTxID1, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 4, outputOwners1, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 5, outputOwners1, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 6, outputOwners2, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 5, outputOwners2, depositTxID1, ids.Empty),
			},
			burnedAmount:     1,
			creds:            []verify.Verifiable{cred1, cred1, cred2, cred2},
			appliedLockState: locked.StateBonded,
			expectedErr:      errNotBurnedEnough,
		},
		"Fail: bond, produced + fee > consumed, owner1 has excess as locked": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 5, outputOwners1, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{2}, test.AVAXAssetID, 5, outputOwners1, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{4}, test.AVAXAssetID, 5, outputOwners2, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{5}, test.AVAXAssetID, 5, outputOwners2, depositTxID1, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 2, outputOwners1, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 3, outputOwners1, ids.Empty, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 3, outputOwners1, depositTxID1, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 3, outputOwners1, depositTxID1, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 5, outputOwners2, ids.Empty, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 5, outputOwners2, depositTxID1, locked.ThisTxID),
			},
			creds:            []verify.Verifiable{cred1, cred1, cred2, cred2},
			appliedLockState: locked.StateBonded,
			expectedErr:      errWrongProducedAmount,
		},
		"Fail: bond, produced + fee > consumed, owner2 has excess": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 5, outputOwners1, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{2}, test.AVAXAssetID, 5, outputOwners1, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{4}, test.AVAXAssetID, 5, outputOwners2, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{5}, test.AVAXAssetID, 5, outputOwners2, depositTxID1, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 2, outputOwners1, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 2, outputOwners1, ids.Empty, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 2, outputOwners1, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 3, outputOwners1, depositTxID1, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 1, outputOwners2, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 5, outputOwners2, ids.Empty, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 5, outputOwners2, depositTxID1, locked.ThisTxID),
			},
			burnedAmount:     1,
			creds:            []verify.Verifiable{cred1, cred1, cred2, cred2},
			appliedLockState: locked.StateBonded,
			expectedErr:      errNotBurnedEnough,
		},
		"OK: bond, but no outs are actually bonded; produced + fee == consumed": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{2}, test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{3}, test.AVAXAssetID, 10, outputOwners1, depositTxID2, ids.Empty, true),
				generate.UTXO(ids.ID{4}, test.AVAXAssetID, 10, outputOwners2, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{5}, test.AVAXAssetID, 10, outputOwners2, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{6}, test.AVAXAssetID, 10, outputOwners2, depositTxID2, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 9, outputOwners1, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners1, depositTxID2, ids.Empty),
				generate.Out(test.AVAXAssetID, 9, outputOwners2, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, depositTxID2, ids.Empty),
			},
			burnedAmount:     2,
			creds:            []verify.Verifiable{cred1, cred1, cred1, cred2, cred2, cred2},
			appliedLockState: locked.StateBonded,
		},
		"OK: bond, produced + fee == consumed": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{2}, test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{3}, test.AVAXAssetID, 10, outputOwners1, depositTxID2, ids.Empty, true),
				generate.UTXO(ids.ID{4}, test.AVAXAssetID, 10, outputOwners2, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{5}, test.AVAXAssetID, 10, outputOwners2, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{6}, test.AVAXAssetID, 10, outputOwners2, depositTxID2, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 5, outputOwners1, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 4, outputOwners1, ids.Empty, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 6, outputOwners1, depositTxID1, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 4, outputOwners1, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 7, outputOwners1, depositTxID2, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 3, outputOwners1, depositTxID2, ids.Empty),
				generate.Out(test.AVAXAssetID, 9, outputOwners2, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, depositTxID1, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, depositTxID2, ids.Empty),
			},
			burnedAmount:     2,
			creds:            []verify.Verifiable{cred1, cred1, cred1, cred2, cred2, cred2},
			appliedLockState: locked.StateBonded,
		},
		"OK: bond, but no outs are actually bonded; produced + fee == consumed + minted": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{2}, test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{3}, test.AVAXAssetID, 10, outputOwners1, depositTxID2, ids.Empty, true),
				generate.UTXO(ids.ID{4}, test.AVAXAssetID, 10, outputOwners2, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{5}, test.AVAXAssetID, 10, outputOwners2, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{6}, test.AVAXAssetID, 10, outputOwners2, depositTxID2, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 11, outputOwners1, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners1, depositTxID2, ids.Empty),
				generate.Out(test.AVAXAssetID, 11, outputOwners2, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, depositTxID2, ids.Empty),
			},
			mintedAmount:     4,
			burnedAmount:     2,
			creds:            []verify.Verifiable{cred1, cred1, cred1, cred2, cred2, cred2},
			appliedLockState: locked.StateBonded,
			expectedErr:      nil,
		},
		"OK: bond; produced + fee == consumed + minted": {
			state: noMsigState,
			utxos: []*avax.UTXO{
				generate.UTXO(ids.ID{1}, test.AVAXAssetID, 10, outputOwners1, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{2}, test.AVAXAssetID, 10, outputOwners1, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{3}, test.AVAXAssetID, 10, outputOwners1, depositTxID2, ids.Empty, true),
				generate.UTXO(ids.ID{4}, test.AVAXAssetID, 10, outputOwners2, ids.Empty, ids.Empty, true),
				generate.UTXO(ids.ID{5}, test.AVAXAssetID, 10, outputOwners2, depositTxID1, ids.Empty, true),
				generate.UTXO(ids.ID{6}, test.AVAXAssetID, 10, outputOwners2, depositTxID2, ids.Empty, true),
			},
			ins: generate.InsFromUTXOs,
			outs: []*avax.TransferableOutput{
				generate.Out(test.AVAXAssetID, 8, outputOwners1, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 4, outputOwners1, ids.Empty, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 6, outputOwners1, depositTxID1, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 4, outputOwners1, depositTxID1, ids.Empty),
				generate.Out(test.AVAXAssetID, 7, outputOwners1, depositTxID2, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 3, outputOwners1, depositTxID2, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, ids.Empty, ids.Empty),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, depositTxID1, locked.ThisTxID),
				generate.Out(test.AVAXAssetID, 10, outputOwners2, depositTxID2, ids.Empty),
			},
			mintedAmount:     4,
			burnedAmount:     2,
			creds:            []verify.Verifiable{cred1, cred1, cred1, cred2, cred2, cred2},
			appliedLockState: locked.StateBonded,
			expectedErr:      nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			testHandler := defaultCaminoHandler(t)

			var ins []*avax.TransferableInput
			if tt.ins != nil {
				ins = tt.ins(t, tt.utxos...)
			}

			err := testHandler.VerifyLockUTXOs(
				tt.state(ctrl),
				tx,
				tt.utxos,
				ins,
				tt.outs,
				tt.creds,
				tt.mintedAmount,
				tt.burnedAmount,
				test.AVAXAssetID,
				tt.appliedLockState,
			)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestGetDepositUnlockableAmounts(t *testing.T) {
	addr0 := ids.GenerateTestShortID()
	addresses := set.NewSet[ids.ShortID](0)
	addresses.Add(addr0)

	depositTxSet := set.NewSet[ids.ID](0)
	testID := ids.GenerateTestID()
	depositTxSet.Add(testID)

	tx := &dummyUnsignedTx{txs.BaseTx{}}
	tx.SetBytes([]byte{0})
	outputOwners, _ := generateOwnersAndSig(t, test.Keys[0], tx)
	now := time.Now()
	depositedAmount := uint64(1000)
	type args struct {
		state        func(*gomock.Controller) state.Chain
		depositTxIDs set.Set[ids.ID]
		currentTime  uint64
		addresses    set.Set[ids.ShortID]
	}
	tests := map[string]struct {
		args args
		want map[ids.ID]uint64
		err  error
	}{
		"Success retrieval of all unlockable amounts": {
			args: args{
				state: func(ctrl *gomock.Controller) state.Chain {
					nowMinus20m := uint64(now.Add(-20 * time.Minute).Unix())
					s := state.NewMockChain(ctrl)
					deposit1 := deposit.Deposit{
						DepositOfferID: testID,
						Start:          nowMinus20m,
						Duration:       uint32((20 * time.Minute).Seconds()),
						Amount:         depositedAmount,
					}
					s.EXPECT().GetDeposit(testID).Return(&deposit1, nil)
					s.EXPECT().GetDepositOffer(testID).Return(&deposit.Offer{
						Start:                nowMinus20m,
						UnlockPeriodDuration: uint32((20 * time.Minute).Seconds()),
					}, nil)
					return s
				},
				depositTxIDs: depositTxSet,
				currentTime:  uint64(now.Unix()),
				addresses:    outputOwners.AddressesSet(),
			},
			want: map[ids.ID]uint64{testID: depositedAmount},
		},
		"Success retrieval of 50% unlockable amounts": {
			args: args{
				state: func(ctrl *gomock.Controller) state.Chain {
					nowMinus20m := uint64(now.Add(-20 * time.Minute).Unix())
					s := state.NewMockChain(ctrl)
					deposit1 := deposit.Deposit{
						DepositOfferID: testID,
						Start:          nowMinus20m,
						Duration:       uint32((40 * time.Minute).Seconds()),
						Amount:         depositedAmount,
					}
					s.EXPECT().GetDeposit(testID).Return(&deposit1, nil)
					s.EXPECT().GetDepositOffer(testID).Return(&deposit.Offer{
						Start:                nowMinus20m,
						UnlockPeriodDuration: uint32((40 * time.Minute).Seconds()),
					}, nil)
					return s
				},
				depositTxIDs: depositTxSet,
				currentTime:  uint64(now.Unix()),
				addresses:    outputOwners.AddressesSet(),
			},
			want: map[ids.ID]uint64{testID: depositedAmount / 2},
		},
		"Failed to get deposit offer": {
			args: args{
				state: func(ctrl *gomock.Controller) state.Chain {
					s := state.NewMockChain(ctrl)
					s.EXPECT().GetDeposit(testID).Return(&deposit.Deposit{DepositOfferID: testID}, nil)
					s.EXPECT().GetDepositOffer(testID).Return(nil, database.ErrNotFound)
					return s
				},
				depositTxIDs: depositTxSet,
				currentTime:  uint64(now.Unix()),
			},
			err: database.ErrNotFound,
		},
		"Failed to get deposit": {
			args: args{
				state: func(ctrl *gomock.Controller) state.Chain {
					s := state.NewMockChain(ctrl)
					s.EXPECT().GetDeposit(gomock.Any()).Return(nil, errors.New("some_error"))
					return s
				},
				depositTxIDs: depositTxSet,
				currentTime:  uint64(now.Unix()),
			},
			err: fmt.Errorf("%w: %s", errFailToGetDeposit, "some_error"),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			got, err := getDepositUnlockableAmounts(test.args.state(ctrl), test.args.depositTxIDs, test.args.currentTime)

			if test.err != nil {
				require.ErrorContains(t, err, test.err.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestUnlockDeposit(t *testing.T) {
	depositOwnerKey1, depositOwnerAddr1, depositOwner1 := generate.KeyAndOwner(t, test.Keys[0])
	depositOwnerKey2, depositOwnerAddr2, depositOwner2 := generate.KeyAndOwner(t, test.Keys[1])

	newOwner := &secp256k1fx.OutputOwners{Threshold: 1, Addrs: []ids.ShortID{{1}}}

	depositTxID1 := ids.ID{2}
	depositTxID2 := ids.ID{3}
	bondTxID := ids.ID{4}
	offerID1 := ids.ID{5}
	offerID2 := ids.ID{6}

	depositAmount1 := uint64(2000)
	depositAmount2 := uint64(5000)
	deposit1UTXO1Amount := depositAmount1 / 4
	deposit1UTXO2Amount := depositAmount1 / 4
	deposit1UTXO3Amount := depositAmount1 / 2
	deposit2UTXO1Amount := depositAmount2 / 4
	deposit2UTXO2Amount := depositAmount2 / 4
	deposit2BondedUTXO3Amount := depositAmount2 / 4
	deposit2BondedUTXO4Amount := depositAmount2 / 4

	deposit1UTXO1 := generate.UTXO(ids.ID{7}, test.AVAXAssetID, deposit1UTXO1Amount, depositOwner1, depositTxID1, ids.Empty, true)       // 25% of depositAmount1
	deposit1UTXO2 := generate.UTXO(ids.ID{8}, test.AVAXAssetID, deposit1UTXO2Amount, depositOwner1, depositTxID1, ids.Empty, true)       // 25% of depositAmount1
	deposit1UTXO3 := generate.UTXO(ids.ID{9}, test.AVAXAssetID, deposit1UTXO3Amount, depositOwner1, depositTxID1, ids.Empty, true)       // 50% of depositAmount1
	deposit1BondedUTXO3 := generate.UTXO(ids.ID{10}, test.AVAXAssetID, deposit1UTXO3Amount, depositOwner1, depositTxID1, bondTxID, true) // 50% of depositAmount1, alternate utxo
	deposit1UTXOs := []*avax.UTXO{deposit1UTXO1, deposit1UTXO2, deposit1UTXO3}
	deposit1UTXOsWithBond := []*avax.UTXO{deposit1UTXO1, deposit1UTXO2, deposit1BondedUTXO3} // alternate utxo set

	deposit2UTXO1 := generate.UTXO(ids.ID{11}, test.AVAXAssetID, deposit2UTXO1Amount, depositOwner2, depositTxID2, ids.Empty, true)            // 25% of depositAmount2
	deposit2UTXO2 := generate.UTXO(ids.ID{12}, test.AVAXAssetID, deposit2UTXO2Amount, depositOwner2, depositTxID2, ids.Empty, true)            // 25% of depositAmount2
	deposit2BondedUTXO3 := generate.UTXO(ids.ID{13}, test.AVAXAssetID, deposit2BondedUTXO3Amount, depositOwner2, depositTxID2, bondTxID, true) // 25% of depositAmount2
	deposit2BondedUTXO4 := generate.UTXO(ids.ID{14}, test.AVAXAssetID, deposit2BondedUTXO4Amount, depositOwner2, depositTxID2, bondTxID, true) // 25% of depositAmount2
	deposit2UTXOs := []*avax.UTXO{deposit2UTXO1, deposit2UTXO2, deposit2BondedUTXO3, deposit2BondedUTXO4}

	depositUnlockDuration1 := uint32(100)
	depositUnlockDuration2 := uint32(200)

	offer1 := &deposit.Offer{UnlockPeriodDuration: depositUnlockDuration1}
	offer2 := &deposit.Offer{UnlockPeriodDuration: depositUnlockDuration2}

	deposit1 := func(now time.Time, unlockPercent uint32) *deposit.Deposit {
		return &deposit.Deposit{
			DepositOfferID: offerID1,
			Start:          uint64(now.Add(-time.Duration(depositUnlockDuration1*unlockPercent/100) * time.Second).Unix()),
			Duration:       depositUnlockDuration1,
			Amount:         depositAmount1,
		}
	}

	deposit2 := func(now time.Time, unlockPercent uint32) *deposit.Deposit {
		return &deposit.Deposit{
			DepositOfferID: offerID2,
			Start:          uint64(now.Add(-time.Duration(depositUnlockDuration2*unlockPercent/100) * time.Second).Unix()),
			Duration:       depositUnlockDuration2,
			Amount:         depositAmount2,
		}
	}

	// verifying that deposit funcs are correct
	now := time.Now()
	require.Equal(t, depositAmount1/2, deposit1(now, 50).UnlockableAmountByTime(offer1, now))
	require.Equal(t, depositAmount2/2, deposit2(now, 50).UnlockableAmountByTime(offer2, now))

	tests := map[string]struct {
		state              func(*gomock.Controller, time.Time, []*deposit.Deposit) state.Chain
		keys               []*secp256k1.PrivateKey
		amountsToUndeposit func(time.Time, []*deposit.Deposit) map[ids.ID]uint64
		undepositToOwner   *secp256k1fx.OutputOwners
		deposits           func(time.Time) []*deposit.Deposit
		expectedIns        []*avax.TransferableInput
		expectedOuts       func(time.Time, []*deposit.Deposit) []*avax.TransferableOutput
		expectedSigners    [][]*secp256k1.PrivateKey
		expectedOwners     []*secp256k1fx.OutputOwners
		expectedErr        error
	}{
		"Syntactically invalid to-owner": {
			undepositToOwner: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{{100}},
				Threshold: 2,
			},
			expectedErr: errInvalidToOwner,
		},
		"Nested multisig to-owner": {
			state: func(ctrl *gomock.Controller, _ time.Time, _ []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				expect.VerifyMultisigOwner(
					t, s, newOwner,
					[]ids.ShortID{newOwner.Addrs[0], {100}},
					[]*multisig.AliasWithNonce{
						{Alias: multisig.Alias{
							Owners: &secp256k1fx.OutputOwners{Addrs: []ids.ShortID{{100}}},
						}},
						{Alias: multisig.Alias{Owners: &secp256k1fx.OutputOwners{}}},
					},
					false,
				)
				return s
			},
			undepositToOwner: newOwner,
			expectedErr:      errInvalidToOwner,
		},
		"Composite multisig to-owner": {
			state: func(ctrl *gomock.Controller, _ time.Time, _ []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				expect.VerifyMultisigOwner(
					t, s, newOwner,
					[]ids.ShortID{{100}},
					[]*multisig.AliasWithNonce{{Alias: multisig.Alias{Owners: &secp256k1fx.OutputOwners{}}}},
					false,
				)
				return s
			},
			undepositToOwner: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{{100}, {101}},
				Threshold: 1,
			},
			expectedErr: errInvalidToOwner,
		},
		"Non-existing deposit": {
			state: func(ctrl *gomock.Controller, _ time.Time, _ []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				s.EXPECT().GetDeposit(depositTxID1).Return(nil, database.ErrNotFound)
				return s
			},
			amountsToUndeposit: func(now time.Time, _ []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{depositTxID1: 0}
			},
			expectedErr: errFailToGetDeposit,
		},
		"Not enough unlockable": {
			state: func(ctrl *gomock.Controller, now time.Time, deposits []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				expect.GetDepositUnlockableAmounts(t, s, []ids.ID{depositTxID1}, deposits, []*deposit.Offer{offer1})
				return s
			},
			amountsToUndeposit: func(now time.Time, deposits []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{depositTxID1: deposits[0].UnlockableAmountByTime(offer1, now) + 1} // 35% unlockable + 1
			},
			deposits:    func(now time.Time) []*deposit.Deposit { return []*deposit.Deposit{deposit1(now, 35)} }, // 35% unlockable
			expectedErr: errInsufficientUnlockable,
		},
		"OK: all available unlockable (35%), use 2 out of 3 utxos": {
			state: func(ctrl *gomock.Controller, now time.Time, deposits []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				expect.GetDepositUnlockableAmounts(t, s, []ids.ID{depositTxID1}, deposits, []*deposit.Offer{offer1})
				s.EXPECT().LockedUTXOs(
					set.Set[ids.ID]{depositTxID1: struct{}{}},
					set.Set[ids.ShortID]{depositOwnerAddr1: struct{}{}},
					locked.StateDeposited,
				).Return(deposit1UTXOs, nil)
				s.EXPECT().GetMultisigAlias(depositOwnerAddr1).Return(nil, database.ErrNotFound).Times(2) // per number of used utxos
				return s
			},
			keys: []*secp256k1.PrivateKey{depositOwnerKey1},
			amountsToUndeposit: func(now time.Time, deposits []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{depositTxID1: deposits[0].UnlockableAmountByTime(offer1, now)} // 35% unlockable
			},
			deposits:    func(now time.Time) []*deposit.Deposit { return []*deposit.Deposit{deposit1(now, 35)} }, // 35% unlockable
			expectedIns: generate.InsFromUTXOs(t, deposit1UTXO1, deposit1UTXO2),
			expectedOuts: func(now time.Time, deposits []*deposit.Deposit) []*avax.TransferableOutput {
				unlockedAmount := deposits[0].UnlockableAmountByTime(offer1, now)             // 35% unlockable
				remainingAmount := deposit1UTXO1Amount + deposit1UTXO2Amount - unlockedAmount // utxo3 is not used
				return []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, unlockedAmount, depositOwner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, remainingAmount, depositOwner1, depositTxID1, ids.Empty),
				}
			},
			expectedSigners: [][]*secp256k1.PrivateKey{{depositOwnerKey1}, {depositOwnerKey1}},
			expectedOwners:  []*secp256k1fx.OutputOwners{&depositOwner1, &depositOwner1},
		},
		"OK: almost all available unlockable (80%-1), use 3 out of 3 utxos": {
			state: func(ctrl *gomock.Controller, now time.Time, deposits []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				expect.GetDepositUnlockableAmounts(t, s, []ids.ID{depositTxID1}, deposits, []*deposit.Offer{offer1})
				s.EXPECT().LockedUTXOs(
					set.Set[ids.ID]{depositTxID1: struct{}{}},
					set.Set[ids.ShortID]{depositOwnerAddr1: struct{}{}},
					locked.StateDeposited,
				).Return(deposit1UTXOs, nil)
				s.EXPECT().GetMultisigAlias(depositOwnerAddr1).Return(nil, database.ErrNotFound).Times(3) // per number of used utxos
				return s
			},
			keys: []*secp256k1.PrivateKey{depositOwnerKey1},
			amountsToUndeposit: func(now time.Time, deposits []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{depositTxID1: deposits[0].UnlockableAmountByTime(offer1, now) - 1} // 80% unlockable - 1
			},
			deposits:    func(now time.Time) []*deposit.Deposit { return []*deposit.Deposit{deposit1(now, 80)} }, // 80% unlockable
			expectedIns: generate.InsFromUTXOs(t, deposit1UTXOs...),
			expectedOuts: func(now time.Time, deposits []*deposit.Deposit) []*avax.TransferableOutput {
				unlockedAmount := deposits[0].UnlockableAmountByTime(offer1, now) - 1 // 80% unlockable - 1
				return []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, unlockedAmount, depositOwner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, depositAmount1-unlockedAmount, depositOwner1, depositTxID1, ids.Empty),
				}
			},
			expectedSigners: [][]*secp256k1.PrivateKey{{depositOwnerKey1}, {depositOwnerKey1}, {depositOwnerKey1}},
			expectedOwners:  []*secp256k1fx.OutputOwners{&depositOwner1, &depositOwner1, &depositOwner1},
		},
		"OK: almost all available unlockable (80%-1), use 3 out of 3 utxos, new owner": {
			state: func(ctrl *gomock.Controller, now time.Time, deposits []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				s.EXPECT().GetMultisigAlias(newOwner.Addrs[0]).Return(nil, database.ErrNotFound)
				expect.GetDepositUnlockableAmounts(t, s, []ids.ID{depositTxID1}, deposits, []*deposit.Offer{offer1})
				s.EXPECT().LockedUTXOs(
					set.Set[ids.ID]{depositTxID1: struct{}{}},
					set.Set[ids.ShortID]{depositOwnerAddr1: struct{}{}},
					locked.StateDeposited,
				).Return(deposit1UTXOs, nil)
				s.EXPECT().GetMultisigAlias(depositOwnerAddr1).Return(nil, database.ErrNotFound).Times(3) // per number of used utxos
				return s
			},
			keys: []*secp256k1.PrivateKey{depositOwnerKey1},
			amountsToUndeposit: func(now time.Time, deposits []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{depositTxID1: deposits[0].UnlockableAmountByTime(offer1, now) - 1} // 80% unlockable - 1
			},
			undepositToOwner: newOwner,
			deposits:         func(now time.Time) []*deposit.Deposit { return []*deposit.Deposit{deposit1(now, 80)} }, // 80% unlockable
			expectedIns:      generate.InsFromUTXOs(t, deposit1UTXOs...),
			expectedOuts: func(now time.Time, deposits []*deposit.Deposit) []*avax.TransferableOutput {
				unlockedAmount := deposits[0].UnlockableAmountByTime(offer1, now) - 1 // 80% unlockable - 1
				return []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, unlockedAmount, *newOwner, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, depositAmount1-unlockedAmount, depositOwner1, depositTxID1, ids.Empty),
				}
			},
			expectedSigners: [][]*secp256k1.PrivateKey{{depositOwnerKey1}, {depositOwnerKey1}, {depositOwnerKey1}},
			expectedOwners:  []*secp256k1fx.OutputOwners{&depositOwner1, &depositOwner1, &depositOwner1},
		},
		"OK: 1 nCAM, use 1 out of 3 utxo": {
			state: func(ctrl *gomock.Controller, now time.Time, deposits []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				expect.GetDepositUnlockableAmounts(t, s, []ids.ID{depositTxID1}, deposits, []*deposit.Offer{offer1})
				s.EXPECT().LockedUTXOs(
					set.Set[ids.ID]{depositTxID1: struct{}{}},
					set.Set[ids.ShortID]{depositOwnerAddr1: struct{}{}},
					locked.StateDeposited,
				).Return(deposit1UTXOs, nil)
				s.EXPECT().GetMultisigAlias(depositOwnerAddr1).Return(nil, database.ErrNotFound) // per number of used utxos
				return s
			},
			keys: []*secp256k1.PrivateKey{depositOwnerKey1},
			amountsToUndeposit: func(now time.Time, deposits []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{depositTxID1: 1}
			},
			deposits:    func(now time.Time) []*deposit.Deposit { return []*deposit.Deposit{deposit1(now, 35)} }, // 35% unlockable
			expectedIns: generate.InsFromUTXOs(t, deposit1UTXO1),
			expectedOuts: func(now time.Time, deposits []*deposit.Deposit) []*avax.TransferableOutput {
				return []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 1, depositOwner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, deposit1UTXO1Amount-1, depositOwner1, depositTxID1, ids.Empty),
				}
			},
			expectedSigners: [][]*secp256k1.PrivateKey{{depositOwnerKey1}},
			expectedOwners:  []*secp256k1fx.OutputOwners{&depositOwner1},
		},
		"OK: all from 1 utxo using all unlockable (25%)": {
			state: func(ctrl *gomock.Controller, now time.Time, deposits []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				expect.GetDepositUnlockableAmounts(t, s, []ids.ID{depositTxID1}, deposits, []*deposit.Offer{offer1})
				s.EXPECT().LockedUTXOs(
					set.Set[ids.ID]{depositTxID1: struct{}{}},
					set.Set[ids.ShortID]{depositOwnerAddr1: struct{}{}},
					locked.StateDeposited,
				).Return([]*avax.UTXO{deposit1UTXO1}, nil)
				s.EXPECT().GetMultisigAlias(depositOwnerAddr1).Return(nil, database.ErrNotFound) // per number of used utxos
				return s
			},
			keys: []*secp256k1.PrivateKey{depositOwnerKey1},
			amountsToUndeposit: func(now time.Time, deposits []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{depositTxID1: deposits[0].UnlockableAmountByTime(offer1, now)} // 25% unlockable
			},
			deposits:    func(now time.Time) []*deposit.Deposit { return []*deposit.Deposit{deposit1(now, 25)} }, // 25% unlockable
			expectedIns: generate.InsFromUTXOs(t, deposit1UTXO1),
			expectedOuts: func(now time.Time, deposits []*deposit.Deposit) []*avax.TransferableOutput {
				unlockedAmount := deposits[0].UnlockableAmountByTime(offer1, now) // 25% unlockable
				return []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, unlockedAmount, depositOwner1, ids.Empty, ids.Empty),
				}
			},
			expectedSigners: [][]*secp256k1.PrivateKey{{depositOwnerKey1}},
			expectedOwners:  []*secp256k1fx.OutputOwners{&depositOwner1},
		},
		"OK: 50%-1 and 2 out of 3 utxos from deposit1, 99%-1 from deposit2, 4 out of 4 utxos, half bond": {
			state: func(ctrl *gomock.Controller, now time.Time, deposits []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				expect.GetDepositUnlockableAmounts(t, s, []ids.ID{depositTxID1, depositTxID2}, deposits, []*deposit.Offer{offer1, offer2})
				s.EXPECT().LockedUTXOs(
					set.Set[ids.ID]{depositTxID1: struct{}{}, depositTxID2: struct{}{}},
					set.Set[ids.ShortID]{depositOwnerAddr1: struct{}{}, depositOwnerAddr2: struct{}{}},
					locked.StateDeposited,
				).Return(append(deposit1UTXOs, deposit2UTXOs...), nil)
				s.EXPECT().GetMultisigAlias(depositOwnerAddr1).Return(nil, database.ErrNotFound).Times(2) // per number of used utxos
				s.EXPECT().GetMultisigAlias(depositOwnerAddr2).Return(nil, database.ErrNotFound).Times(4) // per number of used utxos
				return s
			},
			keys: []*secp256k1.PrivateKey{depositOwnerKey1, depositOwnerKey2},
			amountsToUndeposit: func(now time.Time, deposits []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{
					depositTxID1: deposits[0].UnlockableAmountByTime(offer1, now) - 1, // 50% unlockable - 1
					depositTxID2: deposits[1].UnlockableAmountByTime(offer2, now) - 1, // 99% unlockable - 1
				}
			},
			deposits: func(now time.Time) []*deposit.Deposit {
				return []*deposit.Deposit{deposit1(now, 50), deposit2(now, 99)} // 50% and 99% unlockable
			},
			expectedIns: generate.InsFromUTXOs(t, append([]*avax.UTXO{deposit1UTXO1, deposit1UTXO2}, deposit2UTXOs...)...), // all 6 utxos except deposit1UTXO3
			expectedOuts: func(now time.Time, deposits []*deposit.Deposit) []*avax.TransferableOutput {
				deposit1UnlockedAmount := deposits[0].UnlockableAmountByTime(offer1, now) - 1          // 50% unlockable - 1
				remainingAmount1 := deposit1UTXO1Amount + deposit1UTXO2Amount - deposit1UnlockedAmount // utxo3 is not used

				deposit2UnlockedAmount := deposits[1].UnlockableAmountByTime(offer2, now) - 1 // 99% unlockable - 1
				unlockedAmount2NotBonded := deposit2UTXO1Amount + deposit2UTXO2Amount         // 50% unlockable, all not bonded utxos
				require.GreaterOrEqual(t, deposit2UnlockedAmount, unlockedAmount2NotBonded)
				unlockedAmount2Bonded := deposit2UnlockedAmount - unlockedAmount2NotBonded // 49% unlockable - 1, all bonded utxos
				remainingAmount2 := depositAmount2 - deposit2UnlockedAmount                // 1% deposited, bonded
				return []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, deposit1UnlockedAmount, depositOwner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, unlockedAmount2NotBonded, depositOwner2, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, unlockedAmount2Bonded, depositOwner2, ids.Empty, bondTxID),
					generate.Out(test.AVAXAssetID, remainingAmount1, depositOwner1, depositTxID1, ids.Empty),
					generate.Out(test.AVAXAssetID, remainingAmount2, depositOwner2, depositTxID2, bondTxID),
				}
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{depositOwnerKey1}, {depositOwnerKey1}, // deposit1 utxos
				{depositOwnerKey2}, {depositOwnerKey2}, {depositOwnerKey2}, {depositOwnerKey2}, // deposit2 utxos
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&depositOwner1, &depositOwner1,
				&depositOwner2, &depositOwner2, &depositOwner2, &depositOwner2,
			},
		},
		"OK: 99%-1 and 2 out of 3 utxos from deposit1, 99%-1 from deposit2, 4 out of 4 utxos, half bond, new owner": {
			state: func(ctrl *gomock.Controller, now time.Time, deposits []*deposit.Deposit) state.Chain {
				s := state.NewMockChain(ctrl)
				s.EXPECT().GetMultisigAlias(newOwner.Addrs[0]).Return(nil, database.ErrNotFound)
				expect.GetDepositUnlockableAmounts(t, s, []ids.ID{depositTxID1, depositTxID2}, deposits, []*deposit.Offer{offer1, offer2})
				s.EXPECT().LockedUTXOs(
					set.Set[ids.ID]{depositTxID1: struct{}{}, depositTxID2: struct{}{}},
					set.Set[ids.ShortID]{depositOwnerAddr1: struct{}{}, depositOwnerAddr2: struct{}{}},
					locked.StateDeposited,
				).Return(append(deposit1UTXOsWithBond, deposit2UTXOs...), nil)
				s.EXPECT().GetMultisigAlias(depositOwnerAddr1).Return(nil, database.ErrNotFound).Times(3) // per number of used utxos
				s.EXPECT().GetMultisigAlias(depositOwnerAddr2).Return(nil, database.ErrNotFound).Times(4) // per number of used utxos
				return s
			},
			keys: []*secp256k1.PrivateKey{depositOwnerKey1, depositOwnerKey2},
			amountsToUndeposit: func(now time.Time, deposits []*deposit.Deposit) map[ids.ID]uint64 {
				return map[ids.ID]uint64{
					depositTxID1: deposits[0].UnlockableAmountByTime(offer1, now) - 1, // 99% unlockable - 1
					depositTxID2: deposits[1].UnlockableAmountByTime(offer2, now) - 1, // 99% unlockable - 1
				}
			},
			undepositToOwner: newOwner,
			deposits: func(now time.Time) []*deposit.Deposit {
				return []*deposit.Deposit{deposit1(now, 99), deposit2(now, 99)} // 99% and 99% unlockable
			},
			expectedIns: generate.InsFromUTXOs(t, append(deposit1UTXOsWithBond, deposit2UTXOs...)...), // all 7 utxos
			expectedOuts: func(now time.Time, deposits []*deposit.Deposit) []*avax.TransferableOutput {
				deposit1UnlockedAmount := deposits[0].UnlockableAmountByTime(offer1, now) - 1 // 99% unlockable - 1
				unlockedAmount1NotBonded := deposit1UTXO1Amount + deposit1UTXO2Amount         // 50% unlockable, all not bonded utxos
				require.GreaterOrEqual(t, deposit1UnlockedAmount, unlockedAmount1NotBonded)
				unlockedAmount1Bonded := deposit1UnlockedAmount - unlockedAmount1NotBonded // 49% unlockable - 1, all bonded utxos
				remainingAmount1 := depositAmount1 - deposit1UnlockedAmount                // 1% deposited, bonded

				deposit2UnlockedAmount := deposits[1].UnlockableAmountByTime(offer2, now) - 1 // 99% unlockable - 1
				unlockedAmount2NotBonded := deposit2UTXO1Amount + deposit2UTXO2Amount         // 50% unlockable, all not bonded utxos
				require.GreaterOrEqual(t, deposit2UnlockedAmount, unlockedAmount2NotBonded)
				unlockedAmount2Bonded := deposit2UnlockedAmount - unlockedAmount2NotBonded // 49% unlockable - 1, all bonded utxos
				remainingAmount2 := depositAmount2 - deposit2UnlockedAmount                // 1% deposited, bonded
				return []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, unlockedAmount1NotBonded+unlockedAmount2NotBonded, *newOwner, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, unlockedAmount1Bonded, depositOwner1, ids.Empty, bondTxID),
					generate.Out(test.AVAXAssetID, unlockedAmount2Bonded, depositOwner2, ids.Empty, bondTxID),
					generate.Out(test.AVAXAssetID, remainingAmount1, depositOwner1, depositTxID1, bondTxID),
					generate.Out(test.AVAXAssetID, remainingAmount2, depositOwner2, depositTxID2, bondTxID),
				}
			},
			expectedSigners: [][]*secp256k1.PrivateKey{
				{depositOwnerKey1}, {depositOwnerKey1}, {depositOwnerKey1}, // deposit1 utxos
				{depositOwnerKey2}, {depositOwnerKey2}, {depositOwnerKey2}, {depositOwnerKey2}, // deposit2 utxos
			},
			expectedOwners: []*secp256k1fx.OutputOwners{
				&depositOwner1, &depositOwner1, &depositOwner1,
				&depositOwner2, &depositOwner2, &depositOwner2, &depositOwner2,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			handler := defaultCaminoHandler(t)
			now := handler.clk.Time()

			var deposits []*deposit.Deposit
			if tt.deposits != nil {
				deposits = tt.deposits(now)
			}

			var expectedOuts []*avax.TransferableOutput
			if tt.expectedOuts != nil {
				expectedOuts = tt.expectedOuts(now, deposits)
			}

			var amountsToUndeposit map[ids.ID]uint64
			if tt.amountsToUndeposit != nil {
				amountsToUndeposit = tt.amountsToUndeposit(now, deposits)
			}

			var state state.Chain
			if tt.state != nil {
				state = tt.state(gomock.NewController(t), now, deposits)
			}

			require.True(t, utils.IsSortedAndUniqueSortable(tt.expectedIns))
			require.True(t, avax.IsSortedTransferableOutputs(expectedOuts, txs.Codec))

			ins, outs, signers, owners, err := handler.UnlockDeposit(
				state,
				tt.keys,
				amountsToUndeposit,
				tt.undepositToOwner,
			)

			require.ErrorIs(t, err, tt.expectedErr)
			require.Equal(t, tt.expectedIns, ins)
			require.Equal(t, expectedOuts, outs)
			require.Equal(t, tt.expectedSigners, signers)
			require.Equal(t, tt.expectedOwners, owners)
		})
	}
}

func TestVerifyUnlockDepositedUTXOsBeforeCairo(t *testing.T) {
	tx := &dummyUnsignedTx{txs.BaseTx{}}
	tx.SetBytes([]byte{0})
	owner1, cred1 := generateOwnersAndSig(t, test.Keys[0], tx)
	owner2, cred2 := generateOwnersAndSig(t, test.Keys[1], tx)
	depositTxID1 := ids.ID{1}
	depositTxID2 := ids.ID{2}
	bondTxID1 := ids.ID{3}
	bondTxID2 := ids.ID{4}

	noMsigState := func(ctrl *gomock.Controller) *state.MockChain {
		s := state.NewMockChain(ctrl)
		s.EXPECT().GetMultisigAlias(gomock.Any()).Return(nil, database.ErrNotFound).AnyTimes()
		return s
	}

	type args struct {
		tx           txs.UnsignedTx
		utxos        []*avax.UTXO
		ins          []*avax.TransferableInput
		outs         []*avax.TransferableOutput
		creds        []verify.Verifiable
		burnedAmount uint64
		assetID      ids.ID
		verifyCreds  bool
	}
	tests := map[string]struct {
		handlerState func(ctrl *gomock.Controller) *state.MockChain
		args         args
		expectedErr  error
	}{
		"Number of inputs-utxos mismatch": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{{}, {}},
				ins:   []*avax.TransferableInput{{}},
			},
			expectedErr: errInputsUTXOsMismatch,
		},
		"Verify creds, number of inputs-credentials mismatch": {
			handlerState: noMsigState,
			args: args{
				utxos:       []*avax.UTXO{{}},
				ins:         []*avax.TransferableInput{{}},
				creds:       []verify.Verifiable{cred1, cred1},
				verifyCreds: true,
			},
			expectedErr: errInputsCredentialsMismatch,
		},
		"Verify creds, bad credentials": {
			handlerState: noMsigState,
			args: args{
				utxos:       []*avax.UTXO{{}},
				ins:         []*avax.TransferableInput{{}},
				creds:       []verify.Verifiable{(*secp256k1fx.Credential)(nil)},
				verifyCreds: true,
			},
			expectedErr: errBadCredentials,
		},
		"UTXO AssetID mismatch": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.OtherAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errAssetIDMismatch,
		},
		"Input AssetID mismatch": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.OtherAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errAssetIDMismatch,
		},
		"UTXO locked, but not deposited (e.g. just bonded)": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, ids.Empty, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, ids.Empty, bondTxID1, []uint32{0}),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errUnlockingUnlockedUTXO,
		},
		"Input and utxo lock IDs mismatch: different depositTxIDs": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(depositTxID1, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID2, ids.Empty, []uint32{0}),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errLockIDsMismatch,
		},
		"Input and utxo lock IDs mismatch: different bondTxIDs": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(depositTxID1, test.AVAXAssetID, 1, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, bondTxID2, []uint32{0}),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errLockIDsMismatch,
		},
		"Input and utxo lock IDs mismatch: utxo is locked, but input isn't locked": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, ids.Empty, ids.Empty, []uint32{0}),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errLockedFundsNotMarkedAsLocked,
		},
		"Input and utxo lock IDs mismatch: utxo isn't locked, but input is locked": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(depositTxID1, test.AVAXAssetID, 1, owner1, ids.Empty, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errLockIDsMismatch,
		},
		"Ignoring creds, input and utxo amount mismatch, utxo is deposited": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1+1, depositTxID1, ids.Empty, []uint32{0}),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errUTXOOutTypeOrAmtMismatch,
		},
		"Wrong credentials": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 5, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 5, depositTxID1, ids.Empty, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 5, owner1, ids.Empty, ids.Empty),
				},
				creds:       []verify.Verifiable{cred2},
				assetID:     test.AVAXAssetID,
				verifyCreds: true,
			},
			expectedErr: errCantSpend,
		},
		"Not burned enough": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, ids.Empty, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, ids.Empty, ids.Empty, []uint32{0}),
				},
				burnedAmount: 2,
				assetID:      test.AVAXAssetID,
			},
			expectedErr: errNotBurnedEnough,
		},
		"Produced unlocked more, than consumed deposited or unlocked": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 2, owner1, ids.Empty, ids.Empty),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errWrongProducedAmount,
		},
		"Consumed-produced amount mismatch (per lockIDs)": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 1, owner1, ids.Empty, bondTxID1),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errWrongProducedAmount,
		},
		"Consumed-produced amount mismatch (per ownerID, e.g. produced unlocked for different owner)": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 1, owner2, ids.Empty, ids.Empty),
				},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errWrongProducedAmount,
		},
		"OK": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 5, owner1, depositTxID1, ids.Empty, true),
					generate.UTXO(ids.ID{101}, test.AVAXAssetID, 11, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 5, depositTxID1, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 11, depositTxID1, bondTxID1, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 5, owner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, 11, owner1, ids.Empty, bondTxID1),
				},
				assetID: test.AVAXAssetID,
			},
		},
		"OK: with burn": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 2, owner1, ids.Empty, ids.Empty, true),
					generate.UTXO(ids.ID{102}, test.AVAXAssetID, 5, owner1, depositTxID1, ids.Empty, true),
					generate.UTXO(ids.ID{103}, test.AVAXAssetID, 11, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 2, ids.Empty, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 5, depositTxID1, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 11, depositTxID1, bondTxID1, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 6, owner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, 11, owner1, ids.Empty, bondTxID1),
				},
				burnedAmount: 1,
				assetID:      test.AVAXAssetID,
			},
		},
		"OK: verify creds": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 5, owner1, depositTxID1, ids.Empty, true),
					generate.UTXO(ids.ID{101}, test.AVAXAssetID, 11, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 5, depositTxID1, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 11, depositTxID1, bondTxID1, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 5, owner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, 11, owner1, ids.Empty, bondTxID1),
				},
				creds:       []verify.Verifiable{cred1, cred1},
				assetID:     test.AVAXAssetID,
				verifyCreds: true,
			},
		},
		"OK: verify creds, with burn": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 2, owner1, ids.Empty, ids.Empty, true),
					generate.UTXO(ids.ID{102}, test.AVAXAssetID, 5, owner1, depositTxID1, ids.Empty, true),
					generate.UTXO(ids.ID{103}, test.AVAXAssetID, 11, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 2, ids.Empty, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 5, depositTxID1, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 11, depositTxID1, bondTxID1, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 6, owner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, 11, owner1, ids.Empty, bondTxID1),
				},
				creds:        []verify.Verifiable{cred1, cred1, cred1},
				burnedAmount: 1,
				assetID:      test.AVAXAssetID,
				verifyCreds:  true,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			err := defaultCaminoHandler(t).verifyUnlockDepositedUTXOsBeforeCairo(
				tt.handlerState(ctrl),
				tt.args.tx,
				tt.args.utxos,
				tt.args.ins,
				tt.args.outs,
				tt.args.creds,
				tt.args.burnedAmount,
				tt.args.assetID,
				tt.args.verifyCreds,
			)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestVerifyUnlockDepositedUTXOs(t *testing.T) {
	tx := &dummyUnsignedTx{txs.BaseTx{}}
	tx.SetBytes([]byte{0})
	owner1, cred1 := generateOwnersAndSig(t, test.Keys[0], tx)
	owner2, cred2 := generateOwnersAndSig(t, test.Keys[1], tx)
	depositTxID1 := ids.ID{1}
	depositTxID2 := ids.ID{2}
	bondTxID1 := ids.ID{3}
	bondTxID2 := ids.ID{4}

	noMsigState := func(ctrl *gomock.Controller) *state.MockChain {
		s := state.NewMockChain(ctrl)
		s.EXPECT().GetMultisigAlias(gomock.Any()).Return(nil, database.ErrNotFound).AnyTimes()
		return s
	}

	type args struct {
		tx           txs.UnsignedTx
		utxos        []*avax.UTXO
		ins          []*avax.TransferableInput
		outs         []*avax.TransferableOutput
		creds        []verify.Verifiable
		amountToBurn uint64
		assetID      ids.ID
	}
	tests := map[string]struct {
		handlerState func(ctrl *gomock.Controller) *state.MockChain
		args         args
		expectedErr  error
	}{
		"Number of inputs-utxos mismatch": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{{}, {}},
				ins:   []*avax.TransferableInput{{}},
			},
			expectedErr: errInputsUTXOsMismatch,
		},
		"Number of inputs-credentials mismatch": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{{}},
				ins:   []*avax.TransferableInput{{}},
				creds: []verify.Verifiable{cred1, cred1},
			},
			expectedErr: errInputsCredentialsMismatch,
		},
		"Bad credential": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{{}},
				ins:   []*avax.TransferableInput{{}},
				creds: []verify.Verifiable{(*secp256k1fx.Credential)(nil)},
			},
			expectedErr: errBadCredentials,
		},
		"UTXO AssetID mismatch": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.OtherAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errAssetIDMismatch,
		},
		"Input AssetID mismatch": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.OtherAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errAssetIDMismatch,
		},
		"UTXO locked, but not deposited (e.g. just bonded)": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, ids.Empty, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, ids.Empty, bondTxID1, []uint32{0}),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errUnlockingUnlockedUTXO,
		},
		"Input and utxo lock IDs mismatch: different depositTxIDs": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(depositTxID1, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID2, ids.Empty, []uint32{0}),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errLockIDsMismatch,
		},
		"Input and utxo lock IDs mismatch: different bondTxIDs": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(depositTxID1, test.AVAXAssetID, 1, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, bondTxID2, []uint32{0}),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errLockIDsMismatch,
		},
		"Input and utxo lock IDs mismatch: utxo is locked, but input isn't locked": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, ids.Empty, ids.Empty, []uint32{0}),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errLockedFundsNotMarkedAsLocked,
		},
		"Input and utxo lock IDs mismatch: utxo isn't locked, but input is locked": {
			handlerState: noMsigState,
			args: args{
				utxos: []*avax.UTXO{
					generate.UTXO(depositTxID1, test.AVAXAssetID, 1, owner1, ids.Empty, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errLockIDsMismatch,
		},
		"Wrong credential": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 5, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 5, depositTxID1, ids.Empty, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 5, owner1, ids.Empty, ids.Empty),
				},
				creds:   []verify.Verifiable{cred2},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errCantSpend,
		},
		"Not burned enough": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, ids.Empty, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, ids.Empty, ids.Empty, []uint32{0}),
				},
				amountToBurn: 2,
				creds:        []verify.Verifiable{cred1},
				assetID:      test.AVAXAssetID,
			},
			expectedErr: errNotBurnedEnough,
		},
		"Produced unlocked more, than consumed deposited or unlocked": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 2, owner1, ids.Empty, ids.Empty),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errWrongProducedAmount,
		},
		"Consumed-produced amount mismatch (per lockIDs)": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, ids.Empty, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 1, owner1, ids.Empty, bondTxID1),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errWrongProducedAmount,
		},
		"Consumed-produced amount mismatch (per ownerID, e.g. produced locked for different owner)": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 1, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 1, depositTxID1, bondTxID1, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 1, owner2, ids.Empty, bondTxID1),
				},
				creds:   []verify.Verifiable{cred1},
				assetID: test.AVAXAssetID,
			},
			expectedErr: errWrongProducedAmount,
		},
		"OK": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 5, owner1, depositTxID1, ids.Empty, true),
					generate.UTXO(ids.ID{101}, test.AVAXAssetID, 11, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 5, depositTxID1, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 11, depositTxID1, bondTxID1, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 5, owner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, 11, owner1, ids.Empty, bondTxID1),
				},
				creds:   []verify.Verifiable{cred1, cred1},
				assetID: test.AVAXAssetID,
			},
		},
		"OK: with burn": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 2, owner1, ids.Empty, ids.Empty, true),
					generate.UTXO(ids.ID{102}, test.AVAXAssetID, 5, owner1, depositTxID1, ids.Empty, true),
					generate.UTXO(ids.ID{103}, test.AVAXAssetID, 11, owner1, depositTxID1, bondTxID1, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 2, ids.Empty, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 5, depositTxID1, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 11, depositTxID1, bondTxID1, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 6, owner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, 11, owner1, ids.Empty, bondTxID1),
				},
				amountToBurn: 1,
				creds:        []verify.Verifiable{cred1, cred1, cred1},
				assetID:      test.AVAXAssetID,
			},
		},
		"OK: with burn, partial unlock, no bond": {
			handlerState: noMsigState,
			args: args{
				tx: tx,
				utxos: []*avax.UTXO{
					generate.UTXO(ids.ID{100}, test.AVAXAssetID, 100, owner1, ids.Empty, ids.Empty, true),
					generate.UTXO(ids.ID{101}, test.AVAXAssetID, 50, owner1, depositTxID1, ids.Empty, true),
				},
				ins: []*avax.TransferableInput{
					generate.In(test.AVAXAssetID, 100, ids.Empty, ids.Empty, []uint32{0}),
					generate.In(test.AVAXAssetID, 50, depositTxID1, ids.Empty, []uint32{0}),
				},
				outs: []*avax.TransferableOutput{
					generate.Out(test.AVAXAssetID, 10, owner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, 99, owner1, ids.Empty, ids.Empty),
					generate.Out(test.AVAXAssetID, 40, owner1, depositTxID1, ids.Empty),
				},
				creds:        []verify.Verifiable{cred1, cred1},
				amountToBurn: 1,
				assetID:      test.AVAXAssetID,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			err := defaultCaminoHandler(t).verifyUnlockDepositedUTXOs(
				tt.handlerState(ctrl),
				tt.args.tx,
				tt.args.utxos,
				tt.args.ins,
				tt.args.outs,
				tt.args.creds,
				tt.args.amountToBurn,
				tt.args.assetID,
			)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

// TODO @evlekht add otherAssetID utxos to test, equal amounts and equal lockTxIDs
func TestSortUTXOs(t *testing.T) {
	bondTxID1 := ids.ID{1, 1}
	bondTxID2 := ids.ID{1, 2}
	depositTxID1 := ids.ID{2, 1}
	depositTxID2 := ids.ID{2, 2}

	originalUTXOs := []*avax.UTXO{
		// depositTxID1, bondTxID1
		generate.UTXO(ids.ID{0}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, depositTxID1, bondTxID1, true),
		generate.UTXO(ids.ID{1}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, depositTxID1, bondTxID1, true),
		generate.UTXO(ids.ID{2}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, depositTxID1, bondTxID1, true),
		// depositTxID1, bondTxID2
		generate.UTXO(ids.ID{3}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, depositTxID1, bondTxID2, true),
		generate.UTXO(ids.ID{4}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, depositTxID1, bondTxID2, true),
		generate.UTXO(ids.ID{5}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, depositTxID1, bondTxID2, true),

		// depositTxID2, bondTxID1
		generate.UTXO(ids.ID{6}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, depositTxID2, bondTxID1, true),
		generate.UTXO(ids.ID{7}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, depositTxID2, bondTxID1, true),
		generate.UTXO(ids.ID{8}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, depositTxID2, bondTxID1, true),
		// depositTxID2, bondTxID2
		generate.UTXO(ids.ID{9}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, depositTxID2, bondTxID2, true),
		generate.UTXO(ids.ID{10}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, depositTxID2, bondTxID2, true),
		generate.UTXO(ids.ID{11}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, depositTxID2, bondTxID2, true),

		// depositTxID1, ids.Empty
		generate.UTXO(ids.ID{12}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, depositTxID1, ids.Empty, true),
		generate.UTXO(ids.ID{13}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, depositTxID1, ids.Empty, true),
		generate.UTXO(ids.ID{14}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, depositTxID1, ids.Empty, true),
		// depositTxID2, ids.Empty
		generate.UTXO(ids.ID{15}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, depositTxID2, ids.Empty, true),
		generate.UTXO(ids.ID{16}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, depositTxID2, ids.Empty, true),
		generate.UTXO(ids.ID{17}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, depositTxID2, ids.Empty, true),

		// ids.Empty, bondTxID1
		generate.UTXO(ids.ID{18}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, ids.Empty, bondTxID1, true),
		generate.UTXO(ids.ID{19}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, ids.Empty, bondTxID1, true),
		generate.UTXO(ids.ID{20}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, ids.Empty, bondTxID1, true),
		// ids.Empty, bondTxID2
		generate.UTXO(ids.ID{21}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, ids.Empty, bondTxID2, true),
		generate.UTXO(ids.ID{22}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, ids.Empty, bondTxID2, true),
		generate.UTXO(ids.ID{23}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, ids.Empty, bondTxID2, true),

		// ids.Empty, ids.Empty
		generate.UTXO(ids.ID{24}, test.AVAXAssetID, 2, secp256k1fx.OutputOwners{}, ids.Empty, ids.Empty, true),
		generate.UTXO(ids.ID{25}, test.AVAXAssetID, 3, secp256k1fx.OutputOwners{}, ids.Empty, ids.Empty, true),
		generate.UTXO(ids.ID{26}, test.AVAXAssetID, 1, secp256k1fx.OutputOwners{}, ids.Empty, ids.Empty, true),
	}
	tests := map[string]struct {
		allowedAssetID ids.ID
		lockState      locked.State
		expectedUTXOs  func([]*avax.UTXO) []*avax.UTXO
	}{
		"StateUnlocked": {
			allowedAssetID: test.AVAXAssetID,
			lockState:      locked.StateUnlocked,
			expectedUTXOs: func(utxos []*avax.UTXO) []*avax.UTXO {
				return []*avax.UTXO{
					utxos[26], utxos[24], utxos[25], // unlocked, ascend amount
					// in case of equal amounts, order is shuffled, but its deterministic by sort inner algorithm
					utxos[11], utxos[2], utxos[5], utxos[23], // locked, ignore lockTxIDs order, 1 nCAM
					utxos[20], utxos[8], utxos[17], utxos[14], //
					utxos[12], utxos[18], utxos[3], utxos[6], // locked, ignore lockTxIDs order, 2 nCAM
					utxos[15], utxos[21], utxos[9], utxos[0], //
					utxos[19], utxos[7], utxos[16], utxos[22], // locked, ignore lockTxIDs order, 2 nCAM
					utxos[10], utxos[4], utxos[13], utxos[1], //
				}
			},
		},
		"StateDeposited": {
			allowedAssetID: test.AVAXAssetID,
			lockState:      locked.StateDeposited,
			expectedUTXOs: func(utxos []*avax.UTXO) []*avax.UTXO {
				return []*avax.UTXO{
					// bondTxID descending, amount ascending
					utxos[23], utxos[21], utxos[22], // bonded, bondTxID2, ascend amount
					utxos[20], utxos[18], utxos[19], // bonded, bondTxID1, ascend amount
					utxos[26], utxos[24], utxos[25], // unlocked, ascend amount
					// DepositTxID order is actually semi-shuffled and considered "ignored".
					// It's deterministic by sort inner algorithm
					utxos[11], utxos[5], //             deposited-bonded, bondTxID2, 1 nCAM
					utxos[3], utxos[9], //              deposited-bonded, bondTxID2, 2 nCAM
					utxos[4], utxos[10], //             deposited-bonded, bondTxID2, 3 nCAM
					utxos[8], utxos[2], //              deposited-bonded, bondTxID1, 1 nCAM
					utxos[6], utxos[0], //              deposited-bonded, bondTxID1, 2 nCAM
					utxos[7], utxos[1], //              deposited-bonded, bondTxID1, 3 nCAM
					utxos[17], utxos[14], //            deposited, 1 nCAM
					utxos[15], utxos[12], //            deposited, 2 nCAM
					utxos[16], utxos[13], //            deposited, 3 nCAM
				}
			},
		},
		"StateBonded": {
			allowedAssetID: test.AVAXAssetID,
			lockState:      locked.StateBonded,
			expectedUTXOs: func(utxos []*avax.UTXO) []*avax.UTXO {
				return []*avax.UTXO{
					// depositTxID descending, amount ascending
					utxos[17], utxos[15], utxos[16], // deposited, depositTxID2, ascend amount
					utxos[14], utxos[12], utxos[13], // deposited, depositTxID1, ascend amount
					utxos[26], utxos[24], utxos[25], // unlocked, ascend amount
					// BondTxID order is actually semi-shuffled and considered "ignored".
					// It's deterministic by sort inner algorithm
					utxos[11], utxos[8], //             deposited-bonded, depositTxID2, 1 nCAM
					utxos[6], utxos[9], //              deposited-bonded, depositTxID2, 2 nCAM
					utxos[7], utxos[10], //             deposited-bonded, depositTxID2, 3 nCAM
					utxos[5], utxos[2], //              deposited-bonded, depositTxID1, 1 nCAM
					utxos[0], utxos[3], //              deposited-bonded, depositTxID1, 2 nCAM
					utxos[4], utxos[1], //              deposited-bonded, depositTxID1, 3 nCAM
					utxos[23], utxos[20], //            bonded, 1 nCAM
					utxos[18], utxos[21], //            bonded, 2 nCAM
					utxos[22], utxos[19], //            bonded, 3 nCAM
				}
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			utxos := make([]*avax.UTXO, len(originalUTXOs))
			copy(utxos, originalUTXOs)

			sortUTXOs(utxos, tt.allowedAssetID, tt.lockState)

			// Uncomment in case of debugging. This will provide more readable error messages with exact utxo indexes
			//
			// expectedUTXOs := tt.expectedUTXOs(originalUTXOs)
			// for i := range utxos {
			// 	require.Equal(t, expectedUTXOs[i], utxos[i], "[%d], expect %d, got %d",
			// 		i, expectedUTXOs[i].TxID[0], utxos[i].TxID[0])
			// }
			require.Equal(t, tt.expectedUTXOs(originalUTXOs), utxos)
		})
	}
}

// VerifyUnlockDepositedUTXOs requires outputs to be sorted exactly, that unlocked outs will go first
func TestAVAXSortTransferableOutputsUnlockedFirst(t *testing.T) {
	owner1 := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{{1}},
	}
	owner2 := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{{2}},
	}
	owner1And2 := secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{{1}, {2}},
	}
	depositTxID1 := ids.ID{3}
	bondTxID1 := ids.ID{4}
	depositTxID2 := ids.ID{5}
	bondTxID2 := ids.ID{6}

	outsWithLockIDs := func(depositTxID, bondTxID ids.ID) []*avax.TransferableOutput {
		return []*avax.TransferableOutput{
			generate.Out(test.AVAXAssetID, 1, owner1, depositTxID, bondTxID),
			generate.Out(test.AVAXAssetID, 2, owner1, depositTxID, bondTxID),
			generate.Out(test.AVAXAssetID, 1, owner2, depositTxID, bondTxID),
			generate.Out(test.AVAXAssetID, 2, owner2, depositTxID, bondTxID),
			generate.Out(test.AVAXAssetID, 1, owner1And2, depositTxID, bondTxID),
			generate.Out(test.AVAXAssetID, 2, owner1And2, depositTxID, bondTxID),
		}
	}

	outs := outsWithLockIDs(ids.Empty, ids.Empty)
	outs = append(outs, outsWithLockIDs(depositTxID1, ids.Empty)...)
	outs = append(outs, outsWithLockIDs(depositTxID2, ids.Empty)...)
	outs = append(outs, outsWithLockIDs(ids.Empty, bondTxID1)...)
	outs = append(outs, outsWithLockIDs(ids.Empty, bondTxID2)...)
	outs = append(outs, outsWithLockIDs(depositTxID1, bondTxID1)...)
	outs = append(outs, outsWithLockIDs(depositTxID1, bondTxID2)...)
	outs = append(outs, outsWithLockIDs(depositTxID2, bondTxID1)...)
	outs = append(outs, outsWithLockIDs(depositTxID2, bondTxID2)...)

	avax.SortTransferableOutputs(outs, txs.Codec)

	doneWithUnlockedOuts := false
	for i, out := range outs {
		_, isLocked := out.Out.(*locked.Out)
		require.Truef(t, isLocked || !doneWithUnlockedOuts, "outs[%d] is locked, but there are unlocked outs after it", i-1)
		doneWithUnlockedOuts = isLocked
	}
}
