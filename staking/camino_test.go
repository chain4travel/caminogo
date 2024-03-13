// Copyright (C) 2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package staking

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	utilsSecp256k1 "github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/stretchr/testify/require"
)

func TestGenerateTestCert(t *testing.T) {
	t.SkipNow()
	const certPath = "/home/dev/Documents/Chain4Travel/caminogo/staking/camino_large_rsa_key.cert"
	require.NoError(t, GenerateTestCert(certPath))
}

func GenerateTestCert(certPath string) error {
	// Create RSA key to sign cert with
	rsaKey, err := rsa.GenerateKey(rand.Reader, 8192) // twice as much bytes!
	if err != nil {
		return fmt.Errorf("couldn't generate rsa key: %w", err)
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
		ExtraExtensions:       []pkix.Extension{*extension},
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		return fmt.Errorf("couldn't create certificate: %w", err)
	}

	// Ensure directory where key/cert will live exist
	if err := os.MkdirAll(filepath.Dir(certPath), perms.ReadWriteExecute); err != nil {
		return fmt.Errorf("couldn't create path for cert: %w", err)
	}

	// Write cert to disk
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("couldn't create cert file: %w", err)
	}
	if _, err := certFile.Write(certBytes); err != nil {
		return fmt.Errorf("couldn't write cert file: %w", err)
	}
	if err := certFile.Close(); err != nil {
		return fmt.Errorf("couldn't close cert file: %w", err)
	}
	return nil
}
