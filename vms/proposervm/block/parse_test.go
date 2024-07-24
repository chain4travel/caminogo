// Copyright (C) 2022-2024, Chain4Travel AG. All rights reserved.
//
// This file is a derived work, based on ava-labs code whose
// original notices appear below.
//
// It is distributed under the same license conditions as the
// original code from which it is derived.
//
// Much love to the original authors for their work.
// **********************************************************
// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package block

import (
	"crypto"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
)

func TestParse(t *testing.T) {
	require := require.New(t)

	parentID := ids.ID{1}
	timestamp := time.Unix(123, 0)
	pChainHeight := uint64(2)
	innerBlockBytes := []byte{3}
	chainID := ids.ID{4}

	tlsCert, err := staking.NewTLSCert()
	require.NoError(err)

	cert, err := staking.CertificateFromX509(tlsCert.Leaf)
	require.NoError(err)
	key := tlsCert.PrivateKey.(crypto.Signer)

	builtBlock, err := Build(
		parentID,
		timestamp,
		pChainHeight,
		cert,
		innerBlockBytes,
		chainID,
		key,
	)
	require.NoError(err)

	builtBlockBytes := builtBlock.Bytes()
	durangoTimes := []time.Time{
		timestamp.Add(time.Second),  // Durango not activated yet
		timestamp.Add(-time.Second), // Durango activated
	}
	for _, durangoTime := range durangoTimes {
		parsedBlockIntf, err := Parse(builtBlockBytes, durangoTime)
		require.NoError(err)

		parsedBlock, ok := parsedBlockIntf.(SignedBlock)
		require.True(ok)

		equal(require, chainID, builtBlock, parsedBlock)
	}
}

func TestParseDuplicateExtension(t *testing.T) {
	require := require.New(t)

	blockHex := "0000000000000100000000000000000000000000000000000000000000000000000000000000000000000000007b000000000000000200000549308205453082032da003020102020100300d06092a864886f70d01010b050030003020170d3939313233313030303030305a180f32313234303331383134303631345a300030820222300d06092a864886f70d01010105000382020f003082020a0282020100d26e5f3da1caab11ce37919f7e307ee7c3c994498e78a7b8ab54c1c7c5246cb72b29a8fe1288f0938860bdca7335a885c645dcb7bc53cf80775945533cb9d46548f0038ae15ba63c5dcbab1600b42abaf70f467054cced3cd17142c031c43626b10db7986ad858581f6ead5185b77102602fdf2c7e2cddb7c7f11d8d461e3022c0b853ee18a5a93f18b321c8391c745be4c36d5c1759ab8b0bf6779e36529af4b3fcd924b1a33bdc0d807d47bc20040d32f11f1210f3088d55a7282ea07c59da0442805998bcb50ffe98420fc9835d6e664d25e6e41766761588e0fbfc6dacdb9c724f877c28dc45e79aecc4fa5fc24b238aa4512fd7823879edff32073ef8f34c8e609605014712254c4a7cf50f8b35d406e587e5b24a5f75d43d43c57591ee8b2c9ad1c2044c581dac3227e2d404e1e9af4674e762fc125c169b9a1b254a485d656f5c91d0388b956ad52cdac520b701555c2fe0e09087b6bbcffda981a58d8e98456af6a69ae24127ee7b438e24c67d88872f2363b505ac427c49e1592c2436de5ec245fac56cc24111b8a38a24e0bdfbef7627d6ca27af96d6b20d6fecb032dee7f3a459dc34730f290fda40f0eea1024c9b2a087b0055fdbc1621d9a9d87dd4b356b7caf121ba00022bf8a87711ca39583890128d01333b9ddb0ec4447c5bb0c85c6b295b2481f3a8f86b45536b3d15a0582fd3ac780ab01739fd6cd4d70203010001a381c73081c4300e0603551d0f0101ff0404030204b0300c0603551d130101ff04023000305106092a864886f70d0109150101ff04411cd3184187185ef0be03549b4c5d9b9d7592fd75eebfbd3de12c71e7360e2776543cf4edf4dbb5d674f61c58841abad64fb1e0ca0c24255d119fd658387cf2b800305106092a864886f70d0109150101ff04411cd3184187185ef0be03549b4c5d9b9d7592fd75eebfbd3de12c71e7360e2776543cf4edf4dbb5d674f61c58841abad64fb1e0ca0c24255d119fd658387cf2b800300d06092a864886f70d01010b050003820201001cf95b768b37bde828ca239e739a4229bacff2c53eb09e6b7f1499cb5157851b51ebdb45f5a94a3d0dc16c3d844ce57bb1f551b9bb6f92bcbdc08a7692e98ac257e594696a6f124df3b8a230a2f6ea34a8dd996516993cd91a2c0993e2c77f73454e77ee0f9d9a191f0a1d6b6b1bec901a1466bc0bcf781aa2e96bc65abc20bb2f5643829d811c50af8360022ee1da37f14d3e46e3d23e17fab57a847f7f3ba685090abf16d548c275654ab832935ecc73d496159078e124223314d0e2d8fc9f27426c8fbe6721684d205bac75d955ee71dd8ce6a1ae3c94da7c87c9c3126f9ae4715cbcccb1a9213357c0115e89e9b8d31cc9bbe0ad7e41e25d7473bdc30eaa541228182f650f53b952bdac8c4e9e5f3ceebe5858d85dd58431eb9dba5e4ff28f4212dd9c5ebf6abcae5dcad6b5f09144befb5a7c3f02c0ba5bff781c3acedc22c1cde635a39fb245bcf9f514949fac8321d6ec054377dbc1b24839caaabc29e3884c4de84523e6fa549253b691f6b5c7bdba6a410dc176c765ca14a499ef01916742138fc8156f2c14e4a122e581d1b6ca79e82dfd015b13c38011e248d25e0daccbe266dafbdff3f4ec99227c56795fe75d0d32876d054e5d124d873bebbaeff57ecb9f35146e97f7683809a615c54b89a8b21d0120cfedf133d4253ab9ae521106d245b50de8163b3e97b2e9eae63a72fc283d73b086e35b83fff3cb3d60000000010300000200032444fef47bcd6f77f9b5890a51a1de3b52269d476a04506727aa20b61dc535d09511c4c403058e2fdd7ee5d751b1b6153c4d02f07bf60988be15bf3ff6469bcfde45bdf12e979879d9537586b7394df60ca465f5facdac1722570b5f51f1eb2e8fa20c46a390d4319555d1a39a289563de511d36d517ecdb21b02f76a76d518a6b0eb40d15544f6d1b2e7fd70108af12260e6eaca8efbb2e254b5a3bcf486da1ebabace68c42c13a2a8f04cc626711f0b26f2d66bd0b451b5b4db474364b2dea51b93a41c9c676c00f54e30d4ddad249faa851bf7e99a5dc1b6431c0f79fc4748fb8fa299ad0eb8d92b24aa083f6d93f60384bccc980fc7ba957b71068977eedb7da7884d8a969fb84f3ef921055d63ceebde7c45ead163e19f6425668ff5c205f8368d4df57179efd64312ea4ddcbbabc1e99438e8d2bd05c5728edf505b9caf87cc07ec19f8b457667fc402d0bf53b437b7079c57bbd1dc004950d016440a178061582d4f5431dcb7f7be3b44c085ea982938800272bb140a1aa53208c849c342cb534bad44d06fddb156c0429b9afa920d765dbf9fd09a9dfc9adcb8abe6e238d1a586ffb8164f05e44822d6130662a358d0ed54c0031fe48f0157d211d307a5ef423a7bea821c0886f562140d0347fb429cc978e69a3fe6733a373224acaccf9cbbd5574f5157c78cf2c1623d8f984efd730f7a9a553058073672d49a0"
	blockBytes, err := hex.DecodeString(blockHex)
	require.NoError(err)

	// Note: The above blockHex specifies 123 as the block's timestamp.
	timestamp := time.Unix(123, 0)
	durangoNotYetActivatedTime := timestamp.Add(time.Second)
	durangoAlreadyActivatedTime := timestamp.Add(-time.Second)

	_, err = Parse(blockBytes, durangoNotYetActivatedTime)
	require.ErrorIs(err, errInvalidCertificate)

	_, err = Parse(blockBytes, durangoAlreadyActivatedTime)
	require.NoError(err)
}

