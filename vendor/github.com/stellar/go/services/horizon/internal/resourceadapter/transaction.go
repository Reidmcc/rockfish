package resourceadapter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/guregu/null"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/httpx"
	. "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/render/hal"
)

// Populate fills out the details
func PopulateTransaction(
	ctx context.Context,
	dest *Transaction,
	row history.Transaction,
) (err error) {

	dest.ID = row.TransactionHash
	dest.PT = row.PagingToken()
	dest.Hash = row.TransactionHash
	dest.Ledger = row.LedgerSequence
	dest.LedgerCloseTime = row.LedgerCloseTime
	dest.Account = row.Account
	dest.AccountSequence = row.AccountSequence
	dest.FeePaid = row.FeePaid
	dest.OperationCount = row.OperationCount
	dest.EnvelopeXdr = row.TxEnvelope
	dest.ResultXdr = row.TxResult
	dest.ResultMetaXdr = row.TxMeta
	dest.FeeMetaXdr = row.TxFeeMeta
	dest.MemoType = row.MemoType
	dest.Memo = row.Memo.String
	dest.Signatures = strings.Split(row.SignatureString, ",")
	dest.ValidBefore = timeString(dest, row.ValidBefore)
	dest.ValidAfter = timeString(dest, row.ValidAfter)

	lb := hal.LinkBuilder{Base: httpx.BaseURL(ctx)}
	dest.Links.Account = lb.Link("/accounts", dest.Account)
	dest.Links.Ledger = lb.Link("/ledgers", fmt.Sprintf("%d", dest.Ledger))
	dest.Links.Operations = lb.PagedLink("/transactions", dest.ID, "operations")
	dest.Links.Effects = lb.PagedLink("/transactions", dest.ID, "effects")
	dest.Links.Self = lb.Link("/transactions", dest.ID)
	dest.Links.Succeeds = lb.Linkf("/transactions?order=desc&cursor=%s", dest.PT)
	dest.Links.Precedes = lb.Linkf("/transactions?order=asc&cursor=%s", dest.PT)
	return
}


func timeString(res *Transaction, in null.Int) string {
	if !in.Valid {
		return ""
	}

	return time.Unix(in.Int64, 0).UTC().Format(time.RFC3339)
}
