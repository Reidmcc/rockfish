package plugins

import (
	"log"

	"github.com/interstellar/kelp/api"
	"github.com/interstellar/kelp/model"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
)

// deleteSideStrategy is a sideStrategy to delete the orders for a given currency pair on one side of the orderbook
type deleteSideStrategy struct {
	sdex       *SDEX
	assetBase  *horizon.Asset
	assetQuote *horizon.Asset
}

// ensure it implements SideStrategy
var _ api.SideStrategy = &deleteSideStrategy{}

// makeDeleteSideStrategy is a factory method for deleteSideStrategy
func makeDeleteSideStrategy(
	sdex *SDEX,
	assetBase *horizon.Asset,
	assetQuote *horizon.Asset,
) api.SideStrategy {
	return &deleteSideStrategy{
		sdex:       sdex,
		assetBase:  assetBase,
		assetQuote: assetQuote,
	}
}

// PruneExistingOffers impl
func (s *deleteSideStrategy) PruneExistingOffers(offers []horizon.Offer) ([]build.TransactionMutator, []horizon.Offer) {
	log.Printf("deleteSideStrategy: deleting %d offers\n", len(offers))
	pruneOps := []build.TransactionMutator{}
	for i := 0; i < len(offers); i++ {
		pOp := s.sdex.DeleteOffer(offers[i])
		pruneOps = append(pruneOps, &pOp)
	}
	return pruneOps, []horizon.Offer{}
}

// PreUpdate impl
func (s *deleteSideStrategy) PreUpdate(maxAssetBase float64, maxAssetQuote float64, trustBase float64, trustQuote float64) error {
	return nil
}

// UpdateWithOps impl
func (s *deleteSideStrategy) UpdateWithOps(offers []horizon.Offer) (ops []build.TransactionMutator, newTopOffer *model.Number, e error) {
	return []build.TransactionMutator{}, nil, nil
}

// PostUpdate impl
func (s *deleteSideStrategy) PostUpdate() error {
	return nil
}

// GetFillHandlers impl
func (s *deleteSideStrategy) GetFillHandlers() ([]api.FillHandler, error) {
	return nil, nil
}
