// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/nodeid"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/multisig"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/dac"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/ava-labs/avalanchego/vms/platformvm/treasury"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"

	smcon "github.com/ava-labs/avalanchego/snow/consensus/snowman"
	blockexecutor "github.com/ava-labs/avalanchego/vms/platformvm/blocks/executor"
	txexecutor "github.com/ava-labs/avalanchego/vms/platformvm/txs/executor"
)

func TestRemoveDeferredValidator(t *testing.T) {
	require := require.New(t)
	addr := caminoPreFundedKeys[0].Address()
	hrp := constants.NetworkIDToHRP[testNetworkID]
	bech32Addr, err := address.FormatBech32(hrp, addr.Bytes())
	require.NoError(err)

	nodeKey, nodeID := nodeid.GenerateCaminoNodeKeyAndID()

	consortiumMemberKey, err := testKeyFactory.NewPrivateKey()
	require.NoError(err)

	outputOwners := &secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{addr},
	}
	caminoGenesisConf := api.Camino{
		VerifyNodeSignature: true,
		LockModeBondDeposit: true,
		InitialAdmin:        addr,
	}
	genesisUTXOs := []api.UTXO{
		{
			Amount:  json.Uint64(defaultCaminoValidatorWeight),
			Address: bech32Addr,
		},
	}

	vm := newCaminoVM(caminoGenesisConf, genesisUTXOs, nil)
	vm.ctx.Lock.Lock()
	defer func() {
		require.NoError(vm.Shutdown(context.Background()))
		vm.ctx.Lock.Unlock()
	}()

	utxo := generateTestUTXO(ids.GenerateTestID(), avaxAssetID, defaultBalance, *outputOwners, ids.Empty, ids.Empty)
	vm.state.AddUTXO(utxo)
	err = vm.state.Commit()
	require.NoError(err)

	// Set consortium member
	tx, err := vm.txBuilder.NewAddressStateTx(
		consortiumMemberKey.Address(),
		false,
		txs.AddressStateBitConsortium,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0]},
		outputOwners,
	)
	require.NoError(err)
	err = vm.Builder.AddUnverifiedTx(tx)
	require.NoError(err)
	blk, err := vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)
	err = blk.Accept(context.Background())
	require.NoError(err)
	err = vm.SetPreference(context.Background(), vm.manager.LastAccepted())
	require.NoError(err)

	// Register node
	tx, err = vm.txBuilder.NewRegisterNodeTx(
		ids.EmptyNodeID,
		nodeID,
		consortiumMemberKey.Address(),
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0], nodeKey, consortiumMemberKey},
		outputOwners,
	)
	require.NoError(err)
	err = vm.Builder.AddUnverifiedTx(tx)
	require.NoError(err)
	blk, err = vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)
	err = blk.Accept(context.Background())
	require.NoError(err)
	err = vm.SetPreference(context.Background(), vm.manager.LastAccepted())
	require.NoError(err)

	// Add the validator
	startTime := vm.clock.Time().Add(txexecutor.SyncBound).Add(1 * time.Second)
	endTime := defaultValidateEndTime.Add(-1 * time.Hour)
	addValidatorTx, err := vm.txBuilder.NewCaminoAddValidatorTx(
		vm.Config.MinValidatorStake,
		uint64(startTime.Unix()),
		uint64(endTime.Unix()),
		nodeID,
		consortiumMemberKey.Address(),
		ids.ShortEmpty,
		reward.PercentDenominator,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0], consortiumMemberKey},
		ids.ShortEmpty,
	)
	require.NoError(err)

	staker, err := state.NewCurrentStaker(
		addValidatorTx.ID(),
		addValidatorTx.Unsigned.(*txs.CaminoAddValidatorTx),
		0,
	)
	require.NoError(err)
	vm.state.PutCurrentValidator(staker)
	vm.state.AddTx(addValidatorTx, status.Committed)
	err = vm.state.Commit()
	require.NoError(err)

	utxo = generateTestUTXO(ids.GenerateTestID(), avaxAssetID, defaultBalance, *outputOwners, ids.Empty, ids.Empty)
	vm.state.AddUTXO(utxo)
	err = vm.state.Commit()
	require.NoError(err)

	// Defer the validator
	tx, err = vm.txBuilder.NewAddressStateTx(
		consortiumMemberKey.Address(),
		false,
		txs.AddressStateBitNodeDeferred,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0]},
		outputOwners,
	)
	require.NoError(err)
	err = vm.Builder.AddUnverifiedTx(tx)
	require.NoError(err)
	blk, err = vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)
	err = blk.Accept(context.Background())
	require.NoError(err)
	err = vm.SetPreference(context.Background(), vm.manager.LastAccepted())
	require.NoError(err)

	// Verify that the validator is deferred (moved from current to deferred stakers set)
	_, err = vm.state.GetCurrentValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)
	_, err = vm.state.GetDeferredValidator(constants.PrimaryNetworkID, nodeID)
	require.NoError(err)

	// Verify that the validator's owner's deferred state and consortium member is true
	ownerState, _ := vm.state.GetAddressStates(consortiumMemberKey.Address())
	require.Equal(ownerState, txs.AddressStateNodeDeferred|txs.AddressStateConsortiumMember)

	// Fast-forward clock to time for validator to be rewarded
	vm.clock.Set(endTime)
	blk, err = vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)

	// Assert preferences are correct
	block := blk.(smcon.OracleBlock)
	options, err := block.Options(context.Background())
	require.NoError(err)

	commit := options[1].(*blockexecutor.Block)
	_, ok := commit.Block.(*blocks.BanffCommitBlock)
	require.True(ok)

	abort := options[0].(*blockexecutor.Block)
	_, ok = abort.Block.(*blocks.BanffAbortBlock)
	require.True(ok)

	require.NoError(block.Accept(context.Background()))
	require.NoError(commit.Verify(context.Background()))
	require.NoError(abort.Verify(context.Background()))

	txID := blk.(blocks.Block).Txs()[0].ID()
	{
		onAccept, ok := vm.manager.GetState(abort.ID())
		require.True(ok)

		_, txStatus, err := onAccept.GetTx(txID)
		require.NoError(err)
		require.Equal(status.Aborted, txStatus)
	}

	require.NoError(commit.Accept(context.Background()))
	require.NoError(vm.SetPreference(context.Background(), vm.manager.LastAccepted()))

	_, txStatus, err := vm.state.GetTx(txID)
	require.NoError(err)
	require.Equal(status.Committed, txStatus)

	// Verify that the validator is rewarded
	_, err = vm.state.GetCurrentValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)
	_, err = vm.state.GetDeferredValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)

	// Verify that the validator's owner's deferred state is false
	ownerState, _ = vm.state.GetAddressStates(consortiumMemberKey.Address())
	require.Equal(ownerState, txs.AddressStateConsortiumMember)

	timestamp := vm.state.GetTimestamp()
	require.Equal(endTime.Unix(), timestamp.Unix())
}

