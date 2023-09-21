// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package touristicvm

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/blocks/executor"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	"github.com/ava-labs/avalanchego/vms/touristicvm/status"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs/builder"

	utilsjson "github.com/ava-labs/avalanchego/utils/json"
	platformapi "github.com/ava-labs/avalanchego/vms/platformvm/api"
)

const (
	// Max number of addresses that can be passed in as argument to GetUTXOs
	maxGetUTXOsAddrs = 1024

	// Max number of items allowed in a page
	maxPageSize uint64 = 1024
)

var (
	errBadData                = errors.New("data must be hex representation of 32 bytes")
	errNoSuchBlock            = errors.New("couldn't get block from database. Does it exist?")
	errUnexpectedBlockType    = errors.New("unexpected block type")
	errCannotGetLastAccepted  = errors.New("problem getting last accepted")
	errNoAddresses            = errors.New("no addresses provided")
	errNoKeys                 = errors.New("user has no keys or funds")
	errInvalidChangeAddr      = "couldn't parse changeAddr: %w"
	errCreateTransferables    = errors.New("can't create transferables")
	errSerializeTransferables = errors.New("can't serialize transferables")
	errEncodeTransferables    = errors.New("can't encode transferables as string")
	errSerializeOwners        = errors.New("can't serialize owners")
	errInvalidKeysNumber      = errors.New("expected different number of keys")
	errUnknownLockState       = errors.New("unknown lockState")
)

// Service is the API service for this VM
type Service struct {
	vm          *VM
	addrManager avax.AddressManager
}

// ProposeBlockArgs are the arguments to function ProposeValue

// GetBlockArgs are the arguments to GetBlock
type GetBlockArgs struct {
	// ID of the block we're getting.
	// If left blank, gets the latest block
	BlockID  *ids.ID             `json:"blockID"` // to be consistent across vms
	Encoding formatting.Encoding `json:"encoding"`
}

// GetBlockReply is the reply from GetBlock
type GetBlockReply struct {
	ID    ids.ID      `json:"id"` // String repr. of ID of block
	Block interface{} `json:"block"`
}

// GetBlock gets the block whose ID is [args.ID]
// If [args.ID] is empty, get the latest block
func (s *Service) GetBlock(r *http.Request, args *GetBlockArgs, reply *GetBlockReply) error {
	s.vm.snowCtx.Log.Debug("API called",
		zap.String("service", "touristic"),
		zap.String("method", "getBlock"),
	)

	// If an ID is given, parse its string representation to an ids.ID
	// If no ID is given, ID becomes the ID of last accepted block
	var (
		id  ids.ID
		err error
	)

	if args.BlockID == nil {
		id = s.vm.State.GetLastAccepted()
		// TODO nikos check change here
		//if err != nil {
		//	return errCannotGetLastAccepted
		//}
	} else {
		id = *args.BlockID
	}
	ctx := r.Context()
	// Get the block from the database
	block, err := s.vm.GetBlock(ctx, id)
	if err != nil {
		return errNoSuchBlock
	}

	if args.Encoding == formatting.JSON {
		blk, ok := block.(*executor.Block)
		if !ok {
			return errUnexpectedBlockType
		}
		blk.Block.InitCtx(s.vm.snowCtx)
		reply.Block = blk.Block
	} else {
		reply.Block, err = formatting.Encode(args.Encoding, block.Bytes())
		if err != nil {
			return fmt.Errorf("couldn't encode block %s as string: %w", args.BlockID, err)
		}
	}

	reply.ID = block.ID()

	return err
}

// IssueTx issues a tx
func (s *Service) IssueTx(_ *http.Request, args *api.FormattedTx, response *api.JSONTxID) error {
	s.vm.snowCtx.Log.Debug("API called",
		zap.String("service", "touristic"),
		zap.String("method", "issueTx"),
	)

	txBytes, err := formatting.Decode(args.Encoding, args.Tx)
	if err != nil {
		return fmt.Errorf("problem decoding transaction: %w", err)
	}
	tx, err := txs.Parse(txs.Codec, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't parse tx: %w", err)
	}
	if err := s.vm.Builder.AddUnverifiedTx(tx); err != nil {
		return fmt.Errorf("couldn't issue tx: %w", err)
	}

	response.TxID = tx.ID()
	return nil
}

