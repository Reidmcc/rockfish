package handlers

import (
	"github.com/stellar/go/clients/federation"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/clients/stellartoml"
	"github.com/stellar/go/services/bridge/internal/config"
	"github.com/stellar/go/services/bridge/internal/db"
	"github.com/stellar/go/services/bridge/internal/listener"
	"github.com/stellar/go/services/bridge/internal/submitter"
	"github.com/stellar/go/support/http"
)

// RequestHandler implements bridge server request handlers
type RequestHandler struct {
	Config               *config.Config                          `inject:""`
	Client               http.SimpleHTTPClientInterface          `inject:""`
	Horizon              horizon.ClientInterface                 `inject:""`
	Database             db.Database                             `inject:""`
	StellarTomlResolver  stellartoml.ClientInterface             `inject:""`
	FederationResolver   federation.ClientInterface              `inject:""`
	TransactionSubmitter submitter.TransactionSubmitterInterface `inject:""`
	PaymentListener      *listener.PaymentListener               `inject:""`
}

func (rh *RequestHandler) isAssetAllowed(code string, issuer string) bool {
	for _, asset := range rh.Config.Assets {
		if asset.Code == code && asset.Issuer == issuer {
			return true
		}
	}
	return false
}