func TestRemoveReactivatedValidator(t *testing.T) {
	require := require.New(t)
	addr := caminoPreFundedKeys[0].Address()
	hrp := constants.NetworkIDToHRP[testNetworkID]
	bech32Addr, err := address.FormatBech32(hrp, addr.Bytes())
	require.NoError(err)

	nodeKey, nodeID := nodeid.GenerateCaminoNodeKeyAndID()

	consortiumMemberKey, err := testKeyFactory.NewPrivateKey()
	require.NoError(err)

	outputOwners := &secp256k1fx.OutputOwners{
		Locktime:  0,
		Threshold: 1,
		Addrs:     []ids.ShortID{addr},
	}
	caminoGenesisConf := api.Camino{
		VerifyNodeSignature: true,
		LockModeBondDeposit: true,
		InitialAdmin:        addr,
	}
	genesisUTXOs := []api.UTXO{
		{
			Amount:  json.Uint64(defaultCaminoValidatorWeight),
			Address: bech32Addr,
		},
	}

	vm := newCaminoVM(caminoGenesisConf, genesisUTXOs, nil)
	vm.ctx.Lock.Lock()
	defer func() {
		require.NoError(vm.Shutdown(context.Background()))
		vm.ctx.Lock.Unlock()
	}()

	utxo := generateTestUTXO(ids.GenerateTestID(), avaxAssetID, defaultBalance, *outputOwners, ids.Empty, ids.Empty)
	vm.state.AddUTXO(utxo)
	err = vm.state.Commit()
	require.NoError(err)

	// Set consortium member
	tx, err := vm.txBuilder.NewAddressStateTx(
		consortiumMemberKey.Address(),
		false,
		txs.AddressStateBitConsortium,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0]},
		outputOwners,
	)
	require.NoError(err)
	err = vm.Builder.AddUnverifiedTx(tx)
	require.NoError(err)
	blk, err := vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)
	err = blk.Accept(context.Background())
	require.NoError(err)
	err = vm.SetPreference(context.Background(), vm.manager.LastAccepted())
	require.NoError(err)

	// Register node
	tx, err = vm.txBuilder.NewRegisterNodeTx(
		ids.EmptyNodeID,
		nodeID,
		consortiumMemberKey.Address(),
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0], nodeKey, consortiumMemberKey},
		outputOwners,
	)
	require.NoError(err)
	err = vm.Builder.AddUnverifiedTx(tx)
	require.NoError(err)
	blk, err = vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)
	err = blk.Accept(context.Background())
	require.NoError(err)
	err = vm.SetPreference(context.Background(), vm.manager.LastAccepted())
	require.NoError(err)

	// Add the validator
	vm.state.SetShortIDLink(ids.ShortID(nodeID), state.ShortLinkKeyRegisterNode, &addr)
	startTime := vm.clock.Time().Add(txexecutor.SyncBound).Add(1 * time.Second)
	endTime := defaultValidateEndTime.Add(-1 * time.Hour)
	addValidatorTx, err := vm.txBuilder.NewCaminoAddValidatorTx(
		vm.Config.MinValidatorStake,
		uint64(startTime.Unix()),
		uint64(endTime.Unix()),
		nodeID,
		consortiumMemberKey.Address(),
		ids.ShortEmpty,
		reward.PercentDenominator,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0], nodeKey, consortiumMemberKey},
		ids.ShortEmpty,
	)
	require.NoError(err)

	staker, err := state.NewCurrentStaker(
		addValidatorTx.ID(),
		addValidatorTx.Unsigned.(*txs.CaminoAddValidatorTx),
		0,
	)
	require.NoError(err)
	vm.state.PutCurrentValidator(staker)
	vm.state.AddTx(addValidatorTx, status.Committed)
	err = vm.state.Commit()
	require.NoError(err)

	utxo = generateTestUTXO(ids.GenerateTestID(), avaxAssetID, defaultBalance, *outputOwners, ids.Empty, ids.Empty)
	vm.state.AddUTXO(utxo)
	err = vm.state.Commit()
	require.NoError(err)

	// Defer the validator
	tx, err = vm.txBuilder.NewAddressStateTx(
		consortiumMemberKey.Address(),
		false,
		txs.AddressStateBitNodeDeferred,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0]},
		outputOwners,
	)
	require.NoError(err)
	err = vm.Builder.AddUnverifiedTx(tx)
	require.NoError(err)
	blk, err = vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)
	err = blk.Accept(context.Background())
	require.NoError(err)
	err = vm.SetPreference(context.Background(), vm.manager.LastAccepted())
	require.NoError(err)

	// Verify that the validator is deferred (moved from current to deferred stakers set)
	_, err = vm.state.GetCurrentValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)
	_, err = vm.state.GetDeferredValidator(constants.PrimaryNetworkID, nodeID)
	require.NoError(err)

	// Reactivate the validator
	tx, err = vm.txBuilder.NewAddressStateTx(
		consortiumMemberKey.Address(),
		true,
		txs.AddressStateBitNodeDeferred,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0]},
		outputOwners,
	)
	require.NoError(err)
	err = vm.Builder.AddUnverifiedTx(tx)
	require.NoError(err)
	blk, err = vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)
	err = blk.Accept(context.Background())
	require.NoError(err)
	err = vm.SetPreference(context.Background(), vm.manager.LastAccepted())
	require.NoError(err)

	// Verify that the validator is activated again (moved from deferred to current stakers set)
	_, err = vm.state.GetCurrentValidator(constants.PrimaryNetworkID, nodeID)
	require.NoError(err)
	_, err = vm.state.GetDeferredValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)

	// Fast-forward clock to time for validator to be rewarded
	vm.clock.Set(endTime)
	blk, err = vm.Builder.BuildBlock(context.Background())
	require.NoError(err)
	err = blk.Verify(context.Background())
	require.NoError(err)

	// Assert preferences are correct
	block := blk.(smcon.OracleBlock)
	options, err := block.Options(context.Background())
	require.NoError(err)

	commit := options[1].(*blockexecutor.Block)
	_, ok := commit.Block.(*blocks.BanffCommitBlock)
	require.True(ok)

	abort := options[0].(*blockexecutor.Block)
	_, ok = abort.Block.(*blocks.BanffAbortBlock)
	require.True(ok)

	require.NoError(block.Accept(context.Background()))
	require.NoError(commit.Verify(context.Background()))
	require.NoError(abort.Verify(context.Background()))

	txID := blk.(blocks.Block).Txs()[0].ID()
	{
		onAccept, ok := vm.manager.GetState(abort.ID())
		require.True(ok)

		_, txStatus, err := onAccept.GetTx(txID)
		require.NoError(err)
		require.Equal(status.Aborted, txStatus)
	}

	require.NoError(commit.Accept(context.Background()))
	require.NoError(vm.SetPreference(context.Background(), vm.manager.LastAccepted()))

	_, txStatus, err := vm.state.GetTx(txID)
	require.NoError(err)
	require.Equal(status.Committed, txStatus)

	// Verify that the validator is rewarded
	_, err = vm.state.GetCurrentValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)
	_, err = vm.state.GetDeferredValidator(constants.PrimaryNetworkID, nodeID)
	require.ErrorIs(err, database.ErrNotFound)

	timestamp := vm.state.GetTimestamp()
	require.Equal(endTime.Unix(), timestamp.Unix())
}

