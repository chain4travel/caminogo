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
// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"math"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

const (
	// Version is the current default codec version
	Version                                  = 0
	NoPositionsToSkipToMatchAVMCodecOrdering = 5
)

// Codecs do serialization and deserialization
var (
	Codec        codec.Manager
	GenesisCodec codec.Manager
)

func init() {
	// Create default codec and manager
	c := linearcodec.NewDefault()
	Codec = codec.NewDefaultManager()
	gc := linearcodec.NewCustomMaxLength(math.MaxInt32)
	GenesisCodec = codec.NewManager(math.MaxInt32)

	errs := wrappers.Errs{}
	for _, c := range []linearcodec.Codec{c, gc} {
		// Order in which type are registered affect the byte representation
		// generated by marshalling ops. To maintain codec type ordering,
		// we skip positions for the blocks.
		c.SkipRegistrations(NoPositionsToSkipToMatchAVMCodecOrdering)
		errs.Add(RegisterUnsignedTxsTypes(c))
	}
	errs.Add(
		Codec.RegisterCodec(Version, c),
		GenesisCodec.RegisterCodec(Version, gc),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}

func RegisterUnsignedTxsTypes(targetCodec codec.Registry) error {
	errs := wrappers.Errs{}
	errs.Add(
		// The Fx is registered here because this is the same place it is
		// registered in the AVM. This ensures that the typeIDs match up for
		// utxos in shared memory.
		targetCodec.RegisterType(&secp256k1fx.TransferInput{}),
		targetCodec.RegisterType(&secp256k1fx.MintOutput{}),
		targetCodec.RegisterType(&secp256k1fx.TransferOutput{}),
		targetCodec.RegisterType(&secp256k1fx.MintOperation{}),
		targetCodec.RegisterType(&secp256k1fx.Credential{}),
		targetCodec.RegisterType(&secp256k1fx.Input{}),
		targetCodec.RegisterType(&secp256k1fx.OutputOwners{}),

		targetCodec.RegisterType(&BaseTx{}),
		targetCodec.RegisterType(&ImportTx{}),

		targetCodec.RegisterType(&locked.In{}),
		targetCodec.RegisterType(&locked.Out{}),
		targetCodec.RegisterType(&LockMessengerFundsTx{}),
		targetCodec.RegisterType(&CashoutChequeTx{}))
	return errs.Err
}