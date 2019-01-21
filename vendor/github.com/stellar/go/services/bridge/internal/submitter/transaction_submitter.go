package submitter

import (
	"database/sql"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/services/bridge/internal/db"
	shared "github.com/stellar/go/services/internal/bridge-compliance-shared"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

// TransactionSubmitterInterface helps mocking TransactionSubmitter
type TransactionSubmitterInterface interface {
	SubmitTransaction(paymentID *string, seed string, operation, memo interface{}) (response horizon.TransactionSuccess, err error)
	SignAndSubmitRawTransaction(paymentID *string, seed string, tx *xdr.Transaction) (response horizon.TransactionSuccess, err error)
}

// TransactionSubmitter submits transactions to Stellar Network
type TransactionSubmitter struct {
	Horizon       horizon.ClientInterface
	Accounts      map[string]*Account // seed => *Account
	AccountsMutex sync.Mutex
	Database      db.Database
	Network       build.Network
	log           *logrus.Entry
	now           func() time.Time
}

// Account represents account used to signing and sending transactions
type Account struct {
	Keypair        keypair.KP
	Seed           string
	SequenceNumber uint64
	Mutex          sync.Mutex
}

// NewTransactionSubmitter creates a new TransactionSubmitter
func NewTransactionSubmitter(
	horizon horizon.ClientInterface,
	database db.Database,
	networkPassphrase string,
	now func() time.Time,
) (ts TransactionSubmitter) {
	ts.Horizon = horizon
	ts.Database = database
	ts.Accounts = make(map[string]*Account)
	ts.Network = build.Network{networkPassphrase}
	ts.log = logrus.WithFields(logrus.Fields{
		"service": "TransactionSubmitter",
	})
	ts.now = now
	return
}

// LoadAccount loads current state of Stellar account and creates a map entry if it didn't exist
func (ts *TransactionSubmitter) LoadAccount(seed string) (*Account, error) {
	ts.AccountsMutex.Lock()

	account, exist := ts.Accounts[seed]
	if exist {
		ts.AccountsMutex.Unlock()
		return account, nil
	}

	kp, err := keypair.Parse(seed)
	if err != nil {
		ts.log.Print("Invalid seed")
		ts.AccountsMutex.Unlock()
		return nil, err
	}

	ts.Accounts[seed] = &Account{
		Seed:    seed,
		Keypair: kp,
	}
	ts.AccountsMutex.Unlock()

	// Load account sequence number
	ts.Accounts[seed].Mutex.Lock()
	defer ts.Accounts[seed].Mutex.Unlock()

	if ts.Accounts[seed].SequenceNumber != 0 {
		return ts.Accounts[seed], nil
	}

	accountResponse, err := ts.Horizon.LoadAccount(ts.Accounts[seed].Keypair.Address())
	if err != nil {
		return nil, err
	}

	ts.Accounts[seed].SequenceNumber, err = strconv.ParseUint(accountResponse.Sequence, 10, 64)
	if err != nil {
		return nil, err
	}

	return ts.Accounts[seed], nil
}

// InitAccount loads an account and returns error if it fails
func (ts *TransactionSubmitter) InitAccount(seed string) (err error) {
	_, err = ts.LoadAccount(seed)
	return
}

// SignAndSubmitRawTransaction will:
// - update sequence number of the transaction to the current one,
// - sign it,
// - submit it to the network.
func (ts *TransactionSubmitter) SignAndSubmitRawTransaction(paymentID *string, seed string, tx *xdr.Transaction) (response horizon.TransactionSuccess, err error) {
	account, err := ts.LoadAccount(seed)
	if err != nil {
		ts.log.WithFields(logrus.Fields{"err": err}).Error("Error loading account")
		return
	}

	account.Mutex.Lock()
	account.SequenceNumber++
	tx.SeqNum = xdr.SequenceNumber(account.SequenceNumber)
	account.Mutex.Unlock()

	hash, err := shared.TransactionHash(tx, ts.Network.Passphrase)
	if err != nil {
		ts.log.WithFields(logrus.Fields{"err": err}).Error("Error calculating transaction hash")
		return
	}

	sig, err := account.Keypair.SignDecorated(hash[:])
	if err != nil {
		ts.log.WithFields(logrus.Fields{"err": err}).Error("Error signing a transaction")
		return
	}

	envelopeXdr := xdr.TransactionEnvelope{
		Tx:         *tx,
		Signatures: []xdr.DecoratedSignature{sig},
	}

	txeB64, err := xdr.MarshalBase64(envelopeXdr)
	if err != nil {
		ts.log.WithFields(logrus.Fields{"err": err}).Error("Cannot encode transaction envelope")
		return
	}

	transactionHashBytes, err := shared.TransactionHash(tx, ts.Network.Passphrase)
	if err != nil {
		ts.log.WithFields(logrus.Fields{"err": err}).Warn("Error calculating tx hash")
		return
	}

	nullPaymentID := sql.NullString{Valid: false}
	if paymentID != nil {
		nullPaymentID = sql.NullString{
			String: *paymentID,
			Valid:  true,
		}
	}

	sentTransaction := &db.SentTransaction{
		PaymentID:     nullPaymentID,
		TransactionID: hex.EncodeToString(transactionHashBytes[:]),
		Status:        db.SentTransactionStatusSending,
		Source:        account.Keypair.Address(),
		SubmittedAt:   ts.now(),
		EnvelopeXdr:   txeB64,
	}
	err = ts.Database.InsertSentTransaction(sentTransaction)
	if err != nil {
		ts.log.WithFields(logrus.Fields{"err": err}).Error("Error inserting sent transaction")
		return
	}

	ts.log.WithFields(logrus.Fields{"tx": txeB64}).Info("Submitting transaction")

	var herr *horizon.Error
	response, err = ts.Horizon.SubmitTransaction(txeB64)
	if err == nil {
		sentTransaction.Status = db.SentTransactionStatusSuccess
		sentTransaction.Ledger = &response.Ledger
		now := time.Now()
		sentTransaction.SucceededAt = &now
	} else {
		var isHorizonError bool
		herr, isHorizonError = err.(*horizon.Error)
		if !isHorizonError {
			ts.log.WithFields(logrus.Fields{"err": err}).Error("Error submitting transaction ", err)
			return
		}

		result, err := herr.ResultString()
		if err != nil {
			result = errors.Wrap(err, "Error getting tx result").Error()
		}
		sentTransaction.Status = db.SentTransactionStatusFailure
		sentTransaction.ResultXdr = &result
	}

	err = ts.Database.UpdateSentTransaction(sentTransaction)
	if err != nil {
		ts.log.WithFields(logrus.Fields{"err": err}).Error("Error updating sent transaction")
		return
	}

	// Sync sequence number
	if herr != nil {
		codes, rerr := herr.ResultCodes()
		if rerr != nil {
			return response, herr
		}

		if codes.TransactionCode != "tx_bad_seq" {
			return response, herr
		}

		account.Mutex.Lock()
		ts.log.Print("Syncing sequence number for ", account.Keypair.Address())
		accountResponse, err2 := ts.Horizon.LoadAccount(account.Keypair.Address())
		if err2 != nil {
			ts.log.Error("Error updating sequence number ", err)
		} else {
			account.SequenceNumber, _ = strconv.ParseUint(accountResponse.Sequence, 10, 64)
		}
		account.Mutex.Unlock()

		return response, herr
	}
	return
}

// SubmitTransaction builds and submits transaction to Stellar network
func (ts *TransactionSubmitter) SubmitTransaction(paymentID *string, seed string, operation, memo interface{}) (horizon.TransactionSuccess, error) {
	account, err := ts.LoadAccount(seed)
	if err != nil {
		return horizon.TransactionSuccess{}, errors.Wrap(err, "Error loading an account")
	}

	operationMutator, ok := operation.(build.TransactionMutator)
	if !ok {
		ts.log.Error("Cannot cast operationMutator to build.TransactionMutator")
		return horizon.TransactionSuccess{}, errors.New("Cannot cast operationMutator to build.TransactionMutator")
	}

	mutators := []build.TransactionMutator{
		build.SourceAccount{account.Seed},
		ts.Network,
		operationMutator,
	}

	if memo != nil {
		memoMutator, ok := memo.(build.TransactionMutator)
		if !ok {
			ts.log.Error("Cannot cast memo to build.TransactionMutator")
			return horizon.TransactionSuccess{}, errors.New("Cannot cast memo to build.TransactionMutator")
		}
		mutators = append(mutators, memoMutator)
	}

	txBuilder, err := build.Transaction(mutators...)
	if err != nil {
		return horizon.TransactionSuccess{}, errors.Wrap(err, "Error building a transaction")
	}

	return ts.SignAndSubmitRawTransaction(paymentID, seed, txBuilder.TX)
}
