package secp256k1fx

import "golang.org/x/exp/slices"

func (in *TransferInput) Equal(to any) bool {
	toIn, ok := to.(*TransferInput)
	return ok && in.Amt == toIn.Amt && slices.Equal(in.SigIndices, toIn.SigIndices)
}

func (out *TransferOutput) Equal(to any) bool {
	toOut, ok := to.(*TransferOutput)
	return ok && out.Amt == toOut.Amt && out.OutputOwners.Equals(&toOut.OutputOwners)
}
