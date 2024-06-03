// Copyright (C) 2024, Chain4Travel AG. All rights reserved.
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
	"crypto"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/ips"
)

func TestSignedIpVerify(t *testing.T) {
	tlsCert1, err := staking.NewTLSCert()
	require.NoError(t, err)
	cert1, err := staking.CertificateFromX509(tlsCert1.Leaf)
	require.NoError(t, err)
	require.NoError(t, staking.ValidateCertificate(cert1))

	tlsCert2, err := staking.NewTLSCert()
	require.NoError(t, err)
	cert2, err := staking.CertificateFromX509(tlsCert2.Leaf)
	require.NoError(t, err)
	require.NoError(t, staking.ValidateCertificate(cert2))

	now := time.Now()

	type test struct {
		name         string
		signer       crypto.Signer
		expectedCert *staking.Certificate
		ip           UnsignedIP
		maxTimestamp time.Time
		expectedErr  error
	}

	tests := []test{
		{
			name:         "valid (before max time)",
			signer:       tlsCert1.PrivateKey.(crypto.Signer),
			expectedCert: cert1,
			ip: UnsignedIP{
				IPPort: ips.IPPort{
					IP:   net.IPv4(1, 2, 3, 4),
					Port: 1,
				},
				Timestamp: uint64(now.Unix()) - 1,
			},
			maxTimestamp: now,
			expectedErr:  nil,
		},
		{
			name:         "valid (at max time)",
			signer:       tlsCert1.PrivateKey.(crypto.Signer),
			expectedCert: cert1,
			ip: UnsignedIP{
				IPPort: ips.IPPort{
					IP:   net.IPv4(1, 2, 3, 4),
					Port: 1,
				},
				Timestamp: uint64(now.Unix()),
			},
			maxTimestamp: now,
			expectedErr:  nil,
		},
		{
			name:         "timestamp too far ahead",
			signer:       tlsCert1.PrivateKey.(crypto.Signer),
			expectedCert: cert1,
			ip: UnsignedIP{
				IPPort: ips.IPPort{
					IP:   net.IPv4(1, 2, 3, 4),
					Port: 1,
				},
				Timestamp: uint64(now.Unix()) + 1,
			},
			maxTimestamp: now,
			expectedErr:  errTimestampTooFarInFuture,
		},
		{
			name:         "sig from wrong cert",
			signer:       tlsCert1.PrivateKey.(crypto.Signer),
			expectedCert: cert2, // note this isn't cert1
			ip: UnsignedIP{
				IPPort: ips.IPPort{
					IP:   net.IPv4(1, 2, 3, 4),
					Port: 1,
				},
				Timestamp: uint64(now.Unix()),
			},
			maxTimestamp: now,
			expectedErr:  errInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signedIP, err := tt.ip.Sign(tt.signer)
			require.NoError(t, err)

			err = signedIP.Verify(tt.expectedCert, tt.maxTimestamp)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
