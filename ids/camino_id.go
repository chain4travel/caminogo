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
// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package ids

import "encoding/binary"

// FromInt converts an int to an ID
//
// Examples:
//
//	FromInt(0).Hex() == "0000000000000000000000000000000000000000000000000000000000000000"
//	FromInt(1).Hex() == "0100000000000000000000000000000000000000000000000000000000000000"
//	FromInt(math.MaxUint64).Hex() == "ffffffffffffffff000000000000000000000000000000000000000000000000"
func FromInt(idx uint64) ID {
	bytes := [32]byte{}
	binary.LittleEndian.PutUint64(bytes[:], idx)
	id := ID(bytes)
	return id
}
