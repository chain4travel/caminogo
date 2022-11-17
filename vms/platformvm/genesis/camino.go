// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/ids"
)

// Camino genesis args
type Camino struct {
	VerifyNodeSignature bool           `serialize:"true" json:"verifyNodeSignature"`
	LockModeBondDeposit bool           `serialize:"true" json:"lockModeBondDeposit"`
	InitialAdmin        ids.ShortID    `serialize:"true" json:"initialAdmin"`
	DepositOffers       []DepositOffer `serialize:"true" json:"depositOffers"` // TODO@ remove from here
}

type DepositOffer struct {
	GradualUnlock         bool   `serialize:"true"`
	InterestRateNominator uint64 `serialize:"true"`
	Start                 uint64 `serialize:"true"`
	End                   uint64 `serialize:"true"`
	MinAmount             uint64 `serialize:"true"`
	Duration              uint64 `serialize:"true"`
}
