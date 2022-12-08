package dao

type VoteType uint64

const (
	Accept  VoteType = iota // I support this proposal
	Reject                  // I dont support this propsal
	Abstain                 // I want to remain neutral
)

type Vote struct {
	Vote VoteType `serialize:"true"`
}
