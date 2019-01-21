package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	b "github.com/stellar/go/build"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/http/helpers"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/protocols/bridge"
)

// Builder implements /builder endpoint
func (rh *RequestHandler) Builder(w http.ResponseWriter, r *http.Request) {
	var request bridge.BuilderRequest
	var sequenceNumber uint64

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&request)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error decoding request")
		helpers.Write(w, helpers.NewInvalidParameterError("", "Request body is not a valid JSON"))
		return
	}

	err = request.Process()
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

	if request.SequenceNumber == "" {
		accountResponse, err := rh.Horizon.LoadAccount(request.Source)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Error when loading account")
			helpers.Write(w, helpers.InternalServerError)
			return
		}
		sequenceNumber, err = strconv.ParseUint(accountResponse.Sequence, 10, 64)
		if err == nil {
			// increment sequence number when none is provided
			sequenceNumber = sequenceNumber + 1
		}
	} else {
		sequenceNumber, err = strconv.ParseUint(request.SequenceNumber, 10, 64)
	}

	if err != nil {
		errorResponse := helpers.NewInvalidParameterError("sequence_number", "Sequence number must be a number")
		helpers.Write(w, errorResponse)
		return
	}

	mutators := []b.TransactionMutator{
		b.SourceAccount{request.Source},
		b.Sequence{sequenceNumber},
		b.Network{rh.Config.NetworkPassphrase},
	}

	for _, operation := range request.Operations {
		mutators = append(mutators, operation.Body.ToTransactionMutator())
	}

	tx, err := b.Transaction(mutators...)

	if err != nil {
		log.WithFields(log.Fields{"err": err, "request": request}).Error("TransactionBuilder returned error")
		helpers.Write(w, helpers.InternalServerError)
		return
	}

	txe, err := tx.Sign(request.Signers...)
	if err != nil {
		log.WithFields(log.Fields{"err": err, "request": request}).Error("Error signing transaction")
		helpers.Write(w, helpers.InternalServerError)
		return
	}

	txeB64, err := txe.Base64()
	if err != nil {
		log.WithFields(log.Fields{"err": err, "request": request}).Error("Error encoding transaction envelope")
		helpers.Write(w, helpers.InternalServerError)
		return
	}

	helpers.Write(w, &bridge.BuilderResponse{TransactionEnvelope: txeB64})
}
