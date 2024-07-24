// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/api"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/keystore"
	as "github.com/ava-labs/avalanchego/vms/platformvm/addrstate"
	"github.com/ava-labs/avalanchego/vms/platformvm/deposit"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/types"

	platformapi "github.com/ava-labs/avalanchego/vms/platformvm/api"
)

var (
	errInvalidChangeAddr      = "couldn't parse changeAddr: %w"
	errCreateTx               = "couldn't create tx: %w"
	errCreateTransferables    = errors.New("can't create transferables")
	errSerializeTransferables = errors.New("can't serialize transferables")
	errEncodeTransferables    = errors.New("can't encode transferables as string")
	ErrWrongOwnerType         = errors.New("wrong owner type")
	errSerializeOwners        = errors.New("can't serialize owners")
)

// CaminoService defines the API calls that can be made to the platform chain
type CaminoService struct {
	Service
}

type GetBalanceResponseV2 struct {
	Balances               map[ids.ID]json.Uint64 `json:"balances"`
	UnlockedOutputs        map[ids.ID]json.Uint64 `json:"unlockedOutputs"`
	BondedOutputs          map[ids.ID]json.Uint64 `json:"bondedOutputs"`
	DepositedOutputs       map[ids.ID]json.Uint64 `json:"depositedOutputs"`
	DepositedBondedOutputs map[ids.ID]json.Uint64 `json:"bondedDepositedOutputs"`
	UTXOIDs                []*avax.UTXOID         `json:"utxoIDs"`
}
type GetBalanceResponseWrapper struct {
	LockModeBondDeposit bool
	avax                GetBalanceResponse
	camino              GetBalanceResponseV2
}

func (response GetBalanceResponseWrapper) MarshalJSON() ([]byte, error) {
	if !response.LockModeBondDeposit {
		return stdjson.Marshal(response.avax)
	}
	return stdjson.Marshal(response.camino)
}

// GetBalance gets the balance of an address
func (s *CaminoService) GetBalance(_ *http.Request, args *GetBalanceRequest, response *GetBalanceResponseWrapper) error {
	s.vm.ctx.Log.Debug("Platform: GetBalance called",
		logging.UserStrings("addresses", args.Addresses),
	)

	s.vm.ctx.Lock.Lock()
	caminoConfig, err := s.vm.state.CaminoConfig()
	s.vm.ctx.Lock.Unlock()
	if err != nil {
		return err
	}

	response.LockModeBondDeposit = caminoConfig.LockModeBondDeposit
	if !caminoConfig.LockModeBondDeposit {
		return s.Service.GetBalance(nil, args, &response.avax)
	}

	// Parse to address
	addrs, err := avax.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	s.vm.ctx.Lock.Lock()
	utxos, err := avax.GetAllUTXOs(s.vm.state, addrs)
	s.vm.ctx.Lock.Unlock()
	if err != nil {
		return fmt.Errorf("couldn't get UTXO set of %v: %w", args.Addresses, err)
	}

	unlockedOutputs := map[ids.ID]json.Uint64{}
	bondedOutputs := map[ids.ID]json.Uint64{}
	depositedOutputs := map[ids.ID]json.Uint64{}
	depositedBondedOutputs := map[ids.ID]json.Uint64{}
	balances := map[ids.ID]json.Uint64{}
	var utxoIDs []*avax.UTXOID

utxoFor:
	for _, utxo := range utxos {
		assetID := utxo.AssetID()
		switch out := utxo.Out.(type) {
		case *secp256k1fx.TransferOutput:
			unlockedOutputs[assetID] = json.SafeAdd(unlockedOutputs[assetID], json.Uint64(out.Amount()))
			balances[assetID] = json.SafeAdd(balances[assetID], json.Uint64(out.Amount()))
		case *locked.Out:
			switch out.LockState() {
			case locked.StateBonded:
				bondedOutputs[assetID] = json.SafeAdd(bondedOutputs[assetID], json.Uint64(out.Amount()))
				balances[assetID] = json.SafeAdd(balances[assetID], json.Uint64(out.Amount()))
			case locked.StateDeposited:
				depositedOutputs[assetID] = json.SafeAdd(depositedOutputs[assetID], json.Uint64(out.Amount()))
				balances[assetID] = json.SafeAdd(balances[assetID], json.Uint64(out.Amount()))
			case locked.StateDepositedBonded:
				depositedBondedOutputs[assetID] = json.SafeAdd(depositedBondedOutputs[assetID], json.Uint64(out.Amount()))
				balances[assetID] = json.SafeAdd(balances[assetID], json.Uint64(out.Amount()))
			default:
				s.vm.ctx.Log.Warn("Unexpected utxo lock state")
				continue utxoFor
			}
		default:
			s.vm.ctx.Log.Warn("unexpected output type in UTXO",
				zap.String("type", fmt.Sprintf("%T", out)),
			)
			continue utxoFor
		}

		utxoIDs = append(utxoIDs, &utxo.UTXOID)
	}

	response.camino = GetBalanceResponseV2{balances, unlockedOutputs, bondedOutputs, depositedOutputs, depositedBondedOutputs, utxoIDs}
	return nil
}

