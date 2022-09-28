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

package staking

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/chain4travel/caminogo/utils/hashing"
	"github.com/stretchr/testify/assert"
)

func TestMakeKeys(t *testing.T) {
	assert := assert.New(t)

	cert, err := NewTLSCert()
	assert.NoError(err)

	msg := []byte(fmt.Sprintf("msg %d", time.Now().Unix()))
	msgHash := hashing.ComputeHash256(msg)

	sig, err := cert.PrivateKey.(crypto.Signer).Sign(rand.Reader, msgHash, crypto.SHA256)
	assert.NoError(err)

	err = cert.Leaf.CheckSignature(cert.Leaf.SignatureAlgorithm, msg, sig)
	assert.NoError(err)
}

func TestLoadTLSPairSuccessfully(t *testing.T) {
	assert := assert.New(t)

	certBytes, keyBytes, err := NewCertAndKeyBytes()
	assert.NoError(err)
	_, err = LoadTLSCertFromBytes(keyBytes, certBytes)
	assert.NoError(err)
}

func TestLoadWrongTLSPairType(t *testing.T) {
	assert := assert.New(t)

	certBytes, keyBytes, err := generatePKCS1KeyPair()
	assert.NoError(err)
	_, err = LoadTLSCertFromBytes(keyBytes, certBytes)
	assert.ErrorIs(err, ErrPrivateKeyNotPKCS8)
}

func generatePKCS1KeyPair() ([]byte, []byte, error) {
	// Create key to sign cert with
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't generate rsa key: %w", err)
	}

	// Create self-signed staking cert
	certTemplate := NewCertTemplate()
	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create certificate: %w", err)
	}

	certPEMBytes := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certBytes,
		},
	)

	keyPEMBytes := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)

	return certPEMBytes, keyPEMBytes, nil
}
