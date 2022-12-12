package executor

import (
	"fmt"

	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/dao"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

func verifyNOPProposal(
	backend *Backend,
	chainState state.Chain,
	sTx *txs.Tx,
	tx *txs.CreateProposalTx) error {

	return nil
}

// verifyAddValidatorTxWithBonding carries out the validation for an AddValidatorTx
// using bonding lock-mode instead of avax staking.
// It returns the tx outputs that should be returned if this validator is not
// added to the staking set.
func verifyCreateProposalTx(
	backend *Backend,
	chainState state.Chain,
	sTx *txs.Tx,
	tx *txs.CreateProposalTx,
) (
	[]*avax.TransferableOutput,
	error,
) {
	// Verify the tx is well-formed
	if err := sTx.SyntacticVerify(backend.Ctx); err != nil {
		return nil, err
	}

	duration := tx.Proposal.Duration()

	// handle common verification
	switch {
	case duration > backend.Config.CaminoConfig.DaoProposalMaxDurration:
		return nil, fmt.Errorf("voting durration exceeds maximum allowed duration")
	case duration < backend.Config.CaminoConfig.DaoProposalMinDurration:
		return nil, fmt.Errorf("voting durration is shorter than minimum required duration")
	}

	// handle type specifc verification
	switch tx.Proposal.Type {
	case dao.ProposalTypeNOP:
		break
	// here would go the verify process of different proposal types
	default:
		return nil, fmt.Errorf("invalid ProposalType")

	}

	// switch {
	// case tx.Validator.Wght < backend.Config.MinValidatorStake:
	// 	// Ensure validator is staking at least the minimum amount
	// 	return nil, errWeightTooSmall

	// case tx.Validator.Wght > backend.Config.MaxValidatorStake:
	// 	// Ensure validator isn't staking too much
	// 	return nil, errWeightTooLarge

	// case tx.DelegationShares < backend.Config.MinDelegationFee:
	// 	// Ensure the validator fee is at least the minimum amount
	// 	return nil, errInsufficientDelegationFee

	// case duration < backend.Config.MinStakeDuration:
	// 	// Ensure staking length is not too short
	// 	return nil, errStakeTooShort

	// case duration > backend.Config.MaxStakeDuration:
	// 	// Ensure staking length is not too long
	// 	return nil, errStakeTooLong
	// }

	// TODO @Jax is there a good reason to create a proposal before the chain has finished bootstrapping?
	if !backend.Bootstrapped.GetValue() {
		return nil, fmt.Errorf("chain not bootstrapped")
	}

	// _, err := GetValidator(chainState, constants.PrimaryNetworkID, tx.Validator.NodeID)
	// if err == nil {
	// 	return nil, errValidatorExists
	// }
	// if err != database.ErrNotFound {
	// 	return nil, fmt.Errorf(
	// 		"failed to find whether %s is a primary network validator: %w",
	// 		tx.Validator.NodeID,
	// 		err,
	// 	)
	// }

	currentTimestamp := chainState.GetTimestamp()
	// Ensure the proposed validator starts after the current time
	startTime := tx.Proposal.StartTime
	if !currentTimestamp.Before(startTime) {
		return nil, fmt.Errorf(
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
		return nil, fmt.Errorf("%w: %s", errFlowCheckFailed, err)
	}

	// Make sure the tx doesn't start too far in the future. This is done last
	// to allow the verifier visitor to explicitly check for this error.
	maxStartTime := currentTimestamp.Add(MaxFutureStartTime)
	if startTime.After(maxStartTime) {
		return nil, errFutureStakeTime
	}

	return tx.Outs, nil
}

func isKycVerified(roles uint64) error {
	if txs.AddressStateKycBits&roles != txs.AddressStateKycVerified || txs.AddressStateKycBits&roles != txs.AddressStateKycExpired {
		return errNotKYCVerified
	}
	return nil
}
