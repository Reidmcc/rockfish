package modules

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/kelp/model"
	"github.com/stellar/kelp/support/logger"
	"github.com/stellar/kelp/support/utils"
)

// DexWatcher is an object that queries the DEX
type DexWatcher struct {
	API            *horizon.Client
	Network        build.Network
	TradingAccount string
	l              logger.Logger
}

// MakeDexWatcher is the factory method
func MakeDexWatcher(
	api *horizon.Client,
	network build.Network,
	tradingAccount string,
	l logger.Logger) *DexWatcher {
	return &DexWatcher{
		API:            api,
		Network:        network,
		TradingAccount: tradingAccount,
		l:              l,
	}
}

// type pathResponse

// GetTopBid returns the top bid's price and amount for a trading pair
func (w *DexWatcher) GetTopBid(pair TradingPair) (*model.Number, *model.Number, error) {
	orderBook, e := w.GetOrderBook(w.API, pair)
	if e != nil {
		return nil, nil, fmt.Errorf("unable to get sdex price: %s", e)
	}

	bids := orderBook.Bids
	if len(bids) == 0 {
		//w.l.Infof("No bids for pair: %s - %s ", pair.Base.Code, pair.Quote.Code)
		return model.NumberConstants.Zero, model.NumberConstants.Zero, nil
	}
	topBidPrice, e := model.NumberFromString(bids[0].Price, utils.SdexPrecision)
	if e != nil {
		return nil, nil, fmt.Errorf("Error converting to model.Number: %s", e)
	}
	topBidAmount, e := model.NumberFromString(bids[0].Amount, utils.SdexPrecision)
	if e != nil {
		return nil, nil, fmt.Errorf("Error converting to model.Number: %s", e)
	}

	// now invert the bid amount because the network sends it upside down
	floatPrice := topBidPrice.AsFloat()
	topBidAmount = topBidAmount.Scale(1 / floatPrice)

	//w.l.Infof("topBidPrice for pair Base = %s Quote =%s was %v", pair.Base.Code, pair.Quote.Code, topBidPrice)

	return topBidPrice, topBidAmount, nil
}

// GetLowAsk returns the low ask's price and amount for a trading pair
func (w *DexWatcher) GetLowAsk(pair TradingPair) (*model.Number, *model.Number, error) {
	orderBook, e := w.GetOrderBook(w.API, pair)
	if e != nil {
		return nil, nil, fmt.Errorf("unable to get sdex price: %s", e)
	}

	asks := orderBook.Asks
	if len(asks) == 0 {
		//w.l.Infof("No asks for pair: %s - %s ", pair.Base.Code, pair.Quote.Code)
		return model.NumberConstants.Zero, model.NumberConstants.Zero, nil
	}
	lowAskPrice, e := model.NumberFromString(asks[0].Price, utils.SdexPrecision)
	if e != nil {
		return nil, nil, fmt.Errorf("Error converting to model.Number: %s", e)
	}
	lowAskAmount, e := model.NumberFromString(asks[0].Amount, utils.SdexPrecision)
	if e != nil {
		return nil, nil, fmt.Errorf("Error converting to model.Number: %s", e)
	}

	//w.l.Infof("lowAskPrice for pair Base = %s Quote =%s was %v", pair.Base.Code, pair.Quote.Code, lowAskPrice)

	return lowAskPrice, lowAskAmount, nil
}

// GetOrderBook gets the SDEX order book
func (w *DexWatcher) GetOrderBook(api *horizon.Client, pair TradingPair) (orderBook horizon.OrderBookSummary, e error) {
	baseAsset, quoteAsset := Pair2Assets(pair)
	b, e := api.LoadOrderBook(baseAsset, quoteAsset)
	if e != nil {
		log.Printf("Can't get SDEX orderbook: %s\n", e)
		return
	}

	return b, e
}

// GetPaths gets and parses the find-path data for an asset from horizon
func (w *DexWatcher) GetPaths(endAsset horizon.Asset, amount *model.Number) (*FindPathResponse, error) {
	var s strings.Builder
	var paths FindPathResponse
	amountString := amount.AsString()

	s.WriteString(fmt.Sprintf("%s/paths?source_account=%s", w.API.URL, w.TradingAccount))
	s.WriteString(fmt.Sprintf("&destination_account=%s", w.TradingAccount))
	s.WriteString(fmt.Sprintf("&destination_asset_type=%s", endAsset.Type))
	s.WriteString(fmt.Sprintf("&destination_asset_code=%s", endAsset.Code))
	s.WriteString(fmt.Sprintf("&destination_asset_issuer=%s", endAsset.Issuer))
	s.WriteString(fmt.Sprintf("&destination_amount=%s", amountString))

	//w.l.Infof("GET string built as: %s", s.String())

	resp, e := w.API.HTTP.Get(s.String())

	//resp, e := w.API.HTTP.Get("https://horizon.stellar.org/paths?source_account=GDJQ7DGRBAPJMBMDNDZIBCW4SFZ55GWEHYEPGLRJQW46NGBUSYSCLSLV&destination_account=GDJQ7DGRBAPJMBMDNDZIBCW4SFZ55GWEHYEPGLRJQW46NGBUSYSCLSLV&destination_asset_type=credit_alphanum4&destination_asset_code=BTC&destination_asset_issuer=GBSTRH4QOTWNSVA6E4HFERETX4ZLSR3CIUBLK7AXYII277PFJC4BBYOG&destination_amount=.0000001")

	if e != nil {
		return nil, e
	}
	// w.l.Info("")
	// w.l.Infof("Raw horizon response was %s\n", resp.Body)
	// w.l.Info("")
	defer resp.Body.Close()
	byteResp, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		return nil, e
	}

	json.Unmarshal([]byte(byteResp), &paths)
	// if len(paths.Embedded.Records) > 0 {
	// 	w.l.Info("")
	// 	w.l.Infof("Unmarshalled into: %+v\n", paths.Embedded.Records[0])
	// 	w.l.Info("")
	// }

	return &paths, nil
}
