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

package blocks

import (
	"math"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/timestampvm/txs"
)

// Version is the current default codec version
const Version = txs.Version

// GenesisCodec allows blocks of larger than usual size to be parsed.
// While this gives flexibility in accommodating large genesis blocks
// it must not be used to parse new, unverified blocks which instead
// must be processed by Codec
var (
	Codec        codec.Manager
	GenesisCodec codec.Manager
)

func init() {
	c := linearcodec.NewCaminoDefault()
	Codec = codec.NewDefaultManager()
	gc := linearcodec.NewCaminoCustomMaxLength(math.MaxInt32)
	GenesisCodec = codec.NewManager(math.MaxInt32)

	errs := wrappers.Errs{}
	for _, c := range []codec.CaminoRegistry{c, gc} {
		errs.Add(
			txs.RegisterUnsignedTxsTypes(c),
		)
	}
	errs.Add(
		Codec.RegisterCodec(Version, c),
		GenesisCodec.RegisterCodec(Version, gc),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}
