package history_test

import (
	"testing"

	"github.com/stellar/go/services/horizon/internal/db2"
	. "github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/xdr"
)

func TestTradeQueries(t *testing.T) {
	tt := test.Start(t).Scenario("kahuna")
	defer tt.Finish()
	q := &Q{tt.HorizonSession()}
	var trades []Trade

	// All trades
	err := q.Trades().Page(db2.MustPageQuery("", "asc", 100)).Select(&trades)
	if tt.Assert.NoError(err) {
		tt.Assert.Len(trades, 4)
	}

	// Paging
	pq := db2.MustPageQuery(trades[0].PagingToken(), "asc", 1)
	var pt []Trade

	err = q.Trades().Page(pq).Select(&pt)
	if tt.Assert.NoError(err) {
		if tt.Assert.Len(pt, 1) {
			tt.Assert.Equal(trades[1], pt[0])
		}
	}

	// Cursor bounds checking
	pq = db2.MustPageQuery("", "desc", 1)
	err = q.Trades().Page(pq).Select(&pt)
	tt.Require.NoError(err)

	// test for asset pairs
	err = q.TradesForAssetPair(2, 3).Select(&trades)
	tt.Require.NoError(err)

	tt.Assert.Len(trades, 0)

	err = q.TradesForAssetPair(1, 4).Select(&trades)
	tt.Require.NoError(err)
	tt.Assert.Len(trades, 1)

	tt.Assert.Equal(xdr.Int64(2000000000), trades[0].BaseAmount)
	tt.Assert.Equal(xdr.Int64(1000000000), trades[0].CounterAmount)
	tt.Assert.Equal(true, trades[0].BaseIsSeller)

	// reverse assets
	err = q.TradesForAssetPair(4, 1).Select(&trades)
	tt.Require.NoError(err)
	tt.Assert.Len(trades, 1)

	tt.Assert.Equal(xdr.Int64(1000000000), trades[0].BaseAmount)
	tt.Assert.Equal(xdr.Int64(2000000000), trades[0].CounterAmount)
	tt.Assert.Equal(false, trades[0].BaseIsSeller)
}