// GetConfigurationReply is the response from calling GetConfiguration.
type GetConfigurationReply struct {
	// The NetworkID
	NetworkID json.Uint32 `json:"networkID"`
	// The fee asset ID
	AssetID ids.ID `json:"assetID"`
	// The symbol of the fee asset ID
	AssetSymbol string `json:"assetSymbol"`
	// beech32HRP use in addresses
	Hrp string `json:"hrp"`
	// Primary network blockchains
	Blockchains []APIBlockchain `json:"blockchains"`
	// The minimum duration a validator has to stake
	MinStakeDuration json.Uint64 `json:"minStakeDuration"`
	// The maximum duration a validator can stake
	MaxStakeDuration json.Uint64 `json:"maxStakeDuration"`
	// The minimum amount of tokens one must bond to be a validator
	MinValidatorStake json.Uint64 `json:"minValidatorStake"`
	// The maximum amount of tokens bondable to a validator
	MaxValidatorStake json.Uint64 `json:"maxValidatorStake"`
	// The minimum delegation fee
	MinDelegationFee json.Uint32 `json:"minDelegationFee"`
	// Minimum stake, in nAVAX, that can be delegated on the primary network
	MinDelegatorStake json.Uint64 `json:"minDelegatorStake"`
	// The minimum consumption rate
	MinConsumptionRate json.Uint64 `json:"minConsumptionRate"`
	// The maximum consumption rate
	MaxConsumptionRate json.Uint64 `json:"maxConsumptionRate"`
	// The supply cap for the native token (AVAX)
	SupplyCap json.Uint64 `json:"supplyCap"`
	// The codec version used for serializing
	CodecVersion json.Uint16 `json:"codecVersion"`
	// Camino VerifyNodeSignature
	VerifyNodeSignature bool `json:"verifyNodeSignature"`
	// Camino LockModeBondDeposit
	LockModeBondDeposit bool `json:"lockModeBondDeposit"`
}

func (s *CaminoService) appendBlockchains(subnetID ids.ID, response *GetBlockchainsResponse) error {
	chains, err := s.vm.state.GetChains(subnetID)
	if err != nil {
		return fmt.Errorf(
			"couldn't retrieve chains for subnet %q: %w",
			subnetID,
			err,
		)
	}

	for _, chainTx := range chains {
		chainID := chainTx.ID()
		chain, ok := chainTx.Unsigned.(*txs.CreateChainTx)
		if !ok {
			return fmt.Errorf("expected tx type *txs.CreateChainTx but got %T", chainTx.Unsigned)
		}
		response.Blockchains = append(response.Blockchains, APIBlockchain{
			ID:       chainID,
			Name:     chain.ChainName,
			SubnetID: subnetID,
			VMID:     chain.VMID,
		})
	}
	return nil
}

// GetConfiguration returns platformVM configuration
func (s *CaminoService) GetConfiguration(_ *http.Request, _ *struct{}, reply *GetConfigurationReply) error {
	s.vm.ctx.Log.Debug("Platform: GetConfiguration called")

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	// Fee Asset ID, NetworkID and HRP
	reply.NetworkID = json.Uint32(s.vm.ctx.NetworkID)
	reply.AssetID = s.vm.GetFeeAssetID()
	reply.AssetSymbol = constants.TokenSymbol(s.vm.ctx.NetworkID)
	reply.Hrp = constants.GetHRP(s.vm.ctx.NetworkID)

	// Blockchains of the primary network
	blockchains := &GetBlockchainsResponse{}
	if err := s.appendBlockchains(constants.PrimaryNetworkID, blockchains); err != nil {
		return err
	}
	reply.Blockchains = blockchains.Blockchains

	// Staking information
	reply.MinStakeDuration = json.Uint64(s.vm.MinStakeDuration)
	reply.MaxStakeDuration = json.Uint64(s.vm.MaxStakeDuration)

	reply.MaxValidatorStake = json.Uint64(s.vm.MaxValidatorStake)
	reply.MinValidatorStake = json.Uint64(s.vm.MinValidatorStake)

	reply.MinDelegationFee = json.Uint32(s.vm.MinDelegationFee)
	reply.MinDelegatorStake = json.Uint64(s.vm.MinDelegatorStake)

	reply.MinConsumptionRate = json.Uint64(s.vm.RewardConfig.MinConsumptionRate)
	reply.MaxConsumptionRate = json.Uint64(s.vm.RewardConfig.MaxConsumptionRate)

	reply.SupplyCap = json.Uint64(s.vm.RewardConfig.SupplyCap)

	// Codec information
	reply.CodecVersion = json.Uint16(txs.CodecVersion)

	caminoConfig, err := s.vm.state.CaminoConfig()
	if err != nil {
		return err
	}
	reply.VerifyNodeSignature = caminoConfig.VerifyNodeSignature
	reply.LockModeBondDeposit = caminoConfig.LockModeBondDeposit

	return nil
}

