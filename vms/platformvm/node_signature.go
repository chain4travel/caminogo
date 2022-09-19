package platformvm

import (
	cryptoc "crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/formatting"
	"github.com/chain4travel/caminogo/utils/hashing"
)

var (
	errCertificateDontMatch = errors.New("node certificate key doesn't match node ID")
	errKeyPairDontMatch     = errors.New("public key doesn't match private key")
	errWrongCredentialType  = errors.New("wrong credential type")
	errWrongPublicKeyType   = errors.New("wrong public key type")
	errWrongPrivateKeyType  = errors.New("wrong private key type")
	errMissingPEMPK         = errors.New("pem encoded bytes doesn't contain PRIVATE KEY pem block")
	errMissingPEMCert       = errors.New("pem encoded bytes doesn't contain CERTIFICATE pem block")
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
	addValidatorTx, ok := tx.UnsignedTx.(*UnsignedAddValidatorTx)
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

	cert, err := x509.ParseCertificate(addValidatorTx.NodeCertificate)
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

func parseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	privDER, _ := pem.Decode(pemBytes)
	if privDER == nil || privDER.Type != "PRIVATE KEY" {
		return nil, errMissingPEMPK
	}

	nodePrivateKey, err := x509.ParsePKCS8PrivateKey(privDER.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPrivateKey, ok := nodePrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errWrongPrivateKeyType
	}

	return rsaPrivateKey, nil
}

func parseX509CertFromPEM(pemBytes []byte) (*x509.Certificate, error) {
	certDER, _ := pem.Decode(pemBytes)
	if certDER == nil || certDER.Type != "CERTIFICATE" {
		return nil, errMissingPEMCert
	}

	x509Cert, err := x509.ParseCertificate(certDER.Bytes)
	if err != nil {
		return nil, err
	}

	return x509Cert, nil
}

func checkCertificateAndKeyPair(cert *x509.Certificate, privateKey *rsa.PrivateKey) error {
	rsaPublicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errWrongPublicKeyType
	}

	if rsaPublicKey.N.Cmp(privateKey.N) != 0 {
		return errKeyPairDontMatch
	}

	return nil
}
