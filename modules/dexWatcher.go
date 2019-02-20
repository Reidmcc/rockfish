package modules

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/interstellar/kelp/model"
	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/manucorporat/sse"
	"github.com/pkg/errors"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
)

// DexWatcher is an object that queries the DEX
type DexWatcher struct {
	API        *horizon.Client
	Network    build.Network
	booksOut   chan<- *horizon.OrderBookSummary
	ledgerPing chan<- bool
	l          logger.Logger
}

// MakeDexWatcher is the factory method
func MakeDexWatcher(
	api *horizon.Client,
	network build.Network,
	booksOut chan<- *horizon.OrderBookSummary,
	ledgerPing chan<- bool,
	l logger.Logger) *DexWatcher {

	return &DexWatcher{
		API:        api,
		Network:    network,
		booksOut:   booksOut,
		ledgerPing: ledgerPing,
		l:          l,
	}
}

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
func (w *DexWatcher) GetPaths(endAsset horizon.Asset, amount *model.Number, tradingAccount string) (*FindPathResponse, error) {
	var s strings.Builder
	var paths FindPathResponse
	amountString := amount.AsString()

	s.WriteString(fmt.Sprintf("%s/paths?source_account=%s", w.API.URL, tradingAccount))
	s.WriteString(fmt.Sprintf("&destination_account=%s", tradingAccount))
	s.WriteString(fmt.Sprintf("&destination_asset_type=%s", endAsset.Type))
	s.WriteString(fmt.Sprintf("&destination_asset_code=%s", endAsset.Code))
	s.WriteString(fmt.Sprintf("&destination_asset_issuer=%s", endAsset.Issuer))
	s.WriteString(fmt.Sprintf("&destination_amount=%s", amountString))

	resp, e := w.API.HTTP.Get(s.String())

	if e != nil {
		return nil, e
	}

	defer resp.Body.Close()
	byteResp, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		return nil, e
	}

	json.Unmarshal([]byte(byteResp), &paths)
	return &paths, nil
}

// StreamManager just refreshes the ledger streams as they expire; horizon closes them after 55 seconds on its own
func (w *DexWatcher) StreamManager() {
	streamTicker := time.NewTicker(50 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
		go w.StreamLedgers(ctx, nil, w.ledgerNotify)
		<-streamTicker.C
		cancel()
	}
}

// StreamLedgers streams Stellar ledgers
func (w *DexWatcher) StreamLedgers(
	ctx context.Context,
	cursor *horizon.Cursor,
	handler horizon.LedgerHandler,
) (err error) {
	url := fmt.Sprintf("%s/ledgers", w.API.URL)
	return w.stream(ctx, url, cursor, func(data []byte) error {
		var ledger horizon.Ledger
		e := json.Unmarshal(data, &ledger)
		if e != nil {
			return fmt.Errorf("error unmarshalling data: %s", e)
		}
		handler(ledger)
		return nil
	})
}

// ledgerNotify just says that a ledger came in for sync purposes
func (w *DexWatcher) ledgerNotify(ledger horizon.Ledger) {
	w.ledgerPing <- true
}

// AddTrackedBook adds a pair's orderbook to the BookTracker
func (w *DexWatcher) AddTrackedBook(pair TradingPair, limit string, stop <-chan bool) error {
	quoteCodeDisplay := pair.Quote.Code
	if pair.Quote.Type == "native" {
		quoteCodeDisplay = "XLM"
	}

	w.l.Infof("Adding stream for %s|%s vs %s|%s", pair.Base.Code, pair.Base.Issuer, quoteCodeDisplay, pair.Quote.Issuer)

	var s strings.Builder
	s.WriteString(fmt.Sprintf(strings.TrimRight(w.API.URL, "/")))
	s.WriteString("/order_book?")
	s.WriteString(fmt.Sprintf("selling_asset_type=%s", pair.Base.Type))
	s.WriteString(fmt.Sprintf("&selling_asset_code=%s", pair.Base.Code))
	s.WriteString(fmt.Sprintf("&selling_asset_issuer=%s", pair.Base.Issuer))
	s.WriteString(fmt.Sprintf("&buying_asset_type=%s", pair.Quote.Type))
	s.WriteString(fmt.Sprintf("&buying_asset_code=%s", pair.Quote.Code))
	s.WriteString(fmt.Sprintf("&buying_asset_issuer=%s", pair.Quote.Issuer))
	s.WriteString(fmt.Sprintf("&limit=%s", limit))

	url := s.String()

	go w.stream(context.Background(), url, nil, func(data []byte) error {
		var book *horizon.OrderBookSummary
		e := json.Unmarshal(data, &book)
		if e != nil {
			return fmt.Errorf("error unmarshaling data: %s", e)
		}
		w.handleReturnedBook(book)
		return nil
	})

	return nil
}

func (w *DexWatcher) handleReturnedBook(book *horizon.OrderBookSummary) {
	quoteCodeDisplay := book.Buying.Code

	w.l.Infof("orderbook for %s|%s vs %s|%s updated, checking relevant paths", book.Selling.Code, book.Selling.Issuer, quoteCodeDisplay, book.Buying.Issuer)
	w.booksOut <- book
}

// this is adapted from the stream function from horizon.client
func (w *DexWatcher) stream(
	ctx context.Context,
	baseURL string,
	cursor *horizon.Cursor,
	handler func(data []byte) error,
) error {
	query := url.Values{}
	if cursor != nil {
		query.Set("cursor", string(*cursor))
	}

	client := http.Client{}

	// for {

	// for {
	if cursor != nil {
		baseURL = fmt.Sprintf("%s?%s", baseURL, query.Encode())
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s", baseURL), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	// Make sure we don't use c.HTTP that can have Timeout set.
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	ticker := time.NewTicker(10 * time.Millisecond)

	for {

		<-ticker.C

		// Read events one by one. Break this loop when there is no more data to be
		// read from resp.Body (io.EOF).
	Events:
		for {
			var buffer bytes.Buffer
			nonEmptylinesRead := 0
			for {
				// Check if ctx is not cancelled
				select {
				case <-ctx.Done():
					return nil
				default:
					// Continue
				}

				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						if nonEmptylinesRead == 0 {
							break Events
						}
					} else {
						return err
					}
				}

				buffer.WriteString(line)

				if strings.TrimRight(line, "\n\r") == "" {
					break
				}

				nonEmptylinesRead++
			}

			events, err := sse.Decode(strings.NewReader(buffer.String()))
			if err != nil {
				return err
			}

			for _, event := range events {
				if event.Event != "message" {
					continue
				}

				// Update cursor with event ID
				if event.Id != "" {
					query.Set("cursor", event.Id)
				}

				switch data := event.Data.(type) {
				case string:
					err = handler([]byte(data))
				case []byte:
					err = handler(data)
				default:
					err = errors.New("Invalid event.Data type")
				}
				if err != nil {
					return err
				}
			}
		}
	}
}
