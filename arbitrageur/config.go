package arbitrageur

import (
	"fmt"

	"github.com/interstellar/kelp/support/utils"
)

// ArbitConfig represents the configuration params for the arbitrage bot
type ArbitConfig struct {
	SourceSecretSeed    string `valid:"-" toml:"SOURCE_SECRET_SEED"`
	TradingSecretSeed   string `valid:"-" toml:"TRADING_SECRET_SEED"`
	TickIntervalSeconds int32  `valid:"-" toml:"TICK_INTERVAL_SECONDS"`
	HorizonURL          string `valid:"-" toml:"HORIZON_URL"`
	AlertType           string `valid:"-" toml:"ALERT_TYPE"`
	AlertAPIKey         string `valid:"-" toml:"ALERT_API_KEY"`
	MonitoringPort      uint16 `valid:"-" toml:"MONITORING_PORT"`
	MonitoringTLSCert   string `valid:"-" toml:"MONITORING_TLS_CERT"`
	MonitoringTLSKey    string `valid:"-" toml:"MONITORING_TLS_KEY"`
	GoogleClientID      string `valid:"-" toml:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret  string `valid:"-" toml:"GOOGLE_CLIENT_SECRET"`
	AcceptableEmails    string `valid:"-" toml:"ACCEPTABLE_GOOGLE_EMAILS"`

	tradingAccount *string
	sourceAccount  *string // can be nil
}

// String impl.
func (a *ArbitConfig) String() string {
	return utils.StructString(a, map[string]func(interface{}) interface{}{
		"SOURCE_SECRET_SEED":  utils.SecretKey2PublicKey,
		"TRADING_SECRET_SEED": utils.SecretKey2PublicKey,
	})
}

// TradingAccount returns the config's trading account
func (a *ArbitConfig) TradingAccount() string {
	return *a.tradingAccount
}

// SourceAccount returns the config's source account
func (a *ArbitConfig) SourceAccount() string {
	if a.sourceAccount == nil {
		return ""
	}
	return *a.sourceAccount
}

// Init initializes this config
func (a *ArbitConfig) Init() error {
	var tradingAccount *string
	tradingAccount, e := utils.ParseSecret(a.TradingSecretSeed)
	if e != nil {
		return e
	}
	if a.tradingAccount == nil {
		return fmt.Errorf("no trading account specified")
	}

	a.tradingAccount = tradingAccount

	a.sourceAccount, e = utils.ParseSecret(a.SourceSecretSeed)
	return e
}
