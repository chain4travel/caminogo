// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package staking

import (
	"crypto"
	"crypto/x509"

	"github.com/ava-labs/avalanchego/ids"
)

type Certificate struct {
	Raw       []byte
	NodeID    ids.NodeID
	PublicKey crypto.PublicKey
	// TODO: Remove after v1.11.x activates.
	SignatureAlgorithm x509.SignatureAlgorithm
}

// CertificateFromX509 converts an x509 certificate into a staking certificate.
//
// Invariant: The provided certificate must be a parseable into a staking
// certificate.
func CertificateFromX509(cert *x509.Certificate) (*Certificate, error) {
	nodeID, err := TLSCertToID(cert)
	if err != nil {
		return nil, err
	}
	return &Certificate{
		Raw:                cert.Raw,
		NodeID:             nodeID,
		PublicKey:          cert.PublicKey,
		SignatureAlgorithm: cert.SignatureAlgorithm,
	}, nil
}
