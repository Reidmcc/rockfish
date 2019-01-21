package resourceadapter

import (
	"context"

	"github.com/stellar/go/xdr"
	. "github.com/stellar/go/protocols/horizon"

)

func PopulateAsset(ctx context.Context, dest *Asset, asset xdr.Asset) error {
	return asset.Extract(&dest.Type, &dest.Code, &dest.Issuer)
}
