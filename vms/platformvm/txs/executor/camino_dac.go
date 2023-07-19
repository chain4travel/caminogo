package executor

import (
	"errors"

	"github.com/ava-labs/avalanchego/vms/platformvm/dac"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

var (
	_ dac.VerifierVisitor = (*proposalVerifier)(nil)
	_ dac.ExecutorVisitor = (*proposalExecutor)(nil)

	errNotPermittedToCreateProposal = errors.New("don't have permission to create proposal of this type")
	errAlreadyActiveProposal        = errors.New("there is already active proposal of this type")
)

type proposalVerifier struct {
	state               state.Chain
	fx                  fx.Fx
	signedAddProposalTx *txs.Tx
	addProposalTx       *txs.AddProposalTx
}

// executor calls should never error
type proposalExecutor struct {
	state state.Chain
	fx    fx.Fx
}

func (e *CaminoStandardTxExecutor) proposalVerifier(tx *txs.AddProposalTx) *proposalVerifier {
	return &proposalVerifier{
		state:               e.State,
		fx:                  e.Fx,
		signedAddProposalTx: e.Tx,
		addProposalTx:       tx,
	}
}

func (e *CaminoStandardTxExecutor) proposalExecutor() *proposalExecutor {
	return &proposalExecutor{state: e.State, fx: e.Fx}
}

// BaseFeeProposal

func (e *proposalVerifier) BaseFeeProposal(*dac.BaseFeeProposal) error {
	// verify proposer credential and address state (role)
	proposerAddressState, err := e.state.GetAddressStates(e.addProposalTx.ProposerAddress)
	if err != nil {
		return err
	}

	if proposerAddressState.IsNot(txs.AddressStateCaminoProposer) {
		return errNotPermittedToCreateProposal
	}

	proposalsIterator, err := e.state.GetProposalIterator()
	defer proposalsIterator.Release()
	if err != nil {
		return err
	}
	for proposalsIterator.Next() {
		proposal, err := proposalsIterator.Value()
		if err != nil {
			return err
		}
		if _, ok := proposal.(*dac.BaseFeeProposalState); ok {
			return errAlreadyActiveProposal
		}
	}

	if err := proposalsIterator.Error(); err != nil {
		return err
	}

	return nil
}

// should never error
func (e *proposalExecutor) BaseFeeProposal(proposal *dac.BaseFeeProposalState) error {
	_, mostVotedOptionIndex, unambiguous := proposal.GetMostVoted()
	if unambiguous {
		e.state.SetBaseFee(proposal.Options[mostVotedOptionIndex].Value)
	}
	return nil
}
