package arbitrageur

import (
	"github.com/Reidmcc/rockfish/modules"
	"github.com/interstellar/kelp/api"
	"github.com/nikhilsaraf/go-tools/multithreading"
	"github.com/stellar/go/clients/horizon"
)

// Arbitrageur is the bot structure
type Arbitrageur struct {
	api             *horizon.Client
	tradingAccount  string
	multiDex        modules.MultiDex
	strat           api.Strategy // the instance of this bot is bound to this strategy
	timeController  api.TimeController
	threadTracker   *multithreading.ThreadTracker
	fixedIterations *uint64
	alert           api.Alert
	pathManager     modules.PathManager

	// uninitialized runtime vars

}
