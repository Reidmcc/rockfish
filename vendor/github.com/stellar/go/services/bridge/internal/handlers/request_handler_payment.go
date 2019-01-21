package handlers

import (
	"encoding/hex"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/stellar/go/address"
	b "github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/protocols/compliance"
	"github.com/stellar/go/protocols/federation"
	shared "github.com/stellar/go/services/internal/bridge-compliance-shared"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/http/helpers"
	"github.com/stellar/go/services/internal/bridge-compliance-shared/protocols/bridge"
	callback "github.com/stellar/go/services/internal/bridge-compliance-shared/protocols/compliance"
	"github.com/stellar/go/xdr"
)

// Payment implements /payment endpoint
func (rh *RequestHandler) Payment(w http.ResponseWriter, r *http.Request) {
	request := &bridge.PaymentRequest{}
	err := helpers.FromRequest(r, request)
	if err != nil {
		log.Error(err.Error())
		helpers.Write(w, helpers.InvalidParameterError)
		return
	}

	err = helpers.Validate(request, rh.Config.Accounts.BaseSeed)
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

	if request.Source == "" {
		request.Source = rh.Config.Accounts.BaseSeed
	}

	// Will use compliance if compliance server is connected and:
	// * User passed extra memo OR
	// * User explicitly wants to use compliance protocol
	if rh.Config.Compliance != "" &&
		(request.ExtraMemo != "" || (request.ExtraMemo == "" && request.UseCompliance)) {
		rh.complianceProtocolPayment(w, request)
	} else {
		rh.standardPayment(w, request)
	}
}

func (rh *RequestHandler) complianceProtocolPayment(w http.ResponseWriter, request *bridge.PaymentRequest) {
	var paymentID *string
	if request.ID != "" {
		paymentID = &request.ID
	}

	// Compliance server part
	sendRequest := request.ToComplianceSendRequest()

	resp, err := rh.Client.PostForm(
		rh.Config.Compliance+"/send",
		helpers.ToValues(sendRequest),
	)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error sending request to compliance server")
		helpers.Write(w, helpers.InternalServerError)
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("Error reading compliance server response")
		helpers.Write(w, helpers.InternalServerError)
		return
	}

	if resp.StatusCode != 200 {
		log.WithFields(log.Fields{
			"status": resp.StatusCode,
			"body":   string(body),
		}).Error("Error response from compliance server")
		helpers.Write(w, helpers.InternalServerError)
		return
	}

	var callbackSendResponse callback.SendResponse
	err = json.Unmarshal(body, &callbackSendResponse)
	if err != nil {
		log.Error("Error unmarshalling from compliance server")
		helpers.Write(w, helpers.InternalServerError)
		return
	}

	if callbackSendResponse.AuthResponse.InfoStatus == compliance.AuthStatusPending ||
		callbackSendResponse.AuthResponse.TxStatus == compliance.AuthStatusPending {
		log.WithFields(log.Fields{"response": callbackSendResponse}).Info("Compliance response pending")
		helpers.Write(w, bridge.NewPaymentPendingError(callbackSendResponse.AuthResponse.Pending))
		return
	}

	if callbackSendResponse.AuthResponse.InfoStatus == compliance.AuthStatusDenied ||
		callbackSendResponse.AuthResponse.TxStatus == compliance.AuthStatusDenied {
		log.WithFields(log.Fields{"response": callbackSendResponse}).Info("Compliance response denied")
		helpers.Write(w, bridge.PaymentDenied)
		return
	}

	var tx xdr.Transaction
	err = xdr.SafeUnmarshalBase64(callbackSendResponse.TransactionXdr, &tx)
	if err != nil {
		log.Error("Error unmarshalling transaction returned by compliance server")
		helpers.Write(w, helpers.InternalServerError)
		return
	}

	submitResponse, err := rh.TransactionSubmitter.SignAndSubmitRawTransaction(paymentID, request.Source, &tx)
	rh.handleTransactionSubmitResponse(w, submitResponse, err)
}

