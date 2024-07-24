// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package staking

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"

	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
)

var (
	errWrongCertificateVersion = errors.New("certificate version must be 3 (2 if versioning starts with 0)")
	errNoCertificateExtensions = errors.New("certificate must have extensions")
	errMissingNodePubKey       = errors.New("certificate must have extension with node public key")
)

func TLSCertToID(cert *x509.Certificate) (ids.NodeID, error) {
	pubKeyBytes, err := secp256k1.RecoverSecp256PublicKey(cert)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	return ids.ToNodeID(pubKeyBytes)
}

func getNodeID(input cryptobyte.String, certVersion int, pubKey crypto.PublicKey) (ids.NodeID, error) {
	if certVersion != 2 {
		return ids.EmptyNodeID, errWrongCertificateVersion
	}
	var extensions cryptobyte.String
	var hasExtensions bool
	if !input.ReadOptionalASN1(&extensions, &hasExtensions, cryptobyte_asn1.Tag(3).Constructed().ContextSpecific()) {
		return ids.EmptyNodeID, errors.New("x509: malformed extensions")
	}
	if !hasExtensions {
		return ids.EmptyNodeID, errNoCertificateExtensions
	}
	if !extensions.ReadASN1(&extensions, cryptobyte_asn1.SEQUENCE) {
		return ids.EmptyNodeID, errors.New("x509: malformed extensions")
	}
	var secp256k1PubKeyBytes []byte
L:
	for !extensions.Empty() {
		var extension cryptobyte.String
		if !extensions.ReadASN1(&extension, cryptobyte_asn1.SEQUENCE) {
			return ids.EmptyNodeID, errors.New("x509: malformed extension")
		}
		ext, err := parseExtension(extension)
		if err != nil {
			return ids.EmptyNodeID, err
		}
		secp256k1PubKeyBytes, err = secp256k1.RecoverSecp256PublicKeyFromExtension(&ext, pubKey)
		switch {
		case err == secp256k1.ErrWrongExtensionType:
			continue
		case err == nil:
			break L
		default:
			return ids.EmptyNodeID, err
		}
	}

	if secp256k1PubKeyBytes == nil {
		return ids.EmptyNodeID, errMissingNodePubKey
	}

	return ids.ToNodeID(secp256k1PubKeyBytes)
}

func parseExtension(der cryptobyte.String) (pkix.Extension, error) {
	var ext pkix.Extension
	if !der.ReadASN1ObjectIdentifier(&ext.Id) {
		return ext, errors.New("x509: malformed extension OID field")
	}
	if der.PeekASN1Tag(cryptobyte_asn1.BOOLEAN) {
		if !der.ReadASN1Boolean(&ext.Critical) {
			return ext, errors.New("x509: malformed extension critical field")
		}
	}
	var val cryptobyte.String
	if !der.ReadASN1(&val, cryptobyte_asn1.OCTET_STRING) {
		return ext, errors.New("x509: malformed extension value field")
	}
	ext.Value = val
	return ext, nil
}
