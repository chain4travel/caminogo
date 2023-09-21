// Copyright (C) 2023, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************
// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package touristicvm

import (
	"context"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/rpc"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	pApi "github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
)

var _ Client = (*client)(nil)

// Client interface for interacting with the P Chain endpoint
type Client interface {
	// IssueTx issues the transaction and returns its txID
	IssueTx(ctx context.Context, tx []byte, options ...rpc.Option) (ids.ID, error)
	// GetTxStatus returns the status of the transaction corresponding to [txID]
	GetTxStatus(ctx context.Context, txID ids.ID, options ...rpc.Option) (*GetTxStatusResponse, error)

	// Helps to construct ins and outs
	SpendWithWrapper(
		ctx context.Context,
		from ids.ShortID,
		agent ids.ShortID,
		to ids.ShortID,
		amountToTransitState uint64,
		amountToBurn uint64,
		lockMode locked.State,
		changeOwner pApi.Owner,
		options ...rpc.Option,
	) (
		ins []*avax.TransferableInput,
		outs []*avax.TransferableOutput,
		signers [][]ids.ShortID,
		owners []*secp256k1fx.OutputOwners,
		err error,
	)
}

// Client implementation for interacting with the P Chain endpoint
type client struct {
	requester rpc.EndpointRequester
}

// NewClient returns a Client for interacting with the P Chain endpoint
func NewClient(uri string) Client {
	return &client{requester: rpc.NewEndpointRequester(
		uri + "/ext/T",
	)}
}

func (c *client) IssueTx(ctx context.Context, txBytes []byte, options ...rpc.Option) (ids.ID, error) {
	txStr, err := formatting.Encode(formatting.Hex, txBytes)
	if err != nil {
		return ids.ID{}, err
	}

	res := &api.JSONTxID{}
	err = c.requester.SendRequest(ctx, "touristicvm.issueTx", &api.FormattedTx{
		Tx:       txStr,
		Encoding: formatting.Hex,
	}, res, options...)
	return res.TxID, err
}

func (c *client) GetTxStatus(ctx context.Context, txID ids.ID, options ...rpc.Option) (*GetTxStatusResponse, error) {
	res := &GetTxStatusResponse{}
	err := c.requester.SendRequest(
		ctx,
		"touristicvm.getTxStatus",
		&api.JSONTxID{txID},
		res,
		options...,
	)
	return res, err
}

func (c *client) SpendWithWrapper(
	ctx context.Context,
	from ids.ShortID,
	agent ids.ShortID,
	to ids.ShortID,
	amountToTransitState uint64,
	amountToBurn uint64,
	lockMode locked.State,
	changeOwner pApi.Owner,
	options ...rpc.Option,
) (
	ins []*avax.TransferableInput,
	outs []*avax.TransferableOutput,
	signers [][]ids.ShortID,
	owners []*secp256k1fx.OutputOwners,
	err error,
) {
	res := &SpendWithWrapperReply{}
	if err := c.requester.SendRequest(ctx, "touristicvm.spendWithWrapper", &SpendWithWrapperArgs{
		JSONFromAddrs: api.JSONFromAddrs{
			From: []string{from.String()},
		},
		Agent: agent.String(),
		To: pApi.Owner{
			Threshold: 1,
			Addresses: []string{to.String()},
		},
		Change:               changeOwner,
		AmountToTransitState: json.Uint64(amountToTransitState),
		AmountToBurn:         json.Uint64(amountToBurn),
		LockMode:             byte(lockMode),
		Encoding:             formatting.Hex,
	}, res, options...); err != nil {
		return nil, nil, nil, nil, err
	}

	wrapperBytes, err := formatting.Decode(formatting.Hex, res.EncodedData)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	wrapper := &SpendWrapper{}
	if _, err := txs.Codec.Unmarshal(wrapperBytes, wrapper); err != nil {
		return nil, nil, nil, nil, err
	}

	return wrapper.Ins, wrapper.Outs, res.Signers, wrapper.Owners, err
}
