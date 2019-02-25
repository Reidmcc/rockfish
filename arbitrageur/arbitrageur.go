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
	PathFinder      modules.PathFinder
	DexWatcher      modules.DexWatcher
	DexAgent        *modules.DexAgent
	threadTracker   *multithreading.ThreadTracker
	fixedIterations *uint64
	simMode         bool
	booksOut        <-chan *horizon.OrderBookSummary
	ledgerOut       <-chan horizon.Ledger
	findIt          chan<- bool
	pathReturn      <-chan modules.PathFindOutcome
	refresh         chan<- bool
	l               logger.Logger

	// uninitialized
	endAssetDisplay string
}

// MakeArbitrageur is the factory method
func MakeArbitrageur(
	pathFinder modules.PathFinder,
	dexWatcher modules.DexWatcher,
	dexAgent *modules.DexAgent,
	threadTracker *multithreading.ThreadTracker,
	fixedIterations *uint64,
	simMode bool,
	booksOut chan *horizon.OrderBookSummary,
	ledgerOut chan horizon.Ledger,
	findIt chan<- bool,
	pathReturn <-chan modules.PathFindOutcome,
	refresh chan<- bool,
	l logger.Logger,
) *Arbitrageur {
	return &Arbitrageur{
		PathFinder:      pathFinder,
		DexWatcher:      dexWatcher,
		DexAgent:        dexAgent,
		threadTracker:   threadTracker,
		fixedIterations: fixedIterations,
		simMode:         simMode,
		booksOut:        booksOut,
		ledgerOut:       ledgerOut,
		findIt:          findIt,
		pathReturn:      pathReturn,
		refresh:         refresh,
		l:               l,
	}
}

// StartLedgerSynced starts in ledger-synced mode
func (a *Arbitrageur) StartLedgerSynced() {
	// we use streaming of the relevant orderbooks as a proxy for net-ledger notification pending fix for ledger streaming
	// trim the duplicate pairs to avoid duplicate streams
	encountered := make(map[modules.TradingPair]bool)
	var trimmedPairBook []modules.TradingPair
	for _, v := range a.PathFinder.PairBook {
		if !encountered[v] && !encountered[modules.TradingPair{Base: v.Quote, Quote: v.Base}] {
			encountered[v] = true
			trimmedPairBook = append(trimmedPairBook, v)
		}
	}
	go a.DexWatcher.StreamManager(trimmedPairBook)

	// create a ticker to regulate the rate of path checking
	shouldDelay := false
	go func() {
		delayticker := time.NewTicker(2 * time.Second)
		for {
			<-delayticker.C
			shouldDelay = false
		}
	}()

	for {
		go a.PathFinder.FindBestPathConcurrent()
		<-a.booksOut
		if !shouldDelay {

			a.findIt <- true
			shouldDelay = true

			r := <-a.pathReturn
			if r.MetThreshold {
				a.DexAgent.SendPaymentCycle(r.BestPath, r.MaxAmount)
			}
		} else {
			a.refresh <- true
		}
	}
}

// Start ...starts the legacy method
func (a *Arbitrageur) Start() {
	a.l.Info("----------------------------------------------------------------------------------------------------")
	var lastUpdateTime time.Time

	for {
		currentUpdateTime := time.Now()
		curBalance, e := a.DexAgent.JustAssetBalance(a.PathFinder.HoldAsset)
		if e != nil {
			a.l.Errorf("Error while checking pre-cycle hold balance: %s", e)
		}
		a.l.Infof("Going into the cycle %s balance was %v", a.endAssetDisplay, curBalance)
		if lastUpdateTime.IsZero() {

			// a.cycle()

			if a.fixedIterations != nil {
				*a.fixedIterations = *a.fixedIterations - 1
				if *a.fixedIterations <= 0 {
					a.l.Infof("finished requested number of iterations, waiting for all threads to finish...\n")
					a.threadTracker.Wait()
					a.l.Infof("...all threads finished, stopping bot update loop\n")
					return
				}
			}

			// wait for any goroutines from the current update to finish so we don't have inconsistent state reads
			a.threadTracker.Wait()
			a.l.Info("----------------------------------------------------------------------------------------------------")
			lastUpdateTime = currentUpdateTime
		}
	}
}

func (a *Arbitrageur) blockStats() {
	// for {
	pprof.Lookup("block").WriteTo(os.Stdout, 1)
	a.l.Infof("# Goroutines: %v\n", runtime.NumGoroutine())
	// }
}