func TestDepositsAutoUnlock(t *testing.T) {
	require := require.New(t)

	depositOwnerKey, depositOwnerAddr, depositOwner := generateKeyAndOwner(t)
	ownerID, err := txs.GetOwnerID(depositOwner)
	require.NoError(err)
	depositOwnerAddrBech32, err := address.FormatBech32(constants.NetworkIDToHRP[testNetworkID], depositOwnerAddr.Bytes())
	require.NoError(err)

	depositOffer := &deposit.Offer{
		End:                   uint64(defaultGenesisTime.Unix() + 365*24*60*60 + 1),
		MinAmount:             10000,
		MaxDuration:           100,
		InterestRateNominator: 1_000_000 * 365 * 24 * 60 * 60, // 100% per year
	}
	caminoGenesisConf := api.Camino{
		VerifyNodeSignature: true,
		LockModeBondDeposit: true,
		DepositOffers:       []*deposit.Offer{depositOffer},
	}
	require.NoError(genesis.SetDepositOfferID(caminoGenesisConf.DepositOffers[0]))

	vm := newCaminoVM(caminoGenesisConf, []api.UTXO{{
		Amount:  json.Uint64(depositOffer.MinAmount + defaultTxFee),
		Address: depositOwnerAddrBech32,
	}}, nil)
	vm.ctx.Lock.Lock()
	defer func() { require.NoError(vm.Shutdown(context.Background())) }() //nolint:lint

	// Add deposit
	depositTx, err := vm.txBuilder.NewDepositTx(
		depositOffer.MinAmount,
		depositOffer.MaxDuration,
		depositOffer.ID,
		depositOwnerAddr,
		[]*secp256k1.PrivateKey{depositOwnerKey},
		&depositOwner,
	)
	require.NoError(err)
	buildAndAcceptBlock(t, vm, depositTx)
	deposit, err := vm.state.GetDeposit(depositTx.ID())
	require.NoError(err)
	require.Zero(getUnlockedBalance(t, vm.state, treasury.Addr))
	require.Zero(getUnlockedBalance(t, vm.state, depositOwnerAddr))

	// Fast-forward clock to time a bit forward, but still before deposit will be unlocked
	vm.clock.Set(vm.Clock().Time().Add(time.Duration(deposit.Duration) * time.Second / 2))
	_, err = vm.Builder.BuildBlock(context.Background())
	require.Error(err)

	// Fast-forward clock to time for deposit to be unlocked
	vm.clock.Set(deposit.EndTime())
	blk := buildAndAcceptBlock(t, vm, nil)
	txID := blk.Txs()[0].ID()
	onAccept, ok := vm.manager.GetState(blk.ID())
	require.True(ok)
	_, txStatus, err := onAccept.GetTx(txID)
	require.NoError(err)
	require.Equal(status.Committed, txStatus)
	_, txStatus, err = vm.state.GetTx(txID)
	require.NoError(err)
	require.Equal(status.Committed, txStatus)

	// Verify that the deposit is unlocked and reward is transferred to treasury
	_, err = vm.state.GetDeposit(depositTx.ID())
	require.ErrorIs(err, database.ErrNotFound)
	claimable, err := vm.state.GetClaimable(ownerID)
	require.NoError(err)
	require.Equal(&state.Claimable{
		Owner:                &depositOwner,
		ExpiredDepositReward: deposit.TotalReward(depositOffer),
	}, claimable)
	require.Equal(getUnlockedBalance(t, vm.state, depositOwnerAddr), depositOffer.MinAmount)
	require.Equal(deposit.EndTime(), vm.state.GetTimestamp())
	_, err = vm.state.GetNextToUnlockDepositTime(nil)
	require.ErrorIs(err, database.ErrNotFound)
}

