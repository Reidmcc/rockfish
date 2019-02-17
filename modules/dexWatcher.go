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
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/pkg/errors"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
)

// DexWatcher is an object that queries the DEX
type DexWatcher struct {
	API           *horizon.Client
	Network       build.Network
	threadTracker *multithreading.ThreadTracker
	booksOut      chan<- *horizon.OrderBookSummary
	l             logger.Logger
}

// MakeDexWatcher is the factory method
func MakeDexWatcher(
	api *horizon.Client,
	network build.Network,
	threadTracker *multithreading.ThreadTracker,
	booksOut chan<- *horizon.OrderBookSummary,
	l logger.Logger) *DexWatcher {

	return &DexWatcher{
		API:           api,
		Network:       network,
		threadTracker: threadTracker,
		booksOut:      booksOut,
		l:             l,
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

// AddTrackedBook adds a pair's orderbook to the BookTracker
// just do it manually FFS
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

	w.threadTracker.TriggerGoroutine(func(inputs []interface{}) {
		w.stream(context.Background(), url, nil, stop, func(data []byte) error {
			var book *horizon.OrderBookSummary
			e := json.Unmarshal(data, &book)
			if e != nil {
				return fmt.Errorf("error unmarshaling data: %s", e)
			}
			w.handleReturnedBook(book)
			return nil
		})
	}, nil)

	return nil
}

func (w *DexWatcher) handleReturnedBook(book *horizon.OrderBookSummary) {
	quoteCodeDisplay := book.Buying.Code
	// if book.Buying.Type == "native" {
	// 	quoteCodeDisplay = "XLM"
	// }

	w.l.Infof("orderbook for %s|%s vs %s|%s updated, checking relevant paths", book.Selling.Code, book.Selling.Issuer, quoteCodeDisplay, book.Buying.Issuer)
	w.booksOut <- book
}

// this is adapted from the stream function from horizon.client
func (w *DexWatcher) stream(
	ctx context.Context,
	baseURL string,
	cursor *horizon.Cursor,
	stop <-chan bool,
	handler func(data []byte) error,
) error {
	// inboundStream := make(chan *http.Response)
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
	// whoa, got stuck in a loop spamming Horizon!

	w.l.Infof("Submitting url: %s", baseURL)

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
		// time.Sleep(10 * time.Millisecond)
		select {
		case <-stop:
			w.l.Info("I stopped!")
			return nil
		case <-ticker.C:
		}
		// w.l.Info("ya")
		// Read events one by one. Break this loop when there is no more data to be
		// read from resp.Body (io.EOF).
	Events:
		for {
			// Read until empty line = event delimiter. The perfect solution would be to read
			// as many bytes as possible and forward them to sse.Decode. However this
			// requires much more complicated code.
			// We could also write our own `sse` package that works fine with streams directly
			// (github.com/manucorporat/sse is just using io/ioutils.ReadAll).
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
						// Currently Horizon appends a new line after the last event so this is not really
						// needed. We have this code here in case this behaviour is changed in a future.
						// From spec:
						// > Once the end of the file is reached, the user agent must dispatch the
						// > event one final time, as defined below.
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

			// Right now len(events) should always be 1. This loop will be helpful after writing
			// new SSE decoder that can handle io.Reader without using ioutils.ReadAll().
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
	} //return fmt.Errorf("reached end of streaming func")
}
