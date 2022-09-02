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
	lockRuleOffersReply := GetDepositOffersReply{}
	err = service.GetDepositOffers(nil, nil, &lockRuleOffersReply)
	assert.NoError(t, err)
	assert.Equal(t, lockRuleOffersReply, expectedReply)
}

func TestGetGenesisDepositOfferById(t *testing.T) {
	service := defaultService(t)
	service.vm.ctx.Lock.Lock()
	defer func() {
		err := service.vm.Shutdown()
		assert.NoError(t, err)
		service.vm.ctx.Lock.Unlock()
	}()

	err := genesisDepositOffer.SetID()
	assert.NoError(t, err)

	lockRuleOfferReply := service.vm.internalState.DepositOffersChainState().GetOfferByID(genesisDepositOffer.id)
	assert.Equal(t, *lockRuleOfferReply, genesisDepositOffer)
}

func TestAddDepositOffer(t *testing.T) {
	service := defaultService(t)
	service.vm.ctx.Lock.Lock()
	defer func() {
		err := service.vm.Shutdown()
		assert.NoError(t, err)
		service.vm.ctx.Lock.Unlock()
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
	offersState := service.vm.internalState.DepositOffersChainState()
	newOffersState := offersState.AddOffer(newDepositOffer)
	newOffersState.Apply(service.vm.internalState)
	err = service.vm.internalState.Commit()
	assert.NoError(t, err)

	lockRuleOfferReply := newOffersState.GetOfferByID(newDepositOffer.id)
	assert.Equal(t, lockRuleOfferReply, newDepositOffer)
}
