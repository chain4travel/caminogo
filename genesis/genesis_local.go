// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************

// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"time"

	"github.com/chain4travel/caminogo/utils/constants"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/utils/formatting"
	"github.com/chain4travel/caminogo/utils/units"
	"github.com/chain4travel/caminogo/utils/wrappers"
	"github.com/chain4travel/caminogo/vms/platformvm/reward"
)

// PrivateKey-vmRQiZeXEXYMyJhEiqdC2z5JhuDbxL8ix9UVvjgMu2Er1NepE => P-local1g65uqn6t77p656w64023nh8nd9updzmxyymev2
// PrivateKey-ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN => X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u
// 56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027 => 0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC

const (
	VMRQKeyStr          = "vmRQiZeXEXYMyJhEiqdC2z5JhuDbxL8ix9UVvjgMu2Er1NepE"
	VMRQKeyFormattedStr = constants.SecretKeyPrefix + VMRQKeyStr

	EWOQKeyStr          = "ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"
	EWOQKeyFormattedStr = constants.SecretKeyPrefix + EWOQKeyStr
)

var (
	VMRQKey *crypto.PrivateKeySECP256K1R
	EWOQKey *crypto.PrivateKeySECP256K1R

	localGenesisConfigJSON = `{
		"networkID": 12345,
		"allocations": [
			{
				"ethAddr": "0xb3d82b1367d362de99ab59a658165aff520cbd4d",
				"avaxAddr": "X-local1g65uqn6t77p656w64023nh8nd9updzmxyymev2",
				"initialAmount": 0,
				"unlockSchedule": [
					{
						"amount": 10000000000000000,
						"locktime": 1633824000
					}
				]
			},
			{
				"ethAddr": "0xb3d82b1367d362de99ab59a658165aff520cbd4d",
				"avaxAddr": "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
				"initialAmount": 300000000000000000,
				"unlockSchedule": [
					{
						"amount": 20000000000000000
					},
					{
						"amount": 10000000000000000,
						"locktime": 1633824000
					}
				]
			},
			{
				"ethAddr": "0xb3d82b1367d362de99ab59a658165aff520cbd4d",
				"avaxAddr": "X-local1ur873jhz9qnaqv5qthk5sn3e8nj3e0kmggalnu",
				"initialAmount": 10000000000000000,
				"unlockSchedule": [
					{
						"amount": 10000000000000000,
						"locktime": 1633824000
					}
				]
			}
		],
		"startTime": 1662523200,
		"initialStakeDuration": 31536000,
		"initialStakeDurationOffset": 5400,
		"initialStakedFunds": [
			"X-local1g65uqn6t77p656w64023nh8nd9updzmxyymev2"
		],
		"initialStakers": [
			{
				"nodeID": "NodeID-AuEZMvcizcF3tbGXTKn463ppPWo1Yvjod",
				"rewardAddress": "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"
			},
			{
				"nodeID": "NodeID-M5bHJNmSPWDucfUwaL44b3PjNRtAqPsib",
				"rewardAddress": "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"
			},
			{
				"nodeID": "NodeID-7j5oUJYqAPWwRrsdsN5W9V3CSoAWDbr19",
				"rewardAddress": "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"
			},
			{
				"nodeID": "NodeID-JiRgENDnvAhL9SeejL43HhRfCfjdsX8KN",
				"rewardAddress": "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"
			},
			{
				"nodeID": "NodeID-2UaFan8QDAN7GVLu8avxtHkMP9Hr17dZS",
				"rewardAddress": "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"
			}
		],
		"validatorBondAmount": 2000000000000000,
		"cChainGenesis": "{\"config\":{\"chainId\":43112,\"homesteadBlock\":0,\"daoForkBlock\":0,\"daoForkSupport\":true,\"eip150Block\":0,\"eip150Hash\":\"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0\",\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"apricotPhase1BlockTimestamp\":0,\"apricotPhase2BlockTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x5f5e100\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC\":{\"balance\":\"0x295BE96E64066972000000\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\"}",
		"message": "{{ fun_quote }}"
	}`

	// LocalParams are the params used for local networks
	LocalParams = Params{
		TxFeeConfig: TxFeeConfig{
			TxFee:                 units.MilliAvax,
			CreateAssetTxFee:      units.MilliAvax,
			CreateSubnetTxFee:     100 * units.MilliAvax,
			CreateBlockchainTxFee: 100 * units.MilliAvax,
		},
		StakingConfig: StakingConfig{
			UptimeRequirement: .8, // 80%
			MinStakeDuration:  24 * time.Hour,
			MaxStakeDuration:  365 * 24 * time.Hour,
			RewardConfig: reward.Config{
				MaxConsumptionRate: .12 * reward.PercentDenominator,
				MinConsumptionRate: .10 * reward.PercentDenominator,
				MintingPeriod:      365 * 24 * time.Hour,
				SupplyCap:          720 * units.MegaAvax,
			},
		},
	}
)

func init() {
	errs := wrappers.Errs{}
	vmrqBytes, err := formatting.Decode(formatting.CB58, VMRQKeyStr)
	errs.Add(err)
	ewoqBytes, err := formatting.Decode(formatting.CB58, EWOQKeyStr)
	errs.Add(err)

	factory := crypto.FactorySECP256K1R{}
	vmrqIntf, err := factory.ToPrivateKey(vmrqBytes)
	errs.Add(err)
	ewoqIntf, err := factory.ToPrivateKey(ewoqBytes)
	errs.Add(err)

	if errs.Err != nil {
		panic(errs.Err)
	}

	VMRQKey = vmrqIntf.(*crypto.PrivateKeySECP256K1R)
	EWOQKey = ewoqIntf.(*crypto.PrivateKeySECP256K1R)
}
