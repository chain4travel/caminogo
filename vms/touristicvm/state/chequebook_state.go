// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
)

type Chequebook interface {
	GetPaidOut(issuer, beneficiary ids.ShortID) (uint64, error)
	SetPaidOut(issuer, beneficiary ids.ShortID, amount uint64)
}

type chequebookState struct {
	paidOut map[ids.ShortID]map[ids.ShortID]uint64
}

func (s *chequebookState) GetPaidOut(issuer, beneficiary ids.ShortID) (uint64, error) {
	if _, ok := s.paidOut[issuer][beneficiary]; !ok {
		return 0, database.ErrNotFound
	}
	return s.paidOut[issuer][beneficiary], nil

}

func (s *chequebookState) SetPaidOut(issuer, beneficiary ids.ShortID, amount uint64) {
	s.paidOut[issuer][beneficiary] = amount
}
