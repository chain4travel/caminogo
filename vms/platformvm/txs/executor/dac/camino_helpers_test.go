// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dac

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/chains/atomic"
	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/snowtest"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

var (
	defaultTxFee              = uint64(100)
	defaultMinStakingDuration = 24 * time.Hour
	defaultGenesisTime        = time.Date(1997, 1, 1, 0, 0, 0, 0, time.UTC)
	defaultValidateStartTime  = defaultGenesisTime
	defaultValidateEndTime    = defaultValidateStartTime.Add(10 * defaultMinStakingDuration)
	xChainID                  = ids.Empty.Prefix(0)
	cChainID                  = ids.Empty.Prefix(1)
	avaxAssetID               = ids.ID{'y', 'e', 'e', 't'}

	errMissing = errors.New("missing")
)

type mutableSharedMemory struct {
	atomic.SharedMemory
}

func defaultClock(postBanff bool) mockable.Clock {
	now := defaultGenesisTime
	if postBanff {
		// 1 second after Banff fork
		now = defaultValidateEndTime.Add(-2 * time.Second)
	}
	clk := mockable.Clock{}
	clk.Set(now)
	return clk
}

type fxVMInt struct {
	registry codec.Registry
	clk      *mockable.Clock
	log      logging.Logger
}

func (fvi *fxVMInt) CodecRegistry() codec.Registry {
	return fvi.registry
}

func (fvi *fxVMInt) Clock() *mockable.Clock {
	return fvi.clk
}

func (fvi *fxVMInt) Logger() logging.Logger {
	return fvi.log
}

func defaultFx(t *testing.T) fx.Fx {
	t.Helper()

	var isBootstrapped utils.Atomic[bool]
	isBootstrapped.Set(true)
	clk := defaultClock(true)
	ctx := snowtest.Context(t, snowtest.PChainID)

	fxVMInt := &fxVMInt{
		registry: linearcodec.NewDefault(time.Time{}),
		clk:      &clk,
		log:      ctx.Log,
	}
	res := &secp256k1fx.Fx{}
	require.NoError(t, res.Initialize(fxVMInt))
	require.NoError(t, res.Bootstrapped())
	return res
}

func generateTestUTXO(txID ids.ID, assetID ids.ID, amount uint64, outputOwners secp256k1fx.OutputOwners, depositTxID, bondTxID ids.ID) *avax.UTXO {
	return generateTestUTXOWithIndex(txID, 0, assetID, amount, outputOwners, depositTxID, bondTxID, true)
}

func generateTestUTXOWithIndex(txID ids.ID, outIndex uint32, assetID ids.ID, amount uint64, outputOwners secp256k1fx.OutputOwners, depositTxID, bondTxID ids.ID, init bool) *avax.UTXO {
	var out avax.TransferableOut = &secp256k1fx.TransferOutput{
		Amt:          amount,
		OutputOwners: outputOwners,
	}
	if depositTxID != ids.Empty || bondTxID != ids.Empty {
		out = &locked.Out{
			IDs: locked.IDs{
				DepositTxID: depositTxID,
				BondTxID:    bondTxID,
			},
			TransferableOut: out,
		}
	}
	testUTXO := &avax.UTXO{
		UTXOID: avax.UTXOID{
			TxID:        txID,
			OutputIndex: outIndex,
		},
		Asset: avax.Asset{ID: assetID},
		Out:   out,
	}
	if init {
		testUTXO.InputID()
	}
	return testUTXO
}

func generateTestOut(assetID ids.ID, amount uint64, outputOwners secp256k1fx.OutputOwners, depositTxID, bondTxID ids.ID) *avax.TransferableOutput { //nolint:unparam
	var out avax.TransferableOut = &secp256k1fx.TransferOutput{
		Amt:          amount,
		OutputOwners: outputOwners,
	}
	if depositTxID != ids.Empty || bondTxID != ids.Empty {
		out = &locked.Out{
			IDs: locked.IDs{
				DepositTxID: depositTxID,
				BondTxID:    bondTxID,
			},
			TransferableOut: out,
		}
	}
	return &avax.TransferableOutput{
		Asset: avax.Asset{ID: assetID},
		Out:   out,
	}
}

func generateTestInFromUTXO(utxo *avax.UTXO, sigIndices []uint32) *avax.TransferableInput {
	var in avax.TransferableIn
	switch out := utxo.Out.(type) {
	case *secp256k1fx.TransferOutput:
		in = &secp256k1fx.TransferInput{
			Amt:   out.Amount(),
			Input: secp256k1fx.Input{SigIndices: sigIndices},
		}
	case *locked.Out:
		in = &locked.In{
			IDs: out.IDs,
			TransferableIn: &secp256k1fx.TransferInput{
				Amt:   out.Amount(),
				Input: secp256k1fx.Input{SigIndices: sigIndices},
			},
		}
	default:
		panic("unknown utxo.Out type")
	}

	// to be sure that utxoid.id is set in both entities
	utxo.InputID()
	return &avax.TransferableInput{
		UTXOID: utxo.UTXOID,
		Asset:  utxo.Asset,
		In:     in,
	}
}

func generateKeyAndOwner(t *testing.T) (*secp256k1.PrivateKey, ids.ShortID, secp256k1fx.OutputOwners) {
	key, err := secp256k1.NewPrivateKey()
	require.NoError(t, err)
	addr := key.Address()
	return key, addr, secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{addr},
	}
}
