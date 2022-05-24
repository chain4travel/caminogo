// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256k1fx

import (
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/crypto"
)

type SignatureKeyMap struct {
	sigs      map[[crypto.SECP256K1RSigLen]byte]struct{}
	addresses map[ids.ShortID]struct{}
}

func NewSignatureKeyMap() *SignatureKeyMap {
	return &SignatureKeyMap{
		sigs:      make(map[[crypto.SECP256K1RSigLen]byte]struct{}, 16),
		addresses: make(map[ids.ShortID]struct{}, 16),
	}
}

// Returns true if there are no signatures collected
func (p *SignatureKeyMap) Empty() bool { return len(p.sigs) == 0 }

// Return true if the passed address is in addresses set
func (p *SignatureKeyMap) Contains(addr ids.ShortID) bool {
	_, ok := p.addresses[addr]
	return ok
}

// Return true if the signature is already known
func (p *SignatureKeyMap) Has(sig [crypto.SECP256K1RSigLen]byte) bool {
	_, ok := p.sigs[sig]
	return ok
}

// Record hash and address (but without any relation)
func (p *SignatureKeyMap) Set(sig [crypto.SECP256K1RSigLen]byte, pk crypto.PublicKey) {
	p.sigs[sig] = struct{}{}
	p.addresses[pk.Address()] = struct{}{}
}
