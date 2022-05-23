// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/chain4travel/caminogo/database"
	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/timer/mockable"
)

var (
	_ daoProposalChainState = &daoProposalChainStateImpl{}
	_ daoProposal           = &daoProposalImpl{}
)

type daoProposal interface {
	DaoProposalTx() *UnsignedDaoProposalTx
	Votes() []*UnsignedDaoVoteTx
	Voted(nodeID ids.ShortID) bool
}

type daoProposalImpl struct {
	daoProposalTx *UnsignedDaoProposalTx
	// sorted in order of nodeId.
	votes []*UnsignedDaoVoteTx
}

func (d *daoProposalImpl) DaoProposalTx() *UnsignedDaoProposalTx {
	return d.daoProposalTx
}

func (d *daoProposalImpl) Votes() []*UnsignedDaoVoteTx {
	return d.votes
}

func (d *daoProposalImpl) Voted(nodeID ids.ShortID) bool {
	for _, vote := range d.votes {
		if vote.NodeID == nodeID {
			return true
		}
	}
	return false
}

/*
 ******************************************************
 ****************** Chain State ***********************
 ******************************************************
 */

type DaoProposalState interface {
	DaoProposalChainState() daoProposalChainState
}

type daoProposalChainState interface {
	// The NextProposal value returns the next DaoProposal that is going to be
	// removed using AdvanceTimestampTxs.
	GetNextProposal() (daoProposalTx *Tx, err error)
	GetProposal(proposalID ids.ID) (daoProposal, error)

	AddProposal(daoProposalTx *Tx) daoProposalChainState
	ArchiveNextProposals(numToDelete int) (daoProposalChainState, error)
	AddVote(daoVoteTx *Tx) daoProposalChainState

	// Stakers returns the current stakers on the network sorted in order of the
	// order of their future removal from the validator set.
	Proposals() []*Tx

	Apply(InternalState)
}

// currentStakerChainStateImpl is a copy on write implementation for versioning
// the validator set. None of the slices, maps, or pointers should be modified
// after initialization.
type daoProposalChainStateImpl struct {
	nextProposal *Tx

	// proposalID -> proposal, contains archive proposals, too
	proposalsByID map[ids.ID]*daoProposalImpl

	// list of active proposals sorted by end time
	proposals []*Tx

	addedProposals    []*Tx
	addedVotes        []*Tx
	archivedProposals []*Tx
}

func (ds *daoProposalChainStateImpl) GetNextProposal() (daoProposalTx *Tx, err error) {
	if ds.nextProposal == nil {
		return nil, database.ErrNotFound
	}
	return ds.nextProposal, nil
}

func (ds *daoProposalChainStateImpl) GetProposal(proposalID ids.ID) (daoProposal, error) {
	pro, exists := ds.proposalsByID[proposalID]
	if !exists {
		return nil, database.ErrNotFound
	}
	return pro, nil
}

func (ds *daoProposalChainStateImpl) AddProposal(daoProposalTx *Tx) daoProposalChainState {
	newDS := &daoProposalChainStateImpl{
		proposals:      make([]*Tx, len(ds.proposals)+1),
		addedProposals: []*Tx{daoProposalTx},
	}
	copy(newDS.proposals, ds.proposals)
	newDS.proposals[len(ds.proposals)] = daoProposalTx
	sortDaoProposalsByRemoval(newDS.proposals)

	switch tx := daoProposalTx.UnsignedTx.(type) {
	case *UnsignedDaoProposalTx:
		newDS.proposalsByID = make(map[ids.ID]*daoProposalImpl, len(ds.proposalsByID)+1)
		for id, pro := range ds.proposalsByID {
			newDS.proposalsByID[id] = pro
		}
		newDS.proposalsByID[tx.DaoProposal.ID()] = &daoProposalImpl{daoProposalTx: tx}
	default:
		panic(fmt.Errorf("expected proposal tx type but got %T", daoProposalTx.UnsignedTx))
	}

	newDS.setNextProposal()
	return newDS
}

