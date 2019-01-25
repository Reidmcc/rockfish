package modules

import (
	"fmt"
	"log"

	"github.com/interstellar/kelp/plugins"
	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/config"
)

type arbitCycleConfig struct {
	HoldAssetCode   string       `valid:"-" toml:"HOLD_ASSET_CODE"`
	HoldAssetIssuer string       `valid:"-" toml:"HOLD_ASSET_ISSUER"`
	Assets          []assetInput `valid:"-" toml:"ASSETS"`
	MinRatio        float64      `valid:"-" toml:"MIN_RATIO"`
}

// PathFinder keeps track of all the possible payment paths
type PathFinder struct {
	// multiDex  MultiDex won't reside here
	SDEX      *plugins.SDEX
	HoldAsset *horizon.Asset
	AssetBook []*horizon.Asset
	PathList  []PaymentPath
	l         logger.Logger
}

// assetInput holds the inbound asset strings
type assetInput struct {
	CODE   string `valid:"-"`
	ISSUER string `valid:"-"`
}

// TradingPair represents a trading pair
type TradingPair struct {
	Base  *horizon.Asset
	Quote *horizon.Asset
}

// PaymentPath is a pair of assets for a payment path and their encoded tradingPair
// probably don't need separate PriceFeed concepts if the tradingPair is here
type PaymentPath struct {
	PathAssetA *horizon.Asset
	PathAssetB *horizon.Asset
	FirstPair  TradingPair
	MidPair    TradingPair
	LastPair   TradingPair
}

// String impl.
func (c arbitCycleConfig) String() string {
	return utils.StructString(c, nil)
}

// MakePathFinder is a factory method
func MakePathFinder(
	SDEX *plugins.SDEX,
	stratConfigPath string,
	simMode bool,
) (*PathFinder, error) {
	l := logger.MakeBasicLogger()
	if stratConfigPath == "" {
		return nil, fmt.Errorf("arbitcycle mode needs a config file")
	}

	var cfg arbitCycleConfig
	err := config.Read(stratConfigPath, &cfg)
	utils.CheckConfigError(cfg, err, stratConfigPath)
	utils.LogConfig(cfg)

	holdAsset, e := ParseAsset(cfg.HoldAssetCode, cfg.HoldAssetIssuer)
	if e != nil {
		return nil, fmt.Errorf("Error while generating holdAsset: %s", e)
	}

	var assetBook []*horizon.Asset

	for _, a := range cfg.Assets {
		asset, e := ParseAsset(a.CODE, a.ISSUER)
		if e != nil {
			return nil, fmt.Errorf("Error while generating assetBook %s", e)
		}
		assetBook = append(assetBook, asset)
	}

	var pathList []PaymentPath

	l.Info("generating path list: ")

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i] != assetBook[n] {
				var path PaymentPath
				path = MakePaymentPath(assetBook[i], assetBook[n])
				l.Infof("added path: %s -> %s -> %s -> %s", holdAsset, assetBook[i], assetBook[n], holdAsset)
				pathList = append(pathList, path)
			}
		}
	}

	return &PathFinder{
		SDEX:      SDEX,
		HoldAsset: holdAsset,
		AssetBook: assetBook,
		PathList:  pathList,
		l:         l,
	}, nil
}

// MakePaymentPath makes a payment path
func (p *PathFinder) MakePaymentPath(assetA *horizon.Asset, assetB *horizon.Asset) PaymentPath {
	firstPair := TradingPair{
		Base:  assetA,
		Quote: p.HoldAsset,
	}
	midPair := TradingPair{
		Base:  assetA,
		Quote: assetB,
	}
	lastPair := TradingPair{
		Base:  assetB,
		Quote: p.HoldAsset,
	}
	return PaymentPath{
		PathAssetA: assetA,
		PathAssetB: assetB,
		FirstPair:  firstPair,
		MidPair:    midPair,
		LastPair:   lastPair,
	}
}

// FindBestPath determines and returns the most profitable payment path
func (p *PathFinder) FindBestPath() PaymentPath {
	bestRatio := 0.0
	var bestPath PaymentPath
	for _, b := range p.PathList {
		ratio := p.calculatePathRatio(b)
		if ratio > bestRatio {
			bestRatio = ratio
			bestPath = b
		}
		l.Infof("Return ratio for pair %s - > %s was %v \n", b.PathAssetA, b.PathAssetB, ratio)
	}
	l.Infof("Best path was %s - > %s with return ratio of %v", bestpath.PathAssetA, bestPath.PathAssetB, bestRatio)

	return bestPath
}

func (p *PathFinder) calculatePathRatio(path PaymentPath) (float64, error) {
	// first pair is buying asset A with the hold asset
	firstPairLowAsk, e := p.GetLowAsk(path.FirstPair)
	if e != nil {
		return 0, fmt.Errorf("Error while calculating path ratio %s", e)
	}

	// mid pair is selling asset A for asset B
	midPairTopBid, e := p.GetTopBid(path.MidPair)
	if e != nil {
		return 0, fmt.Errorf("Error while calculating path ratio %s", e)
	}

	// last pair is selling asset B for the hold asset
	lastPairTopBid, e := p.GetTopBid(path.LastPair)
	if e != nil {
		return 0, fmt.Errorf("Error while calculating path ratio %s", e)
	}

	ratio := (1 / firstPairLowAsk) * midPairTopBid * lastPairTopBid

	return ratio, nil
}

// GetTopBid returns the top bid for a trading pair
func (p *PathFinder) GetTopBid(pair TradingPair) (float64, error) {
	orderBook, e := p.GetOrderBook(p.SDEX.API)
	if e != nil {
		return 0, fmt.Errorf("unable to get sdex price: %s", e)
	}

	bids := orderBook.Bids
	topBidPrice := utils.PriceAsFloat(bids[0].Price)

	return topBidPrice, nil
}

// GetLowAsk returns the low ask for a trading pair
func (p *PathFinder) GetLowAsk(pair TradingPair) (float64, error) {
	orderBook, e := p.GetOrderBook(pair)
	if e != nil {
		return 0, fmt.Errorf("unable to get sdex price: %s", e)
	}

	asks := orderBook.Asks
	lowAskPrice := utils.PriceAsFloat(asks[0].Price)

	return lowAskPrice, nil
}

// GetOrderBook gets the SDEX order book
func (p *PathFinder) GetOrderBook(api *horizon.Client, pair TradingPair) (orderBook horizon.OrderBookSummary, e error) {
	baseAsset, quoteAsset := p.pair2Assets(pair)
	b, e := api.LoadOrderBook(baseAsset, quoteAsset)
	if e != nil {
		log.Printf("Can't get SDEX orderbook: %s\n", e)
		return
	}
	return b, e
}

func (p *PathFinder) pair2Assets(pair TradingPair) (*horizon.Asset, *horizon.Asset) {
	base := pair.Base
	quote := pair.Quote
	return base, quote
}

// ParseAsset returns a horizon asset from strings
// This is temporary until the sdex feed is merged into Kelp master
func ParseAsset(code string, issuer string) (*horizon.Asset, error) {
	if code != "XLM" && issuer == "" {
		return nil, fmt.Errorf("error: issuer can only be empty if asset is XLM")
	}

	if code == "XLM" && issuer != "" {
		return nil, fmt.Errorf("error: issuer needs to be empty if asset is XLM")
	}

	if code == "XLM" {
		asset := utils.Asset2Asset2(build.NativeAsset())
		return &asset, nil
	}

	asset := utils.Asset2Asset2(build.CreditAsset(code, issuer))
	return &asset, nil
}
