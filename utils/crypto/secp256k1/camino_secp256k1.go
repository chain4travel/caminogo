// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256k1

import (
	"crypto"
	rsa "crypto/rsa"
	x509 "crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"

	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	ecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

var oidLocalKeyID = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 21}

var (
	errNoSignature        = errors.New("failed to extract signature from certificate")
	errRecoverFailed      = errors.New("failed to recover public key")
	errNotRSAPublicKey    = errors.New("certificate public key is not rsa public key")
	ErrWrongExtensionType = errors.New("wrong extension type")
)

// Takes a RSA privateKey and builds using it's hash an secp256k1 private key.
func RsaPrivateKeyToSecp256PrivateKey(rPrivKey *rsa.PrivateKey) *secp256k1.PrivateKey {
	// Create 256Bit hash of RSA private key
	data := hashing.ComputeHash256(x509.MarshalPKCS1PrivateKey(rPrivKey))
	// Create secp256k1 private key
	sPrivKey := secp256k1.PrivKeyFromBytes(data)

	return sPrivKey
}

// Sign a rsa public key with the given secp256k1 private key and return
// a x509 Extension. The secp256k1 public key can be recovered for e.g. nodeId
func SignRsaPublicKey(privKey *secp256k1.PrivateKey, pubKey *rsa.PublicKey) *pkix.Extension {
	// Create 256Bit hash of RSA pubic key
	data := hashing.ComputeHash256(x509.MarshalPKCS1PublicKey(pubKey))
	signature := ecdsa.SignCompact(privKey, data, false) // returns [v || r || s]

	return &pkix.Extension{Id: oidLocalKeyID, Critical: true, Value: signature}
}

// Recover the secp256k1 public key using RSA public key and the signature
// This is the reverse what has been done in RsaPrivateKeyToSecp256PrivateKey
// It returns the marshalled public key
func RecoverSecp256PublicKey(cert *x509.Certificate) ([]byte, error) {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oidLocalKeyID) {
			return recoverSecp256PublicKeyFromExtension(&ext, cert.PublicKey) //nolint:gosec
		}
	}
	return nil, errNoSignature
}

func RecoverSecp256PublicKeyFromExtension(ext *pkix.Extension, publicKey crypto.PublicKey) ([]byte, error) {
	if !ext.Id.Equal(oidLocalKeyID) {
		return nil, ErrWrongExtensionType
	}

	return recoverSecp256PublicKeyFromExtension(ext, publicKey)
}

func recoverSecp256PublicKeyFromExtension(ext *pkix.Extension, publicKey crypto.PublicKey) ([]byte, error) {
	if ext.Value == nil {
		return nil, errNoSignature
	}

	rsaPubKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errNotRSAPublicKey
	}

	data := hashing.ComputeHash256(x509.MarshalPKCS1PublicKey(rsaPubKey))
	sPubKey, _, err := ecdsa.RecoverCompact(ext.Value, data)
	if err != nil {
		return nil, errRecoverFailed
	}
	sPubKeyBytes := sPubKey.SerializeCompressed()

	return hashing.PubkeyBytesToAddress(sPubKeyBytes), nil
}

func FakePrivateKey(addr ids.ShortID) *PrivateKey {
	return &PrivateKey{
		sk: &secp256k1.PrivateKey{},
		pk: &PublicKey{
			pk:   &secp256k1.PublicKey{},
			addr: addr,
		},
	}
}

// IsFakeKey returns true if sk's key is zero
func (k *PrivateKey) IsZero() bool {
	return k.sk.Key.IsZero()
}
