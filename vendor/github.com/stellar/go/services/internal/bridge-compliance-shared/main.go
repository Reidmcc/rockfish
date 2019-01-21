package shared

import (
	"bytes"
	"fmt"

	"github.com/stellar/go/build"
	"github.com/stellar/go/hash"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"
)

// BuildTransaction is used in compliance server. The sequence number in built transaction will be equal 0!
func BuildTransaction(accountID, networkPassphrase string, operation, memo interface{}) (transaction *xdr.Transaction, err error) {
	operationMutator, ok := operation.(build.TransactionMutator)
	if !ok {
		err = errors.New("Cannot cast operationMutator to build.TransactionMutator")
		return
	}

	mutators := []build.TransactionMutator{
		build.SourceAccount{accountID},
		build.Sequence{0},
		build.Network{networkPassphrase},
		operationMutator,
	}

	if memo != nil {
		memoMutator, ok := memo.(build.TransactionMutator)
		if !ok {
			err = errors.New("Cannot cast memo to build.TransactionMutator")
			return
		}
		mutators = append(mutators, memoMutator)
	}

	txBuilder, err := build.Transaction(mutators...)
	return txBuilder.TX, err
}

// TransactionHash returns transaction hash for a given Transaction based on the network
func TransactionHash(tx *xdr.Transaction, networkPassphrase string) ([32]byte, error) {
	var txBytes bytes.Buffer

	_, err := fmt.Fprintf(&txBytes, "%s", hash.Hash([]byte(networkPassphrase)))
	if err != nil {
		return [32]byte{}, err
	}

	_, err = xdr.Marshal(&txBytes, xdr.EnvelopeTypeEnvelopeTypeTx)
	if err != nil {
		return [32]byte{}, err
	}

	_, err = xdr.Marshal(&txBytes, tx)
	if err != nil {
		return [32]byte{}, err
	}

	return hash.Hash(txBytes.Bytes()), nil
}

// IsValidAccountID returns true if account ID is valid
func IsValidAccountID(accountID string) bool {
	_, err := keypair.Parse(accountID)
	if err != nil {
		return false
	}

	if accountID[0] != 'G' {
		return false
	}

	return true
}

// IsValidSecret returns true if secret is valid
func IsValidSecret(secret string) bool {
	_, err := keypair.Parse(secret)
	if err != nil {
		return false
	}

	if secret[0] != 'S' {
		return false
	}

	return true
}

// IsValidAssetCode returns true if asset code is valid
func IsValidAssetCode(code string) bool {
	if len(code) < 1 || len(code) > 12 {
		return false
	}
	return true
}
