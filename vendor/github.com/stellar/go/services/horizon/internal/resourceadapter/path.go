package resourceadapter

import (
	"context"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/paths"
)

// PopulatePath converts the paths.Path into a Path
func PopulatePath(ctx context.Context, dest *horizon.Path, q paths.Query, p paths.Path) (err error) {
	dest.DestinationAmount = amount.String(q.DestinationAmount)
	dest.SourceAmount = amount.String(p.Cost)

	err = p.Source.Extract(
		&dest.SourceAssetType,
		&dest.SourceAssetCode,
		&dest.SourceAssetIssuer)
	if err != nil {
		return
	}

	err = p.Destination.Extract(
		&dest.DestinationAssetType,
		&dest.DestinationAssetCode,
		&dest.DestinationAssetIssuer)
	if err != nil {
		return
	}

	dest.Path = make([]horizon.Asset, len(p.Path))
	for i, a := range p.Path {
		err = a.Extract(
			&dest.Path[i].Type,
			&dest.Path[i].Code,
			&dest.Path[i].Issuer)
		if err != nil {
			return
		}
	}
	return
}