func (ds *daoProposalChainStateImpl) AddVote(daoVoteTx *Tx) daoProposalChainState {
	newDS := &daoProposalChainStateImpl{
		nextProposal:  ds.nextProposal,
		proposals:     ds.proposals,
		proposalsByID: make(map[ids.ID]*daoProposalImpl, len(ds.proposalsByID)),
		addedVotes:    []*Tx{daoVoteTx},
	}

	switch tx := daoVoteTx.UnsignedTx.(type) {
	case *UnsignedDaoVoteTx:
		for pID, pro := range ds.proposalsByID {
			if pID != tx.ProposalID {
				newDS.proposalsByID[pID] = pro
			} else {
				newVotes := make([]*UnsignedDaoVoteTx, len(pro.votes)+1)
				num := copy(newVotes, pro.votes)
				newVotes[num] = tx
				newDS.proposalsByID[pID] = &daoProposalImpl{
					daoProposalTx: pro.daoProposalTx,
					votes:         newVotes,
				}
			}
		}
	default:
		panic(fmt.Errorf("expected proposal tx type but got %T", daoVoteTx.UnsignedTx))
	}
	return newDS
}

func (ds *daoProposalChainStateImpl) ArchiveNextProposals(numToDelete int) (daoProposalChainState, error) {
	if numToDelete > len(ds.proposals) {
		return nil, fmt.Errorf("trying to archive %d proposals from %d", numToDelete, len(ds.proposals))
	}

	newDS := &daoProposalChainStateImpl{
		proposalsByID:     ds.proposalsByID,
		proposals:         ds.proposals[numToDelete:], // sorted in order of removal
		archivedProposals: ds.proposals[:numToDelete],
	}
	newDS.setNextProposal()
	return newDS, nil
}

func (ds *daoProposalChainStateImpl) Proposals() []*Tx {
	return ds.proposals
}

func (ds *daoProposalChainStateImpl) Apply(is InternalState) {
	for _, added := range ds.addedProposals {
		is.AddDaoProposal(added)
	}
	for _, archived := range ds.archivedProposals {
		is.ArchiveDaoProposal(archived)
	}
	for _, vote := range ds.addedVotes {
		is.AddDaoVote(vote)
	}
	is.SetDaoProposalChainState(ds)

	// Dao changes should only be applied once.
	ds.addedProposals = nil
	ds.archivedProposals = nil
}

// setNextProposal to the next sproposal that will be removed using a
// AdvanceTimestampTxs.
func (ds *daoProposalChainStateImpl) setNextProposal() {
	if len(ds.proposals) > 0 {
		ds.nextProposal = ds.proposals[0]
	} else {
		ds.nextProposal = nil
	}
}

func getNextDaoProposalChangeTime(ds DaoProposalState) (time.Time, error) {
	earliest := mockable.MaxTime
	currentChainState := ds.DaoProposalChainState()

	if tx, err := currentChainState.GetNextProposal(); err != nil {
		return earliest, nil
	} else {
		if daoProposalTx, ok := tx.UnsignedTx.(*UnsignedDaoProposalTx); !ok {
			return earliest, errWrongTxType
		} else {
			return daoProposalTx.EndTime(), nil
		}
	}
}

/*
 ******************************************************
 ********************* Sorter *************************
 ******************************************************
 */

type innerSortProposalsByRemoval []*Tx

func (s innerSortProposalsByRemoval) Less(i, j int) bool {
	iDel := s[i]
	jDel := s[j]

	var (
		iEndTime time.Time
	)
	switch tx := iDel.UnsignedTx.(type) {
	case *UnsignedDaoProposalTx:
		iEndTime = tx.DaoProposal.EndTime()
	default:
		panic(fmt.Errorf("expected dao tx type but got %T", iDel.UnsignedTx))
	}

	var (
		jEndTime time.Time
	)
	switch tx := jDel.UnsignedTx.(type) {
	case *UnsignedDaoProposalTx:
		jEndTime = tx.DaoProposal.EndTime()
	default:
		panic(fmt.Errorf("expected dao tx type but got %T", jDel.UnsignedTx))
	}

	if iEndTime.Before(jEndTime) {
		return true
	}
	if jEndTime.Before(iEndTime) {
		return false
	}

	// If the end times are the same, then we sort by the txID.
	iTxID := iDel.ID()
	jTxID := jDel.ID()
	return bytes.Compare(iTxID[:], jTxID[:]) == -1
}

func (s innerSortProposalsByRemoval) Len() int {
	return len(s)
}

func (s innerSortProposalsByRemoval) Swap(i, j int) {
	s[j], s[i] = s[i], s[j]
}

func sortDaoProposalsByRemoval(s []*Tx) {
	sort.Sort(innerSortProposalsByRemoval(s))
}
