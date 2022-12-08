package dao

import "github.com/ava-labs/avalanchego/ids"

type VoteType uint64

const (
	Accept  VoteType = iota // I support this proposal
	Reject                  // I dont support this propsal
	Abstain                 // I want to remain neutral
)

type Vote struct {
	TxID ids.ID

	Vote VoteType `serialize:"true"`
}
