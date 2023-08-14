// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256k1fx

import "golang.org/x/exp/slices"

func (in *TransferInput) Equal(to any) bool {
	toIn, ok := to.(*TransferInput)
	return ok && in.Amt == toIn.Amt && slices.Equal(in.SigIndices, toIn.SigIndices)
}
