package bridge

import (
	"github.com/stellar/go/amount"
	b "github.com/stellar/go/build"
	shared "github.com/stellar/go/services/internal/bridge-compliance-shared"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/http/helpers"
)

// CreateAccountOperationBody represents create_account operation
type CreateAccountOperationBody struct {
	Source          *string
	Destination     string
	StartingBalance string `json:"starting_balance"`
}

// ToTransactionMutator returns go-stellar-base TransactionMutator
func (op CreateAccountOperationBody) ToTransactionMutator() b.TransactionMutator {
	mutators := []interface{}{
		b.Destination{op.Destination},
		b.NativeAmount{op.StartingBalance},
	}

	if op.Source != nil {
		mutators = append(mutators, b.SourceAccount{*op.Source})
	}

	return b.CreateAccount(mutators...)
}

// Validate validates if operation body is valid.
func (op CreateAccountOperationBody) Validate() error {
	if !shared.IsValidAccountID(op.Destination) {
		return helpers.NewInvalidParameterError("destination", "Destination must be a public key (starting with `G`)")
	}

	_, err := amount.Parse(op.StartingBalance)
	if err != nil {
		return helpers.NewInvalidParameterError("starting_balance", "Not a valid amount.")
	}

	if op.Source != nil && !shared.IsValidAccountID(*op.Source) {
		return helpers.NewInvalidParameterError("source", "Source must be a public key (starting with `G`)")
	}

	return nil
}
