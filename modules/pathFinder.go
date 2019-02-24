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
	refresh      <-chan bool
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
	refresh <-chan bool,
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
		refresh:         refresh,
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

	// find the pairs from the book
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

// FindBestPathConcurrent determines and returns the most profitable payment path with goroutines
func (p *PathFinder) FindBestPathConcurrent() {
	bestRatio := model.NumberConstants.Zero
	maxAmount := model.NumberConstants.Zero
	foundAnyRoute := false
	metThreshold := false
	var bestPath *PaymentPath
	var bidSet []bidResult
	var pathSet []pathResult
	pathResults := make(chan pathResult, len(p.PathList))
	bidResults := make(chan bidResult, len(p.PairBook))
	var wgOne sync.WaitGroup
	var wgTwo sync.WaitGroup

	select {
	case <-p.refresh:
		return
	case <-p.findIt:

		go func() {
			for {
				b, more := <-bidResults
				bidSet = append(bidSet, b)
				if !more {
					return
				}

			}
		}()

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
		// let the append finish happening
		time.Sleep(time.Millisecond)
		close(bidResults)

		go func() {
			for {
				r, more := <-pathResults
				pathSet = append(pathSet, r)
				if !more {
					return
				}
			}
		}()

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
		}

		wgTwo.Wait()
		time.Sleep(time.Millisecond)
		close(pathResults)

		for _, r := range pathSet {
			if r.Ratio.AsFloat() > bestRatio.AsFloat() {
				foundAnyRoute = true
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
		if foundAnyRoute {
			p.l.Infof("Best path was %s -> %s -> %s %s -> with return ratio of %v\n", p.endAssetDisplay, bestPath.PathAssetA.Code, bestPath.PathAssetB.Code, p.endAssetDisplay, bestRatio.AsFloat())
			p.l.Info("")
		} else {
			p.l.Info("No usable route found")
		}

		p.pathReturn <- PathFindOutcome{BestPath: bestPath, MaxAmount: maxAmount, MetThreshold: metThreshold}
	}
}

func (p *PathFinder) calculatePathValues(path *PaymentPath, bids []bidResult) (*model.Number, *model.Number, error) {
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
		return nil, nil, fmt.Errorf("couldn't match bids (len%v) with path sequence len(%v)", len(pathBids), len(path.PathSequence))
	}

	ratio := pathBids[0].Price
	for i := 1; i < len(pathBids); i++ {
		ratio = ratio.Multiply(*pathBids[i].Price)
	}

	// max input is just firstPairTopBidAmount
	maxCycleAmount := pathBids[0].Amount

	// get lower of AssetA amounts
	maxAreceive := pathBids[0].Amount.Multiply(*pathBids[0].Price)
	maxAsell := pathBids[1].Amount

	if maxAreceive.AsFloat() < maxAsell.AsFloat() {
		maxAsell = maxAreceive
	}

	maxBreceive := maxAsell.Multiply(*pathBids[1].Price)
	maxBsell := pathBids[2].Amount

	if maxBreceive.AsFloat() < maxBsell.AsFloat() {
		maxBsell = maxBreceive
	}

	// maxLastReceive is maxBsell*last pair price
	maxLastReceive := maxBsell.Multiply(*pathBids[2].Price)

	if maxLastReceive.AsFloat() < maxCycleAmount.AsFloat() {
		maxCycleAmount = maxLastReceive
	}

	ratioDisplay := ratio.AsString()
	if ratio.AsFloat() == 0.0 {
		ratioDisplay = "route empty"
	}

	// p.l.Infof("Path %s -> %s - > %s -> %s had return ratio of %v\n", p.endAssetDisplay, path.PathAssetA.Code, path.PathAssetB.Code, p.endAssetDisplay, ratio.AsFloat())
	p.l.Infof("Return ratio | Cycle amount for path %s -> %s -> %s -> %s was %s | %v\n", p.endAssetDisplay, path.PathAssetA.Code, path.PathAssetB.Code, p.endAssetDisplay, ratioDisplay, maxCycleAmount.AsFloat())

	return ratio, maxCycleAmount, nil
}

// WhatRatio returns the minimum ratio
func (p *PathFinder) WhatRatio() *model.Number {
	return p.minRatio
}

// WhatAmount returns the payment amount settings
func (p *PathFinder) WhatAmount() (bool, *model.Number) {
	return p.useBalance, p.staticAmount
}
