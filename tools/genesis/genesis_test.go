package main

import (
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/stretchr/testify/require"
)

func TestA(t *testing.T) {
	hrp := constants.KopernikusHRP

	addrs, err := address.ParseToIDs([]string{
		"X-kopernikus18pry238vq8uzzpar32wa4x4p2rkyc7wx2qnctc",
		"X-kopernikus18l0l8dz03vmk7k7y5cgax8k2l5mphj4p5k040u",
		"X-kopernikus1d0ydd4wyq670lzrp0y7xd3wej4z7ysp2surum8",
		"X-kopernikus1h87ampd07hwtnqywpsagrxm9ygkmcr34ndjah9",
		"X-kopernikus102uap4au55t22m797rr030wyrw0jlgw25ut8vj",
	})
	require.NoError(t, err)

	ma, err := newMultisigAlias(ids.Empty, addrs, 2, "1931")
	require.NoError(t, err)
	printAlias(t, &ma, hrp)

	fmt.Println("")

	ma, err = newMultisigAlias(ids.Empty, addrs, 2, "1932")
	require.NoError(t, err)
	printAlias(t, &ma, hrp)
}

func printAlias(t *testing.T, ma *genesis.MultisigAlias, hrp string) {
	aliasStr, err := address.Format("X", hrp, ma.Alias.Bytes())
	require.NoError(t, err)

	aliasAddrsStrs, err := formatAddresses(ma.Addresses, hrp)
	require.NoError(t, err)

	fmt.Printf("alias: %s\n", aliasStr)
	fmt.Printf("aliasAddrses:\n")
	for i, aliasAddrStr := range aliasAddrsStrs {
		fmt.Printf("  [%d] %s\n", i, aliasAddrStr)
	}
	fmt.Printf("threshold: %d\n", ma.Threshold)
	fmt.Printf("memo: %s\n", ma.Memo)
}

func formatAddresses(addrs []ids.ShortID, hrp string) ([]string, error) {
	formattedAddrs := []string{}
	for _, addr := range addrs {
		aliasStr, err := address.Format("X", hrp, addr.Bytes())
		if err != nil {
			return nil, err
		}
		formattedAddrs = append(formattedAddrs, aliasStr)
	}
	return formattedAddrs, nil
}
