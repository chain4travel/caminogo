// Copyright (C) 2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package staking

import (
	"crypto/x509"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
)

func TLSCertToID(cert *x509.Certificate) (ids.NodeID, error) {
	pubKeyBytes, err := secp256k1.RecoverSecp256PublicKey(cert)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	return ids.ToNodeID(pubKeyBytes)
}
