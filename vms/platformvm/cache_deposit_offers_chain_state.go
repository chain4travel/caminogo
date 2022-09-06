// Copyright (C) 2022, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"time"

	"github.com/chain4travel/caminogo/ids"
	"github.com/chain4travel/caminogo/utils/hashing"
)

var _ depositOffersChainState = &depositOffersChainStateImpl{}

type depositOffersChainState interface {
	GetOfferByID(offerID ids.ID) *depositOffer
	GetAllOffers() []*depositOffer
	AddOffer(offer *depositOffer) depositOffersChainState

	Apply(InternalState)
}

// depositOffersChainStateImpl is a copy on write implementation for versioning
// the deposit offers. None of the slices, maps, or pointers should be modified
// after initialization.
type depositOffersChainStateImpl struct {
	offers map[ids.ID]*depositOffer // offerID -> *depositOffer

	addedOffers []*depositOffer
}

func (cs *depositOffersChainStateImpl) GetOfferByID(offerID ids.ID) *depositOffer {
	return cs.offers[offerID]
}

func (cs *depositOffersChainStateImpl) GetAllOffers() []*depositOffer {
	offers := make([]*depositOffer, len(cs.offers))
	i := 0
	for _, offer := range cs.offers {
		offers[i] = offer
	}
	return offers
}

func (cs *depositOffersChainStateImpl) AddOffer(offer *depositOffer) depositOffersChainState {
	newCS := &depositOffersChainStateImpl{
		offers:      make(map[ids.ID]*depositOffer, len(cs.offers)+1),
		addedOffers: []*depositOffer{offer},
	}

	for offerID, offer := range cs.offers {
		newCS.offers[offerID] = offer
	}

	newCS.offers[offer.id] = offer

	return newCS
}

func (cs *depositOffersChainStateImpl) Apply(is InternalState) {
	for _, offer := range cs.addedOffers {
		is.AddDepositOffer(offer)
	}

	is.SetDepositOffersChainState(cs)

	cs.addedOffers = nil
}

const interestRateDenominator uint64 = 1_000_000

type depositOffer struct {
	id ids.ID

	InterestRateNominator uint64 `serialize:"true"`
	Start                 uint64 `serialize:"true"`
	End                   uint64 `serialize:"true"`
	MinAmount             uint64 `serialize:"true"`
	DepositDuration       uint64 `serialize:"true"`
}

func (o *depositOffer) SetID() error {
	bytes, err := Codec.Marshal(CodecVersion, &o)
	if err != nil {
		return err
	}
	o.id = hashing.ComputeHash256Array(bytes)
	return nil
}

func (o depositOffer) StartTime() time.Time {
	return time.Unix(int64(o.Start), 0)
}

func (o depositOffer) EndTime() time.Time {
	return time.Unix(int64(o.End), 0)
}

func (o depositOffer) InterestRateFloat64() float64 {
	return float64(o.InterestRateNominator) / float64(interestRateDenominator)
}