type SetAddressStateArgs struct {
	api.UserPass
	api.JSONFromAddrs

	Change  platformapi.Owner  `json:"change"`
	Address string             `json:"address"`
	State   as.AddressStateBit `json:"state"`
	Remove  bool               `json:"remove"`
}

// AddAddressState issues an AddAddressStateTx
func (s *CaminoService) SetAddressState(req *http.Request, args *SetAddressStateArgs, response *api.JSONTxID) error {
	s.vm.ctx.Log.Debug("Platform: SetAddressState called")

	tx, err := s.buildAddressStateTx(args)
	if err != nil {
		return err
	}

	response.TxID = tx.ID()

	return s.vm.issueTx(req.Context(), tx)
}

func (s *CaminoService) buildAddressStateTx(args *SetAddressStateArgs) (*txs.Tx, error) {
	targetAddr, err := avax.ParseServiceAddress(s.addrManager, args.Address)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse param Address: %w", err)
	}

	change, err := s.secpOwnerFromAPI(&args.Change)
	if err != nil {
		return nil, fmt.Errorf(errInvalidChangeAddr, err)
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	privKeys, err := s.getKeystoreKeys(&args.UserPass, &args.JSONFromAddrs)
	if err != nil {
		return nil, err
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewAddressStateTx(
		targetAddr,     // Address to change state
		args.Remove,    // Add or remove State
		args.State,     // The state to change
		ids.ShortEmpty, // executor address
		privKeys,       // Keys providing the staked tokens
		change,
	)
	if err != nil {
		return nil, fmt.Errorf(errCreateTx, err)
	}

	return tx, nil
}

// GetAddressStates retrieves the state applied to an address (see setAddressState)
func (s *CaminoService) GetAddressStates(_ *http.Request, args *api.JSONAddress, response *json.Uint64) error {
	s.vm.ctx.Log.Debug("Platform: GetAddressStates called")

	addr, err := avax.ParseServiceAddress(s.addrManager, args.Address)
	if err != nil {
		return err
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	state, err := s.vm.state.GetAddressStates(addr)
	if err != nil {
		return err
	}

	*response = json.Uint64(state)

	return nil
}

type GetMultisigAliasReply struct {
	Memo types.JSONByteSlice `json:"memo"`
	platformapi.Owner
}

// GetMultisigAlias retrieves the owners and threshold for a given multisig alias
func (s *CaminoService) GetMultisigAlias(_ *http.Request, args *api.JSONAddress, response *GetMultisigAliasReply) error {
	s.vm.ctx.Log.Debug("Platform: GetMultisigAlias called")

	addr, err := avax.ParseServiceAddress(s.addrManager, args.Address)
	if err != nil {
		return err
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	alias, err := s.vm.state.GetMultisigAlias(addr)
	if err != nil {
		return err
	}
	owners, ok := alias.Owners.(*secp256k1fx.OutputOwners)
	if !ok {
		return ErrWrongOwnerType
	}

	response.Memo = alias.Memo
	response.Locktime = json.Uint64(owners.Locktime)
	response.Threshold = json.Uint32(owners.Threshold)
	response.Addresses = make([]string, len(owners.Addrs))

	for index, addr := range owners.Addrs {
		addrString, err := s.addrManager.FormatLocalAddress(addr)
		if err != nil {
			return err
		}
		response.Addresses[index] = addrString
	}

	return nil
}

type SpendArgs struct {
	api.JSONFromAddrs

	To           platformapi.Owner   `json:"to"`
	Change       platformapi.Owner   `json:"change"`
	LockMode     byte                `json:"lockMode"`
	AmountToLock json.Uint64         `json:"amountToLock"`
	AmountToBurn json.Uint64         `json:"amountToBurn"`
	AsOf         json.Uint64         `json:"asOf"`
	Encoding     formatting.Encoding `json:"encoding"`
}

type SpendReply struct {
	Ins     string          `json:"ins"`
	Outs    string          `json:"outs"`
	Signers [][]ids.ShortID `json:"signers"`
	Owners  string          `json:"owners"`
}

func (s *CaminoService) Spend(_ *http.Request, args *SpendArgs, response *SpendReply) error {
	s.vm.ctx.Log.Debug("Platform: Spend called")

	to, err := s.secpOwnerFromAPI(&args.To)
	if err != nil {
		return err
	}

	change, err := s.secpOwnerFromAPI(&args.Change)
	if err != nil {
		return fmt.Errorf(errInvalidChangeAddr, err)
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	privKeys, err := s.getFakeKeys(&args.JSONFromAddrs)
	if err != nil {
		return err
	}
	if len(privKeys) == 0 {
		return errNoKeys
	}

	ins, outs, signers, owners, err := s.vm.txBuilder.Lock(
		s.vm.state,
		privKeys,
		uint64(args.AmountToLock),
		uint64(args.AmountToBurn),
		locked.State(args.LockMode),
		to,
		change,
		uint64(args.AsOf),
	)
	if err != nil {
		return fmt.Errorf("%w: %w", errCreateTransferables, err)
	}

	bytes, err := txs.Codec.Marshal(txs.CodecVersion, ins)
	if err != nil {
		return fmt.Errorf("%w: %w", errSerializeTransferables, err)
	}

	if response.Ins, err = formatting.Encode(args.Encoding, bytes); err != nil {
		return fmt.Errorf("%w: %w", errEncodeTransferables, err)
	}

	bytes, err = txs.Codec.Marshal(txs.CodecVersion, outs)
	if err != nil {
		return fmt.Errorf("%w: %w", errSerializeTransferables, err)
	}

	if response.Outs, err = formatting.Encode(args.Encoding, bytes); err != nil {
		return fmt.Errorf("%w: %w", errEncodeTransferables, err)
	}

	response.Signers = make([][]ids.ShortID, len(signers))
	for i, cred := range signers {
		response.Signers[i] = make([]ids.ShortID, len(cred))
		for j, sig := range cred {
			response.Signers[i][j] = sig.Address()
		}
	}

	bytes, err = txs.Codec.Marshal(txs.CodecVersion, owners)
	if err != nil {
		return fmt.Errorf("%w: %w", errSerializeOwners, err)
	}
	if response.Owners, err = formatting.Encode(args.Encoding, bytes); err != nil {
		return fmt.Errorf("%w: %w", errSerializeOwners, err)
	}
	return nil
}

type RegisterNodeArgs struct {
	api.UserPass
	api.JSONFromAddrs

	Change           platformapi.Owner `json:"change"`
	OldNodeID        ids.NodeID        `json:"oldNodeID"`
	NewNodeID        ids.NodeID        `json:"newNodeID"`
	NodeOwnerAddress string            `json:"nodeOwnerAddress"`
}

// RegisterNode issues an RegisterNodeTx
func (s *CaminoService) RegisterNode(req *http.Request, args *RegisterNodeArgs, reply *api.JSONTxID) error {
	s.vm.ctx.Log.Debug("Platform: RegisterNode called")

	tx, err := s.buildRegisterNodeTx(args)
	if err != nil {
		return err
	}

	reply.TxID = tx.ID()

	return s.vm.issueTx(req.Context(), tx)
}

func (s *CaminoService) buildRegisterNodeTx(args *RegisterNodeArgs) (*txs.Tx, error) {
	// Parse the node owner address.
	nodeOwnerAddress, err := avax.ParseServiceAddress(s.addrManager, args.NodeOwnerAddress)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse nodeOwnerAddress: %w", err)
	}

	change, err := s.secpOwnerFromAPI(&args.Change)
	if err != nil {
		return nil, fmt.Errorf(errInvalidChangeAddr, err)
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	privKeys, err := s.getKeystoreKeys(&args.UserPass, &args.JSONFromAddrs)
	if err != nil {
		return nil, err
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewRegisterNodeTx(
		args.OldNodeID,
		args.NewNodeID,
		nodeOwnerAddress,
		privKeys,
		change,
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't create tx: %w", err)
	}

	return tx, nil
}

type ClaimedAmount struct {
	DepositTxID    ids.ID            `json:"depositTxID"`
	ClaimableOwner platformapi.Owner `json:"claimableOwner"`
	Amount         json.Uint64       `json:"amount"`
	ClaimType      txs.ClaimType     `json:"claimType"`
}

type ClaimArgs struct {
	api.UserPass
	api.JSONFromAddrs

	Claimables []ClaimedAmount   `json:"claimables"`
	ClaimTo    platformapi.Owner `json:"claimTo"`
	Change     platformapi.Owner `json:"change"`
}

// Claim issues an ClaimTx
func (s *CaminoService) Claim(req *http.Request, args *ClaimArgs, reply *api.JSONTxID) error {
	s.vm.ctx.Log.Debug("Platform: Claim called")

	tx, err := s.buildClaimTx(args)
	if err != nil {
		return err
	}

	reply.TxID = tx.ID()

	return s.vm.issueTx(req.Context(), tx)
}

func (s *CaminoService) buildClaimTx(args *ClaimArgs) (*txs.Tx, error) {
	change, err := s.secpOwnerFromAPI(&args.Change)
	if err != nil {
		return nil, fmt.Errorf(errInvalidChangeAddr, err)
	}

	claimTo, err := s.secpOwnerFromAPI(&args.ClaimTo)
	if err != nil {
		return nil, err
	}

	claimables := make([]txs.ClaimAmount, len(args.Claimables))
	for i := range args.Claimables {
		switch args.Claimables[i].ClaimType {
		case txs.ClaimTypeActiveDepositReward:
			claimables[i].ID = args.Claimables[i].DepositTxID
		case txs.ClaimTypeExpiredDepositReward, txs.ClaimTypeValidatorReward, txs.ClaimTypeAllTreasury:
			claimableOwner, err := s.secpOwnerFromAPI(&args.Claimables[i].ClaimableOwner)
			if err != nil {
				return nil, fmt.Errorf("failed to parse api owner to secp owner: %w", err)
			}
			ownerID, err := txs.GetOwnerID(claimableOwner)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate ownerID from owner: %w", err)
			}
			claimables[i].ID = ownerID
		default:
			return nil, txs.ErrWrongClaimType
		}
		claimables[i].Amount = uint64(args.Claimables[i].Amount)
		claimables[i].Type = args.Claimables[i].ClaimType
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	privKeys, err := s.getKeystoreKeys(&args.UserPass, &args.JSONFromAddrs)
	if err != nil {
		return nil, err
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewClaimTx(
		claimables,
		claimTo,
		privKeys,
		change,
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't create tx: %w", err)
	}

	return tx, nil
}

type TransferArgs struct {
	api.UserPass
	api.JSONFromAddrs
	Change     string            `json:"change"`
	TransferTo platformapi.Owner `json:"transferTo"`
	Amount     json.Uint64       `json:"amount"`
}

// Transfer issues an BaseTx
func (s *CaminoService) Transfer(req *http.Request, args *TransferArgs, reply *api.JSONTxID) error {
	s.vm.ctx.Log.Debug("Platform: Transfer called")

	tx, err := s.buildBaseTx(args)
	if err != nil {
		return err
	}

	reply.TxID = tx.ID()

	return s.vm.issueTx(req.Context(), tx)
}

func (s *CaminoService) buildBaseTx(args *TransferArgs) (*txs.Tx, error) {
	_, change, err := s.addrManager.ParseAddress(args.Change)
	if err != nil {
		return nil, fmt.Errorf(errInvalidChangeAddr, err)
	}

	transferTo, err := s.secpOwnerFromAPI(&args.TransferTo)
	if err != nil {
		return nil, err
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	privKeys, err := s.getKeystoreKeys(&args.UserPass, &args.JSONFromAddrs)
	if err != nil {
		return nil, err
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewBaseTx(
		uint64(args.Amount),
		*transferTo,
		privKeys,
		change,
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't create tx: %w", err)
	}

	return tx, nil
}

func (s *CaminoService) GetRegisteredShortIDLink(_ *http.Request, args *api.JSONAddress, response *api.JSONAddress) error {
	s.vm.ctx.Log.Debug("Platform: GetRegisteredShortIDLink called")

	var id ids.ShortID
	isNodeID := false
	if nodeID, err := ids.NodeIDFromString(args.Address); err == nil {
		id = ids.ShortID(nodeID)
		isNodeID = true
	} else {
		id, err = avax.ParseServiceAddress(s.addrManager, args.Address)
		if err != nil {
			return err
		}
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	link, err := s.vm.state.GetShortIDLink(id, state.ShortLinkKeyRegisterNode)
	if err != nil {
		return err
	}

	if isNodeID {
		response.Address, err = s.addrManager.FormatLocalAddress(link)
		if err != nil {
			return err
		}
	} else {
		response.Address = ids.NodeID(link).String()
	}
	return nil
}

type APIClaimable struct {
	RewardOwner           platformapi.Owner `json:"rewardOwner"`
	ValidatorRewards      json.Uint64       `json:"validatorRewards"`
	ExpiredDepositRewards json.Uint64       `json:"expiredDepositRewards"`
}

type GetClaimablesArgs struct {
	Owners []platformapi.Owner `json:"owners"`
}

type GetClaimablesReply struct {
	Claimables []APIClaimable `json:"claimables"`
}

// GetClaimables returns the amount of claimable tokens for given owner
func (s *CaminoService) GetClaimables(_ *http.Request, args *GetClaimablesArgs, response *GetClaimablesReply) error {
	s.vm.ctx.Log.Debug("Platform: GetClaimables called")

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	response.Claimables = make([]APIClaimable, 0, len(args.Owners))
	for i := range args.Owners {
		claimableOwner, err := s.secpOwnerFromAPI(&args.Owners[i])
		if err != nil {
			return err
		}

		ownerID, err := txs.GetOwnerID(claimableOwner)
		if err != nil {
			return err
		}

		claimable, err := s.vm.state.GetClaimable(ownerID)
		if err == database.ErrNotFound {
			continue
		} else if err != nil {
			return err
		}

		response.Claimables = append(response.Claimables, APIClaimable{
			RewardOwner:           args.Owners[i],
			ValidatorRewards:      json.Uint64(claimable.ValidatorReward),
			ExpiredDepositRewards: json.Uint64(claimable.ExpiredDepositReward),
		})
	}

	return nil
}

type APIDeposit struct {
	DepositTxID         ids.ID            `json:"depositTxID"`
	DepositOfferID      ids.ID            `json:"depositOfferID"`
	UnlockedAmount      json.Uint64       `json:"unlockedAmount"`
	UnlockableAmount    json.Uint64       `json:"unlockableAmount"`
	ClaimedRewardAmount json.Uint64       `json:"claimedRewardAmount"`
	Start               json.Uint64       `json:"start"`
	Duration            uint32            `json:"duration"`
	Amount              json.Uint64       `json:"amount"`
	RewardOwner         platformapi.Owner `json:"rewardOwner"`
}

func (s *CaminoService) apiDepositFromDeposit(depositTxID ids.ID, deposit *deposit.Deposit) (*APIDeposit, error) {
	rewardOwner, ok := deposit.RewardOwner.(*secp256k1fx.OutputOwners)
	if !ok {
		return nil, ErrWrongOwnerType
	}
	apiOwner, err := s.apiOwnerFromSECP(rewardOwner)
	if err != nil {
		return nil, err
	}
	return &APIDeposit{
		DepositTxID:         depositTxID,
		DepositOfferID:      deposit.DepositOfferID,
		UnlockedAmount:      json.Uint64(deposit.UnlockedAmount),
		ClaimedRewardAmount: json.Uint64(deposit.ClaimedRewardAmount),
		Start:               json.Uint64(deposit.Start),
		Duration:            deposit.Duration,
		Amount:              json.Uint64(deposit.Amount),
		RewardOwner:         *apiOwner,
	}, nil
}

type GetDepositsArgs struct {
	DepositTxIDs []ids.ID `json:"depositTxIDs"`
}

type GetDepositsReply struct {
	Deposits         []*APIDeposit `json:"deposits"`
	AvailableRewards []json.Uint64 `json:"availableRewards"`
	Timestamp        json.Uint64   `json:"timestamp"`
}

// GetDeposits returns deposits by IDs
func (s *CaminoService) GetDeposits(_ *http.Request, args *GetDepositsArgs, reply *GetDepositsReply) error {
	s.vm.ctx.Log.Debug("Platform: GetDeposits called")
	timestamp := s.vm.clock.Unix()
	reply.Deposits = make([]*APIDeposit, len(args.DepositTxIDs))
	reply.AvailableRewards = make([]json.Uint64, len(args.DepositTxIDs))
	reply.Timestamp = json.Uint64(timestamp)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	for i := range args.DepositTxIDs {
		deposit, err := s.vm.state.GetDeposit(args.DepositTxIDs[i])
		if err != nil {
			return fmt.Errorf("could't get deposit from state: %w", err)
		}
		offer, err := s.vm.state.GetDepositOffer(deposit.DepositOfferID)
		if err != nil {
			return err
		}
		reply.Deposits[i], err = s.apiDepositFromDeposit(args.DepositTxIDs[i], deposit)
		if err != nil {
			return err
		}
		reply.Deposits[i].UnlockableAmount = json.Uint64(deposit.UnlockableAmount(offer, timestamp))
		reply.AvailableRewards[i] = json.Uint64(deposit.ClaimableReward(offer, timestamp))
	}
	return nil
}

// GetLastAcceptedBlock returns the last accepted block
func (s *CaminoService) GetLastAcceptedBlock(r *http.Request, args *api.Encoding, reply *api.GetBlockResponse) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getLastAcceptedBlock"),
		zap.Stringer("encoding", args.Encoding),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	ctx := r.Context()
	blockID, err := s.vm.LastAccepted(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get last accepted block ID: %w", err)
	}
	block, err := s.vm.manager.GetStatelessBlock(blockID)
	if err != nil {
		s.vm.ctx.Log.Error("couldn't get accepted block",
			zap.Stringer("blkID", blockID),
			zap.Error(err),
		)
		return fmt.Errorf("couldn't get block with id %s: %w", blockID, err)
	}

	reply.Encoding = args.Encoding

	var result any
	if args.Encoding == formatting.JSON {
		block.InitCtx(s.vm.ctx)
		result = block
	} else {
		result, err = formatting.Encode(args.Encoding, block.Bytes())
		if err != nil {
			return fmt.Errorf("couldn't encode block %s as %s: %w", blockID, args.Encoding, err)
		}
	}

	reply.Block, err = stdjson.Marshal(result)
	return err
}

func (s *Service) getKeystoreKeys(creds *api.UserPass, from *api.JSONFromAddrs) ([]*secp256k1.PrivateKey, error) {
	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, creds.Username, creds.Password)
	if err != nil {
		return nil, err
	}
	defer user.Close()

	var keys []*secp256k1.PrivateKey
	if len(from.Signer) > 0 {
		if len(from.From) == 0 {
			return nil, errNoKeys
		}

		// Get fake keys for from addresses
		keys, err = s.getFakeKeys(&api.JSONFromAddrs{From: from.From})
		if err != nil {
			return nil, err
		}

		// Parse the signer addresses
		signerAddrs, err := avax.ParseServiceAddresses(s.addrManager, from.Signer)
		if err != nil {
			return nil, err
		}

		// Get keys for multiSig
		keyChain, err := keystore.GetKeychain(user, signerAddrs)
		if err != nil {
			return nil, fmt.Errorf("couldn't get addresses controlled by the user: %w", err)
		}
		if len(keyChain.Keys) == 0 {
			return nil, errNoKeys
		}

		keys = append(keys, nil)
		keys = append(keys, keyChain.Keys...)
	} else {
		// Parse the from addresses
		fromAddrs, err := avax.ParseServiceAddresses(s.addrManager, from.From)
		if err != nil {
			return nil, err
		}

		// Get the user's keys
		keyChain, err := keystore.GetKeychain(user, fromAddrs)
		if err != nil {
			return nil, fmt.Errorf("couldn't get addresses controlled by the user: %w", err)
		}
		if len(keyChain.Keys) == 0 {
			return nil, errNoKeys
		}

		keys = keyChain.Keys
	}

	if err = user.Close(); err != nil {
		return nil, err
	}
	return keys, nil
}

// getFakeKeys creates SECP256K1 private keys which have only the purpose to provide the address.
// Used for calls like spend() which provide fromAddrs and signers required to get the correct UTXOs
// for the transaction. These private keys can never recover to the public address they contain.
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

func (s *CaminoService) secpOwnerFromAPI(apiOwner *platformapi.Owner) (*secp256k1fx.OutputOwners, error) {
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

func (s *CaminoService) apiOwnerFromSECP(secpOwner *secp256k1fx.OutputOwners) (*platformapi.Owner, error) {
	apiOwner := &platformapi.Owner{
		Locktime:  json.Uint64(secpOwner.Locktime),
		Threshold: json.Uint32(secpOwner.Threshold),
		Addresses: make([]string, len(secpOwner.Addrs)),
	}
	for i := range secpOwner.Addrs {
		addr, err := s.addrManager.FormatLocalAddress(secpOwner.Addrs[i])
		if err != nil {
			return nil, err
		}
		apiOwner.Addresses[i] = addr
	}
	return apiOwner, nil
}

type APIDepositOffer struct {
	UpgradeVersion          uint16              `json:"upgradeVersion"`          // codec upgrade version
	ID                      ids.ID              `json:"id"`                      // offer id
	InterestRateNominator   json.Uint64         `json:"interestRateNominator"`   // deposit.Amount * (interestRateNominator / interestRateDenominator) == reward for deposit with 1 year duration
	Start                   json.Uint64         `json:"start"`                   // Unix time in seconds, when this offer becomes active (can be used to create new deposits)
	End                     json.Uint64         `json:"end"`                     // Unix time in seconds, when this offer becomes inactive (can't be used to create new deposits)
	MinAmount               json.Uint64         `json:"minAmount"`               // Minimum amount that can be deposited with this offer
	TotalMaxAmount          json.Uint64         `json:"totalMaxAmount"`          // Maximum amount that can be deposited with this offer in total (across all deposits)
	DepositedAmount         json.Uint64         `json:"depositedAmount"`         // Amount that was already deposited with this offer
	MinDuration             uint32              `json:"minDuration"`             // Minimum duration of deposit created with this offer
	MaxDuration             uint32              `json:"maxDuration"`             // Maximum duration of deposit created with this offer
	UnlockPeriodDuration    uint32              `json:"unlockPeriodDuration"`    // Duration of period during which tokens deposited with this offer will be unlocked. The unlock period starts at the end of deposit minus unlockPeriodDuration
	NoRewardsPeriodDuration uint32              `json:"noRewardsPeriodDuration"` // Duration of period during which rewards won't be accumulated. No rewards period starts at the end of deposit minus unlockPeriodDuration
	Memo                    types.JSONByteSlice `json:"memo"`                    // Arbitrary offer memo
	Flags                   json.Uint64         `json:"flags"`                   // Bitfield with flags
	TotalMaxRewardAmount    json.Uint64         `json:"totalMaxRewardAmount"`    // Maximum amount that can be rewarded for all deposits created with this offer in total
	RewardedAmount          json.Uint64         `json:"rewardedAmount"`          // Amount that was already rewarded (including potential rewards) for deposits created with this offer
	OwnerAddress            ids.ShortID         `json:"ownerAddress"`            // Address that can sign deposit-creator permission
}

type GetAllDepositOffersArgs struct {
	Timestamp json.Uint64 `json:"timestamp"`
}

type GetAllDepositOffersReply struct {
	DepositOffers []*APIDepositOffer `json:"depositOffers"`
}

// GetAllDepositOffers returns an array of all deposit offers that are active at given timestamp
func (s *CaminoService) GetAllDepositOffers(_ *http.Request, args *GetAllDepositOffersArgs, response *GetAllDepositOffersReply) error {
	s.vm.ctx.Log.Debug("Platform: GetAllDepositOffers called")

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	allDepositOffers, err := s.vm.state.GetAllDepositOffers()
	if err != nil {
		return err
	}

	timestamp := uint64(args.Timestamp)
	for _, offer := range allDepositOffers {
		if offer.Start <= timestamp && offer.End >= timestamp {
			response.DepositOffers = append(response.DepositOffers, apiOfferFromOffer(offer))
		}
	}

	return nil
}

func apiOfferFromOffer(offer *deposit.Offer) *APIDepositOffer {
	return &APIDepositOffer{
		UpgradeVersion:          offer.UpgradeVersionID.Version(),
		ID:                      offer.ID,
		InterestRateNominator:   json.Uint64(offer.InterestRateNominator),
		Start:                   json.Uint64(offer.Start),
		End:                     json.Uint64(offer.End),
		MinAmount:               json.Uint64(offer.MinAmount),
		TotalMaxAmount:          json.Uint64(offer.TotalMaxAmount),
		DepositedAmount:         json.Uint64(offer.DepositedAmount),
		MinDuration:             offer.MinDuration,
		MaxDuration:             offer.MaxDuration,
		UnlockPeriodDuration:    offer.UnlockPeriodDuration,
		NoRewardsPeriodDuration: offer.NoRewardsPeriodDuration,
		Memo:                    offer.Memo,
		Flags:                   json.Uint64(offer.Flags),
		TotalMaxRewardAmount:    json.Uint64(offer.TotalMaxRewardAmount),
		RewardedAmount:          json.Uint64(offer.RewardedAmount),
		OwnerAddress:            offer.OwnerAddress,
	}
}

type GetUpgradePhasesReply struct {
	AthensPhase json.Uint32 `json:"athensPhase"`
	BerlinPhase json.Uint32 `json:"berlinPhase"`
}

func (s *CaminoService) GetUpgradePhases(_ *http.Request, _ *struct{}, response *GetUpgradePhasesReply) error {
	s.vm.ctx.Log.Debug("Platform: GetUpgradePhases called")

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	if s.vm.Config.IsAthensPhaseActivated(s.vm.state.GetTimestamp()) {
		response.AthensPhase = 1
	}
	if s.vm.Config.IsBerlinPhaseActivated(s.vm.state.GetTimestamp()) {
		response.BerlinPhase = 1
	}
	return nil
}

type ConsortiumMemberValidator struct {
	ValidatorWeight         json.Uint64 `json:"validatorWeight"`
	ConsortiumMemberAddress string      `json:"consortiumMemberAddress"`
}

type GetValidatorsAtReply2 struct {
	Validators map[ids.NodeID]ConsortiumMemberValidator `json:"validators"`
}

type GetValidatorsAtResponseWrapper struct {
	LockModeBondDeposit bool
	avax                GetValidatorsAtReply
	camino              GetValidatorsAtReply2
}

func (response GetValidatorsAtResponseWrapper) MarshalJSON() ([]byte, error) {
	if !response.LockModeBondDeposit {
		return response.avax.MarshalJSON()
	}
	return stdjson.Marshal(response.camino)
}

// Overrides avax service GetValidatorsAt
func (s *CaminoService) GetValidatorsAt(r *http.Request, args *GetValidatorsAtArgs, reply *GetValidatorsAtResponseWrapper) error {
	height := uint64(args.Height)
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getValidatorsAt-camino"),
		zap.Uint64("height", height),
		zap.Stringer("subnetID", args.SubnetID),
	)

	s.vm.ctx.Lock.Lock()
	caminoConfig, err := s.vm.state.CaminoConfig()
	s.vm.ctx.Lock.Unlock()
	if err != nil {
		return err
	}
	reply.LockModeBondDeposit = caminoConfig.LockModeBondDeposit
	if !caminoConfig.LockModeBondDeposit {
		return s.Service.GetValidatorsAt(r, args, &reply.avax)
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()
	vdrs, err := s.vm.GetValidatorSet(r.Context(), height, args.SubnetID)
	if err != nil {
		return fmt.Errorf("failed to get validator set: %w", err)
	}
	reply.camino.Validators = make(map[ids.NodeID]ConsortiumMemberValidator, len(vdrs))
	for _, vdr := range vdrs {
		cMemberAddr, err := s.vm.state.GetShortIDLink(ids.ShortID(vdr.NodeID), state.ShortLinkKeyRegisterNode)
		if err != nil {
			return fmt.Errorf("failed to get consortium member address: %w", err)
		}

		addrStr, err := address.Format("P", constants.GetHRP(s.vm.ctx.NetworkID), cMemberAddr[:])
		if err != nil {
			return fmt.Errorf("failed to format consortium member address: %w", err)
		}

		reply.camino.Validators[vdr.NodeID] = ConsortiumMemberValidator{
			ValidatorWeight:         json.Uint64(vdr.Weight),
			ConsortiumMemberAddress: addrStr,
		}
	}
	return nil
}

// GetCurrentSupply returns an upper bound on the supply of CAM in the system including potential rewards from not expired or not-locked deposit offers.
// Overrides avax service GetCurrentSupply.
func (s *CaminoService) GetCurrentSupply(_ *http.Request, args *GetCurrentSupplyArgs, reply *GetCurrentSupplyReply) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getCurrentSupply-camino"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	supply, err := s.vm.state.GetCurrentSupply(args.SubnetID)
	if err != nil {
		return err
	}

	allOffers, err := s.vm.state.GetAllDepositOffers()
	if err != nil {
		return err
	}

	chainTimestamp := s.vm.clock.Unix()

	for _, offer := range allOffers {
		if chainTimestamp <= offer.End && offer.Flags&deposit.OfferFlagLocked == 0 {
			if offer.TotalMaxAmount != 0 {
				supply, err = math.Add64(supply, offer.MaxRemainingRewardByTotalMaxAmount())
				if err != nil {
					return err
				}
			} else if offer.TotalMaxRewardAmount != 0 {
				supply, err = math.Add64(supply, offer.RemainingReward())
				if err != nil {
					return err
				}
			}
		}
	}
	reply.Supply = json.Uint64(supply)
	return err
}
