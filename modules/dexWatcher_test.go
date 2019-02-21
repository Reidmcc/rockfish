package modules

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/stellar/go/clients/horizon"
)

func TestStream(t *testing.T) {

	booksOut := make(chan *horizon.OrderBookSummary, 100)
	stop := make(chan bool)
	ledgerPing := make(chan horizon.Ledger)

	client := &horizon.Client{
		URL:  "https://horizon.stellar.org/",
		HTTP: http.DefaultClient,
	}

	l := logger.MakeBasicLogger()

	dexWatcher := MakeDexWatcher(
		client,
		utils.ParseNetwork("https://horizon.stellar.org/"),
		booksOut,
		ledgerPing,
		l)

	assetBase := ParseAsset("BTC", "GBSTRH4QOTWNSVA6E4HFERETX4ZLSR3CIUBLK7AXYII277PFJC4BBYOG")
	assetQuote := ParseAsset("XLM", "")

	dexWatcher.AddTrackedBook(TradingPair{Base: assetBase, Quote: assetQuote}, "20", stop)

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
