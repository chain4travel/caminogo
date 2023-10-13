// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import "github.com/ava-labs/avalanchego/database"

const (
	IsInitializedKey byte = iota
)

var (
	isInitializedKey = []byte{IsInitializedKey}
	timestampKey     = []byte("timestamp")
	currentSupplyKey = []byte("current supply")
	lastAcceptedKey  = []byte("last accepted")
)

type MetadataState struct {
	database.Database
}

func (s *MetadataState) IsInitialized() (bool, error) {
	return s.Has(isInitializedKey)
}

func (s *MetadataState) SetInitialized() error {
	return s.Put(isInitializedKey, nil)
}
