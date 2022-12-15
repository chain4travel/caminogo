package genesis

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/stretchr/testify/require"
)

var (
	sampleID                   = ids.GenerateTestID()
	sampleShortID              = ids.GenerateTestShortID()
	addressWithInvalidFormat   = ids.GenerateTestShortID().String()
	addressWithInvalidChecksum = "X-camino1859dz2uwazfgahey3j53ef2kqrans0c8htcu8n"
	xAddress                   = "X-camino1859dz2uwazfgahey3j53ef2kqrans0c8htcu7n"
	cAddress                   = "C-camino1859dz2uwazfgahey3j53ef2kqrans0c8htcu7n"
	toShortID                  = ignoreError(ids.ToShortID([]byte(cAddress))).(ids.ShortID)
)

func TestParse(t *testing.T) {
	type fields struct {
		VerifyNodeSignature bool
		LockModeBondDeposit bool
		InitialAdmin        string
		DepositOffers       []genesis.DepositOffer
		Allocations         []UnparsedCaminoAllocation
	}
	tests := map[string]struct {
		fields fields
		want   Camino
		err    error
	}{
		"Invalid address - no prefix": {
			fields: fields{
				InitialAdmin: addressWithInvalidFormat,
			},
			err: errCannotParseInitialAdmin,
		},
		"Invalid address - bad checksum": {
			fields: fields{
				InitialAdmin: addressWithInvalidChecksum,
			},
			err: errCannotParseInitialAdmin,
		},
		"Invalid allocation - missing eth address": {
			fields: fields{
				InitialAdmin: xAddress,
				Allocations:  []UnparsedCaminoAllocation{{}},
			},
			err: errInvalidETHAddress,
		},
		"Invalid allocation - invalid eth address": {
			fields: fields{
				InitialAdmin: xAddress,
				Allocations: []UnparsedCaminoAllocation{{
					ETHAddr: ids.GenerateTestShortID().String(),
				}},
			},
			err: errors.New("encoding/hex: invalid byte"),
		},
		"Invalid allocation - invalid avax address": {
			fields: fields{
				InitialAdmin: xAddress,
				Allocations: []UnparsedCaminoAllocation{{
					ETHAddr:  "0x" + hex.EncodeToString(toShortID.Bytes()),
					AVAXAddr: addressWithInvalidFormat,
				}},
			},
			err: errors.New("no separator found in address"),
		},
		"Valid allocation": {
			fields: fields{
				VerifyNodeSignature: true,
				LockModeBondDeposit: true,
				InitialAdmin:        xAddress,
				DepositOffers:       nil,
				Allocations: []UnparsedCaminoAllocation{{
					ETHAddr:  "0x" + hex.EncodeToString(toShortID.Bytes()),
					AVAXAddr: xAddress,
					PlatformAllocations: []UnparsedPlatformAllocation{{
						Amount:            1,
						NodeID:            "NodeID-" + sampleShortID.String(),
						ValidatorDuration: 1,
						DepositOfferID:    sampleID.String(),
					}},
				}},
			},
			want: Camino{
				VerifyNodeSignature: true,
				LockModeBondDeposit: true,
				InitialAdmin:        toAvaxAddr(xAddress),
				DepositOffers:       nil,
				Allocations: []CaminoAllocation{{
					ETHAddr: func() ids.ShortID {
						i, _ := ids.ShortFromString("0x" + hex.EncodeToString(toShortID.Bytes()))
						return i
					}(),
					AVAXAddr: toAvaxAddr(xAddress),
					PlatformAllocations: []PlatformAllocation{{
						Amount:            1,
						NodeID:            ids.NodeID(sampleShortID),
						ValidatorDuration: 1,
						DepositOfferID:    sampleID,
					}},
				}},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			uc := UnparsedCamino{
				VerifyNodeSignature: tt.fields.VerifyNodeSignature,
				LockModeBondDeposit: tt.fields.LockModeBondDeposit,
				InitialAdmin:        tt.fields.InitialAdmin,
				DepositOffers:       tt.fields.DepositOffers,
				Allocations:         tt.fields.Allocations,
			}
			got, err := uc.Parse()

			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func toAvaxAddr(id string) ids.ShortID {
	_, _, avaxAddrBytes, _ := address.Parse(id)
	avaxAddr, _ := ids.ToShortID(avaxAddrBytes)
	return avaxAddr
}
