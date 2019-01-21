package bridge

import (
	"encoding/base64"

	b "github.com/stellar/go/build"
	shared "github.com/stellar/go/services/internal/bridge-compliance-shared"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/http/helpers"
)

// ManageDataOperationBody represents manage_data operation
type ManageDataOperationBody struct {
	Source *string
	Name   string
	Data   string
}

// ToTransactionMutator returns go-stellar-base TransactionMutator
func (op ManageDataOperationBody) ToTransactionMutator() b.TransactionMutator {
	var builder b.ManageDataBuilder

	if op.Data == "" {
		builder = b.ClearData(op.Name)
	} else {
		// This is validated in Validate()
		data, _ := base64.StdEncoding.DecodeString(op.Data)
		builder = b.SetData(op.Name, data)
	}

	if op.Source != nil {
		builder.Mutate(b.SourceAccount{*op.Source})
	}

	return builder
}

// Validate validates if operation body is valid.
func (op ManageDataOperationBody) Validate() error {
	if len(op.Name) > 64 {
		return helpers.NewInvalidParameterError("name", "Name must be less than or equal 64 characters.")
	}

	data, err := base64.StdEncoding.DecodeString(op.Data)
	if err != nil || len(data) > 64 {
		return helpers.NewInvalidParameterError("data", "Not a valid base64 string.")
	}

	if op.Source != nil && !shared.IsValidAccountID(*op.Source) {
		return helpers.NewInvalidParameterError("source", "Source must be a public key (starting with `G`).")
	}

	return nil
}
