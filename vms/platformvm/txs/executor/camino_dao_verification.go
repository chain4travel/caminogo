package executor

import (
	"fmt"

	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

// any semantic checks are done here
func verifyNOPProposal(
	backend *Backend,
	chainState state.Chain,
	sTx *txs.Tx,
	tx *txs.CreateProposalTx) error {

	return nil
}

// verifyCreateProposalTx carries out the validation for an CreateProposalTx
// using bonding lock-mode instead of avax staking.
// returns [error] when tx is not valid for any reason
func verifyCreateProposalTx(
	backend *Backend,
	chainState state.Chain,
	sTx *txs.Tx,
	tx *txs.CreateProposalTx,
) error {
	// Verify the tx is well-formed
	if err := sTx.SyntacticVerify(backend.Ctx); err != nil {
		return err
	}

	duration := tx.Proposal.Duration()

	// handle common verification
	switch {
	case duration > backend.Config.CaminoConfig.DaoProposalMaxDurration:
		return fmt.Errorf("voting durration exceeds maximum allowed duration")
	case duration < backend.Config.CaminoConfig.DaoProposalMinDurration:
		return fmt.Errorf("voting durration is shorter than minimum required duration")
	}

	// handle type specifc verification
	//? id love to do this as an interface method, but import cycles came to save the day
	switch tx.Proposal.Type {
	case dao.ProposalTypeNOP:
		if err := verifyNOPProposal(backend, chainState, sTx, tx); err != nil {
			return err
		}
	// here would go the verify process of different proposal types
	default:
		return fmt.Errorf("invalid ProposalType")

	}

	// TODO @Jax is there a good reason to create a proposal before the chain has finished bootstrapping?
	if !backend.Bootstrapped.GetValue() {
		return fmt.Errorf("chain not bootstrapped")
	}

	currentTimestamp := chainState.GetTimestamp()
	// Ensure the proposal starts after the current time + minPendingDuration
	// TODO @jax this should be a seperate check and give a seperate error
	startTime := tx.Proposal.StartTime.Add(backend.Config.CaminoConfig.DaoProposalMinPendingDuration)
	if !currentTimestamp.Before(startTime) {
		return fmt.Errorf(
			"%w: %s >= %s",
			errTimestampNotBeforeStartTime,
			currentTimestamp,
			startTime,
		)
	}

	// Verify the flowcheck
	if err := backend.FlowChecker.VerifyLock(
		tx,
		chainState,
		tx.Ins,
		tx.Outs,
		sTx.Creds,
		backend.Config.CaminoConfig.DaoProposalBondAmount,
		backend.Ctx.AVAXAssetID,
		locked.StateBonded,
	); err != nil {
		return fmt.Errorf("%w: %s", errFlowCheckFailed, err)
	}

	// Make sure the tx doesn't start too far in the future. This is done last
	// to allow the verifier visitor to explicitly check for this error.
	maxStartTime := currentTimestamp.Add(MaxFutureStartTime)
	if startTime.After(maxStartTime) {
		return errFutureStakeTime
	}

	return nil
}

// verifyCreateVoteTx carries out the validation for an CreateVoteTx
// using bonding lock-mode instead of avax staking.
// returns [error] when tx is not valid for any reason
func verifyCreateVoteTx(
	backend *Backend,
	chainState state.Chain,
	sTx *txs.Tx,
	tx *txs.CreateVoteTx,
) error {
	// Verify the tx is well-formed
	if err := sTx.SyntacticVerify(backend.Ctx); err != nil {
		return err
	}

	// TODO @Jax is there a good reason to create a proposal before the chain has finished bootstrapping?
	if !backend.Bootstrapped.GetValue() {
		return fmt.Errorf("chain not bootstrapped")
	}

	proposalLookup, err := chainState.GetProposalLookup(tx.ProposalID)
	if err != nil {
		return err
	}

	// TODO @jax here the logic against duplicate votes is missing
	// TODO could also be done in the main method cuz we already have addreses there

	proposal := proposalLookup.Proposal

	currentTime := chainState.GetTimestamp()

	if err := proposal.IsActive(currentTime); err != nil {
		return err
	}

	if err := proposal.Metadata.AcceptVote(&tx.Vote); err != nil {
		return fmt.Errorf("proposal %s did not accept vote: %w", tx.ProposalID, err)
	}

	return nil
}

func isKycVerified(roles uint64) error {
	if txs.AddressStateKycBits&roles != txs.AddressStateKycVerified || txs.AddressStateKycBits&roles == txs.AddressStateKycExpired {
		return errNotKYCVerified
	}
	return nil
}
