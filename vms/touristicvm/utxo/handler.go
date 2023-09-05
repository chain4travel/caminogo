// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************
// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utxo

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
)

var (
	_ Handler = (*handler)(nil)
)

// TODO: Stake and Authorize should be replaced by similar methods in the
// P-chain wallet
type Spender interface {
	// Spend the provided amount while deducting the provided fee.
	// Arguments:
	// - [keys] are the owners of the funds
	// - [amount] is the amount of funds that are trying to be staked
	// - [fee] is the amount of AVAX that should be burned
	// - [changeAddr] is the address that change, if there is any, is sent to
	// Returns:
	// - [inputs] the inputs that should be consumed to fund the outputs
	// - [returnedOutputs] the outputs that should be immediately returned to
	//                     the UTXO set
	// - [stakedOutputs] the outputs that should be locked for the duration of
	//                   the staking period
	// - [signers] the proof of ownership of the funds being moved
	Spend(
		utxoReader avax.UTXOReader,
		keys []*secp256k1.PrivateKey,
		amount uint64,
		fee uint64,
		changeAddr ids.ShortID,
	) (
		[]*avax.TransferableInput, // inputs
		[]*avax.TransferableOutput, // returnedOutputs
		[][]*secp256k1.PrivateKey, // signers
		error,
	)
	CaminoSpender
}

type Handler interface {
	Spender
	Verifier

	SumUpUtxos(utxos []*avax.UTXO) uint64
}

func NewHandler(
	ctx *snow.Context,
	clk *mockable.Clock,
	fx fx.Fx,
) Handler {
	return &handler{
		ctx: ctx,
		clk: clk,
		fx:  fx,
	}
}

type handler struct {
	ctx *snow.Context
	clk *mockable.Clock
	fx  fx.Fx
}

func (h *handler) SumUpUtxos(utxos []*avax.UTXO) uint64 {
	sum := uint64(0)
	for _, utxo := range utxos {

		if out, ok := utxo.Out.(avax.TransferableOut); ok {
			sum += out.Amount()
		} else if lockedOut, ok := utxo.Out.(*locked.Out); ok {
			sum += lockedOut.Amount()
		}
	}
	return sum
}

func (h *handler) Spend(
	utxoReader avax.UTXOReader,
	keys []*secp256k1.PrivateKey,
	amount uint64,
	fee uint64,
	changeAddr ids.ShortID,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // returnedOutputs
	[][]*secp256k1.PrivateKey, // signers
	error,
) {
	//// TODO: nikos check from the beginning how spend should look like
	//addrs := set.NewSet[ids.ShortID](len(keys)) // The addresses controlled by [keys]
	//for _, key := range keys {
	//	addrs.Add(key.PublicKey().Address())
	//}
	//utxos, err := avax.GetAllUTXOs(utxoReader, addrs) // The UTXOs controlled by [keys]
	//if err != nil {
	//	return nil, nil, nil, fmt.Errorf("couldn't get UTXOs: %w", err)
	//}
	//
	//kc := secp256k1fx.NewKeychain(keys...) // Keychain consumes UTXOs and creates new ones
	//
	//// Minimum time this transaction will be issued at
	//now := uint64(h.clk.Time().Unix())
	//
	//ins := []*avax.TransferableInput{}
	//returnedOuts := []*avax.TransferableOutput{}
	//signers := [][]*secp256k1.PrivateKey{}
	//
	//// Amount of AVAX that has been burned
	//amountBurned := uint64(0)
	//
	//for _, utxo := range utxos {
	//	// If we have burned more AVAX than we need to,
	//	// then we have no need to consume more AVAX
	//	if amountBurned >= fee {
	//		break
	//	}
	//
	//	if assetID := utxo.AssetID(); assetID != h.ctx.AVAXAssetID {
	//		continue // We only care about burning AVAX, so ignore other assets
	//	}
	//
	//	out := utxo.Out
	//
	//	inIntf, inSigners, err := kc.Spend(out, now)
	//	if err != nil {
	//		// We couldn't spend this UTXO, so we skip to the next one
	//		continue
	//	}
	//	in, ok := inIntf.(avax.TransferableIn)
	//	if !ok {
	//		// Because we only use the secp Fx right now, this should never
	//		// happen
	//		continue
	//	}
	//
	//	// The remaining value is initially the full value of the input
	//	remainingValue := in.Amount()
	//
	//	// Burn any value that should be burned
	//	amountToBurn := math.Min(
	//		fee-amountBurned, // Amount we still need to burn
	//		remainingValue,   // Amount available to burn
	//	)
	//	amountBurned += amountToBurn
	//	remainingValue -= amountToBurn
	//
	//	// Add the input to the consumed inputs
	//	ins = append(ins, &avax.TransferableInput{
	//		UTXOID: utxo.UTXOID,
	//		Asset:  avax.Asset{ID: h.ctx.AVAXAssetID},
	//		In:     in,
	//	})
	//
	//	if remainingValue > 0 {
	//		// This input had extra value, so some of it must be returned
	//		returnedOuts = append(returnedOuts, &avax.TransferableOutput{
	//			Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
	//			Out: &secp256k1fx.TransferOutput{
	//				Amt: remainingValue,
	//				OutputOwners: secp256k1fx.OutputOwners{
	//					Locktime:  0,
	//					Threshold: 1,
	//					Addrs:     []ids.ShortID{changeAddr},
	//				},
	//			},
	//		})
	//	}
	//
	//	// Add the signers needed for this input to the set of signers
	//	signers = append(signers, inSigners)
	//}
	//
	//if amountBurned < fee {
	//	return nil, nil, nil, fmt.Errorf(
	//		"provided keys have balance %d but need (%d, %d)",
	//		amountBurned, fee, amount)
	//}
	//
	//avax.SortTransferableInputsWithSigners(ins, signers)  // sort inputs and keys
	//avax.SortTransferableOutputs(returnedOuts, txs.Codec) // sort outputs
	//
	//return ins, returnedOuts, signers, nil
	var change *secp256k1fx.OutputOwners
	if changeAddr != ids.ShortEmpty {
		change = &secp256k1fx.OutputOwners{
			Locktime:  0,
			Threshold: 1,
			Addrs:     []ids.ShortID{changeAddr},
		}
	}
	inputs, outputs, signers, _, err := h.Lock(utxoReader, keys, amount, fee, locked.StateUnlocked, nil, change, 0)
	if err != nil {
		return nil, nil, nil, err
	}
	return inputs, outputs, signers, nil
}