func TestParseHeader(t *testing.T) {
	require := require.New(t)

	chainID := ids.ID{1}
	parentID := ids.ID{2}
	bodyID := ids.ID{3}

	builtHeader, err := BuildHeader(
		chainID,
		parentID,
		bodyID,
	)
	require.NoError(err)

	builtHeaderBytes := builtHeader.Bytes()

	parsedHeader, err := ParseHeader(builtHeaderBytes)
	require.NoError(err)

	equalHeader(require, builtHeader, parsedHeader)
}

func TestParseOption(t *testing.T) {
	require := require.New(t)

	parentID := ids.ID{1}
	innerBlockBytes := []byte{3}

	builtOption, err := BuildOption(parentID, innerBlockBytes)
	require.NoError(err)

	builtOptionBytes := builtOption.Bytes()

	parsedOption, err := Parse(builtOptionBytes, time.Time{})
	require.NoError(err)

	equalOption(require, builtOption, parsedOption)
}

func TestParseUnsigned(t *testing.T) {
	require := require.New(t)

	parentID := ids.ID{1}
	timestamp := time.Unix(123, 0)
	pChainHeight := uint64(2)
	innerBlockBytes := []byte{3}

	builtBlock, err := BuildUnsigned(parentID, timestamp, pChainHeight, innerBlockBytes)
	require.NoError(err)

	builtBlockBytes := builtBlock.Bytes()
	durangoTimes := []time.Time{
		timestamp.Add(time.Second),  // Durango not activated yet
		timestamp.Add(-time.Second), // Durango activated
	}
	for _, durangoTime := range durangoTimes {
		parsedBlockIntf, err := Parse(builtBlockBytes, durangoTime)
		require.NoError(err)

		parsedBlock, ok := parsedBlockIntf.(SignedBlock)
		require.True(ok)

		equal(require, ids.Empty, builtBlock, parsedBlock)
	}
}

func TestParseGibberish(t *testing.T) {
	require := require.New(t)

	bytes := []byte{0, 1, 2, 3, 4, 5}

	_, err := Parse(bytes, time.Time{})
	require.ErrorIs(err, codec.ErrUnknownVersion)
}