// TODO@ proposals threshold success test

func TestProposalsExpiration(t *testing.T) {
	require := require.New(t)

	proposerKey, proposerAddr, _ := generateKeyAndOwner(t)
	proposerAddrStr, err := address.FormatBech32(constants.NetworkIDToHRP[testNetworkID], proposerAddr.Bytes())
	require.NoError(err)
	caminoPreFundedKey0AddrStr, err := address.FormatBech32(constants.NetworkIDToHRP[testNetworkID], caminoPreFundedKeys[0].Address().Bytes())
	require.NoError(err)

	defaultConfig := defaultCaminoConfig(true)
	proposalBondAmount := defaultConfig.CaminoConfig.DACProposalBondAmount
	initialBaseFee := defaultTxFee
	newBaseFee := defaultTxFee + 7
	proposerInitialBalance := proposalBondAmount*3 + initialBaseFee*6 + newBaseFee*2

	// Prepare vm
	vm := newCaminoVM(api.Camino{
		VerifyNodeSignature: true,
		LockModeBondDeposit: true,
		InitialAdmin:        caminoPreFundedKeys[0].Address(),
	}, []api.UTXO{
		{
			Amount:  json.Uint64(proposerInitialBalance),
			Address: proposerAddrStr,
		},
		{
			Amount:  json.Uint64(defaultTxFee),
			Address: caminoPreFundedKey0AddrStr,
		},
	}, &defaultConfig.BanffTime)
	vm.ctx.Lock.Lock()
	defer func() { require.NoError(vm.Shutdown(context.Background())) }() //nolint:lint
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance,          // total
		0, 0, 0, proposerInitialBalance, // unlocked
	)

	// Give proposer address role to make proposals
	addrStateTx, err := vm.txBuilder.NewAddressStateTx(
		proposerAddr,
		false,
		txs.AddressStateBitCaminoProposer,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0]},
		nil,
	)
	require.NoError(err)
	blk := buildAndAcceptBlock(t, vm, addrStateTx)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), addrStateTx.ID())

	// Add proposal1
	currentChainTime := vm.state.GetTimestamp()
	ins, outs, signers, _, err := vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		proposalBondAmount,
		initialBaseFee,
		locked.StateBonded,
		nil, nil, 0,
	)
	require.NoError(err)
	proposal1 := &txs.ProposalWrapper{Proposal: &dac.BaseFeeProposal{
		Start:   uint64(currentChainTime.Add(100 * time.Second).Unix()),
		End:     uint64(currentChainTime.Add(200 * time.Second).Unix()),
		Options: []uint64{newBaseFee + 10, newBaseFee, newBaseFee + 20},
	}}
	proposalBytes1, err := txs.Codec.Marshal(txs.Version, proposal1)
	require.NoError(err)
	proposalTx1, err := txs.NewSigned(&txs.AddProposalTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalPayload: proposalBytes1,
		ProposerAddress: proposerAddr,
		ProposerAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{proposerKey}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, proposalTx1)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), proposalTx1.ID())
	proposalState1, err := vm.state.GetProposal(proposalTx1.ID())
	require.NoError(err)
	nextProposalIDsToExpire, nexExpirationTime, err := vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx1.ID()}, nextProposalIDsToExpire)
	require.Equal(proposalState1.EndTime(), nexExpirationTime)
	proposalIDsToFinish, err := vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee,                          // total
		proposalBondAmount,                                             // bonded
		0, 0, proposerInitialBalance-proposalBondAmount-initialBaseFee, // unlocked
	)

	// Add proposal2
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		proposalBondAmount,
		initialBaseFee,
		locked.StateBonded,
		nil, nil, 0,
	)
	require.NoError(err)
	proposal2 := &txs.ProposalWrapper{Proposal: &dac.BaseFeeProposal{
		Start:   uint64(proposal1.StartTime().Unix()),         // starts when proposal1 starts
		End:     uint64(proposalState1.EndTime().Unix()) + 50, // ends after proposal1
		Options: []uint64{newBaseFee + 100, newBaseFee + 200},
	}}
	proposalBytes2, err := txs.Codec.Marshal(txs.Version, proposal2)
	require.NoError(err)
	proposalTx2, err := txs.NewSigned(&txs.AddProposalTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalPayload: proposalBytes2,
		ProposerAddress: proposerAddr,
		ProposerAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{proposerKey}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, proposalTx2)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), proposalTx2.ID())
	proposalState2, err := vm.state.GetProposal(proposalTx2.ID())
	require.NoError(err)
	nextProposalIDsToExpire, nexExpirationTime, err = vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx1.ID()}, nextProposalIDsToExpire)
	require.Equal(proposalState1.EndTime(), nexExpirationTime)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*2,                            // total
		proposalBondAmount*2,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount*2-initialBaseFee*2, // unlocked
	)

	// Add proposal3
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		proposalBondAmount,
		initialBaseFee,
		locked.StateBonded,
		nil, nil, 0,
	)
	require.NoError(err)
	proposal3 := &txs.ProposalWrapper{Proposal: &dac.BaseFeeProposal{
		Start:   uint64(proposal2.StartTime().Unix()),    // starts when proposal1 and proposal2 start
		End:     uint64(proposalState2.EndTime().Unix()), // ends after proposal1, when proposal2 ends
		Options: []uint64{newBaseFee + 150, newBaseFee + 250},
	}}
	proposalBytes3, err := txs.Codec.Marshal(txs.Version, proposal3)
	require.NoError(err)
	proposalTx3, err := txs.NewSigned(&txs.AddProposalTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalPayload: proposalBytes3,
		ProposerAddress: proposerAddr,
		ProposerAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{proposerKey}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, proposalTx3)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), proposalTx3.ID())
	_, err = vm.state.GetProposal(proposalTx3.ID())
	require.NoError(err)
	nextProposalIDsToExpire, nexExpirationTime, err = vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx1.ID()}, nextProposalIDsToExpire)
	require.Equal(proposalState1.EndTime(), nexExpirationTime)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*3,                            // total
		proposalBondAmount*3,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount*3-initialBaseFee*3, // unlocked
	)

	// Fast-forward clock to time a bit forward, but still before proposals start
	// Verify that we can't vote on proposal1 yet
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		initialBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	voteBytes1, err := txs.Codec.Marshal(txs.Version, &txs.VoteWrapper{Vote: &dac.SimpleVote{OptionIndex: 1}})
	require.NoError(err)
	addVoteTx1, err := txs.NewSigned(&txs.AddVoteTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalID:   proposalTx1.ID(),
		VotePayload:  voteBytes1,
		VoterAddress: caminoPreFundedKeys[0].Address(),
		VoterAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{caminoPreFundedKeys[0]}))
	require.NoError(err)
	vm.clock.Set(proposal1.StartTime().Add(-time.Second))
	_, err = vm.Builder.BuildBlock(context.Background())
	require.Error(err)

	// Fast-forward clock to time when proposals start
	// Vote on proposal1
	vm.clock.Set(proposal1.StartTime())
	blk = buildAndAcceptBlock(t, vm, addVoteTx1)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), addVoteTx1.ID())
	proposalState1, err = vm.state.GetProposal(proposalTx1.ID())
	require.NoError(err)
	baseFeeProposal, ok := proposalState1.(*dac.BaseFeeProposalState)
	require.True(ok)
	require.EqualValues(0, baseFeeProposal.Options[0].Weight)
	require.EqualValues(1, baseFeeProposal.Options[1].Weight) // voted option
	require.EqualValues(0, baseFeeProposal.Options[2].Weight)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*4,                            // total
		proposalBondAmount*3,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount*3-initialBaseFee*4, // unlocked
	)

	// Vote on proposal2
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		initialBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	voteBytes21, err := txs.Codec.Marshal(txs.Version, &txs.VoteWrapper{Vote: &dac.SimpleVote{OptionIndex: 0}})
	require.NoError(err)
	addVoteTx21, err := txs.NewSigned(&txs.AddVoteTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalID:   proposalTx2.ID(),
		VotePayload:  voteBytes21,
		VoterAddress: caminoPreFundedKeys[0].Address(),
		VoterAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{caminoPreFundedKeys[0]}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, addVoteTx21)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), addVoteTx21.ID())
	proposalState2, err = vm.state.GetProposal(proposalTx2.ID())
	require.NoError(err)
	baseFeeProposal, ok = proposalState2.(*dac.BaseFeeProposalState)
	require.True(ok)
	require.EqualValues(1, baseFeeProposal.Options[0].Weight) // voted option
	require.EqualValues(0, baseFeeProposal.Options[1].Weight)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*5,                            // total
		proposalBondAmount*3,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount*3-initialBaseFee*5, // unlocked
	)

	// Vote on proposal2, but on different options to keep it ambiguous
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		initialBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	voteBytes22, err := txs.Codec.Marshal(txs.Version, &txs.VoteWrapper{Vote: &dac.SimpleVote{OptionIndex: 1}})
	require.NoError(err)
	addVoteTx22, err := txs.NewSigned(&txs.AddVoteTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalID:   proposalTx2.ID(),
		VotePayload:  voteBytes22,
		VoterAddress: caminoPreFundedKeys[1].Address(),
		VoterAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{caminoPreFundedKeys[1]}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, addVoteTx22)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), addVoteTx22.ID())
	proposalState2, err = vm.state.GetProposal(proposalTx2.ID())
	require.NoError(err)
	baseFeeProposal, ok = proposalState2.(*dac.BaseFeeProposalState)
	require.True(ok)
	require.EqualValues(1, baseFeeProposal.Options[0].Weight) // voted option
	require.EqualValues(1, baseFeeProposal.Options[1].Weight) // voted option
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*6,                            // total
		proposalBondAmount*3,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount*3-initialBaseFee*6, // unlocked
	)

	// Fast-forward clock to time when proposal is still active
	// Verify that we can't build block yet
	vm.clock.Set(proposalState1.EndTime().Add(-time.Second))
	_, err = vm.Builder.BuildBlock(context.Background())
	require.Error(err)

	// Fast-forward clock to time when proposal1 is expired
	// Verify that proposal1 is executed and removed from state
	vm.clock.Set(proposalState1.EndTime())
	blk = buildAndAcceptBlock(t, vm, nil)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), blk.Txs()[0].ID())
	// proposals 1 is unambiguous and should be removed from state with execution
	_, err = vm.state.GetProposal(proposalTx1.ID())
	require.ErrorIs(err, database.ErrNotFound)
	baseFee, err := vm.state.GetBaseFee()
	require.NoError(err)
	require.Equal(newBaseFee, baseFee)
	nextProposalIDsToExpire, nexExpirationTime, err = vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx2.ID(), proposalTx3.ID()}, nextProposalIDsToExpire)
	require.Equal(proposalState2.EndTime(), nexExpirationTime)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*6,                            // total
		proposalBondAmount*2,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount*2-initialBaseFee*6, // unlocked
	)

	// Create arbitrary tx to verify that it will use new base fee
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		newBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	feeTestingTx, err := txs.NewSigned(&txs.MultisigAliasTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		MultisigAlias: multisig.Alias{
			Owners: &secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{proposerAddr},
			},
		},
		Auth: &secp256k1fx.Input{},
	}, txs.Codec, signers)
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, feeTestingTx)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), feeTestingTx.ID())
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*6-newBaseFee,                            // total
		proposalBondAmount*2,                                                          // bonded
		0, 0, proposerInitialBalance-proposalBondAmount*2-initialBaseFee*6-newBaseFee, // unlocked
	)

	// Fast-forward clock to time when proposal2-3 are expired
	vm.clock.Set(proposalState2.EndTime())
	blk = buildAndAcceptBlock(t, vm, nil)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), blk.Txs()[0].ID())
	// proposals 2 and 3 are ambiguous and should be removed from state without execution
	baseFee, err = vm.state.GetBaseFee()
	require.NoError(err)
	require.Equal(newBaseFee, baseFee)
	_, _, err = vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.ErrorIs(err, database.ErrNotFound)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*6-newBaseFee, // total
		0, 0, 0, proposerInitialBalance-initialBaseFee*6-newBaseFee, // unlocked
	)

	// Create arbitrary tx to verify that it still uses base fee from proposalTx1
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		newBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	feeTestingTx, err = txs.NewSigned(&txs.MultisigAliasTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		MultisigAlias: multisig.Alias{
			Owners: &secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{proposerAddr},
			},
		},
		Auth: &secp256k1fx.Input{},
	}, txs.Codec, signers)
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, feeTestingTx)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), feeTestingTx.ID())
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*6-newBaseFee*2, // total
		0, 0, 0, proposerInitialBalance-initialBaseFee*6-newBaseFee*2, // unlocked
	)
}

