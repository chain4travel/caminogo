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
// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/touristicvm/blocks"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/touristicvm/status"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
)

var (
	_ Diff = (*diff)(nil)

	ErrMissingParentState = errors.New("missing parent state")
)

type Diff interface {
	Chain

	Apply(State)
}

type diff struct {
	parentID      ids.ID
	stateVersions Versions

	timestamp time.Time

	currentSupply uint64

	addedTxs map[ids.ID]*txAndStatus

	// map of modified UTXOID -> *UTXO if the UTXO is nil, it has been removed
	modifiedUTXOs   map[ids.ID]*avax.UTXO
	modifiedPaidOut map[ids.ShortID]map[ids.ShortID]uint64
}

func (d *diff) GetPaidOut(issuer, beneficiary ids.ShortID) (uint64, error) {
	if amount, ok := d.modifiedPaidOut[issuer][beneficiary]; ok {
		return amount, nil
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	return parentState.GetPaidOut(issuer, beneficiary)
}

func (d *diff) SetPaidOut(issuer, beneficiary ids.ShortID, amount uint64) {
	if d.modifiedPaidOut[issuer] == nil {
		d.modifiedPaidOut[issuer] = make(map[ids.ShortID]uint64)
	}
	d.modifiedPaidOut[issuer][beneficiary] = amount
}

func (d *diff) LockedUTXOs(address ids.ShortID) ([]*avax.UTXO, error) {
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}

	retUtxos, err := parentState.LockedUTXOs(address)
	if err != nil {
		return nil, err
	}

	// Apply modifiedUTXO's
	// Step 1: remove / update existing UTXOs
	remaining := set.NewSet[ids.ID](len(d.modifiedUTXOs))
	for k := range d.modifiedUTXOs {
		remaining.Add(k)
	}
	for i := len(retUtxos) - 1; i >= 0; i-- {
		utxoID := retUtxos[i].InputID()
		if utxo, exists := d.modifiedUTXOs[utxoID]; exists {
			if utxo == nil {
				retUtxos = append(retUtxos[:i], retUtxos[i+1:]...)
			} else {
				retUtxos[i] = utxo
			}
			delete(remaining, utxoID)
		}
	}

	// Step 2: Append new UTXOs
	for utxoID := range remaining {
		if utxo := d.modifiedUTXOs[utxoID]; utxo != nil {
			if _, ok := utxo.Out.(*locked.Out); ok {
				retUtxos = append(retUtxos, utxo)
			}
		}
	}

	return retUtxos, nil
}

func (d *diff) DeleteUTXO(utxoID ids.ID) {
	if d.modifiedUTXOs == nil {
		d.modifiedUTXOs = map[ids.ID]*avax.UTXO{
			utxoID: nil,
		}
	} else {
		d.modifiedUTXOs[utxoID] = nil
	}
}

func (d *diff) GetUTXO(utxoID ids.ID) (*avax.UTXO, error) {
	utxo, modified := d.modifiedUTXOs[utxoID]
	if !modified {
		parentState, ok := d.stateVersions.GetState(d.parentID)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
		}
		return parentState.GetUTXO(utxoID)
	}
	if utxo == nil {
		return nil, database.ErrNotFound
	}
	return utxo, nil
}

func (d *diff) GetBlock(blkID ids.ID) (blocks.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (d *diff) AddUTXO(utxo *avax.UTXO) {
	if d.modifiedUTXOs == nil {
		d.modifiedUTXOs = map[ids.ID]*avax.UTXO{
			utxo.InputID(): utxo,
		}
	} else {
		d.modifiedUTXOs[utxo.InputID()] = utxo
	}
}

func (d *diff) AddBlock(block blocks.Block) {
	//TODO implement me
	panic("implement me")
}

func NewDiff(
	parentID ids.ID,
	stateVersions Versions,
) (Diff, error) {
	parentState, ok := stateVersions.GetState(parentID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrMissingParentState, parentID)
	}
	return &diff{
		parentID:      parentID,
		stateVersions: stateVersions,
		timestamp:     parentState.GetTimestamp(),

		modifiedPaidOut: make(map[ids.ShortID]map[ids.ShortID]uint64),
	}, nil
}

func (d *diff) GetTimestamp() time.Time {
	return d.timestamp
}

func (d *diff) SetTimestamp(timestamp time.Time) {
	d.timestamp = timestamp
}

func (d *diff) GetCurrentSupply() (uint64, error) {

	// If the subnet supply wasn't modified in this diff, ask the parent state.
	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.GetCurrentSupply()
}

func (d *diff) SetCurrentSupply(currentSupply uint64) {
	d.currentSupply = currentSupply
}

func (d *diff) GetTx(txID ids.ID) (*txs.Tx, status.Status, error) {
	if tx, exists := d.addedTxs[txID]; exists {
		return tx.tx, tx.status, nil
	}

	parentState, ok := d.stateVersions.GetState(d.parentID)
	if !ok {
		return nil, status.Unknown, fmt.Errorf("%w: %s", ErrMissingParentState, d.parentID)
	}
	return parentState.GetTx(txID)
}

func (d *diff) AddTx(tx *txs.Tx, status status.Status) {
	txID := tx.ID()
	txStatus := &txAndStatus{
		tx:     tx,
		status: status,
	}
	if d.addedTxs == nil {
		d.addedTxs = map[ids.ID]*txAndStatus{
			txID: txStatus,
		}
	} else {
		d.addedTxs[txID] = txStatus
	}
}

func (d *diff) Apply(baseState State) {
	baseState.SetTimestamp(d.timestamp)
	baseState.SetCurrentSupply(d.currentSupply)
	for _, tx := range d.addedTxs {
		baseState.AddTx(tx.tx, tx.status)
	}
	for utxoID, utxo := range d.modifiedUTXOs {
		if utxo != nil {
			baseState.AddUTXO(utxo)
		} else {
			baseState.DeleteUTXO(utxoID)
		}
	}
	for issuer, beneficiaryToAmount := range d.modifiedPaidOut {
		for beneficiary, amount := range beneficiaryToAmount {
			baseState.SetPaidOut(issuer, beneficiary, amount)
		}
	}
}
