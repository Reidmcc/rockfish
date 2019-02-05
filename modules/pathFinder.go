package modules

import (
	"fmt"

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
	assetBook    []groupedAsset
	pathList     []PaymentPath
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

	var pathList []PaymentPath

	endAssetDisplay := holdAsset.Code //this is just so XLM doesn't show up blank

	if utils.Asset2Asset(holdAsset) == build.NativeAsset() {
		endAssetDisplay = "XLM"
	}

	l.Info("generating path list: ")

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i].Asset != assetBook[n].Asset && assetBook[i].Group == assetBook[n].Group {
				path := makePaymentPath(assetBook[i].Asset, assetBook[n].Asset, holdAsset)
				l.Infof("added path: %s -> %s | %s -> %s | %s -> %s", endAssetDisplay, assetBook[i].Asset.Code, assetBook[i].Asset.Issuer, assetBook[n].Asset.Code, assetBook[n].Asset.Issuer, endAssetDisplay)
				pathList = append(pathList, path)
			}
		}
	}

	// assetBookMark ensures we don't get stuck on one asset
	assetBookMark := 0

	return &PathFinder{
		dexWatcher:      dexWatcher,
		HoldAsset:       holdAsset,
		assetBook:       assetBook,
		pathList:        pathList,
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
// probably don't need separate PriceFeed concepts if the tradingPair is here
type PaymentPath struct {
	HoldAsset  horizon.Asset
	PathAssetA horizon.Asset
	PathAssetB horizon.Asset
	FirstPair  TradingPair
	MidPair    TradingPair
	LastPair   TradingPair
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
func makePaymentPath(assetA horizon.Asset, assetB horizon.Asset, holdAsset horizon.Asset) PaymentPath {
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
func (p *PathFinder) FindBestPath() (*PaymentPath, *model.Number, bool, error) {
	bestRatio := model.NumberConstants.Zero
	maxAmount := model.NumberConstants.Zero
	metThreshold := false
	var bestPath PaymentPath

	for i := p.assetBookMark; i < len(p.pathList); i++ {
		currentPath := p.pathList[i]
		p.assetBookMark++
		if p.assetBookMark >= len(p.pathList) {
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

		p.l.Infof("Return ratio | Cycle amount for path %s -> %s - > %s -> %s was %v | %v\n", p.endAssetDisplay, currentPath.PathAssetA.Code, currentPath.PathAssetB.Code, p.endAssetDisplay, ratio.AsFloat(), amount.AsFloat())

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
	return &bestPath, maxAmount, metThreshold, nil
}

// calculatePathValues returns the path's best ratio and max amount at that ratio
func (p *PathFinder) calculatePathValues(path PaymentPath) (*model.Number, *model.Number, error) {
	// first pair is buying asset A with the hold asset
	// switch to selling direction, which means switch first pair factory order
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

// AnalysePaths asks horizon about available paths and checks if any are profitable
func (p *PathFinder) AnalysePaths() (*PathRecord, *model.Number, bool, error) {
	metThreshold := false
	var bestPath PathRecord
	var bestCost *model.Number
	//var amount *model.Number
	var pathOutput *model.Number

	for i := p.assetBookMark; i < len(p.assetBook); i++ {
		currentAsset := p.assetBook[i]
		p.assetBookMark++
		if p.assetBookMark >= len(p.assetBook) {
			p.assetBookMark = 0
		}
		pair := TradingPair{
			Base:  currentAsset.Asset,
			Quote: p.HoldAsset,
		}

		bidPrice, _, e := p.dexWatcher.GetTopBid(pair)
		if e != nil {
			return nil, nil, false, fmt.Errorf("Error while analysing paths %s", e)
		}

		amountConversion := model.NumberConstants.One.Divide(*bidPrice)
		// if we want to keep useBalance we have to put a balance call here
		minConvertedAmount := amountConversion.Multiply(*p.staticAmount)
		p.l.Info("")
		// p.l.Infof("Amount ratio calced at %v\n", amountConversion.AsFloat())
		// p.l.Infof("Converted minAmount calced at %v\n", minConvertedAmount.AsFloat())
		// p.l.Info("")

		// now find paths where the desired amount goes through
		pathData, e := p.dexWatcher.GetPaths(currentAsset.Asset, minConvertedAmount)
		if e != nil {
			return nil, nil, false, fmt.Errorf("error while analysing paths %s", e)
		}

		var intermediatePaths []PathRecord

		for _, s := range pathData.Embedded.Records {
			//p.l.Infof("checking path %v+, witch has a path struct of length %v", s, len(s.Path))
			if len(s.Path) > 0 {
				//p.l.Info("added the checked path to the candidate list")
				intermediatePaths = append(intermediatePaths, s)
			}
		}

		if len(intermediatePaths) == 0 {
			p.l.Infof("No paths with intermediaries found for %s|%s", currentAsset.Asset.Code, currentAsset.Asset.Issuer)
			continue
		}

		//p.l.Infof("Found intermediate paths: %+v", intermediatePaths)

		for i := 0; i < len(intermediatePaths); i++ {
			if i == 0 {
				bestPath = intermediatePaths[i]
				bestCost, e = model.NumberFromString(intermediatePaths[i].SourceAmount, utils.SdexPrecision)
				if e != nil {
					return nil, nil, false, fmt.Errorf("error while analysing paths %s", e)
				}
				continue
			}

			pathCost, e := model.NumberFromString(intermediatePaths[i].SourceAmount, utils.SdexPrecision)
			if e != nil {
				return nil, nil, false, fmt.Errorf("error while analysing paths %s", e)
			}

			if pathCost.AsFloat() < bestCost.AsFloat() {
				bestPath = intermediatePaths[i]
				bestCost = pathCost
			}
		}

		// now see if output is greater than input; if not go to the next asset
		destAmount, e := model.NumberFromString(bestPath.DestinationAmount, utils.SdexPrecision)
		if e != nil {
			return nil, nil, false, fmt.Errorf("error while analysing paths %s", e)
		}
		pathOutput := destAmount.Multiply(*bidPrice)
		pathRatio := pathOutput.Divide(*bestCost)
		p.l.Infof("Best path amount output for %s|%s was %v for %v", currentAsset.Asset.Code, currentAsset.Asset.Issuer, pathOutput.AsFloat(), bestCost.AsFloat())
		p.l.Infof("With a ratio if %v", pathRatio.AsFloat())
		//p.l.Infof("Raw best cost was %v ; raw converted amount was was %v", bestCost.AsFloat(), minConvertedAmount.AsFloat())

		if pathOutput.Scale(1.0/p.minRatio.AsFloat()).AsFloat() > bestCost.AsFloat() && len(bestPath.Path) > 0 {
			metThreshold = true
			p.l.Info("")
			p.l.Info("***** Minimum profit ratio was met, proceeding to payment! *****")
			p.l.Info("")
			// trying a method that doesn't analyst path throughput, just uses bestCost
			// should be faster and more accurate
			//amount, e := p.findMaxAmount(bestPath)
			if e != nil {
				return nil, nil, false, fmt.Errorf("error while analysing paths %s", e)
			}
			return &bestPath, pathOutput, metThreshold, nil
		}

	}
	return &bestPath, pathOutput, false, nil
}

// findMaxAmount finds the amount for the AnalysePaths functions
// this likely needs math edits due to bid amount inversion
func (p *PathFinder) findMaxAmount(sendPath PathRecord) (*model.Number, error) {
	var pathPairs []TradingPair
	var pair TradingPair

	for i := 0; i < len(sendPath.Path); i++ {
		if i == 0 {
			pair = TradingPair{
				Base:  p.HoldAsset,
				Quote: PathAsset2Asset(sendPath.Path[i]),
			}
			continue
		}

		pair = TradingPair{
			Base:  PathAsset2Asset(sendPath.Path[i-1]),
			Quote: PathAsset2Asset(sendPath.Path[i]),
		}

		pathPairs = append(pathPairs, pair)
	}

	destAsset := ParseAsset(sendPath.DestinationAssetCode, sendPath.DestinationAssetIssuer)

	lastPair := TradingPair{
		Base:  PathAsset2Asset(sendPath.Path[len(sendPath.Path)-1]),
		Quote: destAsset,
	}

	postPair := TradingPair{
		Base:  destAsset,
		Quote: p.HoldAsset,
	}

	pathPairs = append(pathPairs, lastPair)
	pathPairs = append(pathPairs, postPair)

	for _, d := range pathPairs {
		p.l.Infof("found path asset: %s", d)
	}

	var bidSeries []basicOrderBookLevel

	for _, r := range pathPairs {
		topBidPrice, topBidAmount, e := p.dexWatcher.GetTopBid(r)
		if e != nil {
			return nil, fmt.Errorf("Error while calculating path amount %s", e)
		}
		bidSeries = append(
			bidSeries,
			basicOrderBookLevel{
				Price:  topBidPrice,
				Amount: topBidAmount,
			},
		)
	}

	if len(bidSeries) <= 1 {
		return nil, fmt.Errorf("not enough pairs in path, or failed get pair orderbooks")
	}

	p.l.Infof("generated bidSeries of %v+", bidSeries)

	maxInput := bidSeries[0].Amount
	inAmount := bidSeries[0].Amount.Multiply(*bidSeries[0].Price)

	for i := 1; i < len(bidSeries); i++ {
		outAmount := bidSeries[i].Amount
		if inAmount.AsFloat() < outAmount.AsFloat() {
			outAmount = inAmount
		}
		inAmount = outAmount.Multiply(*bidSeries[i].Price)
	}

	maxOutput := inAmount

	if maxInput.AsFloat() < maxOutput.AsFloat() {
		maxOutput = maxInput
	}

	if p.useBalance == false && p.staticAmount.AsFloat() < maxOutput.AsFloat() {
		maxOutput = p.staticAmount
	}

	return maxOutput, nil
}

// WhatRatio returns the minimum ratio
func (p *PathFinder) WhatRatio() *model.Number {
	return p.minRatio
}

// WhatAmount returns the payment amount settings
func (p *PathFinder) WhatAmount() (bool, *model.Number) {
	return p.useBalance, p.staticAmount
}
