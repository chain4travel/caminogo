// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package multisig

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/types"
)

// MaxMemoSize is the maximum number of bytes in the memo field
const MaxMemoSize = 256

type Alias struct {
	ID     ids.ShortID         `serialize:"true" json:"id"`
	Memo   types.JSONByteSlice `serialize:"true" json:"memo"`
	Owners verify.State        `serialize:"true" json:"owners"`
}

// AliasRaw is the definition of a Multisig alias used for storage
type AliasRaw struct {
	ID         ids.ShortID         `serialize:"true" json:"id"`
	Memo       types.JSONByteSlice `serialize:"true" json:"memo"`
	Threshold  uint32              `serialize:"true" json:"threshold"`
	PublicKeys []PublicKey         `serialize:"true" json:"pubKeyBytes"`
}

// Verify returns an error if the basic verification of the multisig AliasRaw fails
func (a *AliasRaw) Verify() error {
	if len(a.Memo) > MaxMemoSize {
		return fmt.Errorf("msig alias memo is larger (%d bytes) than max of %d bytes", len(a.Memo), MaxMemoSize)
	}

	if !utils.IsSortedAndUniqueSortable(a.PublicKeys) {
		return fmt.Errorf("multisig alias public keys are not sorted and unique")
	}

	if a.Threshold > uint32(len(a.PublicKeys)) {
		return fmt.Errorf("multisig alias threshold is greater than the number of public keys")
	}

	if a.Threshold == 0 {
		return fmt.Errorf("multisig alias threshold is 0")
	}

	if a.ID == ids.ShortEmpty {
		return fmt.Errorf("multisig alias is empty")
	}

	return nil
}

func (a *AliasRaw) VerifyState() error {
	return a.Verify()
}

type PublicKey [33]byte

func (pk PublicKey) Bytes() []byte {
	return pk[:]
}

func (pk PublicKey) String() string {
	return hex.EncodeToString(pk.Bytes())
}

func PublicKeyFromBytes(bytes []byte) (PublicKey, error) {
	pubKey := PublicKey{}
	if len(bytes) != 33 {
		return pubKey, fmt.Errorf("expected 33 bytes, got %d", len(bytes))
	}
	copy(pubKey[:], bytes)
	return pubKey, nil
}

func PublicKeyFromString(hexBytes string) (PublicKey, error) {
	bytes, err := hex.DecodeString(strings.TrimPrefix(hexBytes, "0x"))
	if err != nil {
		return PublicKey{}, err
	}
	return PublicKeyFromBytes(bytes)
}

func (pk PublicKey) Less(other PublicKey) bool {
	return bytes.Compare(pk[:], other[:]) < 0
}
