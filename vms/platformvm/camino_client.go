// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/rpc"
	as "github.com/ava-labs/avalanchego/vms/platformvm/addrstate"
	platformapi "github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

type DepositWithAvailableReward struct {
	Deposit         deposit.Deposit
	AvailableReward uint64
}

type CaminoClient interface {
	// GetConfiguration returns genesis information of the primary network
	GetConfiguration(ctx context.Context, options ...rpc.Option) (*GetConfigurationReply, error)

	// GetMultisigAlias returns the alias definition of the given multisig address
	GetMultisigAlias(ctx context.Context, multisigAddress string, options ...rpc.Option) (*GetMultisigAliasReply, error)

	GetAllDepositOffers(ctx context.Context, getAllDepositOffersArgs *GetAllDepositOffersArgs, options ...rpc.Option) ([]*deposit.Offer, error)

	GetRegisteredShortIDLink(ctx context.Context, addrOrNodeID string, options ...rpc.Option) (ids.ShortID, error)
	GetLastAcceptedBlock(ctx context.Context, encoding formatting.Encoding, options ...rpc.Option) (any, error)
	GetBlockAtHeight(ctx context.Context, height uint32, encoding formatting.Encoding, options ...rpc.Option) (any, error)
	GetClaimables(ctx context.Context, owners []*secp256k1fx.OutputOwners, options ...rpc.Option) ([]*state.Claimable, error)
	GetAddressStates(ctx context.Context, addr ids.ShortID, options ...rpc.Option) (as.AddressState, error)

	GetDeposits(ctx context.Context, depositTxIDs []ids.ID, options ...rpc.Option) ([]DepositWithAvailableReward, time.Time, error)
}

func (c *client) GetConfiguration(ctx context.Context, options ...rpc.Option) (*GetConfigurationReply, error) {
	res := &GetConfigurationReply{}
	err := c.requester.SendRequest(ctx, "platform.getConfiguration", struct{}{}, res, options...)
	return res, err
}

func (c *client) GetMultisigAlias(ctx context.Context, multisigAddress string, options ...rpc.Option) (*GetMultisigAliasReply, error) {
	res := &GetMultisigAliasReply{}
	err := c.requester.SendRequest(ctx, "platform.getMultisigAlias", &api.JSONAddress{
		Address: multisigAddress,
	}, res, options...)
	return res, err
}

func (c *client) GetAllDepositOffers(ctx context.Context, getAllDepositOffersArgs *GetAllDepositOffersArgs, options ...rpc.Option) ([]*deposit.Offer, error) {
	res := &GetAllDepositOffersReply{}
	err := c.requester.SendRequest(ctx, "platform.getAllDepositOffers", &getAllDepositOffersArgs, res, options...)
	if err != nil {
		return nil, err
	}

	offers := make([]*deposit.Offer, len(res.DepositOffers))
	for i, apiOffer := range res.DepositOffers {
		offers[i] = offerFromAPI(apiOffer)
	}

	return offers, nil
}

func (c *client) GetRegisteredShortIDLink(ctx context.Context, addrOrNodeID string, options ...rpc.Option) (ids.ShortID, error) {
	res := &api.JSONAddress{}
	err := c.requester.SendRequest(ctx, "platform.getRegisteredShortIDLink", &api.JSONAddress{
		Address: addrOrNodeID,
	}, res, options...)
	if err != nil {
		return ids.ShortEmpty, err
	}
	if _, _, addrBytes, err := address.Parse(res.Address); err == nil {
		return ids.ToShortID(addrBytes)
	}
	nodeID, err := ids.NodeIDFromString(res.Address)
	return ids.ShortID(nodeID), err
}

func (c *client) GetDeposits(ctx context.Context, depositTxIDs []ids.ID, options ...rpc.Option) ([]DepositWithAvailableReward, time.Time, error) {
	res := &GetDepositsReply{}
	err := c.requester.SendRequest(ctx, "platform.getDeposits", &GetDepositsArgs{
		DepositTxIDs: depositTxIDs,
	}, res, options...)
	if err != nil {
		return nil, time.Time{}, err
	}

	if len(res.Deposits) != len(res.AvailableRewards) {
		return nil, time.Time{}, fmt.Errorf("len(deposits) != len(availableRewards): %d != %d", len(res.Deposits), len(res.AvailableRewards))
	}

	deposits := make([]DepositWithAvailableReward, len(res.Deposits))
	for i, apiDeposit := range res.Deposits {
		deposit := &deposit.Deposit{
			DepositOfferID:      apiDeposit.DepositOfferID,
			UnlockedAmount:      uint64(apiDeposit.UnlockedAmount),
			ClaimedRewardAmount: uint64(apiDeposit.ClaimedRewardAmount),
			Start:               uint64(apiDeposit.Start),
			Duration:            apiDeposit.Duration,
			Amount:              uint64(apiDeposit.Amount),
		}
		rewardOwner := &secp256k1fx.OutputOwners{
			Threshold: uint32(apiDeposit.RewardOwner.Threshold),
			Addrs:     make([]ids.ShortID, len(apiDeposit.RewardOwner.Addresses)),
		}
		for i := range apiDeposit.RewardOwner.Addresses {
			addr, err := address.ParseToID(apiDeposit.RewardOwner.Addresses[i])
			if err != nil {
				return nil, time.Time{}, err
			}
			rewardOwner.Addrs[i] = addr
		}
		deposit.RewardOwner = rewardOwner
		deposits[i] = DepositWithAvailableReward{
			Deposit:         *deposit,
			AvailableReward: uint64(res.AvailableRewards[i]),
		}
	}

	return deposits, time.Unix(int64(res.Timestamp), 0), nil
}

