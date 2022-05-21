// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dao

import (
	"errors"
	"time"

	"github.com/chain4travel/caminogo/codec"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/hashing"
)

var (
	errInvalidID              = errors.New("cannot build proposal id")
	errWeightTooSmall         = errors.New("lock amount too small")
	errStartTimeBeforeEndTime = errors.New("startTime must be before endTime")

	Codec codec.Manager
)

const (
	ProposalTypeAddValidator uint64 = iota
	ProposalTypeNetwork
	ProposalTypeMax
)

// Dao Proposal.
type DaoProposal struct {
	// The ID of this proposal
	ProposalID ids.ID `serialize:"true" json:"id"`

	// The threshold of votes which has to be reached (nodeId)
	Proposer ids.ShortID `serialize:"true" json:"proposer"`

	// The ProposalType of this proposal
	ProposalType uint64 `serialize:"true" json:"proposalType"`

	// The weight of this proposal (aka locked amount)
	Wght uint64 `serialize:"true" json:"weight"`

	// The threshold of votes which has to be reached
	Thresh uint32 `serialize:"true" json:"threshold"`

	// Unix time this proposal starts
	Start uint64 `serialize:"true" json:"startTime"`

	// Unix time this Dao finishes
	End uint64 `serialize:"true" json:"endTime"`

	// Proposal payload
	Data []byte `serialize:"true" json:"proposal"`
}

// ID returns the node ID of the validator
func (d *DaoProposal) ID() ids.ID { return d.ProposalID }

// StartTime is the time that this validator will enter the validator set
func (d *DaoProposal) StartTime() time.Time { return time.Unix(int64(d.Start), 0) }

// EndTime is the time that this validator will leave the validator set
func (d *DaoProposal) EndTime() time.Time { return time.Unix(int64(d.End), 0) }

// Duration is the amount of time that this validator will be in the validator set
func (d *DaoProposal) Duration() time.Duration { return d.EndTime().Sub(d.StartTime()) }

// Weight is this validator's weight when sampling
func (d *DaoProposal) Weight() uint64 { return d.Wght }

// Weight is this validator's weight when sampling
func (d *DaoProposal) Due(currentTime time.Time) bool { return currentTime.After(d.EndTime()) }

// Computes the id of this proposal
func (d *DaoProposal) computeID() (ids.ID, error) {
	data := append(d.Proposer.Bytes(), d.Data...)

	toSerialize := [5]uint64{d.ProposalType, d.Wght, d.Start, d.End, uint64(d.Thresh)}
	if typeBytes, err := Codec.Marshal(0, toSerialize); err != nil {
		return ids.ID{}, err
	} else {
		data = append(data, typeBytes...)
	}
	return hashing.ComputeHash256Array(data), nil
}

// Initializes the DaoProposalID
func (d *DaoProposal) InitializeID() error {
	if id, err := d.computeID(); err != nil {
		return err
	} else {
		d.ProposalID = id
	}
	return nil
}

// Verify validates the start / end of the proposal
func (d *DaoProposal) Verify() error {
	if id, err := d.computeID(); err != nil {
		return errInvalidID
	} else if id != d.ProposalID {
		return errInvalidID
	}
	if d.Start >= d.End {
		return errStartTimeBeforeEndTime
	}
	if d.Wght == 0 {
		return errWeightTooSmall
	}
	return nil
}

func SetCodecManager(codec codec.Manager) { Codec = codec }
