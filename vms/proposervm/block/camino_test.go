// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package block

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	utilsSecp256k1 "github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
)

// Convinient way to run generateTestBlock. Comment out SkipNow before run.
func TestGenerateTestBlock(t *testing.T) {
	t.SkipNow()
	key, cert, err := generateTestKeyAndCertWithDupExt()
	require.NoError(t, err)
	blockHex, err := generateTestBlock(key, cert)
	require.NoError(t, err)
	t.Logf("generated block hex: %s\n", blockHex)
}

// Creates block with given key and cert, then prints block bytes hex. This hex is used by tests in this package.
func generateTestBlock(key crypto.Signer, cert *staking.Certificate) (string, error) {
	parentID := ids.ID{1}
	timestamp := time.Unix(123, 0)
	pChainHeight := uint64(2)
	innerBlockBytes := []byte{3}
	chainID := ids.ID{4}

	block, err := Build(
		parentID,
		timestamp,
		pChainHeight,
		cert,
		innerBlockBytes,
		chainID,
		key,
	)
	if err != nil {
		return "", err
	}

	blockBytes, err := Codec.Marshal(CodecVersion, block)
	if err != nil {
		return "", err
	}

	return "00000000" + hex.EncodeToString(blockBytes), nil
}

// Creates key and certificate with duplicated extensions.
func generateTestKeyAndCertWithDupExt() (crypto.Signer, *staking.Certificate, error) {
	// Create RSA key to sign cert with
	rsaKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't generate rsa key: %w", err)
	}
	// Create SECP256K1 key to sign cert with
	secpKey := utilsSecp256k1.RsaPrivateKeyToSecp256PrivateKey(rsaKey)
	extension := utilsSecp256k1.SignRsaPublicKey(secpKey, &rsaKey.PublicKey)

	// Create self-signed staking cert
	certTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(0),
		NotBefore:             time.Date(2000, time.January, 0, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Now().AddDate(100, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
		ExtraExtensions:       []pkix.Extension{*extension, *extension},
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create certificate: %w", err)
	}

	input := cryptobyte.String(certBytes)
	if !input.ReadASN1Element(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, nil, staking.ErrMalformedCertificate
	}

	tlsCert := tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  rsaKey,
		Leaf: &x509.Certificate{
			Raw:        input,
			PublicKey:  &rsaKey.PublicKey,
			Extensions: certTemplate.ExtraExtensions,
		},
	}

	cert, err := staking.CertificateFromX509(tlsCert.Leaf)
	if err != nil {
		return nil, nil, err
	}

	return rsaKey, cert, nil
}
