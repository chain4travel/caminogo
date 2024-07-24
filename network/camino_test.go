// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/perms"
)

// Convinient way to run generateTestKeyAndCertFile. Comment out SkipNow before run.
func TestGenerateTestCert(t *testing.T) {
	t.SkipNow()
	for i := 1; i <= 3; i++ {
		require.NoError(t, generateTestKeyAndCertFile(
			fmt.Sprintf("test_key_%d.key", i),
			fmt.Sprintf("test_cert_%d.crt", i),
		))
	}
}

// Creates key and cert files. Those are used by tests in this package.
func generateTestKeyAndCertFile(keyPath, certPath string) error {
	certBytes, keyBytes, err := staking.NewCertAndKeyBytesWithSecpKey(nil)
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

	return nil
}
