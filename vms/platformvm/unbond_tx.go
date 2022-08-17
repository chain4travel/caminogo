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

// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"fmt"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/snow"
	"github.com/chain4travel/caminogo/utils/crypto"
	"github.com/chain4travel/caminogo/vms/components/avax"
)

var (
	_ UnsignedProposalTx = &UnsignedUnbondTx{}
)

// UnsignedUnbondTx is an unsigned unbondTx
type UnsignedUnbondTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// Describes the delegatee
	BondTxID ids.ID `serialize:"true" json:"bondTxID"`
}

// InitCtx sets the FxID fields in the inputs and outputs of this
// [UnsignedAddValidatorTx]. Also sets the [ctx] to the given [vm.ctx] so that
// the addresses can be json marshalled into human readable format
func (tx *UnsignedUnbondTx) InitCtx(ctx *snow.Context) {
	tx.BaseTx.InitCtx(ctx)
	// for _, out := range tx.Stake {
	// 	out.FxID = secp256k1fx.ID
	// 	out.InitCtx(ctx)
	// }
	// tx.RewardsOwner.InitCtx(ctx)
}

// SyntacticVerify returns nil if [tx] is valid
func (tx *UnsignedUnbondTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return errNilTx
	case tx.syntacticallyVerified: // already passed syntactic verification
		return nil
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return fmt.Errorf("failed to verify BaseTx: %w", err)
	}
	// if err := verify.All(&tx.Validator, tx.RewardsOwner); err != nil {
	// 	return fmt.Errorf("failed to verify validator or rewards owner: %w", err)
	// }

	// totalStakeWeight := uint64(0)
	// for _, out := range tx.Stake {
	// 	if err := out.Verify(); err != nil {
	// 		return fmt.Errorf("failed to verify output: %w", err)
	// 	}
	// 	newWeight, err := safemath.Add64(totalStakeWeight, out.Output().Amount())
	// 	if err != nil {
	// 		return err
	// 	}
	// 	totalStakeWeight = newWeight
	// }

	// switch {
	// case !avax.IsSortedTransferableOutputs(tx.Stake, Codec):
	// 	return errOutputsNotSorted
	// }

	// cache that this is valid
	tx.syntacticallyVerified = true
	return nil
}

// Attempts to verify this transaction with the provided state.
func (tx *UnsignedUnbondTx) SemanticVerify(vm *VM, parentState MutableState, stx *Tx) error {
	_, _, err := tx.Execute(vm, parentState, stx)
	return err
}

// Execute this transaction.
func (tx *UnsignedUnbondTx) Execute(
	vm *VM,
	parentState MutableState,
	stx *Tx,
) (
	VersionedState,
	VersionedState,
	error,
) {
	// Verify the tx is well-formed
	if err := tx.SyntacticVerify(vm.ctx); err != nil {
		return nil, nil, err
	}

	if vm.bootstrapped.GetValue() {
		// Verify the flowcheck
		// TODO@ fee
		if err := vm.semanticVerifySpend(parentState, tx, tx.Ins, tx.Outs, stx.Creds, vm.AddStakerTxFee, vm.ctx.AVAXAssetID, spendModeUnbond); err != nil {
			return nil, nil, fmt.Errorf("failed semanticVerifySpend: %w", err)
		}
	}

	// Set up the state if this tx is committed
	lockedUTXOsState := parentState.LockedUTXOsChainState()
	currentStakers := parentState.CurrentStakerChainState()
	pendingStakers := parentState.PendingStakerChainState()

	txID := tx.ID()

	var updatedUTXOs []lockedUTXOState

	// updating lock state for unbonded utxos
	bondedUTXOIDs := lockedUTXOsState.GetBondedUTXOs(tx.BondTxID)
	for utxoID := range *bondedUTXOIDs {
		updatedUTXOs = append(updatedUTXOs, lockedUTXOState{
			utxoID: utxoID,
			lockState: lockState{
				bondTxID:    nil,
				depositTxID: lockedUTXOsState.GetUTXOLockState(utxoID).depositTxID,
			},
		})
	}

	newlyLockedOutsState := lockedUTXOsState.UpdateUTXOs(updatedUTXOs)

	onCommitState := newVersionedState(parentState, currentStakers, pendingStakers, newlyLockedOutsState)

	// Consume the UTXOS
	consumeInputs(onCommitState, tx.Ins)
	// Produce the UTXOS
	produceOutputs(onCommitState, txID, vm.ctx.AVAXAssetID, tx.Outs)

	// Set up the state if this tx is aborted
	onAbortState := newVersionedState(parentState, currentStakers, pendingStakers, lockedUTXOsState)
	// Consume the UTXOS
	consumeInputs(onAbortState, tx.Ins)
	// Produce the UTXOS
	produceOutputs(onAbortState, txID, vm.ctx.AVAXAssetID, tx.Outs)

	return onCommitState, onAbortState, nil
}

// InitiallyPrefersCommit returns true if the proposed validators start time is
// after the current wall clock time,
func (tx *UnsignedUnbondTx) InitiallyPrefersCommit(vm *VM) bool {
	return true
}

// newUnbondTx returns a new unbondTx
func (vm *VM) newUnbondTx(
	bondTxID ids.ID, //
	keys []*crypto.PrivateKeySECP256K1R, // Keys providing the staked tokens
	changeAddr ids.ShortID, // Address to send change to, if there is any
) (*Tx, error) {
	// bondedUTXOIDs := vm.internalState.LockedUTXOsChainState().GetBondedUTXOs(bondTxID)
	// bondedUTXOs := make([]*avax.UTXO, bondedUTXOIDs.Len())
	// // TODO@ or just pass ids to spend and do the check; fee is problem
	// for bondedUTXOID := range *bondedUTXOIDs {
	// 	utxo, err := vm.internalState.GetUTXO(bondedUTXOID)
	// 	if err != nil {
	// 		return nil, nil // TODO@ err
	// 	}
	// 	bondedUTXOs = append(bondedUTXOs, utxo)
	// }

	ins, bondedOuts, unbondedOuts, signers, err := vm.spend(keys, 0, vm.AddStakerTxFee, changeAddr, spendModeBond) // TODO@ fee
	if err != nil {
		return nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}
	if len(bondedOuts) > 0 {
		return nil, nil // TODO@ err
	}
	utx := &UnsignedUnbondTx{
		BaseTx: BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         unbondedOuts, // TODO@ + bondedOuts
		}},
		BondTxID: bondTxID,
	}
	tx := &Tx{UnsignedTx: utx}
	if err := tx.Sign(Codec, signers); err != nil {
		return nil, err
	}
	return tx, utx.SyntacticVerify(vm.ctx)
}