func TestProposalsThresholdExecution(t *testing.T) {
	require := require.New(t)

	proposerKey, proposerAddr, _ := generateKeyAndOwner(t)
	proposerAddrStr, err := address.FormatBech32(constants.NetworkIDToHRP[testNetworkID], proposerAddr.Bytes())
	require.NoError(err)
	caminoPreFundedKey0AddrStr, err := address.FormatBech32(constants.NetworkIDToHRP[testNetworkID], caminoPreFundedKeys[0].Address().Bytes())
	require.NoError(err)

	defaultConfig := defaultCaminoConfig(true)
	proposalBondAmount := defaultConfig.CaminoConfig.DACProposalBondAmount
	initialBaseFee := defaultTxFee
	newBaseFee := defaultTxFee + 7
	proposerInitialBalance := proposalBondAmount + initialBaseFee*4 + newBaseFee

	// Prepare vm
	vm := newCaminoVM(api.Camino{
		VerifyNodeSignature: true,
		LockModeBondDeposit: true,
		InitialAdmin:        caminoPreFundedKeys[0].Address(),
	}, []api.UTXO{
		{
			Amount:  json.Uint64(proposerInitialBalance),
			Address: proposerAddrStr,
		},
		{
			Amount:  json.Uint64(defaultTxFee),
			Address: caminoPreFundedKey0AddrStr,
		},
	}, &defaultConfig.BanffTime)
	vm.ctx.Lock.Lock()
	defer func() { require.NoError(vm.Shutdown(context.Background())) }() //nolint:lint
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance,          // total
		0, 0, 0, proposerInitialBalance, // unlocked
	)

	// Give proposer address role to make proposals
	addrStateTx, err := vm.txBuilder.NewAddressStateTx(
		proposerAddr,
		false,
		txs.AddressStateBitCaminoProposer,
		[]*secp256k1.PrivateKey{caminoPreFundedKeys[0]},
		nil,
	)
	require.NoError(err)
	blk := buildAndAcceptBlock(t, vm, addrStateTx)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), addrStateTx.ID())

	// Add proposal
	currentChainTime := vm.state.GetTimestamp()
	ins, outs, signers, _, err := vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		proposalBondAmount,
		initialBaseFee,
		locked.StateBonded,
		nil, nil, 0,
	)
	require.NoError(err)
	proposal := &txs.ProposalWrapper{Proposal: &dac.BaseFeeProposal{
		Start:   uint64(currentChainTime.Add(100 * time.Second).Unix()),
		End:     uint64(currentChainTime.Add(200 * time.Second).Unix()),
		Options: []uint64{newBaseFee},
	}}
	proposalBytes1, err := txs.Codec.Marshal(txs.Version, proposal)
	require.NoError(err)
	proposalTx, err := txs.NewSigned(&txs.AddProposalTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalPayload: proposalBytes1,
		ProposerAddress: proposerAddr,
		ProposerAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{proposerKey}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, proposalTx)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), proposalTx.ID())
	proposalState, err := vm.state.GetProposal(proposalTx.ID())
	require.NoError(err)
	nextProposalIDsToExpire, nexExpirationTime, err := vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx.ID()}, nextProposalIDsToExpire)
	require.Equal(proposalState.EndTime(), nexExpirationTime)
	proposalIDsToFinish, err := vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee,                          // total
		proposalBondAmount,                                             // bonded
		0, 0, proposerInitialBalance-proposalBondAmount-initialBaseFee, // unlocked
	)

	// Fast-forward clock to time when proposals start, so we can vote
	vm.clock.Set(proposal.StartTime())
	voteBytes, err := txs.Codec.Marshal(txs.Version, &txs.VoteWrapper{Vote: &dac.SimpleVote{OptionIndex: 0}})
	require.NoError(err)

	// Vote on proposal by validator0
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		initialBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	addVoteTx0, err := txs.NewSigned(&txs.AddVoteTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalID:   proposalTx.ID(),
		VotePayload:  voteBytes,
		VoterAddress: caminoPreFundedKeys[0].Address(),
		VoterAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{caminoPreFundedKeys[0]}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, addVoteTx0)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), addVoteTx0.ID())
	proposalState, err = vm.state.GetProposal(proposalTx.ID())
	require.NoError(err)
	baseFeeProposal, ok := proposalState.(*dac.BaseFeeProposalState)
	require.True(ok)
	require.EqualValues(1, baseFeeProposal.Options[0].Weight)
	nextProposalIDsToExpire, nexExpirationTime, err = vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx.ID()}, nextProposalIDsToExpire)
	require.Equal(proposalState.EndTime(), nexExpirationTime)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*2,                          // total
		proposalBondAmount,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount-initialBaseFee*2, // unlocked
	)

	// Vote on proposal by validator1
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		initialBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	addVoteTx1, err := txs.NewSigned(&txs.AddVoteTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalID:   proposalTx.ID(),
		VotePayload:  voteBytes,
		VoterAddress: caminoPreFundedKeys[1].Address(),
		VoterAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{caminoPreFundedKeys[1]}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, addVoteTx1)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), addVoteTx1.ID())
	proposalState, err = vm.state.GetProposal(proposalTx.ID())
	require.NoError(err)
	baseFeeProposal, ok = proposalState.(*dac.BaseFeeProposalState)
	require.True(ok)
	require.EqualValues(2, baseFeeProposal.Options[0].Weight)
	require.EqualValues(2, baseFeeProposal.SuccessThreshold)
	nextProposalIDsToExpire, nexExpirationTime, err = vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx.ID()}, nextProposalIDsToExpire)
	require.Equal(proposalState.EndTime(), nexExpirationTime)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*3,                          // total
		proposalBondAmount,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount-initialBaseFee*3, // unlocked
	)

	// Vote on proposal by validator2
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		initialBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	addVoteTx2, err := txs.NewSigned(&txs.AddVoteTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		ProposalID:   proposalTx.ID(),
		VotePayload:  voteBytes,
		VoterAddress: caminoPreFundedKeys[2].Address(),
		VoterAuth:    &secp256k1fx.Input{SigIndices: []uint32{0}},
	}, txs.Codec, append(signers, []*secp256k1.PrivateKey{caminoPreFundedKeys[2]}))
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, addVoteTx2)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), addVoteTx2.ID())
	proposalState, err = vm.state.GetProposal(proposalTx.ID())
	require.NoError(err)
	baseFeeProposal, ok = proposalState.(*dac.BaseFeeProposalState)
	require.True(ok)
	require.EqualValues(3, baseFeeProposal.Options[0].Weight)
	nextProposalIDsToExpire, nexExpirationTime, err = vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx.ID()}, nextProposalIDsToExpire)
	require.Equal(proposalState.EndTime(), nexExpirationTime)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Equal([]ids.ID{proposalTx.ID()}, proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*4,                          // total
		proposalBondAmount,                                               // bonded
		0, 0, proposerInitialBalance-proposalBondAmount-initialBaseFee*4, // unlocked
	)

	// Proposal votes threshold is reached, so its automatically executed
	blk = buildAndAcceptBlock(t, vm, nil)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), blk.Txs()[0].ID())
	// proposals 1 is unambiguous and should be removed from state with execution
	_, err = vm.state.GetProposal(proposalTx.ID())
	require.ErrorIs(err, database.ErrNotFound)
	baseFee, err := vm.state.GetBaseFee()
	require.NoError(err)
	require.Equal(newBaseFee, baseFee)
	_, _, err = vm.state.GetNextToExpireProposalIDsAndTime(nil)
	require.ErrorIs(err, database.ErrNotFound)
	proposalIDsToFinish, err = vm.state.GetProposalIDsToFinish()
	require.NoError(err)
	require.Empty(proposalIDsToFinish)
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*4, // total
		0, 0, 0, proposerInitialBalance-initialBaseFee*4, // unlocked
	)

	// Create arbitrary tx to verify that it will use new base fee
	ins, outs, signers, _, err = vm.txBuilder.Lock(
		vm.state,
		[]*secp256k1.PrivateKey{proposerKey},
		0,
		newBaseFee,
		locked.StateUnlocked,
		nil, nil, 0,
	)
	require.NoError(err)
	feeTestingTx, err := txs.NewSigned(&txs.MultisigAliasTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		MultisigAlias: multisig.Alias{
			Owners: &secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{proposerAddr},
			},
		},
		Auth: &secp256k1fx.Input{},
	}, txs.Codec, signers)
	require.NoError(err)
	blk = buildAndAcceptBlock(t, vm, feeTestingTx)
	require.Len(blk.Txs(), 1)
	checkTx(t, vm, blk.ID(), feeTestingTx.ID())
	checkBalance(t, vm.state, proposerAddr,
		proposerInitialBalance-initialBaseFee*4-newBaseFee, // total
		0, 0, 0, proposerInitialBalance-initialBaseFee*4-newBaseFee, // unlocked
	)
}

