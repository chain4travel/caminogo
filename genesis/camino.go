package genesis

import (
	"errors"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"

	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
)

func validateCaminoConfig(config *Config) error {
	_, err := address.Format(
		configChainIDAlias,
		constants.GetHRP(config.NetworkID),
		config.Camino.InitialAdmin.Bytes(),
	)
	if err != nil {
		return fmt.Errorf(
			"unable to format address from %s",
			config.Camino.InitialAdmin.String(),
		)
	}

	if err = validateDepositOffers(config.Camino.DepositOffers); err != nil {
		return err
	}
	if err = validateMultisigAddresses(config.Camino.InitialMultisigAddresses); err != nil {
		return err
	}

	return nil
}

func validateDepositOffers(offers []genesis.DepositOffer) error {
	for _, offer := range offers {
		if offer.Start >= offer.End {
			return fmt.Errorf(
				"deposit offer starttime (%v) is not before its endtime (%v)",
				offer.Start,
				offer.End,
			)
		}

		if offer.MinDuration > offer.MaxDuration {
			return errors.New("deposit minimum duration is greater than maximum duration")
		}

		if offer.MinDuration < offer.UnlockHalfPeriodDuration {
			return fmt.Errorf(
				"deposit offer minimum duration (%v) is less than unlock half-period duration (%v)",
				offer.MinDuration,
				offer.UnlockHalfPeriodDuration,
			)
		}

		if offer.MinDuration == 0 {
			return errors.New("deposit offer has zero  minimum duration")
		}
	}

	return nil
}

func validateMultisigAddresses(multisigAddrs []genesis.MultisigAlias) error {
	addresses := ids.NewShortSet(4 * len(multisigAddrs))
	aliases := ids.NewShortSet(len(multisigAddrs))

	for _, ma := range multisigAddrs {
		if ma.Threshold == 0 {
			return fmt.Errorf("multisig threshold must be greater than 0")
		}
		if ma.Threshold > uint32(len(ma.Addresses)) {
			return fmt.Errorf("multisig threshold exceeds the number of addresses")
		}

		// check alias was not previously defined
		if aliases.Contains(ma.Alias) {
			return fmt.Errorf("alias %s definition is duplicated", ma.Alias)
		}

		aliases.Add(ma.Alias)
		addresses.Add(ma.Addresses...)
	}

	// check alias was not used as an address of another alias
	for _, ma := range multisigAddrs {
		if addresses.Contains(ma.Alias) {
			return fmt.Errorf("alias %s is used as an address of another alias", ma.Alias)
		}
	}

	return nil
}
