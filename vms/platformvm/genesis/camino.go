// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"encoding/binary"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
)

// Camino genesis args
type Camino struct {
	VerifyNodeSignature      bool            `serialize:"true" json:"verifyNodeSignature"`
	LockModeBondDeposit      bool            `serialize:"true" json:"lockModeBondDeposit"`
	InitialAdmin             ids.ShortID     `serialize:"true" json:"initialAdmin"`
	DepositOffers            []DepositOffer  `serialize:"true" json:"depositOffers"`
	InitialMultisigAddresses []MultisigAlias `serialize:"true" json:"initialMultisigAddresses"`
}

type DepositOffer struct {
	UnlockHalfPeriodDuration uint32 `serialize:"true" json:"unlockHalfPeriodDuration"`
	InterestRateNominator    uint64 `serialize:"true" json:"interestRateNominator"`
	Start                    uint64 `serialize:"true" json:"start"`
	End                      uint64 `serialize:"true" json:"end"`
	MinAmount                uint64 `serialize:"true" json:"minAmount"`
	MinDuration              uint32 `serialize:"true" json:"minDuration"`
	MaxDuration              uint32 `serialize:"true" json:"maxDuration"`
}

type MultisigAlias struct {
	Alias     ids.ShortID   `serialize:"true" json:"alias"`
	Threshold uint32        `serialize:"true" json:"threshold"`
	Addresses []ids.ShortID `serialize:"true" json:"addresses"`
}

func NewMultisigAlias(txID ids.ID, addresses []ids.ShortID, threshold uint32) (MultisigAlias, error) {
	var err error
	sorted_addrs := addresses[:]
	ids.SortShortIDs(sorted_addrs)

	ma := MultisigAlias{
		Addresses: sorted_addrs,
		Threshold: threshold,
	}

	ma.Alias, err = ma.ComputeAlias(txID)

	return ma, err
}

func (ma *MultisigAlias) ComputeAlias(txID ids.ID) (ids.ShortID, error) {
	// double check passed addresses are sorted, as required
	if !ids.IsSortedAndUniqueShortIDs(ma.Addresses) {
		return ids.ShortEmpty, fmt.Errorf("addresses must be sorted and unique")
	}

	txIDBytes := [32]byte(txID)
	thresholdBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(thresholdBytes, ma.Threshold)
	allBytes := make([]byte, 32+4+20*len(ma.Addresses))

	copy(allBytes, txIDBytes[:])
	copy(allBytes[32:], thresholdBytes[:])

	beg := 32 + 4
	for _, addr := range ma.Addresses {
		copy(allBytes[beg:], addr.Bytes())
		beg += 20
	}

	idHash := hashing.ComputeHash256(allBytes)
	alias, _ := ids.ToShortID(idHash[:20])
	return alias, nil
}
