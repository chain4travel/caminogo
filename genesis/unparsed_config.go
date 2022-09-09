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

package genesis

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/constants"
	"github.com/chain4travel/caminogo/utils/formatting"
)

var errInvalidETHAddress = errors.New("invalid eth address")

type UnparsedAllocation struct {
	ETHAddr        string         `json:"ethAddr"`
	AVAXAddr       string         `json:"avaxAddr"`
	InitialAmount  uint64         `json:"initialAmount"`
	UnlockSchedule []LockedAmount `json:"unlockSchedule"`
}

func (ua UnparsedAllocation) Parse() (Allocation, error) {
	a := Allocation{
		InitialAmount:  ua.InitialAmount,
		UnlockSchedule: ua.UnlockSchedule,
	}

	if len(ua.ETHAddr) < 2 {
		return a, errInvalidETHAddress
	}

	ethAddrBytes, err := hex.DecodeString(ua.ETHAddr[2:])
	if err != nil {
		return a, err
	}
	ethAddr, err := ids.ToShortID(ethAddrBytes)
	if err != nil {
		return a, err
	}
	a.ETHAddr = ethAddr

	_, _, avaxAddrBytes, err := formatting.ParseAddress(ua.AVAXAddr)
	if err != nil {
		return a, err
	}
	avaxAddr, err := ids.ToShortID(avaxAddrBytes)
	if err != nil {
		return a, err
	}
	a.AVAXAddr = avaxAddr

	return a, nil
}

type UnparsedStaker struct {
	NodeID        string `json:"nodeID"`
	RewardAddress string `json:"rewardAddress"`
	DelegationFee uint32 `json:"delegationFee"`
}

func (us UnparsedStaker) Parse() (Staker, error) {
	s := Staker{
		DelegationFee: us.DelegationFee,
	}

	nodeID, err := ids.ShortFromPrefixedString(us.NodeID, constants.NodeIDPrefix)
	if err != nil {
		return s, err
	}
	s.NodeID = nodeID

	_, _, avaxAddrBytes, err := formatting.ParseAddress(us.RewardAddress)
	if err != nil {
		return s, err
	}
	avaxAddr, err := ids.ToShortID(avaxAddrBytes)
	if err != nil {
		return s, err
	}
	s.RewardAddress = avaxAddr
	return s, nil
}

// UnparsedMultisigAlias defines a multisignature alias address.
// [Alias] is the alias of the multisignature address. It's encoded to string
// the same way as ShortID String() method does.
// [Addresses] are the addresses that are allowed to sign transactions from the multisignature address.
// All addresses are encoded to string the same way as ShortID String() method does.
// [Threshold] is the number of signatures required to sign transactions from the multisignature address.
type UnparsedMultisigAlias struct {
	Alias     string   `json:"alias"`
	Addresses []string `json:"addresses"`
	Threshold uint32   `json:"threshold"`
}

func (uma UnparsedMultisigAlias) Parse() (MultisigAlias, error) {
	ma := MultisigAlias{
		Threshold: uma.Threshold,
		Addresses: make([]ids.ShortID, len(uma.Addresses)),
	}

	var err error
	ma.Alias, err = ids.ShortFromString(uma.Alias)
	if err != nil {
		return ma, err
	}

	for i, addr := range uma.Addresses {
		ma.Addresses[i], err = ids.ShortFromString(addr)
		if err != nil {
			return ma, err
		}
	}

	return ma, nil
}

func (ma *UnparsedMultisigAlias) Validate() error {
	if ma.Threshold == 0 {
		return fmt.Errorf("multisig threshold must be greater than 0")
	}
	if ma.Threshold > uint32(len(ma.Addresses)) {
		return fmt.Errorf("multisig threshold exceeds the number of addresses")
	}
	return nil
}

// UnparsedConfig contains the genesis addresses used to construct a genesis
type UnparsedConfig struct {
	NetworkID uint32 `json:"networkID"`

	Allocations []UnparsedAllocation `json:"allocations"`

	StartTime                  uint64                  `json:"startTime"`
	InitialStakeDuration       uint64                  `json:"initialStakeDuration"`
	InitialStakeDurationOffset uint64                  `json:"initialStakeDurationOffset"`
	InitialStakedFunds         []string                `json:"initialStakedFunds"`
	InitialStakers             []UnparsedStaker        `json:"initialStakers"`
	InitialMultisigAddresses   []UnparsedMultisigAlias `json:"initialMultisigAddresses"`

	CChainGenesis string `json:"cChainGenesis"`

	Message string `json:"message"`
}

func (uc UnparsedConfig) Parse() (Config, error) {
	c := Config{
		NetworkID:                  uc.NetworkID,
		Allocations:                make([]Allocation, len(uc.Allocations)),
		StartTime:                  uc.StartTime,
		InitialStakeDuration:       uc.InitialStakeDuration,
		InitialStakeDurationOffset: uc.InitialStakeDurationOffset,
		InitialStakedFunds:         make([]ids.ShortID, len(uc.InitialStakedFunds)),
		InitialStakers:             make([]Staker, len(uc.InitialStakers)),
		InitialMultisigAddresses:   make([]MultisigAlias, len(uc.InitialMultisigAddresses)),
		CChainGenesis:              uc.CChainGenesis,
		Message:                    uc.Message,
	}
	for i, ua := range uc.Allocations {
		a, err := ua.Parse()
		if err != nil {
			return c, err
		}
		c.Allocations[i] = a
	}
	for i, isa := range uc.InitialStakedFunds {
		_, _, avaxAddrBytes, err := formatting.ParseAddress(isa)
		if err != nil {
			return c, err
		}
		avaxAddr, err := ids.ToShortID(avaxAddrBytes)
		if err != nil {
			return c, err
		}
		c.InitialStakedFunds[i] = avaxAddr
	}
	for i, uis := range uc.InitialStakers {
		is, err := uis.Parse()
		if err != nil {
			return c, err
		}
		c.InitialStakers[i] = is
	}

	if err := validateMultisigAddresses(uc.InitialMultisigAddresses); err != nil {
		return c, fmt.Errorf("failed to validate initial multisig addresses: %w", err)
	}
	for i, uma := range uc.InitialMultisigAddresses {
		ma, err := uma.Parse()
		if err != nil {
			return c, err
		}
		c.InitialMultisigAddresses[i] = ma
	}

	return c, nil
}

func validateMultisigAddresses(multisigAddrs []UnparsedMultisigAlias) error {
	addrs := make(map[string]struct{})
	aliases := make(map[string]struct{})

	for _, uma := range multisigAddrs {
		if err := uma.Validate(); err != nil {
			return err
		}

		// check alias was not previously defined
		if _, ok := aliases[uma.Alias]; ok {
			return fmt.Errorf("alias %s definition is duplicated", uma.Alias)
		}

		aliases[uma.Alias] = struct{}{}
		for _, addr := range uma.Addresses {
			addrs[addr] = struct{}{}
		}
	}

	// check alias was not used as an address of another alias
	for _, ma := range multisigAddrs {
		if _, ok := addrs[ma.Alias]; ok {
			return fmt.Errorf("alias %s is used as an address of another alias", ma.Alias)
		}
	}

	return nil
}
