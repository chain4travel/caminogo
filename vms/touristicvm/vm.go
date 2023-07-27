// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package touristicvm

import (
	"context"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"time"

	"github.com/gorilla/rpc/v2"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/api"
	"github.com/ava-labs/avalanchego/vms/touristicvm/blocks"
	"github.com/ava-labs/avalanchego/vms/touristicvm/config"
	"github.com/ava-labs/avalanchego/vms/touristicvm/fx"
	"github.com/ava-labs/avalanchego/vms/touristicvm/metrics"
	"github.com/ava-labs/avalanchego/vms/touristicvm/state"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs/mempool"
	"github.com/ava-labs/avalanchego/vms/touristicvm/utxo"
	"github.com/prometheus/client_golang/prometheus"

	blockbuilder "github.com/ava-labs/avalanchego/vms/touristicvm/blocks/builder"
	blockexecutor "github.com/ava-labs/avalanchego/vms/touristicvm/blocks/executor"
	txbuilder "github.com/ava-labs/avalanchego/vms/touristicvm/txs/builder"
	txexecutor "github.com/ava-labs/avalanchego/vms/touristicvm/txs/executor"
)

const (
	DataLen = 32
	Name    = "touristicvm"
)

var (
	errBadGenesisBytes = errors.New("genesis data should be bytes (max length 32)")
	Version            = &version.Semantic{
		Major: 0,
		Minor: 1,
		Patch: 0,
	}

	_ block.ChainVM = &VM{}
)

// VM implements the snowman.VM interface
// Each block in this chain contains a Unix timestamp
// and a piece of data (a string)
type VM struct {
	config.Config
	blockbuilder.Builder
	// The context of this vm
	snowCtx   *snow.Context
	dbManager manager.Manager

	clock              mockable.Clock
	metrics            metrics.Metrics
	atomicUtxosManager avax.AtomicUTXOManager

	// State of this VM
	State state.State // TODO nikos refactor to private?

	fx fx.Fx

	// channel to send messages to the consensus engine
	toEngine chan<- common.Message

	// Proposed pieces of data that haven't been put into a block and proposed yet
	mempool [][DataLen]byte

	// Indicates that this VM has finished bootstrapping for the chain
	bootstrapped utils.Atomic[bool]

	txBuilder txbuilder.Builder
	manager   blockexecutor.Manager
}