// GetTx gets a tx
func (s *Service) GetTx(_ *http.Request, args *api.GetTxArgs, response *api.GetTxReply) error {
	s.vm.snowCtx.Log.Debug("API called",
		zap.String("service", "touristic"),
		zap.String("method", "getTx"),
	)

	tx, _, err := s.vm.State.GetTx(args.TxID)
	if err != nil {
		return fmt.Errorf("couldn't get tx: %w", err)
	}
	txBytes := tx.Bytes()
	response.Encoding = args.Encoding

	if args.Encoding == formatting.JSON {
		tx.Unsigned.InitCtx(s.vm.snowCtx)
		response.Tx = tx
		return nil
	}

	response.Tx, err = formatting.Encode(args.Encoding, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't encode tx as a string: %w", err)
	}
	return nil
}

type GetTxStatusResponse struct {
	Status status.Status `json:"status"`
	// Reason this tx was dropped.
	// Only non-empty if Status is dropped
	Reason string `json:"reason,omitempty"`
}

// GetTxStatus gets a tx's status
func (s *Service) GetTxStatus(_ *http.Request, args *api.JSONTxID, response *GetTxStatusResponse) error {
	s.vm.snowCtx.Log.Debug("API called",
		zap.String("service", "touristic"),
		zap.String("method", "getTxStatus"),
	)

	_, txStatus, err := s.vm.State.GetTx(args.TxID)
	if err == nil { // Found the status. Report it.
		response.Status = txStatus
		return nil
	}
	if err != database.ErrNotFound {
		return err
	}

	// The status of this transaction is not in the database - check if the tx
	// is in the preferred block's db. If so, return that it's processing.
	prefBlk, err := s.vm.Preferred()
	if err != nil {
		return err
	}

	preferredID := prefBlk.ID()
	onAccept, ok := s.vm.manager.GetState(preferredID)
	if !ok {
		return fmt.Errorf("could not retrieve state for block %s", preferredID)
	}

	_, _, err = onAccept.GetTx(args.TxID)
	if err == nil {
		// Found the status in the preferred block's db. Report tx is processing.
		response.Status = status.Processing
		return nil
	}
	if err != database.ErrNotFound {
		return err
	}

	if s.vm.Builder.Has(args.TxID) {
		// Found the tx in the mempool. Report tx is processing.
		response.Status = status.Processing
		return nil
	}

	// Note: we check if tx is dropped only after having looked for it
	// in the database and the mempool, because dropped txs may be re-issued.
	reason := s.vm.Builder.GetDropReason(args.TxID)
	if reason == nil {
		// The tx isn't being tracked by the node.
		response.Status = status.Unknown
		return nil
	}

	// The tx was recently dropped because it was invalid.
	response.Status = status.Dropped
	response.Reason = reason.Error()
	return nil
}

