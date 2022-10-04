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

package peer

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/nodeid"
)

var (
	errNoCert = errors.New("tls handshake finished with no peer certificate")

	_ Upgrader = &tlsServerUpgrader{}
	_ Upgrader = &tlsClientUpgrader{}
)

type Upgrader interface {
	// Must be thread safe
	Upgrade(net.Conn) (ids.ShortID, net.Conn, *x509.Certificate, error)
}

type tlsServerUpgrader struct {
	config *tls.Config
}

func NewTLSServerUpgrader(config *tls.Config) Upgrader {
	return tlsServerUpgrader{
		config: config,
	}
}

func (t tlsServerUpgrader) Upgrade(conn net.Conn) (ids.ShortID, net.Conn, *x509.Certificate, error) {
	return connToIDAndCert(tls.Server(conn, t.config))
}

type tlsClientUpgrader struct {
	config *tls.Config
}

func NewTLSClientUpgrader(config *tls.Config) Upgrader {
	return tlsClientUpgrader{
		config: config,
	}
}

func (t tlsClientUpgrader) Upgrade(conn net.Conn) (ids.ShortID, net.Conn, *x509.Certificate, error) {
	return connToIDAndCert(tls.Client(conn, t.config))
}

func connToIDAndCert(conn *tls.Conn) (ids.ShortID, net.Conn, *x509.Certificate, error) {
	if err := conn.Handshake(); err != nil {
		return ids.ShortID{}, nil, nil, err
	}

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return ids.ShortID{}, nil, nil, errNoCert
	}
	peerCert := state.PeerCertificates[0]

	nodeId, err := CertToID(peerCert)
	return nodeId, conn, peerCert, err
}

func CertToID(cert *x509.Certificate) (ids.ShortID, error) {
	pubKeyBytes, err := nodeid.RecoverSecp256PublicKey(cert)
	if err != nil {
		return ids.ShortID{}, err
	}
	return ids.ToShortID(pubKeyBytes)
}
