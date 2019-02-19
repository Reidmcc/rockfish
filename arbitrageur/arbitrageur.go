package arbitrageur

import (
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/Reidmcc/rockfish/modules"
	"github.com/interstellar/kelp/api"
	"github.com/interstellar/kelp/support/logger"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/stellar/go/clients/horizon"
)

// Arbitrageur is the bot struct
type Arbitrageur struct {
	PathFinder       modules.PathFinder
	DexWatcher       modules.DexWatcher
	DexAgent         *modules.DexAgent
	timeController   api.TimeController
	threadTracker    *multithreading.ThreadTracker
	pathCheckTracker *multithreading.ThreadTracker
	tranSendTracker  *multithreading.ThreadTracker
	fixedIterations  *uint64
	simMode          bool
	pairBook         []modules.TradingPair
	booksOut         <-chan *horizon.OrderBookSummary
	pathJobs         chan *modules.PaymentPath
	transJobs        chan *modules.TransData
	l                logger.Logger

	// uninitialized
	endAssetDisplay string
}

// MakeArbitrageur is the factory method
func MakeArbitrageur(
	pathFinder modules.PathFinder,
	dexWatcher modules.DexWatcher,
	dexAgent *modules.DexAgent,
	timeController api.TimeController,
	threadTracker *multithreading.ThreadTracker,
	fixedIterations *uint64,
	simMode bool,
	booksOut <-chan *horizon.OrderBookSummary,
	l logger.Logger,
) *Arbitrageur {

	endAssetDisplay := pathFinder.HoldAsset.Code
	var rawPairBook []modules.TradingPair

	var pathCheckTracker *multithreading.ThreadTracker
	var tranSendTracker *multithreading.ThreadTracker

	pathJobs := make(chan *modules.PaymentPath, 20)
	transJobs := make(chan *modules.TransData, 10)

	assetBook := pathFinder.AssetBook

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i].Asset != assetBook[n].Asset && assetBook[i].Group == assetBook[n].Group {
				rawPairBook = append(rawPairBook, modules.TradingPair{Base: assetBook[i].Asset, Quote: assetBook[n].Asset})
			}
		}
	}

	for i := 0; i < len(assetBook); i++ {
		rawPairBook = append(rawPairBook, modules.TradingPair{Base: assetBook[i].Asset, Quote: pathFinder.HoldAsset})
	}

	// removes inverted book duplicates
	encountered := map[modules.TradingPair]bool{}
	var pairBook []modules.TradingPair

	for v := range rawPairBook {
		if !encountered[rawPairBook[v]] && !encountered[modules.TradingPair{Base: rawPairBook[v].Quote, Quote: rawPairBook[v].Base}] {
			encountered[rawPairBook[v]] = true
			pairBook = append(pairBook, rawPairBook[v])
		}
	}

	return &Arbitrageur{
		PathFinder:       pathFinder,
		DexWatcher:       dexWatcher,
		DexAgent:         dexAgent,
		timeController:   timeController,
		threadTracker:    threadTracker,
		pathCheckTracker: pathCheckTracker,
		tranSendTracker:  tranSendTracker,
		fixedIterations:  fixedIterations,
		simMode:          simMode,
		pairBook:         pairBook,
		booksOut:         booksOut,
		pathJobs:         pathJobs,
		transJobs:        transJobs,
		l:                l,
		endAssetDisplay:  endAssetDisplay,
	}
}

// Start ...starts
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
		if lastUpdateTime.IsZero() || a.timeController.ShouldUpdate(lastUpdateTime, currentUpdateTime) {

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

		sleepTime := a.timeController.SleepTime(lastUpdateTime, currentUpdateTime)
		a.l.Infof("sleeping for %s...\n", sleepTime)
		time.Sleep(sleepTime)
	}
}

