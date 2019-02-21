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
	pathReturn   chan<- PathFindOutcome
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
	pathReturn chan<- PathFindOutcome,
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
	// then add the HoldAsset pairs, both directions
	for i := 0; i < len(assetBook); i++ {
		rawPairBook = append(rawPairBook, TradingPair{Base: assetBook[i].Asset, Quote: holdAsset})
		rawPairBook = append(rawPairBook, TradingPair{Base: holdAsset, Quote: assetBook[i].Asset})
	}

	encountered := make(map[TradingPair]bool)
	pairBook := make(map[int]TradingPair)

	//not going to consider opposite direction pairs as duplicates, can change it later
	for v := range rawPairBook {
		if !encountered[rawPairBook[v]] {
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
		pathReturn:      pathReturn,
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
	PairID int
	Pair   TradingPair
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
			path := PathPair{PairID: id, Pair: b}
			pathSequence = append(pathSequence, path)
			break
		}
	}

	for id, b := range pairBook {
		if assetA == b.Base && assetB == b.Quote {
			path := PathPair{PairID: id, Pair: b}
			pathSequence = append(pathSequence, path)
			break
		}
	}

	for id, b := range pairBook {
		if assetB == b.Base && holdAsset == b.Quote {
			path := PathPair{PairID: id, Pair: b}
			pathSequence = append(pathSequence, path)
			break
		}
	}

	// fmt.Println("")
	// fmt.Printf("holdAsset = %v\n", holdAsset)
	// fmt.Printf("assetA = %v\n", assetA)
	// fmt.Printf("assetB = %v\n", assetB)
	// fmt.Printf("pathSequence generated at %v\n", pathSequence)

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

// PathFindOutcome sends the outcome of the find-best-path
type PathFindOutcome struct {
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
		for {
			b, more := <-bidResults
			bidSet = append(bidSet, b)
			if !more {
				return
			}

		}
		// }
	}()

	var wgOne sync.WaitGroup

	// wgCount := 0
	for id, b := range p.PairBook {
		wgOne.Add(1)
		// wgCount++
		// p.l.Infof("wgOne routines: %v", wgCount)
		go func(id int, pair TradingPair) {
			defer wgOne.Done()
			topBidPrice, topBidAmount, e := p.dexWatcher.GetTopBid(pair)
			if e != nil {
				p.l.Errorf("Error getting orderbook %s", e)
				return
			}
			bidResults <- bidResult{PathID: id, Price: topBidPrice, Amount: topBidAmount}
			// p.l.Infof("returned a bid result for pair %v: %v|%v", id, pair.Base, pair.Quote)
		}(id, b)
		// break
	}

	wgOne.Wait()
	// let the append finish happening
	time.Sleep(time.Millisecond)
	close(bidResults)
	// for i := 0; i < len(p.PairBook); i++ {
	// 	time.Sleep(100 * time.Millisecond)
	// 	p.l.Infof("Have %v records in bidSet", len(bidSet))
	// }

	p.l.Infof("got %v bid records from input of %v pairs", len(bidSet), len(p.PairBook))

	go func() {
		for {
			r, more := <-pathResults
			pathSet = append(pathSet, r)
			if !more {
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
			pathResults <- pathResult{Path: path, Ratio: ratio, Amount: amount}
		}(p.PathList[i], bidSet)
		// break
	}

	wgTwo.Wait()
	time.Sleep(time.Millisecond)
	close(pathResults)

	for _, r := range pathSet {
		if r.Ratio.AsFloat() > bestRatio.AsFloat() {
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

	p.pathReturn <- PathFindOutcome{BestPath: bestPath, MaxAmount: maxAmount, MetThreshold: metThreshold}
}

func (p *PathFinder) calculatePathValues(path *PaymentPath, bids []bidResult) (*model.Number, *model.Number, error) {
	var pathBids []bidResult

	// for i, r := range path.PathSequence {
	// 	p.l.Infof("Path %v: %v\n", i, r)
	// }

	for i := 0; i < len(path.PathSequence); i++ {
		for _, r := range bids {
			if r.PathID == path.PathSequence[i].PairID {
				pathBids = append(pathBids, r)
				break
			}
		}
	}

	// for i, r := range pathBids {
	// 	p.l.Infof("Bid %v: %v\n", i, r)
	// }

	if len(pathBids) != len(path.PathSequence) {
		return nil, nil, fmt.Errorf("couldn't match bids (len%v) with path sequence len(%v)", len(pathBids), len(path.PathSequence))
	}

	ratio := pathBids[0].Price
	for i := 1; i < len(pathBids); i++ {
		ratio = ratio.Multiply(*pathBids[i].Price)
	}

	// max input is just firstPairTopBidAmount
	maxCycleAmount := pathBids[0].Amount

	// get lower of AssetA amounts
	// this isn't part of the loop because it's never reduced by a previous step
	maxAreceive := pathBids[0].Amount.Multiply(*pathBids[0].Price)
	maxAsell := pathBids[1].Amount

	if maxAreceive.AsFloat() < maxAsell.AsFloat() {
		maxAsell = maxAreceive
	}

	lastMaxReceive := maxAsell
	// now get lower of AssetB amounts (now looped for all mid assets)

	for i := 2; i < len(path.PathSequence)-1; i++ {
		maxReceive := lastMaxReceive.Multiply(*pathBids[i-1].Price)
		maxSell := pathBids[i].Amount

		if maxReceive.AsFloat() < maxSell.AsFloat() {
			maxSell = maxReceive

			lastMaxReceive = maxSell
		}
	}

	if lastMaxReceive.AsFloat() < maxCycleAmount.AsFloat() {
		maxCycleAmount = lastMaxReceive
	}

	p.l.Infof("Path %s -> %s - > %s -> %s had return ratio of %v\n", p.endAssetDisplay, path.PathAssetA.Code, path.PathAssetB.Code, p.endAssetDisplay, ratio.AsFloat())

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
