// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package api

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/stretchr/testify/require"
)

func TestBuildCaminoGenesis(t *testing.T) {
	hrp := constants.NetworkIDToHRP[testNetworkID]
	nodeID := ids.NodeID{1}
	addr, err := address.FormatBech32(hrp, nodeID.Bytes())
	require.NoError(t, err)

	weight := json.Uint64(987654321)

	tests := map[string]struct {
		args        BuildGenesisArgs
		reply       BuildGenesisReply
		expectedErr error
	}{
		"Happy Path": {
			args: BuildGenesisArgs{
				AvaxAssetID:   ids.ID{},
				NetworkID:     0,
				UTXOs:         []UTXO{},
				Validators:    []PermissionlessValidator{},
				Chains:        []Chain{},
				Camino:        Camino{LockModeBondDeposit: true},
				Time:          5,
				InitialSupply: 0,
				Message:       "",
				Encoding:      formatting.Hex,
			},
			reply:       BuildGenesisReply{},
			expectedErr: nil,
		},
		"Wrong UTXO Number": {
			args: BuildGenesisArgs{
				UTXOs: []UTXO{
					{
						Address: addr,
						Amount:  0,
					},
				},
				Validators: []PermissionlessValidator{},
				Time:       5,
				Encoding:   formatting.Hex,
				Camino: Camino{
					VerifyNodeSignature: true,
					LockModeBondDeposit: true,
					UTXODeposits:        []ids.ID{ids.GenerateTestID(), ids.GenerateTestID()},
				},
			},
			reply:       BuildGenesisReply{},
			expectedErr: errWrongUTXONumber,
		},
		"Wrong Validator Number": {
			args: BuildGenesisArgs{
				UTXOs: []UTXO{},
				Validators: []PermissionlessValidator{
					{
						Staker: Staker{
							StartTime: 0,
							EndTime:   20,
							NodeID:    nodeID,
						},
						RewardOwner: &Owner{
							Threshold: 1,
							Addresses: []string{addr},
						},
						Staked: []UTXO{{
							Amount:  weight,
							Address: addr,
						}},
					},
				},
				Time:     5,
				Encoding: formatting.Hex,
				Camino: Camino{
					VerifyNodeSignature: true,
					LockModeBondDeposit: true,
					ValidatorDeposits: [][]ids.ID{
						{
							ids.GenerateTestID(),
						},
						{
							ids.GenerateTestID(),
						},
					},
				},
			},
			reply:       BuildGenesisReply{},
			expectedErr: errWrongValidatorNumber,
		},
		"Deposits and Staked Misalignment": {
			args: BuildGenesisArgs{
				UTXOs: []UTXO{},
				Validators: []PermissionlessValidator{
					{
						Staker: Staker{
							StartTime: 0,
							EndTime:   20,
							NodeID:    nodeID,
						},
						RewardOwner: &Owner{
							Threshold: 1,
							Addresses: []string{addr},
						},
						Staked: []UTXO{{
							Amount:  weight,
							Address: addr,
						}},
					},
				},
				Time:     5,
				Encoding: formatting.Hex,
				Camino: Camino{
					VerifyNodeSignature: true,
					LockModeBondDeposit: true,
					ValidatorDeposits: [][]ids.ID{
						{
							ids.GenerateTestID(),
							ids.GenerateTestID(),
						},
					},
				},
			},
			reply:       BuildGenesisReply{},
			expectedErr: errWrongDepositsAndStakedNumber,
		},
		"Non Existing Deposit Offer": {
			args: BuildGenesisArgs{
				AvaxAssetID: ids.ID{},
				NetworkID:   0,
				UTXOs: []UTXO{
					{
						Address: addr,
						Amount:  10,
					},
				},
				Validators: []PermissionlessValidator{
					{
						Staker: Staker{
							StartTime: 0,
							EndTime:   20,
							NodeID:    nodeID,
						},
						RewardOwner: &Owner{
							Threshold: 1,
							Addresses: []string{addr},
						},
						Staked: []UTXO{{
							Amount:  weight,
							Address: addr,
						}},
					},
				},
				Chains: []Chain{
					{
						VMID:     ids.GenerateTestID(),
						SubnetID: ids.GenerateTestID(),
					},
				},
				Camino: Camino{
					VerifyNodeSignature: true,
					LockModeBondDeposit: true,
					InitialAdmin:        ids.ShortID{},
					AddressStates:       nil,
					DepositOffers:       nil,
					ValidatorDeposits: [][]ids.ID{
						{
							ids.GenerateTestID(),
						},
					},
					UTXODeposits: []ids.ID{
						ids.GenerateTestID(),
					},
				},
			},
			reply:       BuildGenesisReply{},
			expectedErr: errNonExistingOffer,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := buildCaminoGenesis(&tt.args, &tt.reply)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
