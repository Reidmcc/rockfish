package plugins

import (
	"github.com/stellar/go/clients/horizon"
	"github.com/interstellar/kelp/support/logger"
)

// MultiDex is an SDEX with a hold asset and a list of payment paths. It should really be independent of the SDEX struct but functionally this works perfectly
type MultiDex struct {
	sdex      SDEX
	holdAsset *horizon.Asset
	pathList  PathList
	l logger.Logger
}

// AssetList is a list of assets 
type AssetList struct {
	assets []*horizon.Asset
}

// PathList is a set of payment paths
type PathList struct {
	paths []PaymentPath
}

// PaymentPath is a pair of assets for a payment path
type PaymentPath struct {
	pathAssetA *horizon.Asset
	pathAssetB *horizon.Asset
}

// Make MultiDex is the factory method
func MakeMultiDex (
	api *horizon.Client,
	sourceSeed string,
	tradingSeed string,
	sourceAccount string,
	tradingAccount string,
	network build.Network,
	threadTracker *multithreading.ThreadTracker,
	operationalBuffer float64,
	simMode bool,
	holdAsset *horizon.Asset,
	pathList PathList,
) *MultiDex {
	l := logger.MakeBasicLogger()
	pairPlaceHolder := model.TradingPair
	mapPlaceHolder := map[model.Asset]horizon.Asset
	sdex := MakeSDEX(
		api,
		sourceSeed,
		tradingSeed,
		sourceAccount,
		tradingAccoun,
		network,
		threadTracker,
		operationalBuffer,
		0,
		simMode,
		pairPlaceHolder,
		mapPlaceHolder,
	)

	return &MultiDex {
		sdex: sdex,
		holdAsset: holdAsset,
		pathList: pathList,
		l: l,
	}
}