package arbitrageur

import (
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/Reidmcc/rockfish/modules"
	"github.com/interstellar/kelp/support/logger"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/stellar/go/clients/horizon"
)

// Arbitrageur is the bot struct
type Arbitrageur struct {
	PathFinder    modules.PathFinder
	DexWatcher    modules.DexWatcher
	DexAgent      *modules.DexAgent
	threadTracker *multithreading.ThreadTracker
	simMode       bool
	rateLimiter   func()
	booksOut      <-chan *horizon.OrderBookSummary
	ledgerOut     <-chan horizon.Ledger
	findIt        chan<- bool
	pathReturn    <-chan modules.PathFindOutcome
	refresh       chan<- bool
	submitDone    <-chan bool
	l             logger.Logger

	// uninitialized
	endAssetDisplay string
}

// MakeArbitrageur is the factory method
func MakeArbitrageur(
	pathFinder modules.PathFinder,
	dexWatcher modules.DexWatcher,
	dexAgent *modules.DexAgent,
	threadTracker *multithreading.ThreadTracker,
	rateLimiter func(),
	simMode bool,
	booksOut chan *horizon.OrderBookSummary,
	ledgerOut chan horizon.Ledger,
	findIt chan<- bool,
	pathReturn <-chan modules.PathFindOutcome,
	refresh chan<- bool,
	submitDone <-chan bool,
	l logger.Logger,
) *Arbitrageur {
	return &Arbitrageur{
		PathFinder:    pathFinder,
		DexWatcher:    dexWatcher,
		DexAgent:      dexAgent,
		threadTracker: threadTracker,
		simMode:       simMode,
		rateLimiter:   rateLimiter,
		booksOut:      booksOut,
		ledgerOut:     ledgerOut,
		findIt:        findIt,
		pathReturn:    pathReturn,
		refresh:       refresh,
		submitDone:    submitDone,
		l:             l,
	}
}

// StartLedgerSynced starts in ledger-synced mode
func (a *Arbitrageur) StartLedgerSynced() {
	// we use streaming of the relevant orderbooks as a proxy for net-ledger notification pending fix for ledger streaming
	// trim the duplicate pairs to avoid duplicate streams
	encountered := make(map[modules.TradingPair]bool)
	var trimmedPairBook []modules.TradingPair

	for _, g := range a.PathFinder.AssetGroups {

		for _, v := range g.PairBook {
			if !encountered[v.Pair] && !encountered[modules.TradingPair{Base: v.Pair.Quote, Quote: v.Pair.Base}] {
				encountered[v.Pair] = true
				trimmedPairBook = append(trimmedPairBook, v.Pair)
			}
		}
	}

	go a.DexWatcher.StreamManager(trimmedPairBook)

	shouldDelay := false

	for {
		go a.PathFinder.FindBestPathConcurrent()
		<-a.booksOut
		if !shouldDelay {
			shouldDelay = true
			a.findIt <- true
			go func() {
				delayTimer := time.NewTimer(2 * time.Second)
				<-delayTimer.C
				shouldDelay = false
			}()
			r := <-a.pathReturn
			if r.MetThreshold {
				a.DexAgent.SendPaymentCycle(r.BestPath, r.MaxAmount)
				submitTimeout := time.NewTimer(5 * time.Second)
				select {
				case <-a.submitDone:
					continue
				case <-submitTimeout.C:
					continue
				}
			}
		} else {
			a.refresh <- true
		}
	}
}

func (a *Arbitrageur) blockStats() {
	// for {
	pprof.Lookup("block").WriteTo(os.Stdout, 1)
	a.l.Infof("# Goroutines: %v\n", runtime.NumGoroutine())
	// }
}
