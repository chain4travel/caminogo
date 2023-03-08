// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/components/multisig"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	"github.com/stretchr/testify/require"
)

var (
	nodeID        = ids.GenerateTestNodeID()
	testMember, _ = generateTestMember()
)

func TestUnparse(t *testing.T) {
	type args struct {
		networkID uint32
	}
	tests := map[string]struct {
		camino Camino
		args   args
		want   UnparsedCamino
		err    error
	}{
		"success": {
			args: args{networkID: 12345},
			camino: Camino{
				VerifyNodeSignature: true,
				LockModeBondDeposit: true,
				InitialAdmin:        sampleShortID,
				DepositOffers: []DepositOffer{{
					InterestRateNominator:   1,
					Start:                   2,
					End:                     3,
					MinAmount:               4,
					MinDuration:             5,
					MaxDuration:             6,
					UnlockPeriodDuration:    7,
					NoRewardsPeriodDuration: 8,
					Memo:                    "offer memo",
					Flags:                   deposit.OfferFlagLocked,
				}},
				Allocations: []CaminoAllocation{{
					ETHAddr:       sampleShortID,
					AVAXAddr:      sampleShortID,
					XAmount:       1,
					AddressStates: AddressStates{},
					PlatformAllocations: []PlatformAllocation{{
						Amount:            1,
						NodeID:            nodeID,
						ValidatorDuration: 1,
						DepositDuration:   1,
						DepositOfferMemo:  "deposit offer memo",
						TimestampOffset:   1,
						Memo:              "some str",
					}},
				}},
				InitialMultisigAddresses: []MultisigAlias{{
					Alias:      sampleShortID,
					Threshold:  1,
					PublicKeys: []multisig.PublicKey{testMember},
				}},
			},
			want: UnparsedCamino{
				VerifyNodeSignature: true,
				LockModeBondDeposit: true,
				InitialAdmin:        "X-" + wrappers.IgnoreError(address.FormatBech32("local", sampleShortID.Bytes())).(string),
				DepositOffers: []UnparsedDepositOffer{{
					InterestRateNominator:   1,
					StartOffset:             2,
					EndOffset:               3,
					MinAmount:               4,
					MinDuration:             5,
					MaxDuration:             6,
					UnlockPeriodDuration:    7,
					NoRewardsPeriodDuration: 8,
					Memo:                    "offer memo",
					Flags: UnparsedDepositOfferFlags{
						Locked: true,
					},
				}},
				Allocations: []UnparsedCaminoAllocation{{
					ETHAddr:       "0x" + hex.EncodeToString(sampleShortID.Bytes()),
					AVAXAddr:      "X-" + wrappers.IgnoreError(address.FormatBech32("local", sampleShortID.Bytes())).(string),
					XAmount:       1,
					AddressStates: AddressStates{},
					PlatformAllocations: []UnparsedPlatformAllocation{{
						Amount:            1,
						NodeID:            nodeID.String(),
						ValidatorDuration: 1,
						DepositDuration:   1,
						DepositOfferMemo:  "deposit offer memo",
						TimestampOffset:   1,
						Memo:              "some str",
					}},
				}},
				InitialMultisigAddresses: []UnparsedMultisigAlias{{
					Alias:      wrappers.IgnoreError(address.Format(configChainIDAlias, "local", sampleShortID.Bytes())).(string),
					Threshold:  1,
					PublicKeys: []string{testMember.String()},
				}},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.camino.Unparse(tt.args.networkID, 0)

			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSameMSigDefinitionsResultedWithSameAlias(t *testing.T) {
	msig1 := MultisigAlias{
		Threshold:  1,
		PublicKeys: []multisig.PublicKey{testMember},
		Memo:       "",
	}
	msig2 := MultisigAlias{
		Threshold:  1,
		PublicKeys: []multisig.PublicKey{testMember},
		Memo:       "",
	}
	require.Equal(t, msig1, msig2)
	require.Equal(t, msig1.ComputeAlias(ids.Empty), msig2.ComputeAlias(ids.Empty))
}

func TestTxIDIsPartOfAliasComputation(t *testing.T) {
	msig := MultisigAlias{
		Threshold:  1,
		PublicKeys: []multisig.PublicKey{testMember},
		Memo:       "",
	}
	require.NotEqual(t, msig.ComputeAlias(ids.ID{1}), msig.ComputeAlias(ids.ID{2}))
}

func TestMemoIsPartOfTheMsigAliasComputation(t *testing.T) {
	msig1 := MultisigAlias{
		Threshold:  1,
		PublicKeys: []multisig.PublicKey{testMember},
		Memo:       "",
	}
	msig2 := MultisigAlias{
		Threshold:  1,
		PublicKeys: []multisig.PublicKey{testMember},
		Memo:       "memo",
	}
	require.NotEqual(t, msig1.ComputeAlias(ids.Empty), msig2.ComputeAlias(ids.Empty))
}

func TestKnownValueAliasComputationTests(t *testing.T) {
	testKeys := []string{
		"02f44e03514f8c89d295597a41baad37603d7fd64acd04be3883b3e980eed4ee82",
		"0290de98362af32f915113ef3b965c36c396b6c3a74e1c56483c580c9563c16b8d",
		"02c65a4128c34fe0e4bc7553595f1f40697bab5b47f929f79b1d55624881985d53",
	}
	members := make([]multisig.PublicKey, len(testKeys))
	for i, key := range testKeys {
		pubKey, err := multisig.PublicKeyFromString(key)
		require.NoError(t, err)
		members[i] = pubKey
	}
	mem1, mem2, mem3 := members[0], members[1], members[2]

	fmt.Println("Generating some keys for members...", mem1.String(), mem2.String(), mem3.String())

	knownValueTests := []struct {
		txID          ids.ID
		members       []multisig.PublicKey
		threshold     uint32
		memo          string
		expectedAlias string
	}{
		{
			txID:          ids.Empty,
			members:       []multisig.PublicKey{mem1},
			threshold:     1,
			expectedAlias: "E7GrmGYeLrrsysMqW3eJz3XdMEjXbDdjf",
		},
		{
			txID:          ids.ID{1},
			members:       []multisig.PublicKey{mem1},
			threshold:     1,
			expectedAlias: "BURckPHXMPsiUnGAJrR71Wb62AdaHqzsq",
		},
		{
			txID:          ids.Empty,
			members:       []multisig.PublicKey{mem1},
			threshold:     1,
			memo:          "Camino Go!",
			expectedAlias: "8pq2zEpPTzvp1jM8WEGeaU43s3PAG3hwi",
		},
		{
			txID:          ids.Empty,
			members:       []multisig.PublicKey{mem1, mem2},
			threshold:     2,
			expectedAlias: "7J3WSpBsG39tdM3tizvDENdZVGUwQve9u",
		},
		{
			txID:          ids.Empty,
			members:       []multisig.PublicKey{mem1, mem3, mem2},
			threshold:     2,
			expectedAlias: "EXBe3aa2CGGLfzF9BsZEtdGTushMzg4Wc",
		},
	}

	for _, tt := range knownValueTests {
		t.Run(fmt.Sprintf("t-%d-%s-%s", tt.threshold, tt.memo, tt.expectedAlias[:7]), func(t *testing.T) {
			msig := MultisigAlias{
				Threshold:  tt.threshold,
				PublicKeys: tt.members,
				Memo:       tt.memo,
			}
			require.Equal(t, tt.expectedAlias, msig.ComputeAlias(tt.txID).String())
		})
	}
}

func generateTestMember() (multisig.PublicKey, error) {
	return multisig.PublicKeyFromString("02f44e03514f8c89d295597a41baad37603d7fd64acd04be3883b3e980eed4ee82")
}
