package modules

import (
	"fmt"
	"time"

	"github.com/interstellar/kelp/model"
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
	MinAmount       float64      `valid:"-" toml:"MIN_AMOUNT"`
	Assets          []assetInput `valid:"-" toml:"ASSETS"`
}

// PathFinder keeps track of all the possible payment paths
type PathFinder struct {
	dexWatcher   DexWatcher
	HoldAsset    horizon.Asset
	AssetBook    []groupedAsset
	PathList     []*PaymentPath
	minRatio     *model.Number
	useBalance   bool
	staticAmount *model.Number
	minAmount    *model.Number
	l            logger.Logger

	//unintialized
	endAssetDisplay string
	assetBookMark   int
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

	var pathList []*PaymentPath

	endAssetDisplay := holdAsset.Code //this is just so XLM doesn't show up blank

	if utils.Asset2Asset(holdAsset) == build.NativeAsset() {
		endAssetDisplay = "XLM"
	}

	l.Info("generating path list: ")

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i].Asset != assetBook[n].Asset && assetBook[i].Group == assetBook[n].Group {
				path := makePaymentPath(i, assetBook[i].Asset, assetBook[n].Asset, holdAsset)
				l.Infof("added path: %s -> %s | %s -> %s | %s -> %s", endAssetDisplay, assetBook[i].Asset.Code, assetBook[i].Asset.Issuer, assetBook[n].Asset.Code, assetBook[n].Asset.Issuer, endAssetDisplay)
				pathList = append(pathList, path)
			}
		}
	}

	// fixed agent IDs for testing; remove agent IDs when done testing
	for i := 0; i < len(pathList); i++ {
		pathList[i].AgentID = i
	}

	// assetBookMark ensures we don't get stuck on one asset
	assetBookMark := 0

	return &PathFinder{
		dexWatcher:      dexWatcher,
		HoldAsset:       holdAsset,
		AssetBook:       assetBook,
		PathList:        pathList,
		minRatio:        model.NumberFromFloat(stratConfig.MinRatio, utils.SdexPrecision),
		useBalance:      stratConfig.UseBalance,
		staticAmount:    model.NumberFromFloat(stratConfig.StaticAmount, utils.SdexPrecision),
		minAmount:       model.NumberFromFloat(stratConfig.MinAmount, utils.SdexPrecision),
		l:               l,
		endAssetDisplay: endAssetDisplay,
		assetBookMark:   assetBookMark,
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
type PaymentPath struct {
	AgentID     int
	HoldAsset   horizon.Asset
	PathAssetA  horizon.Asset
	PathAssetB  horizon.Asset
	FirstPair   TradingPair
	MidPair     TradingPair
	LastPair    TradingPair
	ShouldDelay bool
}

type basicOrderBookLevel struct {
	Price  *model.Number
	Amount *model.Number
}

// String impl.
func (c ArbitCycleConfig) String() string {
	return utils.StructString(c, nil)
}

// makePaymentPath makes a payment path
func makePaymentPath(agentID int, assetA horizon.Asset, assetB horizon.Asset, holdAsset horizon.Asset) *PaymentPath {
	// first pair is selling hold for asset A, so inverted from the intuitive
	firstPair := TradingPair{
		Base:  holdAsset,
		Quote: assetA,
	}
	midPair := TradingPair{
		Base:  assetA,
		Quote: assetB,
	}
	lastPair := TradingPair{
		Base:  assetB,
		Quote: holdAsset,
	}
	return &PaymentPath{
		AgentID:     agentID,
		HoldAsset:   holdAsset,
		PathAssetA:  assetA,
		PathAssetB:  assetB,
		FirstPair:   firstPair,
		MidPair:     midPair,
		LastPair:    lastPair,
		ShouldDelay: false,
	}
}

// FindBestPath determines and returns the most profitable payment path, its max amount, and whether its good enough
func (p *PathFinder) FindBestPath() (*PaymentPath, *model.Number, bool, error) {
	bestRatio := model.NumberConstants.Zero
	maxAmount := model.NumberConstants.Zero
	metThreshold := false
	var bestPath *PaymentPath

	for i := p.assetBookMark; i < len(p.PathList); i++ {
		currentPath := p.PathList[i]
		p.assetBookMark++
		if p.assetBookMark >= len(p.PathList) {
			p.assetBookMark = 0
		}

		ratio, amount, e := p.calculatePathValues(currentPath)
		if e != nil {
			return nil, nil, false, fmt.Errorf("Error while calculating ratios %s", e)
		}

		if ratio.AsFloat() > bestRatio.AsFloat() && amount.AsFloat() > p.minAmount.AsFloat() {
			bestRatio = ratio
			maxAmount = amount
			bestPath = currentPath
		}

		p.l.Infof("Return ratio | Cycle amount for path %v: %s -> %s - > %s -> %s was %v | %v\n", p.PathList[i].AgentID, p.endAssetDisplay, currentPath.PathAssetA.Code, currentPath.PathAssetB.Code, p.endAssetDisplay, ratio.AsFloat(), amount.AsFloat())

		if bestRatio.AsFloat() >= p.minRatio.AsFloat() {
			metThreshold = true
			p.l.Info("")
			p.l.Info("***** Minimum profit ratio was met, proceeding to payment! *****")
			p.l.Info("")
			break
		}
	}

	p.l.Info("")
	p.l.Infof("Best path was %s -> %s - > %s %s -> with return ratio of %v\n", p.endAssetDisplay, bestPath.PathAssetA.Code, bestPath.PathAssetB.Code, p.endAssetDisplay, bestRatio.AsFloat())
	return bestPath, maxAmount, metThreshold, nil
}

// calculatePathValues returns the path's best ratio and max amount at that ratio
func (p *PathFinder) calculatePathValues(path *PaymentPath) (*model.Number, *model.Number, error) {

	// first pair is selling the hold asset for asset A
	firstPairTopBidPrice, firstPairTopBidAmount, e := p.dexWatcher.GetTopBid(path.FirstPair)
	if e != nil {
		return nil, nil, fmt.Errorf("Error while calculating path ratio %s", e)
	}
	if firstPairTopBidPrice == model.NumberConstants.Zero || firstPairTopBidAmount == model.NumberConstants.Zero {
		return model.NumberConstants.Zero, model.NumberConstants.Zero, nil
	}

	// mid pair is selling asset A for asset B
	midPairTopBidPrice, midPairTopBidAmount, e := p.dexWatcher.GetTopBid(path.MidPair)
	if e != nil {
		return nil, nil, fmt.Errorf("Error while calculating path ratio %s", e)
	}
	if midPairTopBidPrice == model.NumberConstants.Zero || midPairTopBidAmount == model.NumberConstants.Zero {
		return model.NumberConstants.Zero, model.NumberConstants.Zero, nil
	}

	// last pair is selling asset B for the hold asset
	lastPairTopBidPrice, lastPairTopBidAmount, e := p.dexWatcher.GetTopBid(path.LastPair)
	if e != nil {
		return nil, nil, fmt.Errorf("Error while calculating path ratio %s", e)
	}
	if lastPairTopBidPrice == model.NumberConstants.Zero || lastPairTopBidAmount == model.NumberConstants.Zero {
		return model.NumberConstants.Zero, model.NumberConstants.Zero, nil
	}

	// initialize as zero to prevent nil pointers; shouldn't be necessary with above returns, but safer
	ratio := model.NumberConstants.Zero

	ratio = firstPairTopBidPrice.Multiply(*midPairTopBidPrice)
	ratio = ratio.Multiply(*lastPairTopBidPrice)

	// max input is just firstPairTopBidAmount
	maxCycleAmount := firstPairTopBidAmount

	//get lower of AssetA amounts
	maxAreceive := firstPairTopBidAmount.Multiply(*firstPairTopBidPrice)
	maxAsell := midPairTopBidAmount

	if maxAreceive.AsFloat() < maxAsell.AsFloat() {
		maxAsell = maxAreceive
	}

	// now get lower of AssetB amounts
	// the most of AssetB you can get is the top bid of the mid pair*mid pair price
	maxBreceive := maxAsell.Multiply(*midPairTopBidPrice)
	maxBsell := lastPairTopBidAmount

	if maxBreceive.AsFloat() < maxBsell.AsFloat() {
		maxBsell = maxBreceive
	}

	// maxLastReceive is maxBsell*last pair price
	maxLastReceive := maxBsell.Multiply(*lastPairTopBidPrice)

	if maxLastReceive.AsFloat() < maxCycleAmount.AsFloat() {
		maxCycleAmount = maxLastReceive
	}

	return ratio, maxCycleAmount, nil
}

// PathChecker checks path ratios for a single path, triggered by orderbook stream returns
func (p *PathFinder) PathChecker(rank int, pathJobs <-chan *PaymentPath, transJobs chan<- *TransData, stop <-chan bool) {
	for {
		select {
		case j := <-pathJobs:
			p.l.Infof("checking path %v", j.AgentID)
			amount, metThreshold, e := p.checkOnePath(j)
			if e != nil {
				p.l.Errorf("error checking path %s", e)
			}
			if metThreshold {
				transJobs <- &TransData{j, amount}
			}
			// this timer prevents useless simultaneous rechecks
			resetTimer := time.NewTimer(3 * time.Second)
			p.l.Infof("delayed resend of path %v", j.AgentID)
			<-resetTimer.C
			p.l.Infof("released path %v", j.AgentID)
			j.ShouldDelay = false
		case <-stop:
			return
		}
	}
}

// checkOnePath does the work of a PathChecker
func (p *PathFinder) checkOnePath(path *PaymentPath) (*model.Number, bool, error) {
	metThreshold := false

	ratio, amount, e := p.calculatePathValues(path)
	if e != nil {
		return nil, false, fmt.Errorf("Error while calculating ratios %s", e)
	}

	p.l.Infof("Return ratio | Cycle amount for path %s -> %s - > %s -> %s was %v | %v\n", p.endAssetDisplay, path.PathAssetA.Code, path.PathAssetB.Code, p.endAssetDisplay, ratio.AsFloat(), amount.AsFloat())

	if ratio.AsFloat() >= p.minRatio.AsFloat() {
		metThreshold = true
		p.l.Info("")
		p.l.Info("***** Minimum profit ratio was met, proceeding to payment! *****")
		p.l.Info("")
	}

	return amount, metThreshold, nil
}

// WhatRatio returns the minimum ratio
func (p *PathFinder) WhatRatio() *model.Number {
	return p.minRatio
}

// WhatAmount returns the payment amount settings
func (p *PathFinder) WhatAmount() (bool, *model.Number) {
	return p.useBalance, p.staticAmount
}
