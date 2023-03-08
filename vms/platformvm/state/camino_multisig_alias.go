// Copyright (C) 2022-2023, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/vms/components/multisig"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/vms/types"
)

var errWrongOwnerType = errors.New("wrong owner type")

type msigAliasRaw struct {
	Memo        types.JSONByteSlice  `serialize:"true"`
	Threshold   uint32               `serialize:"true"`
	PubKeyBytes []multisig.PublicKey `serialize:"true"`
}

func (cs *caminoState) SetMultisigAliasRaw(ma *multisig.AliasRaw) {
	cs.modifiedMultisigOwners[ma.ID] = ma
	cs.multisigOwnersCache.Evict(ma.ID)
}

func (cs *caminoState) GetMultisigAlias(id ids.ShortID) (*multisig.Alias, error) {
	if owner, exist := cs.modifiedMultisigOwners[id]; exist {
		if owner == nil {
			return nil, database.ErrNotFound
		}
		alias, err := msigAliasRawToMsigAlias(owner)
		if err != nil {
			return nil, err
		}

		return alias, nil
	}

	if alias, exist := cs.multisigOwnersCache.Get(id); exist {
		if alias == nil {
			return nil, database.ErrNotFound
		}
		return alias.(*multisig.Alias), nil
	}

	maBytes, err := cs.multisigOwnersDB.Get(id[:])
	if err == database.ErrNotFound {
		cs.multisigOwnersCache.Put(id, nil)
		return nil, err
	} else if err != nil {
		return nil, err
	}

	multisigDef := &msigAliasRaw{}
	if _, err = blocks.GenesisCodec.Unmarshal(maBytes, multisigDef); err != nil {
		return nil, err
	}

	multisigAlias, err := msigAliasRawToMsigAlias(&multisig.AliasRaw{
		ID:         id,
		Memo:       multisigDef.Memo,
		Threshold:  multisigDef.Threshold,
		PublicKeys: multisigDef.PubKeyBytes,
	})
	if err != nil {
		return nil, err
	}
	cs.multisigOwnersCache.Put(id, multisigAlias)

	return multisigAlias, nil
}

func (cs *caminoState) writeMultisigOwners() error {
	for key, alias := range cs.modifiedMultisigOwners {
		delete(cs.modifiedMultisigOwners, key)
		if alias == nil {
			if err := cs.multisigOwnersDB.Delete(key[:]); err != nil {
				return err
			}
		} else {
			multisigAlias := &msigAliasRaw{
				Memo:        alias.Memo,
				Threshold:   alias.Threshold,
				PubKeyBytes: alias.PublicKeys,
			}
			aliasBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, multisigAlias)
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

func GetOwner(state Chain, addr ids.ShortID) (*secp256k1fx.OutputOwners, error) {
	msigOwner, err := state.GetMultisigAlias(addr)
	if err != nil && err != database.ErrNotFound {
		return nil, err
	}

	if msigOwner != nil {
		owners, ok := msigOwner.Owners.(*secp256k1fx.OutputOwners)
		if !ok {
			return nil, errWrongOwnerType
		}
		return owners, nil
	}

	return &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{addr},
	}, nil
}

func msigAliasRawToMsigAlias(def *multisig.AliasRaw) (*multisig.Alias, error) {
	addrs := make([]ids.ShortID, len(def.PublicKeys))
	for i, pubKeyBytes := range def.PublicKeys {
		pk, err := crypto.PublicKeyFromBytes(pubKeyBytes.Bytes())
		if err != nil {
			return nil, err
		}
		addrs[i] = pk.Address()
	}
	utils.Sort(addrs)

	return &multisig.Alias{
		ID:   def.ID,
		Memo: def.Memo,
		Owners: &secp256k1fx.OutputOwners{
			Threshold: def.Threshold,
			Addrs:     addrs,
		},
	}, nil
}
