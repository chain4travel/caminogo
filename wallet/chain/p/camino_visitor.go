// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package p

import (
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// backend

func (b *backendVisitor) AddressStateTx(tx *txs.AddressStateTx) error {
	return b.baseTx(&tx.BaseTx)
}

func (b *backendVisitor) DepositTx(tx *txs.DepositTx) error {
	return b.baseTx(&tx.BaseTx)
}

func (b *backendVisitor) UnlockDepositTx(tx *txs.UnlockDepositTx) error {
	return b.baseTx(&tx.BaseTx)
}

func (b *backendVisitor) ClaimTx(tx *txs.ClaimTx) error {
	return b.baseTx(&tx.BaseTx)
}

func (b *backendVisitor) RegisterNodeTx(tx *txs.RegisterNodeTx) error {
	return b.baseTx(&tx.BaseTx)
}

func (*backendVisitor) RewardsImportTx(*txs.RewardsImportTx) error {
	return errUnsupportedTxType
}

func (b *backendVisitor) BaseTx(tx *txs.BaseTx) error {
	return b.baseTx(tx)
}

// signer

func (s *signerVisitor) AddressStateTx(tx *txs.AddressStateTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, txSigners)
}

func (s *signerVisitor) DepositTx(tx *txs.DepositTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, txSigners)
}

func (s *signerVisitor) UnlockDepositTx(tx *txs.UnlockDepositTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, txSigners)
}

func (s *signerVisitor) ClaimTx(tx *txs.ClaimTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}

	claimableSignersKC := secp256k1fx.NewKeychain()
	for _, depositTxID := range tx.DepositTxs {
		depositRewardOwner, err := s.getDepositRewardsOwner(depositTxID)
		if err != nil {
			return err
		}
		_, keys, able := kc.Match(depositRewardOwner, uint64(time.Now().Unix())) // ? @evlekht ok time?
		if !able {
			return err // TODO @evlekht err
		}

		for _, key := range keys {
			claimableSignersKC.Add(key)
		}

	}

	for _, ownerID := range tx.ClaimableOwnerIDs {
		// TODO @evlekht we need to extend backend state to fetch and store claimables
		// ! @evlelkth or store owners, not ownerIDs in tx !
		claimable, err := s.backend.GetClaimable(ownerID)
		if err != nil {
			return nil, err
		}

		_, keys, able := kc.Match(claimable.Owner, uint64(time.Now().Unix())) // ? @evlekht ok time?
		if !able {
			return err // TODO @evlekht err
		}

		for _, key := range keys {
			claimableSignersKC.Add(key)
		}
	}

	// TODO@ get deposits and claimables signers
	return sign(s.tx, txSigners)
}

func (s *signerVisitor) RegisterNodeTx(tx *txs.RegisterNodeTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}

	nodeSigners := []keychain.Signer{}
	if tx.NewNodeID != ids.EmptyNodeID {
		nodeKey, found := s.kc.Get(ids.ShortID(tx.NewNodeID))
		if !found {
			return err // TODO @evlekht err
		}
		nodeSigners = []keychain.Signer{nodeKey}
	}
	txSigners = append(txSigners, nodeSigners)

	// TODO @evlekht we need to extend backend state to fetch and store msig owners
	owner, err := s.backend.GetOwner(tx.ConsortiumMemberAddress)
	if err != nil {
		return err
	}
	consortiuMemberSigners, err := s.getOwnerSigners(owner, tx.ConsortiumMemberAuth)
	if err != nil {
		return err
	}

	txSigners = append(txSigners, consortiuMemberSigners)
	return sign(s.tx, txSigners)
}

func (*signerVisitor) RewardsImportTx(*txs.RewardsImportTx) error {
	return errUnsupportedTxType
}

func (s *signerVisitor) BaseTx(tx *txs.BaseTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, txSigners)
}


func (s *signerVisitor) getOwnerSigners(owner *secp256k1fx.OutputOwners, authInput *secp256k1fx.Input) ([]keychain.Signer, error) {
	authSigners := make([]keychain.Signer, len(authInput.SigIndices))
	for sigIndex, addrIndex := range authInput.SigIndices {
		if addrIndex >= uint32(len(owner.Addrs)) {
			return nil, errInvalidUTXOSigIndex
		}

		addr := owner.Addrs[addrIndex]
		key, ok := s.kc.Get(addr)
		if !ok {
			// If we don't have access to the key, then we can't sign this
			// transaction. However, we can attempt to partially sign it.
			continue
		}
		authSigners[sigIndex] = key
	}
	return authSigners, nil
}

func (s *signerVisitor) getDepositRewardsOwner(depositTxID ids.ID) (*secp256k1fx.OutputOwners, error) {
	signedDepositTx, err := s.backend.GetTx(s.ctx, depositTxID)
	if err != nil {
		return nil, err
	}
	depositTx, ok := signedDepositTx.Unsigned.(*txs.DepositTx)
	if !ok {
		return nil, errWrongTxType
	}

	depositRewardsOwner, ok := depositTx.RewardsOwner.(*secp256k1fx.OutputOwners)
	if !ok {
		return nil, err // TODO@ errNotSECPOwner
	}

	return depositRewardsOwner, nil
}
