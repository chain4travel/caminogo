// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// CaminoMultisigUTXO interface extends UTXOGetter which is used to get an UTXO for spend
type CaminoMultisigUTXO interface {
	GetMultisigUTXOSigners(utxo *avax.UTXO) (verify.State, error)
}

// Helper functions
func extractTransferOutput(out verify.State) *secp256k1fx.TransferOutput {

	return nil
}
