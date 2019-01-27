package modules

import (
	"fmt"
	"log"

	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
)

// DexWatcher is an object that queries the DEX
type DexWatcher struct {
	API     *horizon.Client
	Network build.Network
	l       logger.Logger
}

// MakeDexWatcher is the factory method
func MakeDexWatcher(api *horizon.Client, network build.Network, l logger.Logger) *DexWatcher {
	return &DexWatcher{
		API:     api,
		Network: network,
		l:       l,
	}
}

// GetTopBid returns the top bid's price and amount for a trading pair
func (w *DexWatcher) GetTopBid(pair TradingPair) (float64, float64, error) {
	orderBook, e := w.GetOrderBook(w.API, pair)
	if e != nil {
		return -1, -1, fmt.Errorf("unable to get sdex price: %s", e)
	}

	bids := orderBook.Bids
	if len(bids) == 0 {
		//w.l.Infof("No bids for pair: %s - %s ", pair.Base.Code, pair.Quote.Code)
		return -1, -1, nil
	}
	topBidPrice := utils.PriceAsFloat(bids[0].Price)
	topBidAmount := utils.AmountStringAsFloat(bids[0].Amount)

	return topBidPrice, topBidAmount, nil
}

// GetLowAsk returns the low ask's price and amount for a trading pair
func (w *DexWatcher) GetLowAsk(pair TradingPair) (float64, float64, error) {
	orderBook, e := w.GetOrderBook(w.API, pair)
	if e != nil {
		return -1, -1, fmt.Errorf("unable to get sdex price: %s", e)
	}

	asks := orderBook.Asks
	if len(asks) == 0 {
		//w.l.Infof("No asks for pair: %s - %s ", pair.Base.Code, pair.Quote.Code)
		return -1, -1, nil
	}
	lowAskPrice := utils.PriceAsFloat(asks[0].Price)
	lowAskAmount := utils.AmountStringAsFloat(asks[0].Amount)

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
