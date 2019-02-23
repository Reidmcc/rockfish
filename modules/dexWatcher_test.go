package modules

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nikhilsaraf/go-tools/multithreading"

	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/stellar/go/clients/horizon"
)

func TestStream(t *testing.T) {

	booksOut := make(chan *horizon.OrderBookSummary, 100)
	ledgerPing := make(chan horizon.Ledger)

	client := &horizon.Client{
		URL:  "https://horizon.stellar.org/",
		HTTP: http.DefaultClient,
	}

	l := logger.MakeBasicLogger()
	threadTracker := multithreading.MakeThreadTracker()

	dexWatcher := MakeDexWatcher(
		client,
		utils.ParseNetwork("https://horizon.stellar.org/"),
		threadTracker,
		booksOut,
		ledgerPing,
		l)

	assetBase := ParseAsset("BTC", "GBSTRH4QOTWNSVA6E4HFERETX4ZLSR3CIUBLK7AXYII277PFJC4BBYOG")
	assetQuote := ParseAsset("XLM", "")

	dexWatcher.AddTrackedBook(context.Background(), TradingPair{Base: assetBase, Quote: assetQuote}, "20")

	counter := 0
	for {
		ticker := time.NewTicker(time.Second)

		select {
		case b := <-booksOut:
			dexWatcher.l.Infof("got book %s\n", b)
		case <-ticker.C:
			counter++
			fmt.Printf("t = %v\n", counter)

		}
		if counter > 10 {
			break
		}
	}

}
