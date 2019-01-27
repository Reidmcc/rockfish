package modules

import (
	"fmt"

	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
)

// ArbitCycleConfig holds the arbitrage strategy settings
type ArbitCycleConfig struct {
	HoldAssetCode   string       `valid:"-" toml:"HOLD_ASSET_CODE"`
	HoldAssetIssuer string       `valid:"-" toml:"HOLD_ASSET_ISSUER"`
	MinRatio        float64      `valid:"-" toml:"MIN_RATIO"`
	UseBalance      bool         `valid:"-" toml:"USE_BALANCE"`
	StaticAmount    float64      `valid:"-" toml:"STATIC_AMOUNT"`
	Assets          []assetInput `valid:"-" toml:"ASSETS"`
}

// PathFinder keeps track of all the possible payment paths
type PathFinder struct {
	// multiDex  MultiDex won't reside here
	dexWatcher   DexWatcher
	HoldAsset    horizon.Asset
	assetBook    []groupedAsset
	pathList     []PaymentPath
	minRatio     float64
	useBalance   bool
	staticAmount float64
	l            logger.Logger

	//unintialized
	endAssetDisplay string
}

type groupedAsset struct {
	Asset horizon.Asset
	Group int
}

// MakePathFinder is a factory method
func MakePathFinder(
	dexWatcher DexWatcher,
	stratConfig ArbitCycleConfig,
	l logger.Logger,
) (*PathFinder, error) {
	holdAsset := ParseAsset(stratConfig.HoldAssetCode, stratConfig.HoldAssetIssuer)

	var assetBook []groupedAsset

	for _, a := range stratConfig.Assets {
		asset := ParseAsset(a.CODE, a.ISSUER)
		group := a.GROUP
		entry := groupedAsset{
			Asset: asset,
			Group: group,
		}
		assetBook = append(assetBook, entry)
	}

	var pathList []PaymentPath
	endAssetDisplay := holdAsset.Code

	if utils.Asset2Asset(holdAsset) == build.NativeAsset() {
		endAssetDisplay = "XLM"
	}

	l.Info("generating path list: ")

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i].Asset != assetBook[n].Asset && assetBook[i].Group == assetBook[n].Group {
				path := makePaymentPath(assetBook[i].Asset, assetBook[n].Asset, holdAsset)
				l.Infof("added path: %s -> %s -> %s -> %s", endAssetDisplay, assetBook[i].Asset.Code, assetBook[n].Asset.Code, endAssetDisplay)
				pathList = append(pathList, path)
			}
		}
	}

	return &PathFinder{
		dexWatcher:      dexWatcher,
		HoldAsset:       holdAsset,
		assetBook:       assetBook,
		pathList:        pathList,
		minRatio:        stratConfig.MinRatio,
		useBalance:      stratConfig.UseBalance,
		staticAmount:    stratConfig.StaticAmount,
		l:               l,
		endAssetDisplay: endAssetDisplay,
	}, nil
}

// assetInput holds the inbound asset strings
type assetInput struct {
	CODE   string `valid:"-"`
	ISSUER string `valid:"-"`
	GROUP  int    `valid:"-"`
}

// TradingPair represents a trading pair
type TradingPair struct {
	Base  horizon.Asset
	Quote horizon.Asset
}

// PaymentPath is a pair of assets for a payment path and their encoded tradingPair
// probably don't need separate PriceFeed concepts if the tradingPair is here
type PaymentPath struct {
	HoldAsset  horizon.Asset
	PathAssetA horizon.Asset
	PathAssetB horizon.Asset
	FirstPair  TradingPair
	MidPair    TradingPair
	LastPair   TradingPair
}

// String impl.
func (c ArbitCycleConfig) String() string {
	return utils.StructString(c, nil)
}

// makePaymentPath makes a payment path
func makePaymentPath(assetA horizon.Asset, assetB horizon.Asset, holdAsset horizon.Asset) PaymentPath {
	firstPair := TradingPair{
		Base:  assetA,
		Quote: holdAsset,
	}
	midPair := TradingPair{
		Base:  assetA,
		Quote: assetB,
	}
	lastPair := TradingPair{
		Base:  assetB,
		Quote: holdAsset,
	}
	return PaymentPath{
		HoldAsset:  holdAsset,
		PathAssetA: assetA,
		PathAssetB: assetB,
		FirstPair:  firstPair,
		MidPair:    midPair,
		LastPair:   lastPair,
	}
}