// StartStreamMode starts in streaming mode
func (a *Arbitrageur) StartStreamMode() {
	// make a channel to stop the streams
	stop := make(chan bool, 100)
	hold := make(chan bool)
	done := make(chan bool)
	// spawn := make(chan *modules.PaymentPath, 20)

	// start streams
	a.startServices(stop, hold, done)
	// a.pathCheckSpawner(spawn, hold, done, stop)

	// prep a ticker
	ticker := time.NewTicker(10 * time.Second)
	idleCounter := 0

	for {
		select {
		case b := <-a.booksOut:
			// a.l.Info("received a book to spawn from")
			idleCounter = 0
			for i := 0; i < len(a.PathFinder.PathList); i++ {
				// go a.PathFinder.PathChecker(0, a.pathJobs, a.transJobs, stop)
				// go a.DexAgent.TranSender(0, a.transJobs, stop, hold, done)

				if b.Selling == a.PathFinder.PathList[i].FirstPair.Base && b.Buying == a.PathFinder.PathList[i].FirstPair.Quote {
					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
					if !a.PathFinder.PathList[i].ShouldDelay {
						a.PathFinder.PathList[i].ShouldDelay = true
						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
						a.pathJobs <- a.PathFinder.PathList[i]
					}
					continue
				}

				if b.Selling == a.PathFinder.PathList[i].FirstPair.Quote && b.Buying == a.PathFinder.PathList[i].FirstPair.Base {
					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
					if !a.PathFinder.PathList[i].ShouldDelay {
						a.PathFinder.PathList[i].ShouldDelay = true
						// a.l.Info("trying to spawn")
						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
						a.pathJobs <- a.PathFinder.PathList[i]
					}
					continue
				}

				if b.Selling == a.PathFinder.PathList[i].MidPair.Base && b.Buying == a.PathFinder.PathList[i].MidPair.Quote {
					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
					if !a.PathFinder.PathList[i].ShouldDelay {
						a.PathFinder.PathList[i].ShouldDelay = true
						// a.l.Info("trying to spawn")
						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
						a.pathJobs <- a.PathFinder.PathList[i]
					}
					continue
				}

				if b.Selling == a.PathFinder.PathList[i].MidPair.Quote && b.Buying == a.PathFinder.PathList[i].MidPair.Base {
					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
					if !a.PathFinder.PathList[i].ShouldDelay {
						a.PathFinder.PathList[i].ShouldDelay = true
						// a.l.Info("trying to spawn")
						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
						a.pathJobs <- a.PathFinder.PathList[i]
					}
					continue
				}

				if b.Selling == a.PathFinder.PathList[i].LastPair.Base && b.Buying == a.PathFinder.PathList[i].LastPair.Quote {
					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
					if !a.PathFinder.PathList[i].ShouldDelay {
						a.PathFinder.PathList[i].ShouldDelay = true
						// a.l.Info("trying to spawn")
						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
						a.pathJobs <- a.PathFinder.PathList[i]
					}
					continue
				}

				if b.Selling == a.PathFinder.PathList[i].LastPair.Quote && b.Buying == a.PathFinder.PathList[i].LastPair.Base {
					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)

					if !a.PathFinder.PathList[i].ShouldDelay {
						a.PathFinder.PathList[i].ShouldDelay = true
						// a.l.Info("trying to spawn")
						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
						a.pathJobs <- a.PathFinder.PathList[i]
					}
					continue
				}
			}
			time.Sleep(10 * time.Millisecond)
		case <-hold:
			<-done
		case <-ticker.C:
			a.l.Infof("watching, idle count = %v\n", idleCounter)
			a.blockStats()
			idleCounter++
			if runtime.NumGoroutine() < len(a.pairBook)*3 {
				a.l.Info("too few routines, restarting streams")
				// if i := 0; i < len(a.pairBook) {
				// 	stop <- true
				// }
				a.restartStreams(stop, hold, done)
			}

			if idleCounter >= 6 {
				a.l.Info("we've gone 1 minute without a proc, streams may have droppped")
				// if i := 0; i < len(a.pairBook) {
				// 	stop <- true
				// }
				a.restartStreams(stop, hold, done)
			}
		}
	}
}

// func (a *Arbitrageur) StartBetterStreamMode() {

// }

// func (a *Arbitrageur) cycle() {
// 	bestPath, maxAmount, thresholdMet, e := a.PathFinder.FindBestPath()
// 	if e != nil {
// 		a.l.Errorf("error while finding best path: %s", e)
// 	}

// 	if thresholdMet {
// 		e := a.DexAgent.SendPaymentCycle(bestPath, maxAmount, hold, done)
// 		if e != nil {
// 			a.l.Errorf("Error while sending payment cycle %s", e)
// 		}
// 		return
// 	}

// 	return
// }

func (a *Arbitrageur) handleLedger(ledger horizon.Ledger) {
	a.l.Infof("got a ledger! %s", ledger)
	// return nil
}

func (a *Arbitrageur) blockStats() {
	// for {
	pprof.Lookup("block").WriteTo(os.Stdout, 1)
	a.l.Infof("# Goroutines: %v\n", runtime.NumGoroutine())
	// }
}

func (a *Arbitrageur) startServices(stop chan bool, hold chan bool, done chan bool) {
	// a.l.Infof("Starting %v goroutines of each type", len(a.pairBook))
	for b := range a.pairBook {
		e := a.DexWatcher.AddTrackedBook(a.pairBook[b], "20", stop)
		if e != nil {
			a.l.Errorf("error adding streams: %s", e)
		}
	}

	// prepare pathCheckers to accept book returns
	for i := 0; i < len(a.pairBook); i++ {
		go a.PathFinder.PathChecker(i, a.pathJobs, a.transJobs, stop)
	}

	// prepare TranSenders to send transactions
	for i := 0; i < len(a.pairBook); i++ {
		go a.DexAgent.TranSender(i, a.transJobs, stop, hold, done)
	}
}

func (a *Arbitrageur) restartStreams(stop chan bool, hold chan bool, done chan bool) {
	// a.l.Infof("Starting %v goroutines of each type", len(a.pairBook))
	for b := range a.pairBook {
		e := a.DexWatcher.AddTrackedBook(a.pairBook[b], "20", stop)
		if e != nil {
			a.l.Errorf("error adding streams: %s", e)
		}
	}
}

func (a *Arbitrageur) pathCheckSpawner(hold chan bool, done chan bool, stop chan bool) {
	// timer := time.NewTimer(30 * time.Second)
	a.l.Info("started spawner")
	for s := range a.pathJobs {
		select {
		case <-a.pathJobs:
			a.l.Info("spawn triggered")
			go a.PathFinder.PathChecker(0, a.pathJobs, a.transJobs, stop)

			// a.pathCheckTracker.TriggerGoroutine(func(inputs []interface{}) {
			// 	a.PathFinder.PathChecker(0, a.pathJobs, a.transJobs, stop)
			// }, nil)

			go a.DexAgent.TranSender(0, a.transJobs, stop, hold, done)

			// a.pathCheckTracker.TriggerGoroutine(func(inputs []interface{}) {
			// 	a.DexAgent.TranSender(0, a.transJobs, stop, hold, done)
			// }, nil)

			a.pathJobs <- s
		case <-hold:
			a.pathCheckTracker.Wait()
			<-done
		case <-stop:
			return
		}
	}
}

func (a *Arbitrageur) spawnAndSend(path *modules.PaymentPath, stop chan bool, hold chan bool, done chan bool) {
	go a.PathFinder.PathChecker(0, a.pathJobs, a.transJobs, stop)
	go a.DexAgent.TranSender(0, a.transJobs, stop, hold, done)
	a.pathJobs <- path
}
