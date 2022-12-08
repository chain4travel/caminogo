package dao

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

const (
	ProposalContentLength = 1024
)

type ProposalType uint64

const (
	NOPProposal ProposalType = iota // no action on chain (NO OPERATION)
)

type ProposalState uint64

const (
	Pending ProposalState = iota
	Active
	Concluded
)

type ProposalStatus struct {
	State ProposalState `serialize:"true"`
}

type Proposal struct {
	TxID ids.ID

	Type  ProposalType  `serialize:"true"`
	State ProposalState `serialize:"true"`

	StartTime time.Time `serialize:"true"`
	EndTime   time.Time `serialize:"true"`

	Content [ProposalContentLength]byte `serialize:"true"`

	Votes map[ids.ID]*Vote `serialize:"true"`

	Priority txs.Priority `serialize:"true"`
}
