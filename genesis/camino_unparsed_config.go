// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
)

var errCannotParseInitialAdmin = "cannot parse initialAdmin from genesis: %w"

type UnparsedCamino struct {
	VerifyNodeSignature      bool                    `json:"verifyNodeSignature"`
	LockModeBondDeposit      bool                    `json:"lockModeBondDeposit"`
	InitialAdmin             string                  `json:"initialAdmin"`
	DepositOffers            []genesis.DepositOffer  `json:"depositOffers"`
	InitialMultisigAddresses []UnparsedMultisigAlias `json:"initialMultisigAddresses"`
}

func (us UnparsedCamino) Parse(networkID uint32) (genesis.Camino, error) {
	c := genesis.Camino{
		VerifyNodeSignature:      us.VerifyNodeSignature,
		LockModeBondDeposit:      us.LockModeBondDeposit,
		DepositOffers:            us.DepositOffers,
		InitialMultisigAddresses: make([]genesis.MultisigAlias, len(us.InitialMultisigAddresses)),
	}

	_, _, avaxAddrBytes, err := address.Parse(us.InitialAdmin)
	if err != nil {
		return c, fmt.Errorf(errCannotParseInitialAdmin, err)
	}
	avaxAddr, err := ids.ToShortID(avaxAddrBytes)
	if err != nil {
		return c, fmt.Errorf(errCannotParseInitialAdmin, err)
	}
	c.InitialAdmin = avaxAddr

	for i, uma := range us.InitialMultisigAddresses {
		c.InitialMultisigAddresses[i], err = uma.Parse(networkID)
		if err != nil {
			return c, err
		}
	}

	return c, nil
}

func (us *UnparsedCamino) Unparse(p genesis.Camino, networkID uint32) error {
	us.VerifyNodeSignature = p.VerifyNodeSignature
	us.LockModeBondDeposit = p.LockModeBondDeposit
	us.DepositOffers = p.DepositOffers
	us.InitialMultisigAddresses = make([]UnparsedMultisigAlias, len(p.InitialMultisigAddresses))

	avaxAddr, err := address.Format(
		"X",
		constants.GetHRP(networkID),
		p.InitialAdmin.Bytes(),
	)
	if err != nil {
		return err
	}
	us.InitialAdmin = avaxAddr

	for i, ma := range p.InitialMultisigAddresses {
		err = us.InitialMultisigAddresses[i].Unparse(ma, networkID)
		if err != nil {
			return err
		}
	}

	return nil
}

// UnparsedMultisigAlias defines a multisignature alias address.
// [Alias] is the alias of the multisignature address. It's encoded to string
// the same way as ShortID String() method does.
// [Addresses] are the addresses that are allowed to sign transactions from the multisignature address.
// All addresses are encoded to string the same way as ShortID String() method does.
// [Threshold] is the number of signatures required to sign transactions from the multisignature address.
type UnparsedMultisigAlias struct {
	Alias     string   `json:"alias"`
	Addresses []string `json:"addresses"`
	Threshold uint32   `json:"threshold"`
}

func (uma UnparsedMultisigAlias) Parse(networkID uint32) (genesis.MultisigAlias, error) {
	ma := genesis.MultisigAlias{}

	var (
		err                   error
		aliasBytes, addrBytes []byte
		alias                 ids.ShortID
		addrs                 = make([]ids.ShortID, len(uma.Addresses))
	)

	if _, _, aliasBytes, err = address.Parse(uma.Alias); err == nil {
		alias, err = ids.ToShortID(aliasBytes)
	}
	if err != nil {
		return ma, err
	}

	for i, addr := range uma.Addresses {
		if _, _, addrBytes, err = address.Parse(addr); err == nil {
			addrs[i], err = ids.ToShortID(addrBytes)
		}
		if err != nil {
			return ma, err
		}
	}

	ma, err = genesis.NewMultisigAlias(
		ids.ID{},
		addrs,
		uma.Threshold,
	)
	if err != nil {
		return ma, err
	}

	if alias != ma.Alias {
		was, _ := address.Format("P", constants.GetHRP(networkID), alias.Bytes())
		expected, _ := address.Format("P", constants.GetHRP(networkID), ma.Alias.Bytes())
		return ma, fmt.Errorf("alias calculation mismatch, was: %s, expected: %s", was, expected)
	}

	return ma, nil
}

func (uma *UnparsedMultisigAlias) Unparse(ma genesis.MultisigAlias, networkID uint32) error {
	addresses := make([]string, len(ma.Addresses))
	for i, a := range ma.Addresses {
		addr, err := address.Format("P", constants.GetHRP(networkID), a.Bytes())
		if err != nil {
			return fmt.Errorf("while unparsing cannot format multisig address %s: %w", a, err)
		}
		addresses[i] = addr
	}

	alias, err := address.Format("P", constants.GetHRP(networkID), ma.Alias.Bytes())
	if err != nil {
		return fmt.Errorf("while unparsing cannot format multisig alias %s: %w", ma.Alias, err)
	}
	uma.Alias = alias
	uma.Addresses = addresses
	uma.Threshold = ma.Threshold

	return nil
}
