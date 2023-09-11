// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
)

type Cheque struct {
	Amount   uint64
	SerialID uint64
}

type Chequebook interface {
	GetLastCheque(issuer, beneficiary ids.ShortID) (Cheque, error)
	SetLastCheque(issuer, beneficiary ids.ShortID, cheque Cheque)
}

type chequebookState struct {
	lastCheque map[ids.ShortID]map[ids.ShortID]Cheque
}

func (s *chequebookState) GetLastCheque(issuer, beneficiary ids.ShortID) (Cheque, error) {
	if _, ok := s.lastCheque[issuer][beneficiary]; !ok {
		return Cheque{}, database.ErrNotFound
	}
	return s.lastCheque[issuer][beneficiary], nil

}

func (s *chequebookState) SetLastCheque(issuer, beneficiary ids.ShortID, cheque Cheque) {
	if _, ok := s.lastCheque[issuer]; !ok {
		s.lastCheque[issuer] = make(map[ids.ShortID]Cheque)
	}
	s.lastCheque[issuer][beneficiary] = cheque
}