// GetUTXOs gets all utxos for passed in addresses
func (s *Service) GetUTXOs(_ *http.Request, args *api.GetUTXOsArgs, response *api.GetUTXOsReply) error {
	s.vm.snowCtx.Log.Debug("API called",
		zap.String("service", "touristicVM"),
		zap.String("method", "getUTXOs"),
	)

	if len(args.Addresses) == 0 {
		return errNoAddresses
	}
	if len(args.Addresses) > maxGetUTXOsAddrs {
		return fmt.Errorf("number of addresses given, %d, exceeds maximum, %d", len(args.Addresses), maxGetUTXOsAddrs)
	}

	var sourceChain ids.ID
	if args.SourceChain == "" {
		sourceChain = s.vm.snowCtx.ChainID
	} else {
		chainID, err := s.vm.snowCtx.BCLookup.Lookup(args.SourceChain)
		if err != nil {
			return fmt.Errorf("problem parsing source chainID %q: %w", args.SourceChain, err)
		}
		sourceChain = chainID
	}

	addrSet, err := avax.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	startAddr := ids.ShortEmpty
	startUTXO := ids.Empty
	if args.StartIndex.Address != "" || args.StartIndex.UTXO != "" {
		startAddr, err = avax.ParseServiceAddress(s.addrManager, args.StartIndex.Address)
		if err != nil {
			return fmt.Errorf("couldn't parse start index address %q: %w", args.StartIndex.Address, err)
		}
		startUTXO, err = ids.FromString(args.StartIndex.UTXO)
		if err != nil {
			return fmt.Errorf("couldn't parse start index utxo: %w", err)
		}
	}

	var (
		utxos     []*avax.UTXO
		endAddr   ids.ShortID
		endUTXOID ids.ID
	)
	limit := int(args.Limit)
	if limit <= 0 || builder.MaxPageSize < limit {
		limit = builder.MaxPageSize
	}
	if sourceChain == s.vm.snowCtx.ChainID {
		utxos, endAddr, endUTXOID, err = avax.GetPaginatedUTXOs(
			s.vm.State,
			addrSet,
			startAddr,
			startUTXO,
			limit,
		)
	} else {
		utxos, endAddr, endUTXOID, err = s.vm.atomicUtxosManager.GetAtomicUTXOs(
			sourceChain,
			addrSet,
			startAddr,
			startUTXO,
			limit,
		)
	}
	if err != nil {
		return fmt.Errorf("problem retrieving UTXOs: %w", err)
	}

	response.UTXOs = make([]string, len(utxos))
	for i, utxo := range utxos {
		if args.Encoding == formatting.JSON {
			utxo.Out.InitCtx(s.vm.snowCtx)
			bytes, err := json.Marshal(utxo)
			if err != nil {
				return fmt.Errorf("couldn't marshal UTXO %q: %w", utxo.InputID(), err)
			}
			response.UTXOs[i] = string(bytes)
			continue
		}
		bytes, err := txs.Codec.Marshal(txs.Version, utxo)
		if err != nil {
			return fmt.Errorf("couldn't serialize UTXO %q: %w", utxo.InputID(), err)
		}
		response.UTXOs[i], err = formatting.Encode(args.Encoding, bytes)
		if err != nil {
			return fmt.Errorf("couldn't encode UTXO %s as string: %w", utxo.InputID(), err)
		}
	}

	endAddress, err := s.addrManager.FormatLocalAddress(endAddr)
	if err != nil {
		return fmt.Errorf("problem formatting address: %w", err)
	}

	response.EndIndex.Address = endAddress
	response.EndIndex.UTXO = endUTXOID.String()
	response.NumFetched = utilsjson.Uint64(len(utxos))
	response.Encoding = args.Encoding
	return nil
}

type SpendArgs struct {
	api.JSONFromAddrs

	To             platformapi.Owner   `json:"to"`
	Change         platformapi.Owner   `json:"change"`
	LockMode       byte                `json:"lockMode"`
	AmountToLock   utilsjson.Uint64    `json:"amountToLock"`
	AmountToUnlock utilsjson.Uint64    `json:"amountToUnlock"`
	AmountToBurn   utilsjson.Uint64    `json:"amountToBurn"`
	AsOf           utilsjson.Uint64    `json:"asOf"`
	Encoding       formatting.Encoding `json:"encoding"`
	Agent          string              `json:"agent"`
}

type SpendReply struct {
	Ins     string          `json:"ins"`
	Outs    string          `json:"outs"`
	Signers [][]ids.ShortID `json:"signers"`
	Owners  string          `json:"owners"`
}

