// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/utils/nodeid"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/locked"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/stretchr/testify/require"
)

func TestCaminoExportLockedInsOrLockedOuts(t *testing.T) {
	nodeKey, nodeID := nodeid.GenerateCaminoNodeKeyAndID()

	type test struct {
		name         string
		caminoConfig genesis.Camino
		nodeID       ids.NodeID
		nodeKey      *crypto.PrivateKeySECP256K1R
		expectedErr  error
	}

	tests := []test{
		{
			name: "Locked out - LockModeBondDeposit: true",
			caminoConfig: genesis.Camino{
				VerifyNodeSignature: true,
				LockModeBondDeposit: true,
			},
			nodeID:      nodeID,
			nodeKey:     nodeKey,
			expectedErr: nil,
		},
		{
			name: "Locked in - LockModeBondDeposit: true",
			caminoConfig: genesis.Camino{
				VerifyNodeSignature: true,
				LockModeBondDeposit: true,
			},
			nodeID:      nodeID,
			nodeKey:     nodeKey,
			expectedErr: nil,
		},
		{
			name: "Locked out - LockModeBondDeposit: false",
			caminoConfig: genesis.Camino{
				VerifyNodeSignature: true,
				LockModeBondDeposit: false,
			},
			nodeID:      nodeID,
			nodeKey:     nodeKey,
			expectedErr: errNodeSignatureMissing,
		},
		{
			name: "Locked in - LockModeBondDeposit: false",
			caminoConfig: genesis.Camino{
				VerifyNodeSignature: true,
				LockModeBondDeposit: false,
			},
			nodeID:      nodeID,
			nodeKey:     nodeKey,
			expectedErr: errNodeSignatureMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newCaminoEnvironment( /*postBanff*/ true, tt.caminoConfig)
			env.ctx.Lock.Lock()
			defer func() {
				err := shutdownEnvironment(env)
				require.NoError(t, err)
			}()
			env.config.BanffTime = env.state.GetTimestamp()

			outputOwners := secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     []ids.ShortID{caminoPreFundedKeys[0].PublicKey().Address()},
			}
			sigIndices := []uint32{0}
			inputSigners := []*crypto.PrivateKeySECP256K1R{caminoPreFundedKeys[0]}
			utxos := []*avax.UTXO{
				generateTestUTXO(ids.ID{1}, env.ctx.AVAXAssetID, defaultMinValidatorStake*2, outputOwners, ids.Empty, ids.Empty),
			}
			outs := []*avax.TransferableOutput{
				generateTestOut(env.ctx.AVAXAssetID, defaultMinValidatorStake-defaultTxFee, outputOwners, ids.Empty, ids.Empty),
				generateTestOut(env.ctx.AVAXAssetID, defaultMinValidatorStake, outputOwners, ids.Empty, locked.ThisTxID),
			}

			ins := make([]*avax.TransferableInput, len(utxos))
			signers := make([][]*crypto.PrivateKeySECP256K1R, len(utxos)+1)
			for i, utxo := range utxos {
				env.state.AddUTXO(utxo)
				ins[i] = generateTestInFromUTXO(utxo, sigIndices)
				signers[i] = inputSigners
			}
			signers[len(signers)-1] = []*crypto.PrivateKeySECP256K1R{tt.nodeKey}

			utx := &txs.ExportTx{
				BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
					NetworkID:    env.ctx.NetworkID,
					BlockchainID: env.ctx.ChainID,
					Ins:          ins,
					Outs:         outs,
				}},
				DestinationChain: env.ctx.XChainID,
				ExportedOutputs: []*avax.TransferableOutput{
					generateTestOut(env.ctx.AVAXAssetID, defaultMinValidatorStake-defaultTxFee, outputOwners, ids.Empty, ids.Empty),
				},
			}
			// tx := &txs.Tx{UnsignedTx: utx}

			tx, err := txs.NewSigned(utx, txs.Codec, signers)
			require.NoError(t, err)

			onAcceptState, err := state.NewDiff(lastAcceptedID, env)
			require.NoError(t, err)

			executor := CaminoStandardTxExecutor{
				StandardTxExecutor{
					Backend: &env.backend,
					State:   onAcceptState,
					Tx:      tx,
				},
			}

			err = executor.ExportTx(utx)
			fmt.Println("err : ", err)
		})
	}
}
