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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chain4travel/caminogo/utils/perms"
)

var (
	ErrPrivateKeyNotPKCS8   = errors.New("node accepts only PKCS8 private keys")
	ErrParsingKeyPair       = errors.New("failed parsing key pair")
	ErrParsingPrivateKey    = errors.New("failed parsing private key")
	ErrWrongPrivateKeyType  = errors.New("wrong private key type")
	ErrWrongCertificateType = errors.New("wrong certificate type")
)

// InitNodeStakingKeyPair generates a self-signed TLS key/cert pair to use in
// staking. The key and files will be placed at [keyPath] and [certPath],
// respectively. If there is already a file at [keyPath], returns nil.
func InitNodeStakingKeyPair(keyPath, certPath string) error {
	// If there is already a file at [keyPath], do nothing
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		return nil
	}

	certBytes, keyBytes, err := NewCertAndKeyBytes()
	if err != nil {
		return err
	}

	// Ensure directory where key/cert will live exist
	if err := os.MkdirAll(filepath.Dir(certPath), perms.ReadWriteExecute); err != nil {
		return fmt.Errorf("couldn't create path for cert: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), perms.ReadWriteExecute); err != nil {
		return fmt.Errorf("couldn't create path for key: %w", err)
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
	if err := os.Chmod(certPath, perms.ReadOnly); err != nil { // Make cert read-only
		return fmt.Errorf("couldn't change permissions on cert: %w", err)
	}

	// Write key to disk
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("couldn't create key file: %w", err)
	}
	if _, err := keyOut.Write(keyBytes); err != nil {
		return fmt.Errorf("couldn't write private key: %w", err)
	}
	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("couldn't close key file: %w", err)
	}
	if err := os.Chmod(keyPath, perms.ReadOnly); err != nil { // Make key read-only
		return fmt.Errorf("couldn't change permissions on key: %w", err)
	}
	return nil
}

func LoadTLSCertFromBytes(keyBytes, certBytes []byte) (*tls.Certificate, error) {
	// Forcing node to accept only PKCS8 private key
	keyDERBlock, _ := pem.Decode(keyBytes)
	if keyDERBlock == nil {
		return nil, ErrParsingPrivateKey
	}
	_, err := x509.ParsePKCS8PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		return nil, ErrPrivateKeyNotPKCS8
	}

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, ErrParsingKeyPair
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	return &cert, err
}

func LoadTLSCertFromFiles(keyPath, certPath string) (*tls.Certificate, error) {
	certBytes, keyBytes, err := ReadRSAKeyPairFiles(keyPath, certPath)
	if err != nil {
		return nil, err
	}

	cert, err := LoadTLSCertFromBytes(keyBytes, certBytes)
	return cert, err
}

func ReadRSAKeyPairFiles(keyPath, certPath string) ([]byte, []byte, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	return certBytes, keyBytes, nil
}

func GetRSAKeyPairFromTLSCert(cert *tls.Certificate) (*x509.Certificate, *rsa.PrivateKey, error) {
	key, ok := cert.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, ErrWrongPrivateKeyType
	}

	return cert.Leaf, key, nil
}

func NewTLSCert() (*tls.Certificate, error) {
	certBytes, keyBytes, err := NewCertAndKeyBytes()
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, err
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	return &cert, err
}

// Creates a new staking private key / staking certificate pair.
// Returns the PEM byte representations of both.
func NewCertAndKeyBytes() ([]byte, []byte, error) {
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
	var certBuff bytes.Buffer
	if err := pem.Encode(&certBuff, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return nil, nil, fmt.Errorf("couldn't write cert file: %w", err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't marshal private key: %w", err)
	}
	var keyBuff bytes.Buffer
	if err := pem.Encode(&keyBuff, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return nil, nil, fmt.Errorf("couldn't write private key: %w", err)
	}
	return certBuff.Bytes(), keyBuff.Bytes(), nil
}

func NewCertTemplate() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber:          big.NewInt(0),
		NotBefore:             time.Date(2000, time.January, 0, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Now().AddDate(100, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
		BasicConstraintsValid: true,
	}
}

func FormatPKCS8PemFile(pemFile string) string {
	pemOneLine := strings.Replace(pemFile, "\n", "", -1)
	pemFormatted := pemOneLine[:27] + `\n` + pemOneLine[27:]
	pemFormatted = pemFormatted[:len(pemFormatted)-25] + `\n` + pemFormatted[len(pemFormatted)-25:]
	return pemFormatted
}
