// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package utxo

import (
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
	"go.uber.org/zap"
)

type Locker interface {
	// Lock the provided amount while deducting the provided fee.
	// Arguments:
	// - [keys] are the owners of the funds
	// - [totalAmountToLock] is the amount of funds that are trying to be locked with [appliedLockState]
	// - [totalAmountToBurn] is the amount of AVAX that should be burned
	// - [appliedLockState] state to set
	// - [to] owner of unlocked amounts if appliedLockState is Unlocked
	// - [change] owner of unlocked amounts resulting from splittig inputs
	// - [asOf] timestamp against LockTime is compared
	// Returns:
	// - [inputs] the inputs that should be consumed to fund the outputs
	// - [outputs] the outputs that should be returned to the UTXO set
	// - [signers] the proof of ownership of the funds being moved
	// - [owners] the owners used for proof of ownership, used e.g. for multiSig
	Lock(
		utxoDB avax.UTXOReader,
		keys []*secp256k1.PrivateKey,
		totalAmountToLock uint64,
		totalAmountToBurn uint64,
		appliedLockState locked.State,
		to *secp256k1fx.OutputOwners,
		change *secp256k1fx.OutputOwners,
		asOf uint64,
	) (
		[]*avax.TransferableInput, // inputs
		[]*avax.TransferableOutput, // outputs
		[][]*secp256k1.PrivateKey, // signers
		[]*secp256k1fx.OutputOwners, // owners
		error,
	)
}