func buildAndAcceptBlock(t *testing.T, vm *VM, tx *txs.Tx) blocks.Block {
	t.Helper()
	if tx != nil {
		require.NoError(t, vm.Builder.AddUnverifiedTx(tx))
	}
	blk, err := vm.Builder.BuildBlock(context.Background())
	require.NoError(t, err)
	block, ok := blk.(blocks.Block)
	require.True(t, ok)
	require.NoError(t, blk.Verify(context.Background()))
	require.NoError(t, blk.Accept(context.Background()))
	require.NoError(t, vm.SetPreference(context.Background(), vm.manager.LastAccepted()))

	return block
}

func getUnlockedBalance(t *testing.T, db avax.UTXOReader, addr ids.ShortID) uint64 {
	t.Helper()
	utxos, err := avax.GetAllUTXOs(db, set.Set[ids.ShortID]{addr: struct{}{}})
	require.NoError(t, err)
	balance := uint64(0)
	for _, utxo := range utxos {
		if out, ok := utxo.Out.(*secp256k1fx.TransferOutput); ok {
			balance += out.Amount()
		}
	}
	return balance
}

func getBalance(t *testing.T, db avax.UTXOReader, addr ids.ShortID) (total, bonded, deposited, depositBonded, unlocked uint64) {
	t.Helper()
	utxos, err := avax.GetAllUTXOs(db, set.Set[ids.ShortID]{addr: struct{}{}})
	require.NoError(t, err)
	for _, utxo := range utxos {
		if out, ok := utxo.Out.(*secp256k1fx.TransferOutput); ok {
			unlocked += out.Amount()
			total += out.Amount()
		} else {
			out, ok := utxo.Out.(*locked.Out)
			require.True(t, ok)
			switch out.LockState() {
			case locked.StateDepositedBonded:
				depositBonded += out.Amount()
			case locked.StateDeposited:
				deposited += out.Amount()
			case locked.StateBonded:
				bonded += out.Amount()
			}
			total += out.Amount()
		}
	}
	return
}

func checkBalance(
	t *testing.T, db avax.UTXOReader, addr ids.ShortID,
	expectedTotal, expectedBonded, expectedDeposited, expectedDepositBonded, expectedUnlocked uint64,
) {
	t.Helper()
	total, bonded, deposited, depositBonded, unlocked := getBalance(t, db, addr)
	require.Equal(t, expectedTotal, total)
	require.Equal(t, expectedBonded, bonded)
	require.Equal(t, expectedDeposited, deposited)
	require.Equal(t, expectedDepositBonded, depositBonded)
	require.Equal(t, expectedUnlocked, unlocked)
}

func checkTx(t *testing.T, vm *VM, blkID, txID ids.ID) {
	t.Helper()
	state, ok := vm.manager.GetState(blkID)
	require.True(t, ok)
	_, txStatus, err := state.GetTx(txID)
	require.NoError(t, err)
	require.Equal(t, status.Committed, txStatus)
	_, txStatus, err = vm.state.GetTx(txID)
	require.NoError(t, err)
	require.Equal(t, status.Committed, txStatus)
}
