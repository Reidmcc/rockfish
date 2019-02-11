package modules

import (
	"fmt"

	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/kelp/model"
	"github.com/stellar/kelp/support/logger"
	"github.com/stellar/kelp/support/utils"
)

// PathRequester keeps the assetbook and settings for the path-requesting method
// this prototype performed strictly worse than pathFinder so it's not implemented at this time
// preserving this for future revamp ideas
type PathRequester struct {
	dexWatcher   DexWatcher
	HoldAsset    horizon.Asset
	assetBook    []groupedAsset
	minRatio     *model.Number
	staticAmount *model.Number
	minAmount    *model.Number
	l            logger.Logger

	//unintialized
	endAssetDisplay string
	assetBookMark   int
}

// MakePathRequester is a factory method
func MakePathRequester(
	dexWatcher DexWatcher,
	stratConfig ArbitCycleConfig,
	l logger.Logger,
) (*PathRequester, error) {
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

	// assetBookMark ensures we don't get stuck on one asset
	assetBookMark := 0

	return &PathRequester{
		dexWatcher:      dexWatcher,
		HoldAsset:       holdAsset,
		assetBook:       assetBook,
		minRatio:        model.NumberFromFloat(stratConfig.MinRatio, utils.SdexPrecision),
		staticAmount:    model.NumberFromFloat(stratConfig.StaticAmount, utils.SdexPrecision),
		minAmount:       model.NumberFromFloat(stratConfig.MinAmount, utils.SdexPrecision),
		l:               l,
		endAssetDisplay: endAssetDisplay,
		assetBookMark:   assetBookMark,
	}, nil
}

// AnalysePaths asks horizon about available paths and checks if any are profitable
func (p *PathRequester) AnalysePaths() (*PathRecord, *model.Number, bool, error) {
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

		// if price is 50 and we want 100, we have to sell 2, so yeah, divide

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
func (p *PathRequester) findMaxAmount(sendPath PathRecord) (*model.Number, error) {
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

	if p.staticAmount.AsFloat() < maxOutput.AsFloat() {
		maxOutput = p.staticAmount
	}

	return maxOutput, nil
}
