package platformvm

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	proposaltypes "github.com/ava-labs/avalanchego/vms/platformvm/dao/proposal_types"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

type ProposalTimeRangeArgs struct {
	StartTime uint64 `json:"startTime"`
	EndTime   uint64 `json:"endTime"`
}

type CreateNOPProposalArgs struct {
	api.JSONSpendHeader
	ProposalTimeRangeArgs

	Metadata proposaltypes.NOPProposalMetadata `json:"metadata"`
	Content  string                            `json:"content"`
}

// CreateNOPProposal issues an CreateProposalTx of type NOPProposal
func (service *Service) CreateNOPProposal(_ *http.Request, args *CreateNOPProposalArgs, response *api.JSONTxID) error {
	service.vm.ctx.Log.Debug("Platform: CreateNOPProposal called")

	keys, err := service.getKeystoreKeys(&args.JSONSpendHeader)
	if err != nil {
		return err
	}

	tx, err := service.buildNOPProposal(args, keys)
	if err != nil {
		return err
	}

	response.TxID = tx.ID()

	if err = service.vm.Builder.AddUnverifiedTx(tx); err != nil {
		return err
	}
	return nil
}

func (service *Service) buildNOPProposal(args *CreateNOPProposalArgs, keys *secp256k1fx.Keychain) (*txs.Tx, error) {
	var changeAddr ids.ShortID
	if len(args.ChangeAddr) > 0 {
		var err error
		if changeAddr, err = avax.ParseServiceAddress(service.addrManager, args.ChangeAddr); err != nil {
			return nil, fmt.Errorf(errInvalidChangeAddr, err)
		}
	}
	proposal := dao.Proposal{
		StartTime: time.Unix(int64(args.StartTime), 0),
		EndTime:   time.Unix(int64(args.EndTime), 0),
		Metadata:  args.Metadata,
		Content:   []byte(args.Content), //TODO @jax think about a proper way to encode this
	}

	// Create the transaction
	tx, err := service.vm.txBuilder.NewCreateProposalTx(proposal, keys.Keys, changeAddr)
	if err != nil {
		return nil, fmt.Errorf(errCreateTx, err)
	}
	return tx, nil
}

type GetProposalArgs struct {
	ProposalID ids.ID `json:"proposalID"` // TODO @jax cloud be an array
}

type GetProposalReply struct {
	Proposal dao.Proposal         `json:"proposal"`
	Votes    map[ids.ID]*dao.Vote `json:"votes"`
	State    dao.ProposalState    `json:"state"`
}

// AddAdressState issues an AddAdressStateTx
func (service *Service) GetProposal(_ *http.Request, args *GetProposalArgs, response *GetProposalReply) error {
	service.vm.ctx.Log.Debug("Platform: GetProposal called")

	lookup, err := service.vm.state.GetProposalLookup(args.ProposalID)
	if err != nil {
		return err
	}

	response.Proposal = *lookup.Proposal
	response.Votes = lookup.Votes
	response.State = lookup.State

	return nil
}

type CreateVoteArgs struct {
	api.JSONSpendHeader

	ProposalID ids.ID `json:"proposalID"`

	Vote dao.Vote `json:"vote"`
}

// CreateVote issues an CreateVoteTx
func (service *Service) CreateVote(_ *http.Request, args *CreateVoteArgs, response *api.JSONTxID) error {
	service.vm.ctx.Log.Debug("Platform: CreateVote called")

	keys, err := service.getKeystoreKeys(&args.JSONSpendHeader)
	if err != nil {
		return err
	}

	tx, err := service.buildVote(args, keys)
	if err != nil {
		return err
	}

	response.TxID = tx.ID()

	if err = service.vm.Builder.AddUnverifiedTx(tx); err != nil {
		return err
	}
	return nil
}

func (service *Service) buildVote(args *CreateVoteArgs, keys *secp256k1fx.Keychain) (*txs.Tx, error) {
	var changeAddr ids.ShortID
	if len(args.ChangeAddr) > 0 {
		var err error
		if changeAddr, err = avax.ParseServiceAddress(service.addrManager, args.ChangeAddr); err != nil {
			return nil, fmt.Errorf(errInvalidChangeAddr, err)
		}
	}

	// Create the transaction
	tx, err := service.vm.txBuilder.NewCreateVoteTx(args.Vote, args.ProposalID, keys.Keys, changeAddr)
	if err != nil {
		return nil, fmt.Errorf(errCreateTx, err)
	}
	return tx, nil
}
