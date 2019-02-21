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
	PathFinder      modules.PathFinder
	DexWatcher      modules.DexWatcher
	DexAgent        *modules.DexAgent
	timeController  api.TimeController
	threadTracker   *multithreading.ThreadTracker
	fixedIterations *uint64
	simMode         bool
	ledgerOut       <-chan horizon.Ledger
	findIt          chan<- bool
	pathReturn      <-chan modules.PathFindOutcome
	l               logger.Logger

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
	ledgerOut chan horizon.Ledger,
	findIt chan<- bool,
	pathReturn <-chan modules.PathFindOutcome,
	l logger.Logger,
) *Arbitrageur {
	return &Arbitrageur{
		PathFinder:      pathFinder,
		DexWatcher:      dexWatcher,
		DexAgent:        dexAgent,
		timeController:  timeController,
		threadTracker:   threadTracker,
		fixedIterations: fixedIterations,
		simMode:         simMode,
		ledgerOut:       ledgerOut,
		findIt:          findIt,
		pathReturn:      pathReturn,
		l:               l,
	}
}

// StartLedgerSynced starts in ledger-synced mode
func (a *Arbitrageur) StartLedgerSynced() {
	go a.DexWatcher.StreamManager()
	// go a.DexWatcher.AddTrackedBook(a.PathFinder.PathList[0].PathSequence[0].Pair, "20", make(chan bool))

	for {
		go a.PathFinder.FindBestPathConcurrent()

		<-a.ledgerOut
		a.findIt <- true

		r := <-a.pathReturn
		if r.MetThreshold {
			a.DexAgent.SendPaymentCycle(r.BestPath, r.MaxAmount)
		}
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

func (a *Arbitrageur) blockStats() {
	// for {
	pprof.Lookup("block").WriteTo(os.Stdout, 1)
	a.l.Infof("# Goroutines: %v\n", runtime.NumGoroutine())
	// }
}

// // StartStreamMode starts in streaming mode
// func (a *Arbitrageur) StartStreamMode() {
// 	// make a channel to stop the streams
// 	stop := make(chan bool, 100)
// 	hold := make(chan bool)
// 	done := make(chan bool)
// 	// spawn := make(chan *modules.PaymentPath, 20)

// 	// start streams
// 	a.startServices(stop, hold, done)
// 	// a.pathCheckSpawner(spawn, hold, done, stop)

// 	// prep a ticker
// 	ticker := time.NewTicker(10 * time.Second)
// 	idleCounter := 0

// 	for {
// 		select {
// 		case b := <-a.booksOut:
// 			// a.l.Info("received a book to spawn from")
// 			idleCounter = 0
// 			for i := 0; i < len(a.PathFinder.PathList); i++ {
// 				// go a.PathFinder.PathChecker(0, a.pathJobs, a.transJobs, stop)
// 				// go a.DexAgent.TranSender(0, a.transJobs, stop, hold, done)

// 				if b.Selling == a.PathFinder.PathList[i].FirstPair.Base && b.Buying == a.PathFinder.PathList[i].FirstPair.Quote {
// 					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
// 					if !a.PathFinder.PathList[i].ShouldDelay {
// 						a.PathFinder.PathList[i].ShouldDelay = true
// 						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
// 						a.pathJobs <- a.PathFinder.PathList[i]
// 					}
// 					continue
// 				}

// 				if b.Selling == a.PathFinder.PathList[i].FirstPair.Quote && b.Buying == a.PathFinder.PathList[i].FirstPair.Base {
// 					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
// 					if !a.PathFinder.PathList[i].ShouldDelay {
// 						a.PathFinder.PathList[i].ShouldDelay = true
// 						// a.l.Info("trying to spawn")
// 						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
// 						a.pathJobs <- a.PathFinder.PathList[i]
// 					}
// 					continue
// 				}

// 				if b.Selling == a.PathFinder.PathList[i].MidPair.Base && b.Buying == a.PathFinder.PathList[i].MidPair.Quote {
// 					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
// 					if !a.PathFinder.PathList[i].ShouldDelay {
// 						a.PathFinder.PathList[i].ShouldDelay = true
// 						// a.l.Info("trying to spawn")
// 						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
// 						a.pathJobs <- a.PathFinder.PathList[i]
// 					}
// 					continue
// 				}

// 				if b.Selling == a.PathFinder.PathList[i].MidPair.Quote && b.Buying == a.PathFinder.PathList[i].MidPair.Base {
// 					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
// 					if !a.PathFinder.PathList[i].ShouldDelay {
// 						a.PathFinder.PathList[i].ShouldDelay = true
// 						// a.l.Info("trying to spawn")
// 						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
// 						a.pathJobs <- a.PathFinder.PathList[i]
// 					}
// 					continue
// 				}

// 				if b.Selling == a.PathFinder.PathList[i].LastPair.Base && b.Buying == a.PathFinder.PathList[i].LastPair.Quote {
// 					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)
// 					if !a.PathFinder.PathList[i].ShouldDelay {
// 						a.PathFinder.PathList[i].ShouldDelay = true
// 						// a.l.Info("trying to spawn")
// 						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
// 						a.pathJobs <- a.PathFinder.PathList[i]
// 					}
// 					continue
// 				}

// 				if b.Selling == a.PathFinder.PathList[i].LastPair.Quote && b.Buying == a.PathFinder.PathList[i].LastPair.Base {
// 					// a.l.Infof("path %v procced", a.PathFinder.PathList[i].AgentID)

// 					if !a.PathFinder.PathList[i].ShouldDelay {
// 						a.PathFinder.PathList[i].ShouldDelay = true
// 						// a.l.Info("trying to spawn")
// 						// a.spawnAndSend(a.PathFinder.PathList[i], stop, hold, done)
// 						a.pathJobs <- a.PathFinder.PathList[i]
// 					}
// 					continue
// 				}
// 			}
// 			time.Sleep(10 * time.Millisecond)
// 		case <-hold:
// 			<-done
// 		case <-ticker.C:
// 			a.l.Infof("watching, idle count = %v\n", idleCounter)
// 			a.blockStats()
// 			idleCounter++
// 			if runtime.NumGoroutine() < len(a.pairBook)*3 {
// 				a.l.Info("too few routines, restarting streams")
// 				// if i := 0; i < len(a.pairBook) {
// 				// 	stop <- true
// 				// }
// 				a.restartStreams(stop, hold, done)
// 			}

// 			if idleCounter >= 6 {
// 				a.l.Info("we've gone 1 minute without a proc, streams may have droppped")
// 				// if i := 0; i < len(a.pairBook) {
// 				// 	stop <- true
// 				// }
// 				a.restartStreams(stop, hold, done)
// 			}
// 		}
// 	}
// }

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
