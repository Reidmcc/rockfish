package handlers

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
	b "github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/http/helpers"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/protocols/bridge"
)

// Authorize implements /authorize endpoint
func (rh *RequestHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	request := &bridge.AuthorizeRequest{}
	err := helpers.FromRequest(r, request)
	if err != nil {
		log.Error(err.Error())
		helpers.Write(w, helpers.InvalidParameterError)
		return
	}

	err = helpers.Validate(request)
	if err != nil {
		switch err := err.(type) {
		case *helpers.ErrorResponse:
			helpers.Write(w, err)
		default:
			log.Error(err)
			helpers.Write(w, helpers.InternalServerError)
		}
		return
	}

	operationMutator := b.AllowTrust(
		b.Trustor{request.AccountID},
		b.Authorize{true},
		b.AllowTrustAsset{request.AssetCode},
	)

	submitResponse, err := rh.TransactionSubmitter.SubmitTransaction(
		nil,
		rh.Config.Accounts.AuthorizingSeed,
		operationMutator,
		nil,
	)

	jsonEncoder := json.NewEncoder(w)

	if err != nil {
		herr, isHorizonError := err.(*horizon.Error)
		if !isHorizonError {
			log.WithFields(log.Fields{"err": err}).Error("Error submitting transaction")
			helpers.Write(w, helpers.InternalServerError)
			return
		}

		w.WriteHeader(herr.Problem.Status)
		err := jsonEncoder.Encode(herr.Problem)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Error encoding response")
			helpers.Write(w, helpers.InternalServerError)
			return
		}

		return
	}

	err = jsonEncoder.Encode(submitResponse)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error encoding response")
		helpers.Write(w, helpers.InternalServerError)
		return
	}
}
