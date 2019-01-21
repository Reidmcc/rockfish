package bridge

import (
	"strconv"

	b "github.com/stellar/go/build"

	shared "github.com/stellar/go/services/internal/bridge-compliance-shared"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/http/helpers"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/protocols"
)

// ManageOfferOperationBody represents manage_offer operation
type ManageOfferOperationBody struct {
	PassiveOffer bool `json:"-"`
	Source       *string
	Selling      protocols.Asset
	Buying       protocols.Asset
	Amount       string
	Price        string
	OfferID      *string `json:"offer_id"`
}

// uint64

// ToTransactionMutator returns go-stellar-base TransactionMutator
func (op ManageOfferOperationBody) ToTransactionMutator() b.TransactionMutator {
	mutators := []interface{}{
		b.Amount(op.Amount),
		b.Rate{
			Selling: op.Selling.ToBaseAsset(),
			Buying:  op.Buying.ToBaseAsset(),
			Price:   b.Price(op.Price),
		},
	}

	if op.OfferID != nil {
		// Validated in Validate()
		u, _ := strconv.ParseUint(*op.OfferID, 10, 64)
		mutators = append(mutators, b.OfferID(u))
	}

	if op.Source != nil {
		mutators = append(mutators, b.SourceAccount{*op.Source})
	}

	return b.ManageOffer(op.PassiveOffer, mutators...)
}

// Validate validates if operation body is valid.
func (op ManageOfferOperationBody) Validate() error {
	if op.OfferID != nil {
		_, err := strconv.ParseUint(*op.OfferID, 10, 64)
		if err != nil {
			return helpers.NewInvalidParameterError("offer_id", "Not a number.")
		}
	}

	if op.Source != nil && !shared.IsValidAccountID(*op.Source) {
		return helpers.NewInvalidParameterError("source", "Source must be a public key (starting with `G`).")
	}

	return nil
}
