package stellar

import (
	"sync"

	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/log"
)

// Status describes status of account processing
type Status string

const (
	StatusCreatingAccount    Status = "creating_account"
	StatusWaitingForSigner   Status = "waiting_for_signer"
	StatusConfiguringAccount Status = "configuring_account"
	StatusRemovingSigner     Status = "removing_signer"
)

// AccountConfigurator is responsible for configuring new Stellar accounts that
// participate in ICO.
type AccountConfigurator struct {
	Horizon               horizon.ClientInterface `inject:""`
	NetworkPassphrase     string
	IssuerPublicKey       string
	DistributionPublicKey string
	SignerSecretKey       string
	LockUnixTimestamp     uint64
	NeedsAuthorize        bool
	TokenAssetCode        string
	TokenPriceBTC         string
	TokenPriceETH         string
	StartingBalance       string
	OnAccountCreated      func(destination string)
	OnExchanged           func(destination string)
	OnExchangedTimelocked func(destination, transaction string)

	signerPublicKey     string
	signerSequence      uint64
	signerSequenceMutex sync.Mutex
	accountStatus       map[string]Status
	accountStatusMutex  sync.Mutex
	log                 *log.Entry
}
