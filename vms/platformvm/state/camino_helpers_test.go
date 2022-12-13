package state

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database/linkeddb"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
)

func generateXNumberOfProposalsWithDifferentIds(times int) map[ids.ID]*ProposalLookup {
	proposals := make(map[ids.ID]*ProposalLookup)
	for i := 0; i < times; i++ {
		proposals[ids.GenerateTestID()] = &ProposalLookup{Proposal: &dao.Proposal{StartTime: time.Now().Add(time.Duration(i) * time.Second)}}
	}
	return proposals
}

func caminoStateWithNProposals(n byte, validID bool, validContent bool) caminoState {
	proposalList := linkeddb.NewDefault(memdb.New())
	for i := byte(0); i < n; i++ {
		id := []byte{i}
		if validID {
			marshalledID, _ := ids.GenerateTestShortID().MarshalText()
			id = hashing.ComputeHash256(marshalledID)
		}
		content := []byte("test")
		if validContent {
			content, _ = blocks.GenesisCodec.Marshal(0, &ProposalLookup{Proposal: &dao.Proposal{Type: dao.ProposalTypeNOP, Metadata: id}, State: dao.ProposalStatePending, Votes: map[ids.ID]*dao.Vote{ids.GenerateTestID(): {Vote: dao.Accept}}})

			proposal := &ProposalLookup{}
			_, err := blocks.GenesisCodec.Unmarshal(content, proposal)
			if err != nil {
				fmt.Printf("failed to unmarshal proposal while loading from db: %v", err)
			}
		}
		proposalList.Put(id, content) //nolint:errcheck
	}
	return caminoState{proposalList: proposalList, proposals: make(map[ids.ID]*ProposalLookup)}
}
