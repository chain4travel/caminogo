// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package utxo

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"github.com/ava-labs/avalanchego/vms/touristicvm/state"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
)

var (
	errNotEnoughLockedFunds = errors.New("not enough locked funds")
)

type Unlocker interface {
	Unlock(
		state state.Chain,
		from *secp256k1fx.OutputOwners,
		to *secp256k1fx.OutputOwners,
		amount uint64,
	) (
		[]*avax.TransferableInput, // inputs
		[]*avax.TransferableOutput, // outputs
		error,
	)
}

func (h *handler) Unlock(
	state state.Chain,
	from *secp256k1fx.OutputOwners,
	to *secp256k1fx.OutputOwners,
	amount uint64,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // outputs
	error,
) {

	if len(from.Addrs) == 0 || len(from.Addrs) > 1 {
		return nil, nil, fmt.Errorf("invalid number of addresses: %d", len(from.Addrs))
	}

	utxos, err := state.LockedUTXOs(from.Addrs[0])
	// for utxos that are locked sum up the amount
	// if the sum is greater than the amount to unlock, return error
	if err != nil {
		return nil, nil, err
	}

	lockedAmount := uint64(0)
	for _, utxo := range utxos {
		lockedAmount += utxo.Out.(*locked.Out).TransferableOut.Amount()
	}
	if lockedAmount < amount {
		return nil, nil, errNotEnoughLockedFunds
	}
	var paidOut uint64
	if paidOut, err = state.GetPaidOut(from.Addrs[0], to.Addrs[0]); err != nil { //TODO nikos refactor
		if err != database.ErrNotFound {
			return nil, nil, err
		}
		paidOut = 0 // first attempt to cash out
	} else if paidOut >= amount {
		return nil, nil, fmt.Errorf("amount already paid out")
	}
	amountToUnlock := amount - paidOut

	return h.unlockUTXOs(utxos, from, to, amountToUnlock)
}

// utxos that are not locked with [removedLockState] will be ignored
func (h *handler) unlockUTXOs(
	utxos []*avax.UTXO,
	from *secp256k1fx.OutputOwners,
	to *secp256k1fx.OutputOwners,
	amountToUnlock uint64,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // outputs
	error,
) {

	ins := []*avax.TransferableInput{}
	outs := []*avax.TransferableOutput{}

	amountUnlocked := uint64(0)
	for _, utxo := range utxos {
		// already unlocked enough utxos
		if amountUnlocked == amountToUnlock {
			break
		}

		out, ok := utxo.Out.(*locked.Out)
		if !ok || !out.IsLockedWith(locked.StateLocked) {
			// This output isn't locked or doesn't have required lockState
			return nil, nil, errNotLockedUTXO
		}

		// if already unlocked amount + current utxo amount surpasses the desired amountToUnlock then a partial unlock is necessary
		// otherwise the whole utxo can be unlocked
		if amountUnlocked+out.TransferableOut.Amount() > amountToUnlock {

			innerOut, ok := out.TransferableOut.(*secp256k1fx.TransferOutput)
			if !ok {
				// We only know how to clone secp256k1 outputs for now
				return nil, nil, errWrongOutType
			}

			// Add the input to the consumed inputs
			ins = append(ins, &avax.TransferableInput{
				UTXOID: avax.UTXOID{
					TxID:        utxo.TxID,
					OutputIndex: utxo.OutputIndex,
				},
				Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
				In: &locked.In{
					IDs: out.IDs,
					TransferableIn: &secp256k1fx.TransferInput{
						Amt:   out.Amount(),
						Input: secp256k1fx.Input{SigIndices: []uint32{}},
					},
				},
			})

			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
				Out: &locked.Out{
					IDs: locked.IDs{LockTxID: out.IDs.LockTxID},
					TransferableOut: &secp256k1fx.TransferOutput{
						Amt:          innerOut.Amount() - (amountToUnlock - amountUnlocked),
						OutputOwners: *from,
					},
				},
			})
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt:          amountToUnlock - amountUnlocked,
					OutputOwners: *to,
				},
			})
			amountUnlocked += amountToUnlock - amountUnlocked // increment amount unlocked so far

		} else { // if utxo amount is less than the amountToUnlock then the whole utxo can be unlocked
			innerOut, ok := out.TransferableOut.(*secp256k1fx.TransferOutput)
			if !ok {
				// We only know how to clone secp256k1 outputs for now
				return nil, nil, errWrongOutType
			}

			// Add the input to the consumed inputs
			ins = append(ins, &avax.TransferableInput{
				UTXOID: avax.UTXOID{
					TxID:        utxo.TxID,
					OutputIndex: utxo.OutputIndex,
				},
				Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
				In: &locked.In{
					IDs: out.IDs,
					TransferableIn: &secp256k1fx.TransferInput{
						Amt:   out.Amount(),
						Input: secp256k1fx.Input{SigIndices: []uint32{}},
					},
				},
			})

			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
				Out: &secp256k1fx.TransferOutput{
					Amt:          innerOut.Amount(),
					OutputOwners: *to,
				},
			})
			amountUnlocked += innerOut.Amount()
		}
	}

	avax.SortTransferableInputs(ins)              // sort inputs
	avax.SortTransferableOutputs(outs, txs.Codec) // sort outputs

	return ins, outs, nil
}