func (h *handler) Lock(
	utxoDB avax.UTXOReader,
	keys []*secp256k1.PrivateKey,
	totalAmountToLock uint64,
	totalAmountToBurn uint64,
	appliedLockState locked.State,
	to *secp256k1fx.OutputOwners,
	change *secp256k1fx.OutputOwners,
	asOf uint64,
) (
	[]*avax.TransferableInput, // inputs
	[]*avax.TransferableOutput, // outputs
	[][]*secp256k1.PrivateKey, // signers
	[]*secp256k1fx.OutputOwners, // owners
	error,
) {
	switch appliedLockState {
	case locked.StateLocked,
		locked.StateUnlocked:
	default:
		return nil, nil, nil, nil, errInvalidTargetLockState
	}

	addrs, signer := secp256k1fx.ExtractFromAndSigners(keys)

	utxos, err := avax.GetAllUTXOs(utxoDB, addrs) // The UTXOs controlled by [keys]
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("couldn't get UTXOs: %w", err)
	}

	sortUTXOs(utxos, h.ctx.AVAXAssetID, appliedLockState)

	kc := secp256k1fx.NewKeychain(signer...) // Keychain consumes UTXOs and creates new ones

	// Minimum time this transaction will be issued at
	now := asOf
	if now == 0 {
		now = uint64(h.clk.Time().Unix())
	}

	ins := []*avax.TransferableInput{}
	outs := []*avax.TransferableOutput{}
	signers := [][]*secp256k1.PrivateKey{}
	owners := []*secp256k1fx.OutputOwners{}

	// Amount of AVAX that has been locked
	totalAmountLocked := uint64(0)

	// Amount of AVAX that has been burned
	totalAmountBurned := uint64(0)

	type lockedAndRemainedAmounts struct {
		locked   uint64
		remained uint64
	}
	type OwnerID struct {
		owners   *secp256k1fx.OutputOwners
		ownersID *ids.ID
	}
	type OwnerAmounts struct {
		amounts map[ids.ID]lockedAndRemainedAmounts
		owners  secp256k1fx.OutputOwners
	}
	// Track the amount of transfers and their owners
	// if appliedLockState == bond, then otherLockTxID is depositTxID and vice versa
	// ownerID -> otherLockTxID -> AAAA
	insAmounts := make(map[ids.ID]OwnerAmounts)

	var toOwnerID *ids.ID
	if to != nil && appliedLockState == locked.StateUnlocked {
		id, err := txs.GetOwnerID(to)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		toOwnerID = &id
	}

	var changeOwnerID *ids.ID
	if change != nil {
		id, err := txs.GetOwnerID(change)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		changeOwnerID = &id
	}

	for i, utxo := range utxos {
		// If we have consumed more AVAX than we are trying to lock,
		// and we have burned more AVAX than we need to,
		// then we have no need to consume more AVAX
		if totalAmountBurned >= totalAmountToBurn && totalAmountLocked >= totalAmountToLock {
			break
		}

		// We only care about locking AVAX,
		// and because utxos are sorted we can skip other utxos
		if assetID := utxo.AssetID(); assetID != h.ctx.AVAXAssetID {
			break
		}

		out := utxo.Out
		lockIDs := locked.IDsEmpty
		if lockedOut, ok := out.(*locked.Out); ok {
			// Resolves to true for StateUnlocked
			if lockedOut.IsLockedWith(appliedLockState) {
				// This output can't be locked with target lockState,
				// and because utxos are sorted we can skip other utxos
				break
			}
			out = lockedOut.TransferableOut
			lockIDs = lockedOut.IDs
		}

		innerOut, ok := out.(*secp256k1fx.TransferOutput)
		if !ok {
			// We only know how to clone secp256k1 outputs for now
			continue
		}

		outOwnerID, err := txs.GetOutputOwnerID(out)
		if err != nil {
			// We couldn't get owner of this output, so move on to the next one
			continue
		}

		inIntf, inSigners, err := kc.SpendMultiSig(innerOut, now, utxoDB)
		if err != nil {
			// We couldn't spend the output, so move on to the next one
			continue
		}
		in, ok := inIntf.(avax.TransferableIn)
		if !ok { // should never happen
			h.ctx.Log.Warn("wrong input type",
				zap.String("expectedType", "avax.TransferableIn"),
				zap.String("actualType", fmt.Sprintf("%T", inIntf)),
			)
			continue
		}

		remainingValue := in.Amount()

		lockedOwnerID := OwnerID{&innerOut.OutputOwners, &outOwnerID}
		remainingOwnerID := lockedOwnerID

		if !lockIDs.IsLocked() {
			// Burn any value that should be burned
			amountToBurn := math.Min(
				totalAmountToBurn-totalAmountBurned, // Amount we still need to burn
				remainingValue,                      // Amount available to burn
			)
			totalAmountBurned += amountToBurn
			remainingValue -= amountToBurn

			if toOwnerID != nil {
				lockedOwnerID = OwnerID{to, toOwnerID}
			}
			if changeOwnerID != nil && !h.isMultisigTransferOutput(utxoDB, innerOut) {
				remainingOwnerID = OwnerID{change, changeOwnerID}
			}
		}

		// Lock any value that should be locked
		amountToLock := math.Min(
			totalAmountToLock-totalAmountLocked, // Amount we still need to lock
			remainingValue,                      // Amount available to lock
		)
		totalAmountLocked += amountToLock
		remainingValue -= amountToLock
		h.ctx.Log.Debug("for utxo ", zap.Int("index", i), zap.Uint64("amountToLock", amountToLock), zap.Uint64("remainingValue", remainingValue), zap.Uint64("totalAmountLocked", totalAmountLocked), zap.Uint64("totalAmountToBurn", totalAmountToBurn))

		if amountToLock > 0 || totalAmountToBurn > 0 {
			if lockIDs.IsLocked() {
				in = &locked.In{
					IDs:            lockIDs,
					TransferableIn: in,
				}
			}

			ins = append(ins, &avax.TransferableInput{
				UTXOID: avax.UTXOID{
					TxID:        utxo.TxID,
					OutputIndex: utxo.OutputIndex,
				},
				Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
				In:    in,
			})
			signers = append(signers, inSigners)
			owners = append(owners, &innerOut.OutputOwners)

			ownerAmounts, ok := insAmounts[*lockedOwnerID.ownersID]
			if !ok {
				ownerAmounts = OwnerAmounts{
					amounts: make(map[ids.ID]lockedAndRemainedAmounts),
					owners:  *lockedOwnerID.owners,
				}
			}

			amounts := ownerAmounts.amounts[lockIDs.LockTxID]
			newAmount, err := math.Add64(amounts.locked, amountToLock)
			if err != nil {
				return nil, nil, nil, nil, err
			}

			amounts.locked = newAmount
			ownerAmounts.amounts[lockIDs.LockTxID] = amounts
			if !ok {
				insAmounts[*lockedOwnerID.ownersID] = ownerAmounts
			}

			ownerAmounts, ok = insAmounts[*remainingOwnerID.ownersID]
			if !ok {
				ownerAmounts = OwnerAmounts{
					amounts: make(map[ids.ID]lockedAndRemainedAmounts),
					owners:  *remainingOwnerID.owners,
				}
			}

			amounts = ownerAmounts.amounts[lockIDs.LockTxID]
			newAmount, err = math.Add64(amounts.remained, remainingValue)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			amounts.remained = newAmount

			ownerAmounts.amounts[lockIDs.LockTxID] = amounts
			if !ok {
				insAmounts[*remainingOwnerID.ownersID] = ownerAmounts
			}
		}
	}

	for _, ownerAmounts := range insAmounts {
		addOut := func(amt uint64, lockIDs locked.IDs, collect bool) uint64 {
			if amt == 0 {
				return 0
			}
			if lockIDs.IsLocked() {
				h.ctx.Log.Debug("creating locked output: ", zap.Uint64("amt", amt), zap.String("owner", ownerAmounts.owners.Addrs[0].String()))
				outs = append(outs, &avax.TransferableOutput{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &locked.Out{
						IDs: lockIDs,
						TransferableOut: &secp256k1fx.TransferOutput{
							Amt:          amt,
							OutputOwners: ownerAmounts.owners,
						},
					},
				})
			} else {
				if collect {
					return amt
				}
				h.ctx.Log.Debug("creating unlocked output: ", zap.Uint64("amt", amt), zap.String("owner", ownerAmounts.owners.Addrs[0].String()))
				outs = append(outs, &avax.TransferableOutput{
					Asset: avax.Asset{ID: h.ctx.AVAXAssetID},
					Out: &secp256k1fx.TransferOutput{
						Amt:          amt,
						OutputOwners: ownerAmounts.owners,
					},
				})
			}
			return 0
		}

		for otherLockTxID, amounts := range ownerAmounts.amounts {
			lockIDs := locked.IDs{}
			switch appliedLockState {
			case locked.StateLocked:
				lockIDs.LockTxID = otherLockTxID
			}

			// If out is unlocked no UTXO is written instead the amount is returned.
			// We apply the unlocked amount in the remaining step to compact UTXOs
			unlockAmount := addOut(amounts.locked, lockIDs.Lock(appliedLockState), true)
			if unlockAmount, err = math.Add64(unlockAmount, amounts.remained); err != nil {
				return nil, nil, nil, nil, err
			}
			addOut(unlockAmount, lockIDs, false)
		}
	}

	if totalAmountBurned < totalAmountToBurn || totalAmountLocked < totalAmountToLock {
		h.ctx.Log.Debug("insufficient balance: ", zap.Uint64("totalAmountBurned", totalAmountBurned), zap.Uint64("totalAmountToBurn", totalAmountToBurn), zap.Uint64("totalAmountLocked", totalAmountLocked), zap.Uint64("totalAmountToLock", totalAmountToLock))
		return nil, nil, nil, nil, errInsufficientBalance
	}

	avax.SortTransferableInputsWithSigners(ins, signers) // sort inputs and keys
	avax.SortTransferableOutputs(outs, txs.Codec)        // sort outputs

	return ins, outs, signers, owners, nil
}
