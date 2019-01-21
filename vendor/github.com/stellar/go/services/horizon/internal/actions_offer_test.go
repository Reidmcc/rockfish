package horizon

import (
	"testing"
)

func TestOfferActions_Index(t *testing.T) {
	ht := StartHTTPTest(t, "trades")
	defer ht.Finish()

	w := ht.Get(
		"/accounts/GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2/offers",
	)

	if ht.Assert.Equal(200, w.Code) {
		ht.Assert.PageOf(3, w.Body)

		//test last modified timestamp
		var records []map[string]interface{}
		ht.UnmarshalPage(w.Body, &records)
		ht.Assert.Equal("2018-07-31T20:16:50Z", records[2]["last_modified_time"])
		ht.Assert.EqualValues(5, records[2]["last_modified_ledger"])
	}
}
