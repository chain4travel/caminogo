package genesis

import (
	"encoding/hex"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/stretchr/testify/require"
)

var (
	nodeID         = ids.GenerateTestNodeID()
	depositOfferID = ids.GenerateTestID()
)

func TestUnparse(t *testing.T) {
	type fields struct {
		VerifyNodeSignature bool
		LockModeBondDeposit bool
		InitialAdmin        ids.ShortID
		DepositOffers       []genesis.DepositOffer
		Allocations         []CaminoAllocation
	}
	type args struct {
		networkID uint32
	}
	tests := map[string]struct {
		fields fields
		args   args
		want   UnparsedCamino
		err    error
	}{
		"success": {
			args: args{networkID: 12345},
			fields: fields{
				VerifyNodeSignature: true,
				LockModeBondDeposit: true,
				InitialAdmin:        sampleShortID,
				DepositOffers:       nil,
				Allocations: []CaminoAllocation{{
					ETHAddr:      sampleShortID,
					AVAXAddr:     sampleShortID,
					XAmount:      1,
					AddressState: 1,
					PlatformAllocations: []PlatformAllocation{{
						Amount:            1,
						NodeID:            nodeID,
						ValidatorDuration: 1,
						DepositOfferID:    depositOfferID,
					}},
				}},
			},
			want: UnparsedCamino{
				VerifyNodeSignature: true,
				LockModeBondDeposit: true,
				InitialAdmin:        "X-" + wrappers.IgnoreError(address.FormatBech32("local", sampleShortID.Bytes())).(string),
				DepositOffers:       nil,
				Allocations: []UnparsedCaminoAllocation{{
					ETHAddr:      "0x" + hex.EncodeToString(sampleShortID.Bytes()),
					AVAXAddr:     "X-" + wrappers.IgnoreError(address.FormatBech32("local", sampleShortID.Bytes())).(string),
					XAmount:      1,
					AddressState: 1,
					PlatformAllocations: []UnparsedPlatformAllocation{{
						Amount:            1,
						NodeID:            nodeID.String(),
						ValidatorDuration: 1,
						DepositOfferID:    depositOfferID.String(),
					}},
				}},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := Camino{
				VerifyNodeSignature: tt.fields.VerifyNodeSignature,
				LockModeBondDeposit: tt.fields.LockModeBondDeposit,
				InitialAdmin:        tt.fields.InitialAdmin,
				DepositOffers:       tt.fields.DepositOffers,
				Allocations:         tt.fields.Allocations,
			}
			got, err := c.Unparse(tt.args.networkID)

			if tt.err != nil {
				require.ErrorContains(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
