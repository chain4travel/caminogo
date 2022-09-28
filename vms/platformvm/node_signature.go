// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	cryptoc "crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/staking"
	"github.com/chain4travel/caminogo/utils/formatting"
	"github.com/chain4travel/caminogo/utils/hashing"
)

var (
	errCertificateDontMatch = errors.New("node certificate key doesn't match node ID")
	errWrongCredentialType  = errors.New("wrong credential type")
	errWrongPublicKeyType   = errors.New("wrong public key type")
	errNilCredential        = errors.New("nil credential")
)

const (
	rsaSignatureLen = 512
)

type RSACredential struct {
	Signature [rsaSignatureLen]byte `serialize:"true" json:"signature"`
}

func (cred *RSACredential) Verify() error {
	switch {
	case cred == nil:
		return errNilCredential
	default:
		return nil
	}
}

// MarshalJSON marshals [cred] to JSON
// The string representation of signature is created using the hex formatter
func (cred *RSACredential) MarshalJSON() ([]byte, error) {
	sigStr, err := formatting.EncodeWithoutChecksum(formatting.Hex, cred.Signature[:])
	if err != nil {
		return nil, fmt.Errorf("couldn't convert signature to string: %w", err)
	}
	return json.Marshal(sigStr)
}

// verifyNodeSignature returns nil if [credIntf] matches [nodeCertificate]
func verifyNodeSignature(tx *Tx) error {
	rsaSignedTx, ok := tx.UnsignedTx.(RSASignedTx)
	if !ok {
		return errWrongTxType
	}

	credIntf := tx.Creds[len(tx.Creds)-1]

	if err := credIntf.Verify(); err != nil {
		return err
	}

	cred, ok := credIntf.(*RSACredential)
	if !ok {
		return errWrongCredentialType
	}

	cert, err := x509.ParseCertificate(rsaSignedTx.CertBytes())
	if err != nil {
		return fmt.Errorf("unable to parse x509 certificate: %w", err)
	}

	rsaPublicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errWrongPublicKeyType
	}

	txHash := hashing.ComputeHash256(tx.UnsignedBytes())

	return rsa.VerifyPKCS1v15(rsaPublicKey, cryptoc.SHA256, txHash, cred.Signature[:])
}

// verifyNodeID returns nil if [certBytes] matches [nodeID]
func verifyNodeID(certBytes []byte, nodeID ids.ShortID) error {
	expectedNodeID := nodeIDFromCertBytes(certBytes)
	if expectedNodeID != nodeID {
		return errCertificateDontMatch
	}
	return nil
}

func nodeIDFromCertBytes(certBytes []byte) ids.ShortID {
	return ids.ShortID(hashing.ComputeHash160Array(hashing.ComputeHash256(certBytes)))
}

func LoadRSAKeyPairFromBytes(keyBytes, certBytes []byte) (*x509.Certificate, *rsa.PrivateKey, error) {
	cert, err := staking.LoadTLSCertFromBytes(keyBytes, certBytes)
	if err != nil {
		return nil, nil, err
	}

	nodeCertificate, nodePrivateKey, err := staking.GetRSAKeyPairFromTLSCert(cert)
	if err != nil {
		return nil, nil, err
	}

	return nodeCertificate, nodePrivateKey, nil
}
