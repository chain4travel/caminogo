// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

// CaminoMultisigUTXO interface extends UTXOGetter which is used to get an UTXO for spend
type CaminoMultisigUTXO interface {
	GetMultisigUTXOSigners(utxo *avax.UTXO) (verify.State, error)
}
