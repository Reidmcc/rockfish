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
	MinRatio float64      `valid:"-" toml:"MIN_RATIO"`
	Groups   []GroupInput `valid:"-" toml:"GROUPS"`
}

// GroupInput holds the inbound strings for the asset groups
type GroupInput struct {
	HoldAssetCode   string       `valid:"-" toml:"HOLD_ASSET_CODE"`
	HoldAssetIssuer string       `valid:"-" toml:"HOLD_ASSET_ISSUER"`
	UseBalance      bool         `valid:"-" toml:"USE_BALANCE"`
	StaticAmount    float64      `valid:"-" toml:"STATIC_AMOUNT"`
	MinAmount       float64      `valid:"-" toml:"MIN_AMOUNT"`
	Assets          []assetInput `valid:"-" toml:"ASSETS"`
}

// assetInput holds the inbound asset strings
type assetInput struct {
	CODE   string `valid:"-" toml:"CODE"`
	ISSUER string `valid:"-" toml:"ISSUER"`
}

// AssetGroup is an operational asset set and its settings
type AssetGroup struct {
	HoldAsset    horizon.Asset
	AssetBook    []horizon.Asset
	PathList     []*PaymentPath
	PairBook     []PairBookEntry
	useBalance   bool
	staticAmount *model.Number
	MinAmount    *model.Number
}

//
// PathFinder keeps track of all the possible payment paths
type PathFinder struct {
	dexWatcher  DexWatcher
	AssetGroups []*AssetGroup
	minRatio    *model.Number
	rateLimiter func()
	findIt      chan bool
	pathReturn  chan<- PathFindOutcome
	refresh     <-chan bool
	l           logger.Logger
}

func configGroup(groupData GroupInput, pairNum int, l logger.Logger) (*AssetGroup, int) {
	holdAsset := ParseAsset(groupData.HoldAssetCode, groupData.HoldAssetIssuer)

	var assetBook []horizon.Asset

	for _, a := range groupData.Assets {
		asset := ParseAsset(a.CODE, a.ISSUER)
		assetBook = append(assetBook, asset)
	}

	endAssetDisplay := holdAsset.Code //this is just so XLM doesn't show up blank
	if utils.Asset2Asset(holdAsset) == build.NativeAsset() {
		endAssetDisplay = "XLM"
	}

	var rawPairBook []TradingPair

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i] != assetBook[n] {
				rawPairBook = append(rawPairBook, TradingPair{Base: assetBook[i], Quote: assetBook[n]})
			}
		}
	}

	// then add the HoldAsset pairs, both directions
	for i := 0; i < len(assetBook); i++ {
		rawPairBook = append(rawPairBook, TradingPair{Base: assetBook[i], Quote: holdAsset})
		rawPairBook = append(rawPairBook, TradingPair{Base: holdAsset, Quote: assetBook[i]})
	}

	encounteredPair := make(map[TradingPair]bool)
	var pairBook []PairBookEntry

	//not going to consider opposite direction pairs as duplicates, can change it later
	for v := 0; v < len(rawPairBook); v++ {
		if !encounteredPair[rawPairBook[v]] {
			encounteredPair[rawPairBook[v]] = true
			pairBook = append(pairBook, PairBookEntry{pairNum, rawPairBook[v]})
			pairNum++
		}
	}

	var pathList []*PaymentPath
	staticAmount := model.NumberFromFloat(groupData.StaticAmount, utils.SdexPrecision)
	minAmount := model.NumberFromFloat(groupData.MinAmount, utils.SdexPrecision)

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i] != assetBook[n] {
				path, e := makePaymentPath(assetBook[i], assetBook[n], holdAsset, pairBook, groupData.UseBalance, staticAmount, minAmount)
				if e != nil {
					logger.Fatal(l, e)
				}
				l.Infof("added path: %s -> %s | %s -> %s | %s -> %s", endAssetDisplay, assetBook[i].Code, assetBook[i].Issuer, assetBook[n].Code, assetBook[n].Issuer, endAssetDisplay)
				pathList = append(pathList, path)
			}
		}
	}

	return &AssetGroup{
		HoldAsset:    holdAsset,
		AssetBook:    assetBook,
		PathList:     pathList,
		PairBook:     pairBook,
		useBalance:   groupData.UseBalance,
		staticAmount: model.NumberFromFloat(groupData.StaticAmount, utils.SdexPrecision),
		MinAmount:    model.NumberFromFloat(groupData.MinAmount, utils.SdexPrecision),
	}, pairNum
}

// MakePathFinder is a factory method
func MakePathFinder(
	dexWatcher DexWatcher,
	stratConfig ArbitCycleConfig,
	rateLimiter func(),
	findIt chan bool,
	pathReturn chan<- PathFindOutcome,
	refresh <-chan bool,
	l logger.Logger,
) (*PathFinder, error) {

	// pairNum keeps there from being duplicate pair IDs
	pairNum := 0
	var assetGroups []*AssetGroup
	for _, g := range stratConfig.Groups {
		group, pairCount := configGroup(g, pairNum, l)
		pairNum = pairCount
		assetGroups = append(assetGroups, group)
	}

	return &PathFinder{
		dexWatcher:  dexWatcher,
		AssetGroups: assetGroups,
		minRatio:    model.NumberFromFloat(stratConfig.MinRatio, utils.SdexPrecision),
		rateLimiter: rateLimiter,
		findIt:      findIt,
		pathReturn:  pathReturn,
		refresh:     refresh,
		l:           l,
	}, nil
}