func (s *Service) Spend(_ *http.Request, args *SpendArgs, response *SpendReply) error {
	s.vm.snowCtx.Log.Debug("Touristicvm: Spend called")

	privKeys, err := s.getFakeKeys(&args.JSONFromAddrs)
	if err != nil {
		return err
	}
	if len(privKeys) == 0 {
		return errNoKeys
	}

	to, err := s.secpOwnerFromAPI(&args.To)
	if err != nil {
		return err
	}

	change, err := s.secpOwnerFromAPI(&args.Change)
	if err != nil {
		return fmt.Errorf(errInvalidChangeAddr, err)
	}

	if args.AmountToUnlock > 0 && args.AmountToLock > 0 {
		return fmt.Errorf("can't both lock and unlock")
	}
	if args.AmountToUnlock > 0 && locked.State(args.LockMode) != locked.StateUnlocked {
		return fmt.Errorf("can't unlock with lock mode %d", args.LockMode)
	}

	// ! @evlekht its just a spend method, it creates ins and outs
	// ! we don't care how this ins and outs will be used
	// ! and shouldn't forbid stuff in that regard here, imo
	// if args.AmountToUnlock > 0 && args.AmountToBurn > 0 {
	// return fmt.Errorf("unlocking funds in T-chain is fee-less")
	// }

	var agent ids.ShortID
	if args.AmountToUnlock > 0 {
		agent, err = ids.ShortFromString(args.Agent)
		if err != nil {
			return fmt.Errorf("couldn't parse agent ID %q: %w", args.Agent, err)
		}
		if agent == ids.ShortEmpty {
			return fmt.Errorf("can't unlock without providing an agent")
		}
	}
	var (
		ins     []*avax.TransferableInput   // inputs
		outs    []*avax.TransferableOutput  // outputs
		signers [][]*secp256k1.PrivateKey   // signers
		owners  []*secp256k1fx.OutputOwners // owners
	)
	signers = [][]*secp256k1.PrivateKey{privKeys}
	owners = []*secp256k1fx.OutputOwners{to}
	if args.AmountToUnlock > 0 {
		ins, outs, err = s.vm.txBuilder.Unlock(
			s.vm.State,
			change,
			to,
			uint64(args.AmountToUnlock),
			agent,
		)
	} else {
		ins, outs, signers, owners, err = s.vm.txBuilder.Lock(
			s.vm.State,
			privKeys,
			uint64(args.AmountToLock),
			uint64(args.AmountToBurn),
			locked.State(args.LockMode),
			to,
			change,
			uint64(args.AsOf),
		)
	}
	if err != nil {
		return fmt.Errorf("%w: %s", errCreateTransferables, err)
	}

	bytes, err := txs.Codec.Marshal(txs.Version, ins)
	if err != nil {
		return fmt.Errorf("%w: %s", errSerializeTransferables, err)
	}

	if response.Ins, err = formatting.Encode(args.Encoding, bytes); err != nil {
		return fmt.Errorf("%w: %s", errEncodeTransferables, err)
	}

	bytes, err = txs.Codec.Marshal(txs.Version, outs)
	if err != nil {
		return fmt.Errorf("%w: %s", errSerializeTransferables, err)
	}

	if response.Outs, err = formatting.Encode(args.Encoding, bytes); err != nil {
		return fmt.Errorf("%w: %s", errEncodeTransferables, err)
	}

	response.Signers = make([][]ids.ShortID, len(signers))
	for i, cred := range signers {
		response.Signers[i] = make([]ids.ShortID, len(cred))
		for j, sig := range cred {
			response.Signers[i][j] = sig.Address()
		}
	}

	bytes, err = txs.Codec.Marshal(txs.Version, owners)
	if err != nil {
		return fmt.Errorf("%w: %s", errSerializeOwners, err)
	}
	if response.Owners, err = formatting.Encode(args.Encoding, bytes); err != nil {
		return fmt.Errorf("%w: %s", errSerializeOwners, err)
	}

	return nil
}

type GetBalanceRequest struct {
	Addresses []string `json:"addresses"`
}

// Note: We explicitly duplicate AVAX out of the maps to ensure backwards
// compatibility.
type GetBalanceResponse struct {
	Balances        map[ids.ID]utilsjson.Uint64 `json:"balances"`
	UnlockedOutputs map[ids.ID]utilsjson.Uint64 `json:"unlockedOutputs"`
	LockedOutputs   map[ids.ID]utilsjson.Uint64 `json:"lockedOutputs"`
	UTXOIDs         []*avax.UTXOID              `json:"utxoIDs"`
}

