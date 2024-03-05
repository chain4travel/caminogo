// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package api

import "github.com/ava-labs/avalanchego/utils/formatting"

// JSONEncoding contains encoding type
type JSONEncoding struct {
	Encoding formatting.Encoding `json:"encoding"`
}
