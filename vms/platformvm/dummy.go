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

package platformvm

// Simple Struct to fill the gap in the codec left
// by the removal of the addDelegatorTx
type Dummy struct {
	DummyField [1]byte `serialize:"true"`
}