func (rh *RequestHandler) standardPayment(w http.ResponseWriter, request *bridge.PaymentRequest) {
	var paymentID *string

	if request.ID != "" {
		sentTransaction, err := rh.Database.GetSentTransactionByPaymentID(request.ID)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Error getting sent transaction")
			helpers.Write(w, helpers.InternalServerError)
			return
		}

		if sentTransaction == nil {
			paymentID = &request.ID
		} else {
			log.WithFields(log.Fields{"paymentID": request.ID, "tx": sentTransaction.EnvelopeXdr}).Info("Transaction with given ID already exists, resubmitting...")
			submitResponse, err := rh.Horizon.SubmitTransaction(sentTransaction.EnvelopeXdr)
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Error("Error submitting transaction")
				helpers.Write(w, helpers.InternalServerError)
				return
			}

			rh.handleTransactionSubmitResponse(w, submitResponse, err)
			return
		}
	}

	destinationObject := &federation.NameResponse{}
	var err error

	if request.ForwardDestination == nil {
		_, _, err = address.Split(request.Destination)
		if err != nil {
			destinationObject.AccountID = request.Destination
		} else {
			destinationObject, err = rh.FederationResolver.LookupByAddress(request.Destination)
			if err != nil {
				log.WithFields(log.Fields{"destination": request.Destination, "err": err}).Print("Cannot resolve address")
				helpers.Write(w, bridge.PaymentCannotResolveDestination)
				return
			}
		}
	} else {
		destinationObject, err = rh.FederationResolver.ForwardRequest(request.ForwardDestination.Domain, request.ForwardDestination.Fields)
		if err != nil {
			log.WithFields(log.Fields{"destination": request.Destination, "err": err}).Print("Cannot resolve address")
			helpers.Write(w, bridge.PaymentCannotResolveDestination)
			return
		}
	}

	if !shared.IsValidAccountID(destinationObject.AccountID) {
		log.WithFields(log.Fields{"AccountId": destinationObject.AccountID}).Print("Invalid AccountId in destination")
		helpers.Write(w, helpers.NewInvalidParameterError("destination", "Destination public key must start with `G`."))
		return
	}

	var payWithMutator *b.PayWithPath

	if request.SendMax != "" {
		// Path payment
		var sendAsset b.Asset
		if request.SendAssetCode == "" && request.SendAssetIssuer == "" {
			sendAsset = b.NativeAsset()
		} else {
			sendAsset = b.CreditAsset(request.SendAssetCode, request.SendAssetIssuer)
		}

		payWith := b.PayWith(sendAsset, request.SendMax)

		for _, asset := range request.Path {
			payWith = payWith.Through(asset.ToBaseAsset())
		}

		payWithMutator = &payWith
	}

	var operationBuilder interface{}

	if request.AssetCode != "" && request.AssetIssuer != "" {
		mutators := []interface{}{
			b.Destination{destinationObject.AccountID},
			b.CreditAmount{request.AssetCode, request.AssetIssuer, request.Amount},
		}

		if payWithMutator != nil {
			mutators = append(mutators, *payWithMutator)
		}

		operationBuilder = b.Payment(mutators...)
	} else {
		mutators := []interface{}{
			b.Destination{destinationObject.AccountID},
			b.NativeAmount{request.Amount},
		}

		if payWithMutator != nil {
			mutators = append(mutators, *payWithMutator)
		}

		// Check if destination account exist
		_, err = rh.Horizon.LoadAccount(destinationObject.AccountID)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("Error loading account")
			operationBuilder = b.CreateAccount(mutators...)
		} else {
			operationBuilder = b.Payment(mutators...)
		}
	}

	memoType := request.MemoType
	memo := request.Memo

	if destinationObject.MemoType != "" {
		if request.MemoType != "" {
			log.Print("Memo given in request but federation returned memo fields.")
			helpers.Write(w, bridge.PaymentCannotUseMemo)
			return
		}

		memoType = destinationObject.MemoType
		memo = destinationObject.Memo.Value
	}

	var memoMutator interface{}
	switch {
	case memoType == "":
		break
	case memoType == "id":
		id, err := strconv.ParseUint(memo, 10, 64)
		if err != nil {
			log.WithFields(log.Fields{"memo": memo}).Print("Cannot convert memo_id value to uint64")
			helpers.Write(w, helpers.NewInvalidParameterError("memo", "Memo.id must be a number"))
			return
		}
		memoMutator = b.MemoID{id}
	case memoType == "text":
		memoMutator = b.MemoText{memo}
	case memoType == "hash":
		memoBytes, err := hex.DecodeString(memo)
		if err != nil || len(memoBytes) != 32 {
			log.WithFields(log.Fields{"memo": memo}).Print("Cannot decode hash memo value")
			helpers.Write(w, helpers.NewInvalidParameterError("memo", "Memo.hash must be 32 bytes and hex encoded."))
			return
		}
		var b32 [32]byte
		copy(b32[:], memoBytes[0:32])
		hash := xdr.Hash(b32)
		memoMutator = b.MemoHash{hash}
	default:
		log.Print("Not supported memo type: ", memoType)
		helpers.Write(w, helpers.NewInvalidParameterError("memo", "Memo type not supported"))
		return
	}

	submitResponse, err := rh.TransactionSubmitter.SubmitTransaction(paymentID, request.Source, operationBuilder, memoMutator)
	rh.handleTransactionSubmitResponse(w, submitResponse, err)
}

func (rh *RequestHandler) handleTransactionSubmitResponse(w http.ResponseWriter, submitResponse horizon.TransactionSuccess, err error) {
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
