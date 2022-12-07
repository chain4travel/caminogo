// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/stakeable"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

type MultisigOwner struct {
	Alias  ids.ShortID              `serialize:"true" json:"alias"`
	Owners secp256k1fx.OutputOwners `serialize:"true" json:"owners"`
}

func FromGenesisMultisigAlias(gma genesis.MultisigAlias) *MultisigOwner {
	// Important! OutputOwners expects sorted list of addresses
	owners := gma.Addresses
	ids.SortShortIDs(owners)

	return &MultisigOwner{
		Alias: gma.Alias,
		Owners: secp256k1fx.OutputOwners{
			Threshold: gma.Threshold,
			Addrs:     owners,
		},
	}
}

func (cs *caminoState) SetMultisigOwner(ma *MultisigOwner) {
	cs.modifiedMultisigOwners[ma.Alias] = ma
}

func (cs *caminoState) GetMultisigOwner(alias ids.ShortID) (*MultisigOwner, error) {
	if owner, exist := cs.modifiedMultisigOwners[alias]; exist {
		return owner, nil
	}

	multisigAlias := &MultisigOwner{}
	maBytes, err := cs.multisigOwnersDB.Get(alias.Bytes())
	if err != nil {
		return multisigAlias, err
	}

	_, err = blocks.GenesisCodec.Unmarshal(maBytes, multisigAlias)

	return multisigAlias, err
}

func (cs *caminoState) GetMultisigUTXOSigners(utxo *avax.UTXO) (verify.State, error) {
	// It preprocesses the UTXO the same way as the `platformvm.utxo.handler.VerifySpendUTXOs`.
	// Prepared `utxos` will be used only for signature verification
	out := utxo.Out
	if inner, ok := out.(*stakeable.LockOut); ok {
		out = inner.TransferableOut
	}

	trOut, ok := out.(*secp256k1fx.TransferOutput)
	if !ok {
		// Conversion should succeed, otherwise it will be handled by the caller
		return out, nil
	}

	if len(trOut.Addrs) != 1 {
		// There always should be just one, otherwise it is not a multisig
		return out, nil
	}

	owner, err := cs.GetMultisigOwner(trOut.Addrs[0])
	if err != nil {
		if err == database.ErrNotFound {
			return out, nil
		}

		return out, err
	}

	return verify.State(&secp256k1fx.TransferOutput{
		Amt:          trOut.Amount(),
		OutputOwners: owner.Owners,
	}), nil
}

func (cs *caminoState) writeMultisigOwners() error {
	for key, alias := range cs.modifiedMultisigOwners {
		delete(cs.modifiedMultisigOwners, key)
		if alias == nil {
			if err := cs.multisigOwnersDB.Delete(key[:]); err != nil {
				return err
			}
		} else {
			aliasBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, alias)
			if err != nil {
				return fmt.Errorf("failed to serialize multisig alias: %w", err)
			}
			if err := cs.multisigOwnersDB.Put(key[:], aliasBytes); err != nil {
				return err
			}
		}
	}
	return nil
}
