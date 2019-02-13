package arbitrageur

import (
	"time"

	"github.com/Reidmcc/rockfish/modules"
	"github.com/interstellar/kelp/api"
	"github.com/interstellar/kelp/support/logger"
	"github.com/interstellar/kelp/support/utils"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/stellar/go/build"
)

// Arbitrageur is the bot struct
type Arbitrageur struct {
	PathFinder      modules.PathFinder
	DexWatcher      modules.DexWatcher
	DexAgent        modules.DexAgent
	timeController  api.TimeController
	threadTracker   *multithreading.ThreadTracker
	fixedIterations *uint64
	simMode         bool
	BookTracker     *multithreading.ThreadTracker
	l               logger.Logger

	// uninitialized
	endAssetDisplay string
}

// MakeArbitrageur is the factory method
func MakeArbitrageur(
	pathFinder modules.PathFinder,
	dexWatcher modules.DexWatcher,
	dexAgent modules.DexAgent,
	timeController api.TimeController,
	threadTracker *multithreading.ThreadTracker,
	fixedIterations *uint64,
	simMode bool,
	l logger.Logger,
) *Arbitrageur {

	endAssetDisplay := pathFinder.HoldAsset.Code
	var pairBook []modules.TradingPair

	assetBook := pathFinder.AssetBook

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i].Asset != assetBook[n].Asset && assetBook[i].Group == assetBook[n].Group {
				pairBook = append(pairBook, modules.TradingPair{assetBook[i].Asset, assetBook[n].Asset})
			}
		}
	}

	encountered := map[string]bool{}

	if utils.Asset2Asset(pathFinder.HoldAsset) == build.NativeAsset() {
		endAssetDisplay = "XLM"

	}
	bookTracker := multithreading.MakeThreadTracker()

	a := Arbitrageur{
		PathFinder:      pathFinder,
		DexWatcher:      dexWatcher,
		DexAgent:        dexAgent,
		timeController:  timeController,
		threadTracker:   threadTracker,
		fixedIterations: fixedIterations,
		simMode:         simMode,
		l:               l,
		endAssetDisplay: endAssetDisplay,
	}

	return &a
	// return &Arbitrageur{
	// 	PathFinder:      pathFinder,
	// 	DexWatcher:      dexWatcher,
	// 	DexAgent:        dexAgent,
	// 	timeController:  timeController,
	// 	threadTracker:   threadTracker,
	// 	fixedIterations: fixedIterations,
	// 	simMode:         simMode,
	// 	l:               l,
	// 	endAssetDisplay: endAssetDisplay,
	// }
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

			a.cycle()

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

func (a *Arbitrageur) cycle() {
	bestPath, maxAmount, thresholdMet, e := a.PathFinder.FindBestPath()
	if e != nil {
		a.l.Errorf("error while finding best path: %s", e)
	}

	if thresholdMet {
		e := a.DexAgent.SendPaymentCycle(bestPath, maxAmount)
		if e != nil {
			a.l.Errorf("Error while sending payment cycle %s", e)
		}
		return
	}

	return
}