func (c *client) GetLastAcceptedBlock(ctx context.Context, encoding formatting.Encoding, options ...rpc.Option) (any, error) {
	res := &api.GetBlockResponse{}
	err := c.requester.SendRequest(ctx, "platform.getLastAcceptedBlock", &api.Encoding{
		Encoding: encoding,
	}, res, options...)
	return res.Block, err
}

func (c *client) GetBlockAtHeight(ctx context.Context, height uint32, encoding formatting.Encoding, options ...rpc.Option) (any, error) {
	res := &api.GetBlockResponse{}
	err := c.requester.SendRequest(ctx, "platform.getBlockAtHeight", &GetBlockAtHeightArgs{
		Height:   height,
		Encoding: encoding,
	}, res, options...)
	return res.Block, err
}

func (c *client) GetClaimables(ctx context.Context, owners []*secp256k1fx.OutputOwners, options ...rpc.Option) ([]*state.Claimable, error) {
	res := &GetClaimablesReply{}
	if err := c.requester.SendRequest(ctx, "platform.getClaimables", &GetClaimablesArgs{
		Owners: apiOwnersFromSECP(owners),
	}, res, options...); err != nil {
		return nil, err
	}
	return claimablesFromAPI(res.Claimables)
}

func (c *client) GetAddressStates(ctx context.Context, addr ids.ShortID, options ...rpc.Option) (as.AddressState, error) {
	res := new(json.Uint64)
	err := c.requester.SendRequest(ctx, "platform.getAddressStates", &api.JSONAddress{
		Address: addr.String(),
	}, res, options...)
	return as.AddressState(*res), err
}

func claimablesFromAPI(apiClaimables []APIClaimable) ([]*state.Claimable, error) {
	claimables := make([]*state.Claimable, len(apiClaimables))
	for i := range claimables {
		claimable, err := claimableFromAPI(&apiClaimables[i])
		if err != nil {
			return nil, err
		}
		claimables[i] = &claimable
	}
	return claimables, nil
}

func claimableFromAPI(apiClaimable *APIClaimable) (state.Claimable, error) {
	claimableOwner, err := secpOwnerFromAPI(&apiClaimable.RewardOwner)
	if err != nil {
		return state.Claimable{}, err
	}
	return state.Claimable{
		Owner:                claimableOwner,
		ValidatorReward:      uint64(apiClaimable.ValidatorRewards),
		ExpiredDepositReward: uint64(apiClaimable.ExpiredDepositRewards),
	}, nil
}

func apiOwnersFromSECP(secpOwners []*secp256k1fx.OutputOwners) []platformapi.Owner {
	owners := make([]platformapi.Owner, len(secpOwners))
	for i := range owners {
		owners[i] = *apiOwnerFromSECP(secpOwners[i])
	}
	return owners
}

func apiOwnerFromSECP(secpOwner *secp256k1fx.OutputOwners) *platformapi.Owner {
	apiOwner := &platformapi.Owner{
		Locktime:  json.Uint64(secpOwner.Locktime),
		Threshold: json.Uint32(secpOwner.Threshold),
		Addresses: make([]string, len(secpOwner.Addrs)),
	}
	for i := range secpOwner.Addrs {
		apiOwner.Addresses[i] = secpOwner.Addrs[i].String()
	}
	return apiOwner
}

func secpOwnerFromAPI(apiOwner *platformapi.Owner) (*secp256k1fx.OutputOwners, error) {
	if len(apiOwner.Addresses) > 0 {
		secpOwner := &secp256k1fx.OutputOwners{
			Locktime:  uint64(apiOwner.Locktime),
			Threshold: uint32(apiOwner.Threshold),
			Addrs:     make([]ids.ShortID, len(apiOwner.Addresses)),
		}
		for i := range apiOwner.Addresses {
			addr, err := parseAddr(apiOwner.Addresses[i])
			if err != nil {
				return nil, err
			}
			secpOwner.Addrs[i] = addr
		}
		secpOwner.Sort()
		return secpOwner, nil
	}
	return nil, nil
}

func offerFromAPI(apiOffer *APIDepositOffer) *deposit.Offer {
	return &deposit.Offer{
		UpgradeVersionID:        codec.UpgradeVersionID(apiOffer.UpgradeVersion),
		ID:                      apiOffer.ID,
		InterestRateNominator:   uint64(apiOffer.InterestRateNominator),
		Start:                   uint64(apiOffer.Start),
		End:                     uint64(apiOffer.End),
		MinAmount:               uint64(apiOffer.MinAmount),
		TotalMaxAmount:          uint64(apiOffer.TotalMaxAmount),
		DepositedAmount:         uint64(apiOffer.DepositedAmount),
		MinDuration:             apiOffer.MinDuration,
		MaxDuration:             apiOffer.MaxDuration,
		UnlockPeriodDuration:    apiOffer.UnlockPeriodDuration,
		NoRewardsPeriodDuration: apiOffer.NoRewardsPeriodDuration,
		Memo:                    apiOffer.Memo,
		Flags:                   deposit.OfferFlag(apiOffer.Flags),
		TotalMaxRewardAmount:    uint64(apiOffer.TotalMaxRewardAmount),
		RewardedAmount:          uint64(apiOffer.RewardedAmount),
		OwnerAddress:            apiOffer.OwnerAddress,
	}
}

func parseAddr(addrStr string) (ids.ShortID, error) {
	addr, err1 := address.ParseToID(addrStr)
	if err1 == nil {
		return addr, nil
	}
	addr, err2 := ids.ShortFromString(addrStr)
	if err2 != nil {
		return ids.ShortEmpty, fmt.Errorf("failed to parse addr both as shortID (%s) and as bech32 (%s)",
			err2, err1)
	}
	return addr, nil
}
