package ingest

import (
	"database/sql"

	"github.com/stellar/go/services/horizon/internal/db2/core"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
)

// Load runs queries against `core` to fill in the records of the bundle.
func (lb *LedgerBundle) Load(db *db.Session) error {
	q := &core.Q{Session: db}
	// Load Header
	err := q.LedgerHeaderBySequence(&lb.Header, lb.Sequence)
	if err != nil {
		// Remove when Horizon is able to handle gaps in stellar-core DB.
		// More info: https://github.com/stellar/go/issues/335
		if err == sql.ErrNoRows {
			return errors.New("Gap detected in stellar-core database. Please recreate Horizon DB.")
		}
		return errors.Wrap(err, "failed to load header")
	}

	// Load transactions
	err = q.TransactionsByLedger(&lb.Transactions, lb.Sequence)
	if err != nil {
		return errors.Wrap(err, "failed to load transactions")
	}

	err = q.TransactionFeesByLedger(&lb.TransactionFees, lb.Sequence)
	if err != nil {
		return errors.Wrap(err, "failed to load transaction fees")
	}

	return nil
}
