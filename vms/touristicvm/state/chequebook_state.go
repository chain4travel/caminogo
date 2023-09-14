// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
)

var _ Chequebook = (*chequebookState)(nil)

type Cheque struct {
	Amount   uint64
	SerialID uint64
}
type IssuerAgent struct {
	Issuer ids.ShortID
	Agent  ids.ShortID
}
type Chequebook interface {
	GetLastCheque(issuerAgent IssuerAgent, beneficiary ids.ShortID) (Cheque, error)
	SetLastCheque(issuerAgent IssuerAgent, beneficiary ids.ShortID, cheque Cheque)
}

type chequebookState struct {
	lastCheque map[IssuerAgent]map[ids.ShortID]Cheque
}

func (s *chequebookState) GetLastCheque(issuerAgent IssuerAgent, beneficiary ids.ShortID) (Cheque, error) {
	if _, ok := s.lastCheque[issuerAgent][beneficiary]; !ok {
		return Cheque{}, database.ErrNotFound
	}
	return s.lastCheque[issuerAgent][beneficiary], nil

}

func (s *chequebookState) SetLastCheque(issuerAgent IssuerAgent, beneficiary ids.ShortID, cheque Cheque) {
	if _, ok := s.lastCheque[issuerAgent]; !ok {
		s.lastCheque[issuerAgent] = make(map[ids.ShortID]Cheque)
	}
	s.lastCheque[issuerAgent][beneficiary] = cheque
}
