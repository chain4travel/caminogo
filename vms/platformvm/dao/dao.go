// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package dao

import (
	"errors"
	"time"

	"github.com/chain4travel/caminogo/codec"
)

var (
	errInvalidID                               = errors.New("cannot build proposal id")
	errWeightTooSmall                          = errors.New("lock amount too small")
	errStartTimeBeforeEndTime                  = errors.New("startTime must be before endTime")
	errVotingDurrationForAddValidatorIncorrect = errors.New("the voting durration to propose a new validator must be 2 weeks")
	errNonPositiveThreshold                    = errors.New("the number of votes to accept a proposal must be at least 1")

	Codec codec.Manager
)

type ProposalState int

const (
	ProposalStateUnknown ProposalState = iota
	ProposalStatePending
	ProposalStateAccepted
	ProposalStateConcluded
)

func (p ProposalState) String() string {
	switch p {
	case ProposalStatePending:
		return "Pending"
	case ProposalStateAccepted:
		return "Accepted"
	case ProposalStateConcluded:
		return "Concluded"
	default:
		return "Unknown"
	}
}

// Dao ProposalConfiguration.
type ProposalConfiguration struct {
	// The threshold of votes which has to be reached
	Thresh uint32 `serialize:"true" json:"threshold"`

	// Unix time this proposal starts
	Start uint64 `serialize:"true" json:"startTime"`

	// Unix time this Dao finishes
	End uint64 `serialize:"true" json:"endTime"`
}

func HasConcluded(state ProposalState) bool {
	return state == ProposalStateConcluded
}

// StartTime is the time that this validator will enter the validator set
func (d *ProposalConfiguration) StartTime() time.Time { return time.Unix(int64(d.Start), 0) }

// EndTime is the time that this validator will leave the validator set
func (d *ProposalConfiguration) EndTime() time.Time { return time.Unix(int64(d.End), 0) }

// Duration is the amount of time that this validator will be in the validator set
func (d *ProposalConfiguration) Duration() time.Duration { return d.EndTime().Sub(d.StartTime()) }

// Returns true one the proposal voting time has conlcueded
func (d *ProposalConfiguration) Due(currentTime time.Time) bool {
	return currentTime.After(d.EndTime())
}

// Verify validates the start / end of the proposal
func (d *ProposalConfiguration) Verify() error {
	if d.Start >= d.End {
		return errStartTimeBeforeEndTime
	}

	if d.Thresh > 0 {
		return errNonPositiveThreshold
	}
	return nil
}

func SetCodecManager(codec codec.Manager) { Codec = codec }
