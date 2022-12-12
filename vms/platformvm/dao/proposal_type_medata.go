package dao

import (
	"fmt"

	"github.com/ava-labs/avalanchego/vms/components/verify"
)

type ProposalMetadata interface {
	verify.Verifiable
}

const (
	MaxDummyDataLength = 64
)

type NOPProposalMetadata interface {
	ProposalMetadata
	GetDummyData() []byte
}

type nopProposalMetadata struct {
	dummyData []byte `serialize:"true"`
}

func (nop nopProposalMetadata) GetDummyData() []byte {
	return nop.dummyData
}

func (nop nopProposalMetadata) Verify() error {
	if len(nop.dummyData) > MaxDummyDataLength {
		return fmt.Errorf("dummyData is too long")
	}
	return nil
}
