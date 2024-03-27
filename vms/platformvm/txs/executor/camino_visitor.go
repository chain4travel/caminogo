// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import "github.com/ava-labs/avalanchego/vms/platformvm/txs"

// Camino Visitor implementations

// Standard

func (*StandardTxExecutor) AddressStateTx(*txs.AddressStateTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) DepositTx(*txs.DepositTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) UnlockDepositTx(*txs.UnlockDepositTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) ClaimTx(*txs.ClaimTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) RegisterNodeTx(*txs.RegisterNodeTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) RewardsImportTx(*txs.RewardsImportTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) MultisigAliasTx(*txs.MultisigAliasTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) AddDepositOfferTx(*txs.AddDepositOfferTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) AddProposalTx(*txs.AddProposalTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) AddVoteTx(*txs.AddVoteTx) error {
	return ErrWrongTxType
}

func (*StandardTxExecutor) FinishProposalsTx(*txs.FinishProposalsTx) error {
	return ErrWrongTxType
}

// Proposal

func (*ProposalTxExecutor) AddressStateTx(*txs.AddressStateTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) DepositTx(*txs.DepositTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) UnlockDepositTx(*txs.UnlockDepositTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) ClaimTx(*txs.ClaimTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) RegisterNodeTx(*txs.RegisterNodeTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) RewardsImportTx(*txs.RewardsImportTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) MultisigAliasTx(*txs.MultisigAliasTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) AddDepositOfferTx(*txs.AddDepositOfferTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) AddProposalTx(*txs.AddProposalTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) AddVoteTx(*txs.AddVoteTx) error {
	return ErrWrongTxType
}

func (*ProposalTxExecutor) FinishProposalsTx(*txs.FinishProposalsTx) error {
	return ErrWrongTxType
}

// Atomic

func (*AtomicTxExecutor) AddressStateTx(*txs.AddressStateTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) DepositTx(*txs.DepositTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) UnlockDepositTx(*txs.UnlockDepositTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) ClaimTx(*txs.ClaimTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) RegisterNodeTx(*txs.RegisterNodeTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) RewardsImportTx(*txs.RewardsImportTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) MultisigAliasTx(*txs.MultisigAliasTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) AddDepositOfferTx(*txs.AddDepositOfferTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) AddProposalTx(*txs.AddProposalTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) AddVoteTx(*txs.AddVoteTx) error {
	return ErrWrongTxType
}

func (*AtomicTxExecutor) FinishProposalsTx(*txs.FinishProposalsTx) error {
	return ErrWrongTxType
}
