// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************
// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
)

var (
	errNoCert = errors.New("tls handshake finished with no peer certificate")

	_ Upgrader = (*tlsServerUpgrader)(nil)
	_ Upgrader = (*tlsClientUpgrader)(nil)
)

type Upgrader interface {
	// Must be thread safe
	Upgrade(net.Conn) (ids.NodeID, net.Conn, *staking.Certificate, error)
}

type tlsServerUpgrader struct {
	config       *tls.Config
	invalidCerts prometheus.Counter
	durangoTime  time.Time
}

func NewTLSServerUpgrader(config *tls.Config, invalidCerts prometheus.Counter, durangoTime time.Time) Upgrader {
	return &tlsServerUpgrader{
		config:       config,
		invalidCerts: invalidCerts,
		durangoTime:  durangoTime,
	}
}

func (t *tlsServerUpgrader) Upgrade(conn net.Conn) (ids.NodeID, net.Conn, *staking.Certificate, error) {
	return connToIDAndCert(tls.Server(conn, t.config), t.invalidCerts, t.durangoTime)
}

type tlsClientUpgrader struct {
	config       *tls.Config
	invalidCerts prometheus.Counter
	durangoTime  time.Time
}

func NewTLSClientUpgrader(config *tls.Config, invalidCerts prometheus.Counter, durangoTime time.Time) Upgrader {
	return &tlsClientUpgrader{
		config:       config,
		invalidCerts: invalidCerts,
		durangoTime:  durangoTime,
	}
}

func (t *tlsClientUpgrader) Upgrade(conn net.Conn) (ids.NodeID, net.Conn, *staking.Certificate, error) {
	return connToIDAndCert(tls.Client(conn, t.config), t.invalidCerts, t.durangoTime)
}

func connToIDAndCert(conn *tls.Conn, invalidCerts prometheus.Counter, durangoTime time.Time) (ids.NodeID, net.Conn, *staking.Certificate, error) {
	if err := conn.Handshake(); err != nil {
		return ids.EmptyNodeID, nil, nil, err
	}

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return ids.EmptyNodeID, nil, nil, errNoCert
	}
	tlsCert := state.PeerCertificates[0]

	// Invariant: ParseCertificate is used rather than CertificateFromX509 to
	// ensure that signature verification can assume the certificate was
	// parseable according the staking package's parser.
	//
	// TODO: Remove pre-Durango parsing after v1.11.x has activated.
	var (
		peerCert *staking.Certificate
		err      error
	)
	if time.Now().Before(durangoTime) {
		peerCert, err = staking.ParseCertificate(tlsCert.Raw)
	} else {
		peerCert, err = staking.ParseCertificatePermissive(tlsCert.Raw)
	}
	if err != nil {
		invalidCerts.Inc()
		return ids.EmptyNodeID, nil, nil, err
	}

	return peerCert.NodeID, conn, peerCert, err
}