// FindBestPath determines and returns the most profitable payment path, its max amount, and whether its good enough
func (p *PathFinder) FindBestPath() (*PaymentPath, float64, bool, error) {
	bestRatio := 0.0
	maxAmount := 0.0
	var bestPath PaymentPath

	for _, b := range p.pathList {
		ratio, amount, e := p.calculatePathValues(b)
		if e != nil {
			return nil, 0, false, fmt.Errorf("Error while calculating ratios %s", e)
		}
		if ratio > bestRatio {
			bestRatio = ratio
			maxAmount = amount
			bestPath = b
		}
		p.l.Infof("Return ratio for path %s -> %s - > %s -> %s was %v \n", p.endAssetDisplay, b.PathAssetA.Code, b.PathAssetB.Code, p.endAssetDisplay, ratio)
	}
	p.l.Infof("Best path was %s -> %s - > %s %s -> with return ratio of %v\n", p.endAssetDisplay, bestPath.PathAssetA.Code, bestPath.PathAssetB.Code, p.endAssetDisplay, bestRatio)
	metThreshold := false
	if bestRatio >= p.minRatio {
		metThreshold = true
		p.l.Info("")
		p.l.Info("***** Minimum profit ratio was met, proceeding to payment! *****")
		p.l.Info("")
	}

	return &bestPath, maxAmount, metThreshold, nil
}

// calculatePathValues returns the path's best ratio and max amount at that ratio
func (p *PathFinder) calculatePathValues(path PaymentPath) (float64, float64, error) {
	// first pair is buying asset A with the hold asset
	firstPairLowAskPrice, firstPairLowAskAmount, e := p.dexWatcher.GetLowAsk(path.FirstPair)
	if e != nil {
		return 0, 0, fmt.Errorf("Error while calculating path ratio %s", e)
	}
	if firstPairLowAskPrice == -1 || firstPairLowAskAmount == -1 {
		return 0, 0, nil
	}

	// mid pair is selling asset A for asset B
	midPairTopBidPrice, midPairTopBidAmount, e := p.dexWatcher.GetTopBid(path.MidPair)
	if e != nil {
		return 0, 0, fmt.Errorf("Error while calculating path ratio %s", e)
	}
	if midPairTopBidPrice == -1 || midPairTopBidAmount == -1 {
		return 0, 0, nil
	}

	// last pair is selling asset B for the hold asset
	lastPairTopBidPrice, lastPairTopBidAmount, e := p.dexWatcher.GetTopBid(path.LastPair)
	if e != nil {
		return 0, 0, fmt.Errorf("Error while calculating path ratio %s", e)
	}
	if lastPairTopBidPrice == -1 || lastPairTopBidAmount == -1 {
		return 0, 0, nil
	}

	ratio := (1 / firstPairLowAskPrice) * midPairTopBidPrice * lastPairTopBidPrice

	// max amount is the lowest of the three steps' amounts in units of the hold asset
	// but has to account for the middle trade that doesn't include the hold asset
	maxFirstBuy := firstPairLowAskAmount * firstPairLowAskPrice // how many hold asset tokens spendable on the low ask of AssetA
	maxSecondSell := firstPairLowAskAmount * midPairTopBidPrice // how many AssetB returned if selling max available AssetA
	if midPairTopBidAmount < maxSecondSell {
		maxSecondSell = midPairTopBidAmount // if there aren't enough AssetB on offer to get maxSecondSell from above, reduce to the available AssetB
	}
	maxLastSell := maxSecondSell * lastPairTopBidPrice // how many hold asset tokens returned if selling max available AssetB
	if lastPairTopBidAmount < maxLastSell {
		maxLastSell = lastPairTopBidAmount
	}
	// find which end's max is lower
	maxCycleAmount := maxFirstBuy
	if maxLastSell < maxFirstBuy {
		maxCycleAmount = maxLastSell
	}

	return ratio, maxCycleAmount, nil
}

// WhatRatio returns the minimum ratio
func (p *PathFinder) WhatRatio() float64 {
	return p.minRatio
}

// WhatAmount returns the payment amount settings
func (p *PathFinder) WhatAmount() (bool, float64) {
	return p.useBalance, p.staticAmount
}
