package modules

import (
	"fmt"
	"sync"
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
	PairBook     map[int]TradingPair
	minRatio     *model.Number
	useBalance   bool
	staticAmount *model.Number
	minAmount    *model.Number
	findIt       chan bool
	pathReturn   chan<- FindOutcome
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
	findIt chan bool,
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

	endAssetDisplay := holdAsset.Code //this is just so XLM doesn't show up blank

	if utils.Asset2Asset(holdAsset) == build.NativeAsset() {
		endAssetDisplay = "XLM"
	}

	var rawPairBook []TradingPair

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i].Asset != assetBook[n].Asset && assetBook[i].Group == assetBook[n].Group {
				rawPairBook = append(rawPairBook, TradingPair{Base: assetBook[i].Asset, Quote: assetBook[n].Asset})
			}
		}
	}

	encountered := map[TradingPair]bool{}
	var pairBook map[int]TradingPair

	for v := range rawPairBook {
		if !encountered[rawPairBook[v]] && !encountered[TradingPair{Base: rawPairBook[v].Quote, Quote: rawPairBook[v].Base}] {
			encountered[rawPairBook[v]] = true
			pairBook[v] = rawPairBook[v]
		}
	}

	var pathList []*PaymentPath
	l.Info("generating path list: ")

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i].Asset != assetBook[n].Asset && assetBook[i].Group == assetBook[n].Group {
				path, e := makePaymentPath(assetBook[i].Asset, assetBook[n].Asset, holdAsset, pairBook)
				if e != nil {
					logger.Fatal(l, e)
				}
				l.Infof("added path: %s -> %s | %s -> %s | %s -> %s", endAssetDisplay, assetBook[i].Asset.Code, assetBook[i].Asset.Issuer, assetBook[n].Asset.Code, assetBook[n].Asset.Issuer, endAssetDisplay)
				pathList = append(pathList, path)
			}
		}
	}

	return &PathFinder{
		dexWatcher:      dexWatcher,
		HoldAsset:       holdAsset,
		AssetBook:       assetBook,
		PathList:        pathList,
		PairBook:        pairBook,
		minRatio:        model.NumberFromFloat(stratConfig.MinRatio, utils.SdexPrecision),
		useBalance:      stratConfig.UseBalance,
		staticAmount:    model.NumberFromFloat(stratConfig.StaticAmount, utils.SdexPrecision),
		minAmount:       model.NumberFromFloat(stratConfig.MinAmount, utils.SdexPrecision),
		findIt:          findIt,
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
type PaymentPath struct {
	HoldAsset    horizon.Asset
	PathAssetA   horizon.Asset
	PathAssetB   horizon.Asset
	PathSequence []PathPair
}

// PathPair is a trading pair with a price inversion flag
type PathPair struct {
	PairID     int
	Pair       TradingPair
	InvertFlag bool
}

// BasicOrderBookLevel is just a price and an amount
type BasicOrderBookLevel struct {
	Price  *model.Number
	Amount *model.Number
}

// String impl.
func (c ArbitCycleConfig) String() string {
	return utils.StructString(c, nil)
}

// makePaymentPath makes a payment path
func makePaymentPath(
	assetA horizon.Asset,
	assetB horizon.Asset,
	holdAsset horizon.Asset,
	pairBook map[int]TradingPair) (*PaymentPath, error) {
	var pathSequence []PathPair

	// find the pairs from the book and whether they should be inverted
	for id, b := range pairBook {
		if holdAsset == b.Base && assetA == b.Quote {
			path := PathPair{PairID: id, Pair: b, InvertFlag: false}
			pathSequence = append(pathSequence, path)
			break
		}
		if holdAsset == b.Quote && assetA == b.Base {
			path := PathPair{PairID: id, Pair: b, InvertFlag: true}
			pathSequence = append(pathSequence, path)
			break
		}
	}

	for id, b := range pairBook {
		if assetA == b.Base && assetB == b.Quote {
			path := PathPair{PairID: id, Pair: b, InvertFlag: false}
			pathSequence = append(pathSequence, path)
			break
		}
		if assetB == b.Quote && assetA == b.Base {
			path := PathPair{PairID: id, Pair: b, InvertFlag: true}
			pathSequence = append(pathSequence, path)
			break
		}
	}

	for id, b := range pairBook {
		if assetB == b.Base && holdAsset == b.Quote {
			path := PathPair{PairID: id, Pair: b, InvertFlag: false}
			pathSequence = append(pathSequence, path)
			break
		}
		if holdAsset == b.Quote && assetB == b.Base {
			path := PathPair{PairID: id, Pair: b, InvertFlag: true}
			pathSequence = append(pathSequence, path)
			break
		}
	}

	if len(pathSequence) != 3 {
		return nil, fmt.Errorf("can't find the trading pairs for a path; abort")
	}

	return &PaymentPath{
		HoldAsset:    holdAsset,
		PathAssetA:   assetA,
		PathAssetB:   assetB,
		PathSequence: pathSequence,
	}, nil
}

type pathResult struct {
	Path   *PaymentPath
	Ratio  *model.Number
	Amount *model.Number
}

type bidResult struct {
	PathID int
	Price  *model.Number
	Amount *model.Number
}

// FindOutcome sends the outcome of the find-best-path
type FindOutcome struct {
	BestPath     *PaymentPath
	MaxAmount    *model.Number
	MetThreshold bool
}

// FindBestPathConcurrent determines and returns the most profitable payment path with goroutines
func (p *PathFinder) FindBestPathConcurrent() {
	bestRatio := model.NumberConstants.Zero
	maxAmount := model.NumberConstants.Zero
	metThreshold := false
	var bestPath *PaymentPath
	var bidSet []bidResult
	var pathSet []pathResult
	pathResults := make(chan pathResult, len(p.PathList))
	bidResults := make(chan bidResult, len(p.PairBook))

	<-p.findIt

	go func() {
		timer := time.NewTimer(10 * time.Second)
		for range bidResults {
			select {
			case b := <-bidResults:
				bidSet = append(bidSet, b)
			case <-timer.C:
				return
			}
		}
	}()

	var wgOne sync.WaitGroup

	for id, b := range p.PairBook {
		wgOne.Add(1)
		go func(id int, pair TradingPair) {
			defer wgOne.Done()
			topBidPrice, topBidAmount, e := p.dexWatcher.GetTopBid(pair)
			if e != nil {
				p.l.Errorf("Error getting orderbook %s", e)
				return
			}
			bidResults <- bidResult{PathID: id, Price: topBidPrice, Amount: topBidAmount}
		}(id, b)
	}

	wgOne.Wait()
	close(bidResults)

	go func() {
		timer := time.NewTimer(10 * time.Second)
		for range pathResults {
			select {
			case r := <-pathResults:
				pathSet = append(pathSet, r)
			case <-timer.C:
				return
			}
		}
	}()

	var wgTwo sync.WaitGroup

	for i := 0; i < len(p.PathList); i++ {
		wgTwo.Add(1)
		go func(path *PaymentPath, bids []bidResult) {
			defer wgTwo.Done()
			ratio, amount, e := p.calculatePathValues(path, bids)
			if e != nil {
				p.l.Errorf("error while calculating ratios: %s", e)
				return
			}
			pathResults <- pathResult{Ratio: ratio, Amount: amount}
		}(p.PathList[i], bidSet)
	}

	wgTwo.Wait()
	close(pathResults)

	for _, r := range pathSet {
		if r.Ratio.AsFloat() > bestRatio.AsFloat() && r.Amount.AsFloat() > p.minAmount.AsFloat() {
			bestRatio = r.Ratio
			maxAmount = r.Amount
			bestPath = r.Path
		}
	}

	if bestRatio.AsFloat() >= p.minRatio.AsFloat() && maxAmount.AsFloat() >= p.minAmount.AsFloat() {
		metThreshold = true
		p.l.Info("")
		p.l.Info("***** Minimum profit ratio was met, proceeding to payment! *****")
		p.l.Info("")
	}
	p.l.Infof("Best path was %s -> %s - > %s %s -> with return ratio of %v\n", p.endAssetDisplay, bestPath.PathAssetA.Code, bestPath.PathAssetB.Code, p.endAssetDisplay, bestRatio.AsFloat())
	p.l.Info("")

	p.pathReturn <- FindOutcome{BestPath: bestPath, MaxAmount: maxAmount, MetThreshold: metThreshold}
}

func (p *PathFinder) calculatePathValues(path *PaymentPath, bids []bidResult) (*model.Number, *model.Number, error) {
	var prices []*model.Number
	var amounts []*model.Number
	var pathBids []bidResult

	for i := 0; i < len(path.PathSequence); i++ {
		for _, r := range bids {
			if r.PathID == path.PathSequence[i].PairID {
				pathBids = append(pathBids, r)
				break
			}
		}
	}

	if len(pathBids) != len(path.PathSequence) {
		return nil, nil, fmt.Errorf("couldn't match bids with path sequence")
	}

	for i := 0; i < len(path.PathSequence); i++ {
		topBidPrice := pathBids[i].Price
		topBidAmount := pathBids[i].Amount

		if path.PathSequence[i].InvertFlag {
			topBidPrice = model.NumberFromFloat((1 / topBidPrice.AsFloat()), utils.SdexPrecision)
			topBidAmount = model.NumberFromFloat((1 / topBidAmount.AsFloat()), utils.SdexPrecision)
		}
	}

	var ratio *model.Number

	for i := 0; i < len(path.PathSequence)-1; i++ {
		ratio = prices[i].Multiply(*prices[i+1])
	}

	// max input is just firstPairTopBidAmount
	maxCycleAmount := amounts[0]

	// get lower of AssetA amounts
	// this isn't part of the loop because it's never reduced by a previous step
	maxAreceive := amounts[0].Multiply(*prices[0])
	maxAsell := amounts[1]

	if maxAreceive.AsFloat() < maxAsell.AsFloat() {
		maxAsell = maxAreceive
	}

	lastMaxReceive := maxAsell
	// now get lower of AssetB amounts (now looped for all mid assets)

	for i := 2; i < len(path.PathSequence)-1; i++ {
		maxReceive := lastMaxReceive.Multiply(*prices[i-1])
		maxSell := amounts[i]

		if maxReceive.AsFloat() < maxSell.AsFloat() {
			maxSell = maxReceive

			lastMaxReceive = maxSell
		}
	}

	if lastMaxReceive.AsFloat() < maxCycleAmount.AsFloat() {
		maxCycleAmount = lastMaxReceive
	}

	return ratio, maxCycleAmount, nil
}

// calculatePathValues returns the path's best ratio and max amount at that ratio
// func (p *PathFinder) calculatePathValuesWithBookCall(path *PaymentPath) (*model.Number, *model.Number, error) {

// 	// first pair is selling the hold asset for asset A
// 	firstPairTopBidPrice, firstPairTopBidAmount, e := p.dexWatcher.GetTopBid(path.FirstPair)
// 	if e != nil {
// 		return nil, nil, fmt.Errorf("Error while calculating path ratio %s", e)
// 	}
// 	if firstPairTopBidPrice == model.NumberConstants.Zero || firstPairTopBidAmount == model.NumberConstants.Zero {
// 		return model.NumberConstants.Zero, model.NumberConstants.Zero, nil
// 	}

// 	// mid pair is selling asset A for asset B
// 	midPairTopBidPrice, midPairTopBidAmount, e := p.dexWatcher.GetTopBid(path.MidPair)
// 	if e != nil {
// 		return nil, nil, fmt.Errorf("Error while calculating path ratio %s", e)
// 	}
// 	if midPairTopBidPrice == model.NumberConstants.Zero || midPairTopBidAmount == model.NumberConstants.Zero {
// 		return model.NumberConstants.Zero, model.NumberConstants.Zero, nil
// 	}

// 	// last pair is selling asset B for the hold asset
// 	lastPairTopBidPrice, lastPairTopBidAmount, e := p.dexWatcher.GetTopBid(path.LastPair)
// 	if e != nil {
// 		return nil, nil, fmt.Errorf("Error while calculating path ratio %s", e)
// 	}
// 	if lastPairTopBidPrice == model.NumberConstants.Zero || lastPairTopBidAmount == model.NumberConstants.Zero {
// 		return model.NumberConstants.Zero, model.NumberConstants.Zero, nil
// 	}

// 	// initialize as zero to prevent nil pointers; shouldn't be necessary with above returns, but safer
// 	ratio := model.NumberConstants.Zero

// 	ratio = firstPairTopBidPrice.Multiply(*midPairTopBidPrice)
// 	ratio = ratio.Multiply(*lastPairTopBidPrice)

// 	// max input is just firstPairTopBidAmount
// 	maxCycleAmount := firstPairTopBidAmount

// 	//get lower of AssetA amounts
// 	maxAreceive := firstPairTopBidAmount.Multiply(*firstPairTopBidPrice)
// 	maxAsell := midPairTopBidAmount

// 	if maxAreceive.AsFloat() < maxAsell.AsFloat() {
// 		maxAsell = maxAreceive
// 	}

// 	// now get lower of AssetB amounts
// 	// the most of AssetB you can get is the top bid of the mid pair*mid pair price
// 	maxBreceive := maxAsell.Multiply(*midPairTopBidPrice)
// 	maxBsell := lastPairTopBidAmount

// 	if maxBreceive.AsFloat() < maxBsell.AsFloat() {
// 		maxBsell = maxBreceive
// 	}

// 	// maxLastReceive is maxBsell*last pair price
// 	maxLastReceive := maxBsell.Multiply(*lastPairTopBidPrice)

// 	if maxLastReceive.AsFloat() < maxCycleAmount.AsFloat() {
// 		maxCycleAmount = maxLastReceive
// 	}

// 	return ratio, maxCycleAmount, nil
// }

// WhatRatio returns the minimum ratio
func (p *PathFinder) WhatRatio() *model.Number {
	return p.minRatio
}

// WhatAmount returns the payment amount settings
func (p *PathFinder) WhatAmount() (bool, *model.Number) {
	return p.useBalance, p.staticAmount
}
