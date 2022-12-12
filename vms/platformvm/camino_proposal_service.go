package platformvm

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs/builder"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

type ProposalTimeRangeArgs struct {
	startTime uint64 `json:"startTime"`
	endTime   uint64 `json:"endTime"`
}

type CreateNOPProposalArgs struct {
	api.JSONSpendHeader
	ProposalTimeRangeArgs

	Metadata dao.NOPProposalMetadata `json:"metadata"`
	Content  string                  `json:"content"`
}

// AddAdressState issues an AddAdressStateTx
func (service *Service) CreateNOPProposal(_ *http.Request, args *CreateNOPProposalArgs, response api.JSONTxID) error {
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
	builder, ok := service.vm.txBuilder.(builder.CaminoBuilder)
	if !ok {
		return nil, errNotCaminoBuilder
	}

	proposal := dao.Proposal{
		Type:      dao.ProposalTypeNOP,
		StartTime: time.Unix(int64(args.startTime), 0),
		EndTime:   time.Unix(int64(args.endTime), 0),
		Metadata:  args.Metadata,
		Content:   []byte(args.Content), //TODO @jax think about a proper way to encode this
	}

	// Create the transaction
	tx, err := builder.NewCreateProposalTx(proposal, keys.Keys, changeAddr)
	if err != nil {
		return nil, fmt.Errorf(errCreateTx, err)
	}
	return tx, nil
}
