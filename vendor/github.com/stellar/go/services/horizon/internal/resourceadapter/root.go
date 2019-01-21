package resourceadapter

import (
	"context"

	"github.com/stellar/go/services/horizon/internal/httpx"
	"github.com/stellar/go/services/horizon/internal/ledger"
	. "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/render/hal"
)

// Populate fills in the details
func PopulateRoot(
	ctx context.Context,
	dest *Root,
	ledgerState ledger.State,
	hVersion, cVersion string,
	passphrase string,
	pVersion int32,
) {
	dest.HorizonSequence = ledgerState.HistoryLatest
	dest.HistoryElderSequence = ledgerState.HistoryElder
	dest.CoreSequence = ledgerState.CoreLatest
	dest.HorizonVersion = hVersion
	dest.StellarCoreVersion = cVersion
	dest.NetworkPassphrase = passphrase
	dest.ProtocolVersion = pVersion

	lb := hal.LinkBuilder{httpx.BaseURL(ctx)}
	dest.Links.Account = lb.Link("/accounts/{account_id}")
	dest.Links.AccountTransactions = lb.PagedLink("/accounts/{account_id}/transactions")
	dest.Links.Assets = lb.Link("/assets{?asset_code,asset_issuer,cursor,limit,order}")
	dest.Links.Friendbot = lb.Link("/friendbot{?addr}")
	dest.Links.Metrics = lb.Link("/metrics")
	dest.Links.OrderBook = lb.Link("/order_book{?selling_asset_type,selling_asset_code,selling_issuer,buying_asset_type,buying_asset_code,buying_issuer,limit}")
	dest.Links.Self = lb.Link("/")
	dest.Links.Transaction = lb.Link("/transactions/{hash}")
	dest.Links.Transactions = lb.PagedLink("/transactions")
}