// Initialize this vm
// [ctx] is this vm's context
// [dbManager] is the manager of this vm's database
// [toEngine] is used to notify the consensus engine that new blocks are
//
//	ready to be added to consensus
//
// The data in the genesis block is [genesisData]
func (vm *VM) Initialize(
	ctx context.Context,
	snowCtx *snow.Context,
	dbManager manager.Manager,
	genesisData []byte,
	_ []byte,
	_ []byte,
	toEngine chan<- common.Message,
	_ []*common.Fx,
	appSender common.AppSender,
) error {
	registerer := prometheus.NewRegistry()
	if err := snowCtx.Metrics.Register(registerer); err != nil {
		return err
	}

	version, err := vm.Version(ctx)
	if err != nil {
		snowCtx.Log.Error("error initializing Timestamp VM: %v", zap.Error(err))
		return err
	}
	snowCtx.Log.Info("Initializing Touristic VM", zap.String("Version", version))

	vm.fx = &secp256k1fx.Fx{}

	// Initialize metrics as soon as possible
	vm.metrics, err = metrics.New("", registerer)
	if err != nil {
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	vm.dbManager = dbManager
	vm.snowCtx = snowCtx
	vm.toEngine = toEngine

	// Create new state
	vm.State, err = state.NewState(vm.dbManager.Current().Database, registerer)
	if err != nil {
		return err
	}

	vm.atomicUtxosManager = avax.NewAtomicUTXOManager(snowCtx.SharedMemory, txs.Codec)

	// Initialize genesis
	if err := vm.initGenesis(genesisData); err != nil {
		return err
	}

	// Note: There is a circular dependency between the mempool and block
	//       builder which is broken by passing in the vm.
	mempool, err := mempool.NewMempool("mempool", registerer, vm)
	if err != nil {
		return fmt.Errorf("failed to create mempool: %w", err)
	}

	utxoHandler := utxo.NewHandler(
		vm.snowCtx,
		&vm.clock,
		vm.fx,
	)
	vm.txBuilder = txbuilder.New(
		vm.snowCtx,
		&vm.Config,
		&vm.clock,
		vm.fx,
		vm.State,
		vm.atomicUtxosManager,
		utxoHandler,
	)

	txExecutorBackend := &txexecutor.Backend{
		Config:       &vm.Config,
		Ctx:          vm.snowCtx,
		Clk:          &vm.clock,
		Bootstrapped: &vm.bootstrapped,
	}

	vm.manager = blockexecutor.NewManager(
		mempool,
		vm.metrics,
		vm.State,
		txExecutorBackend,
	)

	vm.Builder = blockbuilder.New(
		mempool,
		vm.txBuilder,
		txExecutorBackend,
		vm.manager,
		toEngine,
		appSender,
	)

	// Get last accepted
	lastAccepted := vm.State.GetLastAccepted()

	snowCtx.Log.Info("initializing last accepted block",
		zap.Any("id", lastAccepted),
	)

	// Build off the most recently accepted block
	return vm.SetPreference(ctx, lastAccepted)
}

// Initializes Genesis if required
func (vm *VM) initGenesis(genesisData []byte) error {
	stateInitialized, err := vm.State.IsInitialized()
	if err != nil {
		return err
	}

	// if state is already initialized, skip init genesis.
	if stateInitialized {
		return nil
	}

	if len(genesisData) > DataLen {
		return errBadGenesisBytes
	}

	// genesisData is a byte slice but each block contains an byte array
	// Take the first [DataLen] bytes from genesisData and put them in an array
	//genesisDataArr := BytesToData(genesisData) TODO add genesis logic back in later if necessary with appropriate parsing/validating logic
	// TODO fix vm.snowCtx.Log.Debug("genesis",  zap.ByteStrings("data", genesisDataArr)

	startTime := 1690290000 // TODO nikos -refactor
	// Create the genesis block
	// Timestamp of genesis block is 0. It has no parent.

	genesisBlock, err := vm.NewBlock(ids.Empty, 0, time.Unix(int64(startTime), 0), []*txs.Tx{})
	if err != nil {
		vm.snowCtx.Log.Error("error while creating genesis block: %v", zap.Error(err))
		return err
	}
	vm.snowCtx.Log.Debug("genesis block: %v", zap.ByteString("genesis bytes ", genesisBlock.Bytes()))

	//TODO nikos - replace with proper genesis logic
	assetID, err := ids.FromString("gbs1MNJvvs493dvRb6M8E2k3BjJ9FXSYmcc6QWu9PZTeFMatb")
	if err != nil {
		return err
	}

	_, addrBytes, err := address.ParseBech32("kopernikus1g65uqn6t77p656w64023nh8nd9updzmxh8ttv3")
	if err != nil {
		return err
	}
	addrSID, err := ids.ToShortID(addrBytes)
	if err != nil {
		return err
	}
	vm.State.AddUTXO(&avax.UTXO{
		UTXOID: avax.UTXOID{},
		Asset:  avax.Asset{ID: assetID},
		Out: &secp256k1fx.TransferOutput{
			Amt: 1000000000000000,
			OutputOwners: secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{addrSID},
			},
		},
	})

	// TODO check if accept is necessary anymore
	genesisBlkID := genesisBlock.ID()
	vm.State.SetLastAccepted(genesisBlkID)
	vm.State.SetTimestamp(genesisBlock.Timestamp())
	vm.State.AddStatelessBlock(genesisBlock, choices.Accepted)
	vm.snowCtx.Log.Debug("genesis block ID: %s", zap.Stringer("genesisBlkID", genesisBlkID))
	//// Accept the genesis block
	//// Sets [vm.lastAccepted] and [vm.preferred]
	//if err := genesisBlock.Accept(context.TODO()); err != nil {
	//	return fmt.Errorf("error accepting genesis block: %w", err)
	//}

	//pb, err := vm.ParseBlock(context.Background(), genesisBlock.Bytes())
	//if err != nil {
	//	return fmt.Errorf("error parsing genesis block: %w", err)
	//}
	//vm.snowCtx.Log.Debug("parsed genesis block: %v", zap.ByteString("genesis bytes ", pb.Bytes()))
	// Mark this vm's state as initialized, so we can skip initGenesis in further restarts
	if err := vm.State.SetInitialized(); err != nil {
		return fmt.Errorf("error while setting db to initialized: %w", err)
	}

	lastAcceptedID := vm.State.GetLastAccepted()
	vm.snowCtx.Log.Info("initializing last accepted",
		zap.Stringer("blkID", lastAcceptedID),
	)
	// Flush VM's database to underlying db
	return vm.State.Commit()
}