// PairBookEntry is a numbered trading pair
type PairBookEntry struct {
	ID   int
	Pair TradingPair
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
	UseBalance   bool
	StaticAmount *model.Number
	MinAmount    *model.Number
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
	Path      *PaymentPath
	Ratio     *model.Number
	Amount    *model.Number
	MetAmount bool
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
	pairBook []PairBookEntry,
	useBalance bool,
	staticAmount *model.Number,
	minAmount *model.Number,
) (*PaymentPath, error) {
	var pathSequence []PathPair

	// find the pairs from the book
	for _, b := range pairBook {
		if holdAsset == b.Pair.Base && assetA == b.Pair.Quote {
			path := PathPair{PairID: b.ID, Pair: b.Pair}
			pathSequence = append(pathSequence, path)
			break
		}
	}

	for _, b := range pairBook {
		if assetA == b.Pair.Base && assetB == b.Pair.Quote {
			path := PathPair{PairID: b.ID, Pair: b.Pair}
			pathSequence = append(pathSequence, path)
			break
		}
	}

	for _, b := range pairBook {
		if assetB == b.Pair.Base && holdAsset == b.Pair.Quote {
			path := PathPair{PairID: b.ID, Pair: b.Pair}
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
		UseBalance:   useBalance,
		StaticAmount: staticAmount,
		MinAmount:    minAmount,
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
	pathResults := make(chan pathResult, 30)
	bidResults := make(chan bidResult, 30)
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

		for _, g := range p.AssetGroups {
			for _, b := range g.PairBook {
				wgOne.Add(1)
				go func(pairSet PairBookEntry) {
					defer wgOne.Done()
					topBidPrice, topBidAmount, e := p.dexWatcher.GetTopBid(pairSet.Pair)
					if e != nil {
						p.l.Errorf("Error getting orderbook %s", e)
						return
					}
					bidResults <- bidResult{PathID: pairSet.ID, Price: topBidPrice, Amount: topBidAmount}
				}(b)
				p.rateLimiter()
			}
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

		for _, g := range p.AssetGroups {
			for i := 0; i < len(g.PathList); i++ {
				wgTwo.Add(1)
				go func(path *PaymentPath, bids []bidResult) {
					defer wgTwo.Done()
					ratio, amount, metAmount, e := p.calculatePathValues(path, bids, g.MinAmount)
					if e != nil {
						p.l.Errorf("error while calculating ratios: %s", e)
						return
					}
					pathResults <- pathResult{Path: path, Ratio: ratio, Amount: amount, MetAmount: metAmount}
				}(g.PathList[i], bidSet)
				p.rateLimiter()
			}
		}

		wgTwo.Wait()
		time.Sleep(time.Millisecond)
		close(pathResults)

		for _, r := range pathSet {
			if r.Ratio.AsFloat() > bestRatio.AsFloat() {
				foundAnyRoute = true
				if r.MetAmount {
					bestRatio = r.Ratio
					maxAmount = r.Amount
					bestPath = r.Path
				}
			}
		}

		if bestRatio.AsFloat() >= p.minRatio.AsFloat() {
			metThreshold = true
			p.l.Info("")
			p.l.Info("***** Minimum profit ratio was met, proceeding to payment! *****")
			p.l.Info("")
		}
		if foundAnyRoute {
			p.l.Infof("Best path was %s -> %s -> %s %s -> with return ratio of %v\n", bestPath.HoldAsset.Code, bestPath.PathAssetA.Code, bestPath.PathAssetB.Code, bestPath.HoldAsset.Code, bestRatio.AsFloat())
			p.l.Info("")
		} else {
			p.l.Info("No usable route found")
		}

		p.pathReturn <- PathFindOutcome{BestPath: bestPath, MaxAmount: maxAmount, MetThreshold: metThreshold}
	}
}

func (p *PathFinder) calculatePathValues(path *PaymentPath, bids []bidResult, minAmount *model.Number) (*model.Number, *model.Number, bool, error) {
	var pathBids []bidResult
	metAmount := false

	for i := 0; i < len(path.PathSequence); i++ {
		for _, r := range bids {
			if r.PathID == path.PathSequence[i].PairID {
				pathBids = append(pathBids, r)
				break
			}
		}
	}

	if len(pathBids) != len(path.PathSequence) {
		return model.NumberConstants.Zero, model.NumberConstants.Zero, false, fmt.Errorf("couldn't match bids with path sequence; an orderbook call may have failed")
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

	if maxCycleAmount.AsFloat() > minAmount.AsFloat() {
		metAmount = true
	}

	ratioDisplay := ratio.AsString()
	if ratio.AsFloat() == 0.0 {
		ratioDisplay = "route empty"
	}

	endAssetDisplay := path.HoldAsset.Code //this is just so XLM doesn't show up blank
	if utils.Asset2Asset(path.HoldAsset) == build.NativeAsset() {
		endAssetDisplay = "XLM"
	}

	// p.l.Infof("Path %s -> %s - > %s -> %s had return ratio of %v\n", p.endAssetDisplay, path.PathAssetA.Code, path.PathAssetB.Code, p.endAssetDisplay, ratio.AsFloat())
	p.l.Infof("Return ratio | Cycle amount for path %s -> %s -> %s -> %s was %s | %v\n", endAssetDisplay, path.PathAssetA.Code, path.PathAssetB.Code, endAssetDisplay, ratioDisplay, maxCycleAmount.AsFloat())

	return ratio, maxCycleAmount, metAmount, nil
}
