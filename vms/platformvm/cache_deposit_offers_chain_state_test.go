// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	cjson "github.com/chain4travel/caminogo/utils/json"
)

var genesisDepositOffer = depositOffer{
	InterestRateNominator: uint64(0.1 * cjson.Float64(interestRateDenominator)),
	Start:                 1659342978,
	End:                   1672516799,
	MinAmount:             0,
	DepositDuration:       60,
}

func TestGetGenesisDepositOffers(t *testing.T) {
	service := defaultService(t)
	service.vm.ctx.Lock.Lock()
	defer func() {
		err := service.vm.Shutdown()
		assert.NoError(t, err)
		service.vm.ctx.Lock.Unlock()
	}()

	err := genesisDepositOffer.SetID()
	assert.NoError(t, err)

	expectedReply := GetDepositOffersReply{
		Offers: []*APIDepositOffer{
			{
				ID:              genesisDepositOffer.id,
				InterestRate:    cjson.Float64(genesisDepositOffer.InterestRateFloat64()),
				Start:           cjson.Uint64(genesisDepositOffer.Start),
				End:             cjson.Uint64(genesisDepositOffer.End),
				MinAmount:       cjson.Uint64(genesisDepositOffer.MinAmount),
				DepositDuration: cjson.Uint64(genesisDepositOffer.DepositDuration),
			},
		},
	}

	service.vm.Clock().Set(time.Now())
	depositOffersReply := GetDepositOffersReply{}
	err = service.GetDepositOffers(nil, nil, &depositOffersReply)
	assert.NoError(t, err)
	assert.Equal(t, depositOffersReply, expectedReply)
}

func TestGetGenesisDepositOfferById(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	err := genesisDepositOffer.SetID()
	assert.NoError(t, err)

	depositOfferReply := vm.internalState.DepositOffersChainState().GetOfferByID(genesisDepositOffer.id)
	assert.Equal(t, *depositOfferReply, genesisDepositOffer)
}

func TestAddDepositOffer(t *testing.T) {
	vm, _, _ := defaultVM()
	vm.ctx.Lock.Lock()
	defer func() {
		if err := vm.Shutdown(); err != nil {
			t.Fatal(err)
		}
		vm.ctx.Lock.Unlock()
	}()

	newDepositOffer := &depositOffer{
		InterestRateNominator: uint64(0.1 * cjson.Float64(interestRateDenominator)),
		Start:                 1672531201,
		End:                   1704067199,
		MinAmount:             0,
		DepositDuration:       120,
	}

	err := newDepositOffer.SetID()
	assert.NoError(t, err)
	offersState := vm.internalState.DepositOffersChainState()
	newOffersState := offersState.AddOffer(newDepositOffer)
	newOffersState.Apply(vm.internalState)
	err = vm.internalState.Commit()
	assert.NoError(t, err)

	depositOfferReply := newOffersState.GetOfferByID(newDepositOffer.id)
	assert.Equal(t, depositOfferReply, newDepositOffer)
}