// CreateHandlers returns a map where:
// Keys: The path extension for this VM's API (empty in this case)
// Values: The handler for the API
func (vm *VM) CreateHandlers(context.Context) (map[string]*common.HTTPHandler, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json.NewCodec(), "application/json")
	server.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	if err := server.RegisterService(&Service{
		vm:          vm,
		addrManager: avax.NewAddressManager(vm.snowCtx),
	}, Name); err != nil {
		return nil, err
	}

	return map[string]*common.HTTPHandler{
		"": {
			LockOptions: common.WriteLock,
			Handler:     server,
		},
	}, nil
}

// CreateStaticHandlers returns a map where:
// Keys: The path extension for this VM's static API
// Values: The handler for that static API
func (*VM) CreateStaticHandlers(_ context.Context) (map[string]*common.HTTPHandler, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json.NewCodec(), "application/json")
	server.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	if err := server.RegisterService(&api.StaticService{}, Name); err != nil {
		return nil, err
	}

	return map[string]*common.HTTPHandler{
		"": {
			LockOptions: common.NoLock,
			Handler:     server,
		},
	}, nil
}

// Health implements the common.VM interface
func (*VM) HealthCheck(_ context.Context) (interface{}, error) { return nil, nil }

// GetBlock implements the snowman.ChainVM interface
func (vm *VM) GetBlock(_ context.Context, blkID ids.ID) (snowman.Block, error) {
	return vm.manager.GetBlock(blkID)
}

// LastAccepted returns the block most recently accepted
func (vm *VM) LastAccepted(_ context.Context) (ids.ID, error) { return vm.State.GetLastAccepted(), nil }

// ParseBlock parses [bytes] to a snowman.Block
// This function is used by the vm's state to unmarshal blocks saved in state
// and by the consensus layer when it receives the byte representation of a block
// from another node
func (vm *VM) ParseBlock(ctx context.Context, bytes []byte) (snowman.Block, error) {
	// Note: blocks to be parsed are not verified, so we must use blocks.Codec
	// rather than blocks.GenesisCodec
	statelessBlk, err := blocks.Parse(blocks.Codec, bytes)
	if err != nil {
		return nil, err
	}
	return vm.manager.NewBlock(statelessBlk), nil
}

// NewBlock returns a new Block where:
// - the block's parent is [parentID]
// - the block's data is [data]
// - the block's timestamp is [timestamp]
func (vm *VM) NewBlock(parentID ids.ID, height uint64, timestamp time.Time,
	transactions []*txs.Tx) (*blocks.StandardBlock, error) {
	return blocks.NewStandardBlock(parentID, height, timestamp, transactions, blocks.Codec)
}

// Shutdown this vm
func (vm *VM) Shutdown(_ context.Context) error {
	if vm.dbManager == nil {
		return nil
	}

	if vm.State == nil {
		return nil
	}

	vm.Builder.Shutdown()
	errs := wrappers.Errs{}
	errs.Add(
		vm.State.Close(),
		vm.dbManager.Close(),
	)
	return errs.Err
}

// SetPreference sets the block with ID [ID] as the preferred block
func (vm *VM) SetPreference(_ context.Context, id ids.ID) error {
	vm.Builder.SetPreference(id)
	return nil
}

// SetState sets this VM state according to given snow.State
func (vm *VM) SetState(_ context.Context, state snow.State) error {
	switch state {
	// Engine reports it's bootstrapping
	case snow.Bootstrapping:
		return vm.onBootstrapStarted()
	case snow.NormalOp:
		// Engine reports it can start normal operations
		return vm.onNormalOperationsStarted()
	default:
		return snow.ErrUnknownState
	}
}

// onBootstrapStarted marks this VM as bootstrapping
func (vm *VM) onBootstrapStarted() error {
	vm.bootstrapped.Set(false)
	return nil
}

// onNormalOperationsStarted marks this VM as bootstrapped
func (vm *VM) onNormalOperationsStarted() error {
	// No need to set it again
	if vm.bootstrapped.Get() {
		return nil
	}
	vm.bootstrapped.Set(true)
	return nil
}

// Returns this VM's version
func (*VM) Version(_ context.Context) (string, error) {
	return Version.String(), nil
}

func (*VM) Connected(_ context.Context, _ ids.NodeID, _ *version.Application) error {
	return nil // noop
}

func (*VM) Disconnected(_ context.Context, _ ids.NodeID) error {
	return nil // noop
}