// GetBalance gets the balance of an address
func (s *Service) GetBalance(_ *http.Request,
	args *GetBalanceRequest, response *GetBalanceResponse) error {
	s.vm.snowCtx.Log.Debug("Touristic: GetBalance called",
		logging.UserStrings("addresses", args.Addresses),
	)

	// Parse to address
	addrs, err := avax.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	utxos, err := avax.GetAllUTXOs(s.vm.State, addrs)
	if err != nil {
		return fmt.Errorf("couldn't get UTXO set of %v: %w", args.Addresses, err)
	}

	unlockedOutputs := map[ids.ID]utilsjson.Uint64{}
	lockedOutputs := map[ids.ID]utilsjson.Uint64{}
	balances := map[ids.ID]utilsjson.Uint64{}
	var utxoIDs []*avax.UTXOID

utxoFor:
	for _, utxo := range utxos {
		assetID := utxo.AssetID()
		switch out := utxo.Out.(type) {
		case *secp256k1fx.TransferOutput:
			unlockedOutputs[assetID] = utilsjson.SafeAdd(unlockedOutputs[assetID], utilsjson.Uint64(out.Amount()))
			balances[assetID] = utilsjson.SafeAdd(balances[assetID], utilsjson.Uint64(out.Amount()))
		case *locked.Out:
			switch out.LockState() {
			case locked.StateLocked:
				lockedOutputs[assetID] = utilsjson.SafeAdd(lockedOutputs[assetID], utilsjson.Uint64(out.Amount()))
				balances[assetID] = utilsjson.SafeAdd(balances[assetID], utilsjson.Uint64(out.Amount()))
			default:
				s.vm.snowCtx.Log.Warn("Unexpected utxo lock state")
				continue utxoFor
			}
		default:
			s.vm.snowCtx.Log.Warn("unexpected output type in UTXO",
				zap.String("type", fmt.Sprintf("%T", out)),
			)
			continue utxoFor
		}

		utxoIDs = append(utxoIDs, &utxo.UTXOID)
	}

	response.Balances = balances
	response.UnlockedOutputs = unlockedOutputs
	response.LockedOutputs = lockedOutputs
	response.UTXOIDs = utxoIDs
	return nil
}
func (s *Service) getFakeKeys(from *api.JSONFromAddrs) ([]*secp256k1.PrivateKey, error) {
	// Parse the from addresses
	fromAddrs, err := avax.ParseServiceAddresses(s.addrManager, from.From)
	if err != nil {
		return nil, err
	}

	keys := []*secp256k1.PrivateKey{}
	for fromAddr := range fromAddrs {
		keys = append(keys, secp256k1.FakePrivateKey(fromAddr))
	}

	if len(from.Signer) > 0 {
		// Parse the signer addresses
		signerAddrs, err := avax.ParseServiceAddresses(s.addrManager, from.Signer)
		if err != nil {
			return nil, err
		}

		keys = append(keys, nil)
		for signerAddr := range signerAddrs {
			keys = append(keys, secp256k1.FakePrivateKey(signerAddr))
		}
	}

	return keys, nil
}

