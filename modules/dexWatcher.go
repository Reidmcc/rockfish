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
	API            *horizon.Client
	Network        build.Network
	TradingAccount string
	threadTracker  *multithreading.ThreadTracker
	l              logger.Logger
}

// MakeDexWatcher is the factory method
func MakeDexWatcher(
	api *horizon.Client,
	network build.Network,
	tradingAccount string,
	l logger.Logger) *DexWatcher {

	threadTracker := multithreading.MakeThreadTracker()

	return &DexWatcher{
		API:            api,
		Network:        network,
		TradingAccount: tradingAccount,
		threadTracker:  threadTracker,
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

// AddTrackedBook adds a pair's orderbook to the BookTracker
func (w *DexWatcher) AddTrackedBook(pair TradingPair) error {
	query := url.Values{}

	query.Add("selling_asset_type", pair.Base.Type)
	query.Add("selling_asset_code", pair.Base.Code)
	query.Add("selling_asset_issuer", pair.Base.Issuer)

	query.Add("buying_asset_type", pair.Quote.Type)
	query.Add("buying_asset_code", pair.Quote.Code)
	query.Add("buying_asset_issuer", pair.Quote.Issuer)

	bookURL := query.Encode()

	// w.BookTracker.TriggerGoroutine(w.stream(context.Background(), bookURL, nil, w.handleReturnedBook))

	w.threadTracker.TriggerGoroutine(func(inputs []interface{}) {
		w.stream(context.Background(), bookURL, nil, w.handleReturnedBook)
	}, nil)

	return nil
}

func (w *DexWatcher) handleReturnedBook(data []byte) error {
	var book *horizon.OrderBookSummary
	json.Unmarshal([]byte(data), &book)

	BookUpdates <- book

	return nil
}

// this is the stream function from horizon.client, but horizon doesn't expose it
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

	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s?%s", baseURL, query.Encode()), nil)
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
	}
}
