package bridge

import (
	b "github.com/stellar/go/build"
	shared "github.com/stellar/go/services/internal/bridge-compliance-shared"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/http/helpers"
)

// AllowTrustOperationBody represents allow_trust operation
type AllowTrustOperationBody struct {
	Source    *string
	AssetCode string `json:"asset_code"`
	Trustor   string
	Authorize bool
}

// ToTransactionMutator returns stellar/go TransactionMutator
func (op AllowTrustOperationBody) ToTransactionMutator() b.TransactionMutator {
	mutators := []interface{}{
		b.AllowTrustAsset{op.AssetCode},
		b.Trustor{op.Trustor},
		b.Authorize{op.Authorize},
	}

	if op.Source != nil {
		mutators = append(mutators, b.SourceAccount{*op.Source})
	}

	return b.AllowTrust(mutators...)
}

// Validate validates if operation body is valid.
func (op AllowTrustOperationBody) Validate() error {
	if !shared.IsValidAssetCode(op.AssetCode) {
		return helpers.NewInvalidParameterError("asset_code", "Asset code is invalid")
	}

	if !shared.IsValidAccountID(op.Trustor) {
		return helpers.NewInvalidParameterError("trustor", "Trustor must be a public key (starting with `G`).")
	}

	if op.Source != nil && !shared.IsValidAccountID(*op.Source) {
		return helpers.NewInvalidParameterError("source", "Source must be a public key (starting with `G`).")
	}

	return nil
}
