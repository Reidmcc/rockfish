package horizon

import (
	"errors"
	"strconv"

	"github.com/stellar/go/services/horizon/internal/db2"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/resourceadapter"
	"github.com/stellar/go/support/time"
	gTime "time"
	"github.com/stellar/go/xdr"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/support/render/hal"
)

type TradeIndexAction struct {
	Action
	BaseAssetFilter       xdr.Asset
	HasBaseAssetFilter    bool
	CounterAssetFilter    xdr.Asset
	HasCounterAssetFilter bool
	OfferFilter           int64
	AccountFilter         string
	PagingParams          db2.PageQuery
	Records               []history.Trade
	Page                  hal.Page
}

// JSON is a method for actions.JSON
func (action *TradeIndexAction) JSON() {
	action.Do(
		action.EnsureHistoryFreshness,
		action.loadParams,
		action.loadRecords,
		action.loadPage,
		func() {
			hal.Render(action.W, action.Page)
		},
	)
}

// loadParams sets action.Query from the request params
func (action *TradeIndexAction) loadParams() {
	action.PagingParams = action.GetPageQuery()
	action.BaseAssetFilter, action.HasBaseAssetFilter = action.MaybeGetAsset("base_")
	action.CounterAssetFilter, action.HasCounterAssetFilter = action.MaybeGetAsset("counter_")
	action.OfferFilter = action.GetInt64("offer_id")
	action.AccountFilter = action.GetString("account_id")
}

// loadRecords populates action.Records
func (action *TradeIndexAction) loadRecords() {
	trades := action.HistoryQ().Trades()

	if action.AccountFilter != "" {
		trades.ForAccount(action.AccountFilter)
	}

	if action.HasBaseAssetFilter {

		baseAssetId, err := action.HistoryQ().GetAssetID(action.BaseAssetFilter)
		if err != nil {
			action.Err = err
			return
		}

		if action.HasCounterAssetFilter {

			counterAssetId, err := action.HistoryQ().GetAssetID(action.CounterAssetFilter)
			if err != nil {
				action.Err = err
				return
			}
			trades = action.HistoryQ().TradesForAssetPair(baseAssetId, counterAssetId)
		} else {
			action.Err = errors.New("this endpoint supports asset pairs but only one asset supplied")
			return
		}
	}

	if action.OfferFilter > int64(0) {
		trades = trades.ForOffer(action.OfferFilter)
	}

	action.Err = trades.Page(action.PagingParams).Select(&action.Records)
}

// loadPage populates action.Page
func (action *TradeIndexAction) loadPage() {
	for _, record := range action.Records {
		var res horizon.Trade

		action.Err = resourceadapter.PopulateTrade(action.R.Context(), &res, record)

		if action.Err != nil {
			return
		}

		action.Page.Add(res)
	}

	action.Page.FullURL = action.FullURL()
	action.Page.Limit = action.PagingParams.Limit
	action.Page.Cursor = action.PagingParams.Cursor
	action.Page.Order = action.PagingParams.Order
	action.Page.PopulateLinks()
}

type TradeAggregateIndexAction struct {
	Action
	BaseAssetFilter    xdr.Asset
	CounterAssetFilter xdr.Asset
	StartTimeFilter    time.Millis
	EndTimeFilter      time.Millis
	ResolutionFilter   int64
	PagingParams       db2.PageQuery
	Records            []history.TradeAggregation
	Page               hal.Page
}

// JSON is a method for actions.JSON
func (action *TradeAggregateIndexAction) JSON() {
	action.Do(
		action.EnsureHistoryFreshness,
		action.loadParams,
		action.loadRecords,
		action.loadPage,
		func() {
			hal.Render(action.W, action.Page)
		},
	)
}

func (action *TradeAggregateIndexAction) loadParams() {
	action.PagingParams = action.GetPageQuery()
	action.BaseAssetFilter = action.GetAsset("base_")
	action.CounterAssetFilter = action.GetAsset("counter_")
	action.StartTimeFilter = action.GetTimeMillis("start_time")
	action.EndTimeFilter = action.GetTimeMillis("end_time")
	action.ResolutionFilter = action.GetInt64("resolution")

	//check if resolution is legal
	resolutionDuration := gTime.Duration(action.ResolutionFilter) * gTime.Millisecond
	if history.StrictResolutionFiltering{
		if _, ok := history.AllowedResolutions[resolutionDuration]; !ok {
			action.SetInvalidField("resolution", errors.New("illegal or missing resolution. " +
				"allowed resolutions are: 1 minute (60000), 5 minutes (300000), 15 minutes (900000), 1 hour (3600000), " +
				"1 day (86400000) and 1 week (604800000)"))
		}
	}
}

// loadRecords populates action.Records
func (action *TradeAggregateIndexAction) loadRecords() {
	historyQ := action.HistoryQ()

	//get asset ids
	baseAssetId, err := historyQ.GetCreateAssetID(action.BaseAssetFilter)
	if err != nil {
		action.Err = err
		return
	}
	counterAssetId, err := historyQ.GetCreateAssetID(action.CounterAssetFilter)
	if err != nil {
		action.Err = err
		return
	}

	//initialize the query builder with required params
	tradeAggregationsQ, err := historyQ.GetTradeAggregationsQ(
		baseAssetId, counterAssetId, action.ResolutionFilter, action.PagingParams)

	if err != nil {
		action.Err = err
		return
	}

	//set time range if supplied
	if !action.StartTimeFilter.IsNil() {
		tradeAggregationsQ.WithStartTime(action.StartTimeFilter)
	}
	if !action.EndTimeFilter.IsNil() {
		tradeAggregationsQ.WithEndTime(action.EndTimeFilter)
	}

	action.Err = historyQ.Select(&action.Records, tradeAggregationsQ.GetSql())
}

func (action *TradeAggregateIndexAction) loadPage() {
	action.Page.Init()
	for _, record := range action.Records {
		var res horizon.TradeAggregation

		action.Err = resourceadapter.PopulateTradeAggregation(action.R.Context(), &res, record)

		if action.Err != nil {
			return
		}

		action.Page.Add(res)
	}

	action.Page.Limit = action.PagingParams.Limit
	action.Page.Order = action.PagingParams.Order

	newUrl := action.FullURL() // preserve scheme and host for the new url links
	q := newUrl.Query()

	action.Page.Links.Self = hal.NewLink(newUrl.String())

	//adjust time range for next page
	if uint64(len(action.Records)) == 0 {
		action.Page.Links.Next = action.Page.Links.Self
	} else {
		if action.PagingParams.Order == "asc" {
			newStartTime := action.Records[len(action.Records)-1].Timestamp + action.ResolutionFilter
			if newStartTime >= action.EndTimeFilter.ToInt64() {
				newStartTime = action.EndTimeFilter.ToInt64()
			}
			q.Set("start_time", strconv.FormatInt(newStartTime, 10))
			newUrl.RawQuery = q.Encode()
			action.Page.Links.Next = hal.NewLink(newUrl.String())
		} else { //desc
			newEndTime := action.Records[len(action.Records)-1].Timestamp
			if newEndTime <= action.StartTimeFilter.ToInt64() {
				newEndTime = action.StartTimeFilter.ToInt64()
			}
			q.Set("end_time", strconv.FormatInt(newEndTime, 10))
			newUrl.RawQuery = q.Encode()
			action.Page.Links.Next = hal.NewLink(newUrl.String())
		}
	}
}
