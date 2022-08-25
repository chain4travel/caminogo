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
	"github.com/chain4travel/caminogo/vms/platformvm/dao"
)

var (
	_ daoProposalChainState = &daoProposalChainStateImpl{}
	_ DaoProposalCache      = &DaoProposalCacheImpl{}
)

const strInvalidType = "expected proposal tx type but got %T"

type DaoProposalCache interface {
	DaoProposalTx() *UnsignedDaoSubmitProposalTx
	Votes() []*Tx
	Voted(nodeID ids.ShortID) bool
}

type DaoProposalCacheImpl struct {
	// The Tx that created this proposal
	daoProposalTx *UnsignedDaoSubmitProposalTx
	// Unsorted list of vote TX
	votes []*Tx
}

func (d *DaoProposalCacheImpl) DaoProposalTx() *UnsignedDaoSubmitProposalTx {
	return d.daoProposalTx
}

func (d *DaoProposalCacheImpl) Votes() []*Tx {
	return d.votes
}

func (d *DaoProposalCacheImpl) Voted(nodeID ids.ShortID) bool {
	for _, vote := range d.votes {
		if vote.UnsignedTx.(*UnsignedDaoAddVoteTx).NodeID == nodeID {
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
	// The NextProposal value returns the next ProposalConfiguration that is going to be
	// removed using AdvanceTimestampTxs.
	GetNextProposal() (daoProposalTx *Tx, err error)
	GetActiveProposal(proposalID ids.ID) (DaoProposalCache, error)

	// Returns the state of a proposal, voted, pending or unknown
	GetProposalState(proposalID ids.ID) dao.ProposalState

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
	nextProposal *Tx // Next Proposal to be processed e.g. the one with the shortest end time

	// proposalID -> proposal, contains voted archive proposals, too
	// Note that archive proposals are nil, but existent
	proposalsByID map[ids.ID]*DaoProposalCacheImpl

	// list of active proposals sorted by end time
	proposals []*Tx

	addedProposals    []*Tx
	archivedProposals []*Tx

	addedVotes   []*Tx
	removedVotes []*Tx
}

func (ds *daoProposalChainStateImpl) GetNextProposal() (daoProposalTx *Tx, err error) {
	if ds.nextProposal == nil {
		return nil, database.ErrNotFound
	}
	return ds.nextProposal, nil
}

func (ds *daoProposalChainStateImpl) GetActiveProposal(proposalID ids.ID) (DaoProposalCache, error) {
	pro, exists := ds.proposalsByID[proposalID]
	if !exists || pro == nil {
		return nil, database.ErrNotFound
	}
	return pro, nil
}

func (ds *daoProposalChainStateImpl) GetProposalState(proposalID ids.ID) dao.ProposalState {
	proposal, exists := ds.proposalsByID[proposalID]
	switch {
	case !exists:
		return dao.ProposalStateUnknown
	case proposal == nil:
		return dao.ProposalStateConcluded
	case len(proposal.votes) >= int(proposal.daoProposalTx.ProposalConfiguration.Thresh):
		return dao.ProposalStateAccepted
	default:
		return dao.ProposalStatePending
	}
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
	case *UnsignedDaoSubmitProposalTx:
		newDS.proposalsByID = make(map[ids.ID]*DaoProposalCacheImpl, len(ds.proposalsByID)+1)
		for id, pro := range ds.proposalsByID {
			newDS.proposalsByID[id] = pro
		}
		newDS.proposalsByID[tx.ID()] = &DaoProposalCacheImpl{daoProposalTx: tx}
	default:
		panic(fmt.Errorf(strInvalidType, daoProposalTx.UnsignedTx))
	}

	newDS.setNextProposal()
	return newDS
}

func (ds *daoProposalChainStateImpl) AddVote(daoVoteTx *Tx) daoProposalChainState {
	newDS := &daoProposalChainStateImpl{
		nextProposal:  ds.nextProposal,
		proposals:     ds.proposals,
		proposalsByID: make(map[ids.ID]*DaoProposalCacheImpl, len(ds.proposalsByID)),
		addedVotes:    []*Tx{daoVoteTx},
	}

	switch tx := daoVoteTx.UnsignedTx.(type) {
	case *UnsignedDaoConcludeProposalTx:
		for pID, pro := range ds.proposalsByID {
			if pID != tx.ID() {
				newDS.proposalsByID[pID] = pro
			} else {
				newVotes := make([]*Tx, len(pro.votes)+1)
				num := copy(newVotes, pro.votes)
				newVotes[num] = daoVoteTx
				newDS.proposalsByID[pID] = &DaoProposalCacheImpl{
					daoProposalTx: pro.daoProposalTx,
					votes:         newVotes,
				}
			}
		}
	default:
		panic(fmt.Errorf(strInvalidType, daoVoteTx.UnsignedTx))
	}
	return newDS
}

func (ds *daoProposalChainStateImpl) ArchiveNextProposals(numToDelete int) (daoProposalChainState, error) {
	if numToDelete > len(ds.proposals) {
		return nil, fmt.Errorf("trying to archive %d proposals from %d", numToDelete, len(ds.proposals))
	}

	newDS := &daoProposalChainStateImpl{
		proposalsByID:     make(map[ids.ID]*DaoProposalCacheImpl, len(ds.proposalsByID)),
		proposals:         ds.proposals[numToDelete:], // sorted in order of removal
		archivedProposals: ds.proposals[:numToDelete],
	}

	// Make a copy of the map
	for pID, pro := range ds.proposalsByID {
		newDS.proposalsByID[pID] = pro
	}
	// nil or delete the history proposals
	for _, archiveTx := range newDS.archivedProposals {
		switch tx := archiveTx.UnsignedTx.(type) {
		case *UnsignedDaoSubmitProposalTx:
			pro := ds.proposalsByID[tx.ID()]
			newDS.removedVotes = append(newDS.removedVotes, pro.votes...)
			if len(pro.votes) >= int(tx.ProposalConfiguration.Thresh) {
				newDS.proposalsByID[tx.ID()] = nil
			} else {
				delete(newDS.proposalsByID, tx.ID())
			}
		default:
			panic(fmt.Errorf(strInvalidType, archiveTx.UnsignedTx))
		}
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
	for _, vote := range ds.removedVotes {
		is.RemoveDaoVote(vote)
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

	tx, err := currentChainState.GetNextProposal()
	if err != nil {
		return earliest, nil
	}

	if daoProposalTx, ok := tx.UnsignedTx.(*UnsignedDaoSubmitProposalTx); ok {
		return daoProposalTx.EndTime(), nil
	}
	return earliest, errWrongTxType
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
	case *UnsignedDaoSubmitProposalTx:
		iEndTime = tx.EndTime()
	default:
		panic(fmt.Errorf(strInvalidType, iDel.UnsignedTx))
	}

	var (
		jEndTime time.Time
	)
	switch tx := jDel.UnsignedTx.(type) {
	case *UnsignedDaoSubmitProposalTx:
		jEndTime = tx.EndTime()
	default:
		panic(fmt.Errorf(strInvalidType, jDel.UnsignedTx))
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
