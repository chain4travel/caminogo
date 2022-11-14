// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/validator"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

var (
	errNodeKeyMissing   = errors.New("couldn't find key matching nodeID")
	errWrongNodeKeyType = errors.New("node key type isn't *crypto.PrivateKeySECP256K1R")
)

type caminoBuilder struct {
	builder
}

func (b *caminoBuilder) NewAddValidatorTx(
	stakeAmount,
	startTime,
	endTime uint64,
	nodeID ids.NodeID,
	rewardAddress ids.ShortID,
	shares uint32,
	keys []*crypto.PrivateKeySECP256K1R,
	changeAddr ids.ShortID,
) (*txs.Tx, error) {
	caminoGenesis, err := b.builder.state.CaminoGenesisState()
	if err != nil {
		return nil, err
	}

	if !caminoGenesis.LockModeBondDeposit &&
		!caminoGenesis.VerifyNodeSignature {
		return b.builder.NewAddValidatorTx(
			stakeAmount,
			startTime,
			endTime,
			nodeID,
			rewardAddress,
			shares,
			keys,
			changeAddr,
		)
	}

	var (
		utx             txs.UnsignedTx
		signers         [][]*crypto.PrivateKeySECP256K1R
		ins             []*avax.TransferableInput
		outs, stakeOuts []*avax.TransferableOutput
	)

	if caminoGenesis.LockModeBondDeposit {
		ins, outs, signers, err = b.Lock(keys, stakeAmount, b.cfg.AddPrimaryNetworkValidatorFee, locked.StateBonded, changeAddr)
	} else {
		ins, outs, stakeOuts, signers, err = b.Spend(keys, stakeAmount, b.cfg.AddPrimaryNetworkValidatorFee, changeAddr)
	}
	if err != nil {
		return nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}

	if caminoGenesis.VerifyNodeSignature {
		nodeSigners, err := getNodeSigners(keys, nodeID)
		if err != nil {
			return nil, err
		}
		signers = append(signers, nodeSigners)
	}

	addValidatorTx := &txs.CaminoAddValidatorTx{
		AddValidatorTx: txs.AddValidatorTx{
			BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
				NetworkID:    b.ctx.NetworkID,
				BlockchainID: b.ctx.ChainID,
				Ins:          ins,
				Outs:         outs,
			}},
			Validator: validator.Validator{
				NodeID: nodeID,
				Start:  startTime,
				End:    endTime,
				Wght:   stakeAmount,
			},
			StakeOuts: stakeOuts,
			RewardsOwner: &secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     []ids.ShortID{rewardAddress},
			},
			DelegationShares: shares,
		},
	}

	if caminoGenesis.LockModeBondDeposit {
		utx = addValidatorTx
	} else {
		utx = &addValidatorTx.AddValidatorTx
	}

	tx, err := txs.NewSigned(utx, txs.Codec, signers)
	if err != nil {
		return nil, err
	}
	return tx, tx.SyntacticVerify(b.ctx)
}

func (b *caminoBuilder) NewAddSubnetValidatorTx(
	weight,
	startTime,
	endTime uint64,
	nodeID ids.NodeID,
	subnetID ids.ID,
	keys []*crypto.PrivateKeySECP256K1R,
	changeAddr ids.ShortID,
) (*txs.Tx, error) {
	if caminoGenesis, err := b.builder.state.CaminoGenesisState(); err != nil {
		return nil, err
	} else if !caminoGenesis.VerifyNodeSignature {
		return b.builder.NewAddSubnetValidatorTx(
			weight,
			startTime,
			endTime,
			nodeID,
			subnetID,
			keys,
			changeAddr,
		)
	}

	ins, outs, signers, err := b.Lock(keys, 0, b.cfg.TxFee, locked.StateUnlocked, changeAddr)
	if err != nil {
		return nil, fmt.Errorf("couldn't generate tx inputs/outputs: %w", err)
	}

	subnetAuth, subnetSigners, err := b.Authorize(b.state, subnetID, keys)
	if err != nil {
		return nil, fmt.Errorf("couldn't authorize tx's subnet restrictions: %w", err)
	}
	signers = append(signers, subnetSigners)

	nodeSigners, err := getNodeSigners(keys, nodeID)
	if err != nil {
		return nil, err
	}
	signers = append(signers, nodeSigners)

	// Create the tx
	utx := &txs.AddSubnetValidatorTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    b.ctx.NetworkID,
			BlockchainID: b.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		Validator: validator.SubnetValidator{
			Validator: validator.Validator{
				NodeID: nodeID,
				Start:  startTime,
				End:    endTime,
				Wght:   weight,
			},
			Subnet: subnetID,
		},
		SubnetAuth: subnetAuth,
	}
	tx, err := txs.NewSigned(utx, txs.Codec, signers)
	if err != nil {
		return nil, err
	}
	return tx, tx.SyntacticVerify(b.ctx)
}

func getNodeSigners(
	keys []*crypto.PrivateKeySECP256K1R,
	nodeID ids.NodeID,
) ([]*crypto.PrivateKeySECP256K1R, error) {
	signer, found := secp256k1fx.NewKeychain(keys...).Get(ids.ShortID(nodeID))
	if !found {
		return nil, errNodeKeyMissing
	}

	key, ok := signer.(*crypto.PrivateKeySECP256K1R)
	if !ok {
		return nil, errWrongNodeKeyType
	}

	return []*crypto.PrivateKeySECP256K1R{key}, nil
}
