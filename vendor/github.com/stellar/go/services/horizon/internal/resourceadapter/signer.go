package resourceadapter

import (
	"context"

	"github.com/stellar/go/services/horizon/internal/db2/core"
	. "github.com/stellar/go/protocols/horizon"
)

// Populate fills out the fields of the signer, using one of an account's
// secondary signers.
func PopulateSigner(ctx context.Context, dest *Signer, row core.Signer) {
	dest.PublicKey = row.Publickey
	dest.Weight = row.Weight
	dest.Key = row.Publickey
	dest.Type = MustKeyTypeFromAddress(dest.PublicKey)
}

// PopulateMaster fills out the fields of the signer, using a stellar account to
// provide the data.
func PopulateMasterSigner(dest *Signer, row core.Account) {
	dest.PublicKey = row.Accountid
	dest.Weight = int32(row.Thresholds[0])
	dest.Key = row.Accountid
	dest.Type = MustKeyTypeFromAddress(dest.PublicKey)
}
