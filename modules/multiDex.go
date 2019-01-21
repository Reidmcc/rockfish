package modules

import (
	"github.com/interstellar/kelp/model"
	"github.com/interstellar/kelp/plugins"
	"github.com/interstellar/kelp/support/logger"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
)

// MultiDex is an SDEX for use with a hold asset and a list of payment paths.
// It should really be independent of the SDEX struct but functionally this works perfectly
type MultiDex struct {
	sdex *plugins.SDEX
	l    logger.Logger
}

// MakeMultiDex is the factory method
func MakeMultiDex(
	api *horizon.Client,
	sourceSeed string,
	tradingSeed string,
	sourceAccount string,
	tradingAccount string,
	network build.Network,
	threadTracker *multithreading.ThreadTracker,
	operationalBuffer float64,
	simMode bool,
) *MultiDex {
	l := logger.MakeBasicLogger()
	var pairPlaceHolder *model.TradingPair
	var mapPlaceHolder map[model.Asset]horizon.Asset
	sdex := plugins.MakeSDEX(
		api,
		sourceSeed,
		tradingSeed,
		sourceAccount,
		tradingAccount,
		network,
		threadTracker,
		operationalBuffer,
		0,
		simMode,
		pairPlaceHolder,
		mapPlaceHolder,
	)

	return &MultiDex{
		sdex: sdex,
		l:    l,
	}
}
