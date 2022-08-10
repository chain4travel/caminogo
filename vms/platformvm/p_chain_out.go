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

package platformvm

import (
	"errors"

	"github.com/chain4travel/caminogo/vms/components/avax"
)

var (
	errInvalidPUTXOState = errors.New("invalid pUTXO state")
	errNestedPUTXO       = errors.New("shouldn't nest pUTXO")
)

type PUTXOState uint8

const (
	// TODO@ we should use PUTXO with PUTXOStateTransferable instead of secp256k1fx.TransferOutput on P-chain or remove this state
	PUTXOStateTransferable       PUTXOState = 0b00 // TODO@ rename
	PUTXOStateDeposited          PUTXOState = 0b01
	PUTXOStateBonded             PUTXOState = 0b10
	PUTXOStateDepositedAndBonded PUTXOState = 0b11
)

var pUTXOStateStrings = map[PUTXOState]string{
	PUTXOStateBonded:             "bonded",
	PUTXOStateDeposited:          "deposited",
	PUTXOStateDepositedAndBonded: "depositedAndBonded",
	PUTXOStateTransferable:       "transferable",
}

func (s PUTXOState) String() string {
	return pUTXOStateStrings[s]
}

type PChainOut struct {
	State                PUTXOState `serialize:"true" json:"state"`
	avax.TransferableOut `serialize:"true" json:"output"`
}

func (o *PChainOut) Addresses() [][]byte {
	if addressable, ok := o.TransferableOut.(avax.Addressable); ok {
		return addressable.Addresses()
	}
	return nil
}

func (o *PChainOut) Verify() error {
	if o.State < PUTXOStateTransferable || o.State > PUTXOStateDepositedAndBonded {
		return errInvalidPUTXOState
	}
	if _, nested := o.TransferableOut.(*PChainOut); nested {
		return errNestedPUTXO
	}
	return o.TransferableOut.Verify()
}

func (o *PChainOut) IsLocked() bool {
	return o.State != PUTXOStateTransferable
}

type PChainIn struct {
	State               PUTXOState `serialize:"true" json:"state"`
	avax.TransferableIn `serialize:"true" json:"input"`
}

func (s *PChainIn) Verify() error {
	if s.State < PUTXOStateTransferable || s.State > PUTXOStateDepositedAndBonded {
		return errInvalidPUTXOState
	}
	if _, nested := s.TransferableIn.(*PChainIn); nested {
		return errNestedPUTXO
	}
	return s.TransferableIn.Verify()
}