func (s *Service) secpOwnerFromAPI(apiOwner *platformapi.Owner) (*secp256k1fx.OutputOwners, error) {
	if len(apiOwner.Addresses) > 0 {
		secpOwner := &secp256k1fx.OutputOwners{
			Locktime:  uint64(apiOwner.Locktime),
			Threshold: uint32(apiOwner.Threshold),
			Addrs:     make([]ids.ShortID, len(apiOwner.Addresses)),
		}
		for i := range apiOwner.Addresses {
			addr, err := avax.ParseServiceAddress(s.addrManager, apiOwner.Addresses[i])
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

type SpendWithWrapperArgs struct {
	api.JSONFromAddrs
	Agent                string              `json:"agent"`
	To                   platformapi.Owner   `json:"to"`
	Change               platformapi.Owner   `json:"change"`
	LockMode             byte                `json:"lockMode"`
	AmountToTransitState utilsjson.Uint64    `json:"amountToTransitState"`
	AmountToBurn         utilsjson.Uint64    `json:"amountToBurn"`
	AsOf                 utilsjson.Uint64    `json:"asOf"`
	Encoding             formatting.Encoding `json:"encoding"`
}

type SpendWithWrapperReply struct {
	EncodedData string          `json:"encodedData"`
	Signers     [][]ids.ShortID `json:"signers"`
}

type SpendWrapper struct {
	Ins    []*avax.TransferableInput   `serialize:"true"`
	Outs   []*avax.TransferableOutput  `serialize:"true"`
	Owners []*secp256k1fx.OutputOwners `serialize:"true"`
}

func (s *Service) SpendWithWrapper(_ *http.Request, args *SpendWithWrapperArgs, response *SpendWithWrapperReply) error {
	s.vm.snowCtx.Log.Debug("Touristic: SpendWithWrapper called")

	privKeys, err := s.getFakeKeys(&args.JSONFromAddrs)
	if err != nil {
		return err
	}
	if len(privKeys) == 0 {
		return errNoKeys
	}

	to, err := s.secpOwnerFromAPI(&args.To)
	if err != nil {
		return err
	}

	var change *secp256k1fx.OutputOwners
	if args.Change.Threshold != 0 || len(args.Change.Addresses) != 0 {
		change, err = s.secpOwnerFromAPI(&args.Change)
		if err != nil {
			return fmt.Errorf(errInvalidChangeAddr, err)
		}
	}

	var (
		ins     []*avax.TransferableInput
		outs    []*avax.TransferableOutput
		signers [][]*secp256k1.PrivateKey
		owners  []*secp256k1fx.OutputOwners
	)
	switch targetLockMode := locked.State(args.LockMode); targetLockMode {
	case locked.StateLocked:
		ins, outs, signers, owners, err = s.vm.txBuilder.Lock(
			s.vm.State,
			privKeys,
			uint64(args.AmountToTransitState),
			uint64(args.AmountToBurn),
			targetLockMode,
			to,
			change,
			uint64(args.AsOf),
		)
	case locked.StateUnlocked:
		if len(privKeys) != 1 {
			return errInvalidKeysNumber
		}
		var agent ids.ShortID
		agent, err = ids.ShortFromString(args.Agent)
		if err != nil {
			return fmt.Errorf("couldn't parse agent ID %q: %w", args.Agent, err)
		}
		ins, outs, err = s.vm.txBuilder.Unlock(
			s.vm.State,
			// * @evlekht we don't currently intend to support avax multi-assets with api
			&secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{privKeys[0].Address()},
			},
			to,
			uint64(args.AmountToTransitState),
			agent,
		)
		signers = [][]*secp256k1.PrivateKey{privKeys}
		owners = []*secp256k1fx.OutputOwners{to}
	default:
		return errUnknownLockState
	}
	if err != nil {
		return fmt.Errorf("%w: %s", errCreateTransferables, err)
	}

	response.Signers = make([][]ids.ShortID, len(signers))
	for i, cred := range signers {
		response.Signers[i] = make([]ids.ShortID, len(cred))
		for j, sig := range cred {
			response.Signers[i][j] = sig.Address()
		}
	}

	wrapperBytes, err := txs.Codec.Marshal(txs.Version, &SpendWrapper{
		Ins:    ins,
		Outs:   outs,
		Owners: owners,
	})
	if err != nil {
		return err
	}
	if response.EncodedData, err = formatting.Encode(args.Encoding, wrapperBytes); err != nil {
		return fmt.Errorf("%w: %s", errEncodeTransferables, err)
	}
	return nil
}

type IssueChequeRequest struct {
	Issuer            ids.ShortID      `serialize:"true" json:"issuer"`
	Beneficiary       ids.ShortID      `serialize:"true" json:"beneficiary"`
	Amount            utilsjson.Uint64 `serialize:"true" json:"amount"`
	SerialID          utilsjson.Uint64 `serialize:"true" json:"serialID"`
	Agent             ids.ShortID      `serialize:"true" json:"agent"`             // Agent is the node that issued the cheque
	UnnormalizedAgent string           `serialize:"true" json:"unnormalizedAgent"` // unnormalized agent id
}
type IssueChequeResponse struct {
	txs.Cheque
	MsgToSign []byte `serialize:"true" json:"msgToSign"`
}

var errMissingChequeAgent = errors.New("can't issue cheque without providing an agent")

func (s *Service) IssueCheque(_ *http.Request, args *IssueChequeRequest, response *IssueChequeResponse) error {
	s.vm.snowCtx.Log.Debug("Touristic: IssueCheque called")

	cheque := txs.Cheque{
		Issuer:      args.Issuer,
		Beneficiary: args.Beneficiary,
		Amount:      uint64(args.Amount),
		SerialID:    uint64(args.SerialID),
		Agent:       args.Agent,
	}

	if args.Agent == ids.ShortEmpty && args.UnnormalizedAgent == "" { // if agent is not provided (either in normalized or unnormalized form), throw error
		return errMissingChequeAgent
	} else if args.Agent == ids.ShortEmpty { // agent provided in unnormalized form
		// normalize agent id
		agent, err := uuidToShortID(args)
		if err != nil {
			return fmt.Errorf("couldn't parse agent ID %q: %w", args.UnnormalizedAgent, err)
		}
		if agent == ids.ShortEmpty {
			return errMissingChequeAgent
		}
		cheque.Agent = agent
	}

	response.Cheque = cheque
	response.MsgToSign = cheque.BuildMsgToSign()
	return nil
}

func uuidToShortID(args *IssueChequeRequest) (ids.ShortID, error) {
	// remove all dashes
	normalizedAgent := strings.ReplaceAll(args.UnnormalizedAgent, "-", "")

	// Convert the agent UUID to bytes (16 bytes)
	uuidBytes, err := hex.DecodeString(normalizedAgent)
	if err != nil {
		return ids.ShortID{}, fmt.Errorf("error decoding agent input: %w", err)
	}

	// Pad the UUID bytes to make it 20 bytes in length
	paddedAgent := append(uuidBytes, make([]byte, 20-len(uuidBytes))...)
	agent, err := ids.ToShortID(paddedAgent)
	return agent, err
}

type ValidateChequeRequest struct {
	Issuer      ids.ShortID      `serialize:"true" json:"issuer"`
	Beneficiary ids.ShortID      `serialize:"true" json:"beneficiary"`
	Amount      utilsjson.Uint64 `serialize:"true" json:"amount"`
	SerialID    utilsjson.Uint64 `serialize:"true" json:"serialID"`
	Agent       ids.ShortID      `serialize:"true" json:"agent"`
	//Signature   secp256k1fx.Credential `serialize:"true" json:"signature"`
	Signature string `serialize:"true" json:"signature"` // hexNC encoded
}
type ValidateChequeResponse struct {
	Valid bool `json:"valid"`
}

func (s *Service) ValidateCheque(_ *http.Request, args *ValidateChequeRequest, response *ValidateChequeResponse) error {
	s.vm.snowCtx.Log.Debug("Touristic: ValidateCheque called")

	sig, e := formatting.Decode(formatting.HexNC, args.Signature)
	// Create a new array of bytes with the desired size
	var sigArray [65]byte

	// Copy the contents of the slice into the array
	copy(sigArray[:], sig)

	if e != nil {
		return fmt.Errorf("invalid signature")
	}
	cred := &secp256k1fx.Credential{
		Sigs: [][secp256k1.SignatureLen]byte{
			sigArray,
		},
	}
	cheque := txs.SignedCheque{
		Cheque: txs.Cheque{
			Issuer:      args.Issuer,
			Beneficiary: args.Beneficiary,
			Amount:      uint64(args.Amount),
			SerialID:    uint64(args.SerialID),
			Agent:       args.Agent,
		},
		Auth: cred,
	}
	// verify that the cheque is valid
	var (
		signer ids.ShortID
		err    error
	)
	if signer, err = s.vm.fx.RecoverAddressFromSignedMessage(cheque.BuildMsgToSign(), cred); err != nil {
		return fmt.Errorf("invalid signature")
	}
	response.Valid = signer == args.Issuer
	return nil
}
