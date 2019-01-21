package plugins

import (
	"errors"
	"fmt"
	"log"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/Beldur/kraken-go-api-client"
	"github.com/interstellar/kelp/api"
	"github.com/interstellar/kelp/model"
	"github.com/interstellar/kelp/support/networking"
)

// ensure that krakenExchange conforms to the Exchange interface
var _ api.Exchange = &krakenExchange{}

const precisionBalances = 10

// krakenExchange is the implementation for the Kraken Exchange
type krakenExchange struct {
	assetConverter           *model.AssetConverter
	assetConverterOpenOrders *model.AssetConverter // kraken uses different symbols when fetching open orders!
	apis                     []*krakenapi.KrakenApi
	apiNextIndex             uint8
	delimiter                string
	withdrawKeys             asset2Address2Key
	isSimulated              bool // will simulate add and cancel orders if this is true
}

type asset2Address2Key map[model.Asset]map[string]string

func (m asset2Address2Key) getKey(asset model.Asset, address string) (string, error) {
	address2Key, ok := m[asset]
	if !ok {
		return "", fmt.Errorf("asset (%v) is not registered in asset2Address2Key: %v", asset, m)
	}

	key, ok := address2Key[address]
	if !ok {
		return "", fmt.Errorf("address is not registered in asset2Address2Key: %v (asset = %v)", address, asset)
	}

	return key, nil
}

// makeKrakenExchange is a factory method to make the kraken exchange
// TODO 2, should take in config file for withdrawalKeys mapping
func makeKrakenExchange(apiKeys []api.ExchangeAPIKey, isSimulated bool) (api.Exchange, error) {
	if len(apiKeys) == 0 || len(apiKeys) > math.MaxUint8 {
		return nil, fmt.Errorf("invalid number of apiKeys: %d", len(apiKeys))
	}

	krakenAPIs := []*krakenapi.KrakenApi{}
	for _, apiKey := range apiKeys {
		krakenAPIClient := krakenapi.New(apiKey.Key, apiKey.Secret)
		krakenAPIs = append(krakenAPIs, krakenAPIClient)
	}

	return &krakenExchange{
		assetConverter:           model.KrakenAssetConverter,
		assetConverterOpenOrders: model.KrakenAssetConverterOpenOrders,
		apis:         krakenAPIs,
		apiNextIndex: 0,
		delimiter:    "",
		withdrawKeys: asset2Address2Key{},
		isSimulated:  isSimulated,
	}, nil
}

// nextAPI rotates the API key being used so we can overcome rate limit issues
func (k *krakenExchange) nextAPI() *krakenapi.KrakenApi {
	log.Printf("returning kraken API key at index %d", k.apiNextIndex)
	api := k.apis[k.apiNextIndex]
	// rotate key for the next call
	k.apiNextIndex = (k.apiNextIndex + 1) % uint8(len(k.apis))
	return api
}

// AddOrder impl.
func (k *krakenExchange) AddOrder(order *model.Order) (*model.TransactionID, error) {
	pairStr, e := order.Pair.ToString(k.assetConverter, k.delimiter)
	if e != nil {
		return nil, e
	}

	if k.isSimulated {
		log.Printf("not adding order to Kraken in simulation mode, order=%s\n", *order)
		return model.MakeTransactionID("simulated"), nil
	}

	orderConstraints := k.GetOrderConstraints(order.Pair)
	if order.Price.Precision() > orderConstraints.PricePrecision {
		return nil, fmt.Errorf("kraken price precision can be a maximum of %d, got %d, value = %.12f", orderConstraints.PricePrecision, order.Price.Precision(), order.Price.AsFloat())
	}
	if order.Volume.Precision() > orderConstraints.VolumePrecision {
		return nil, fmt.Errorf("kraken volume precision can be a maximum of %d, got %d, value = %.12f", orderConstraints.VolumePrecision, order.Volume.Precision(), order.Volume.AsFloat())
	}

	args := map[string]string{
		"price": order.Price.AsString(),
	}
	resp, e := k.nextAPI().AddOrder(
		pairStr,
		order.OrderAction.String(),
		order.OrderType.String(),
		order.Volume.AsString(),
		args,
	)
	if e != nil {
		return nil, e
	}

	// expected case for production orders
	if len(resp.TransactionIds) == 1 {
		return model.MakeTransactionID(resp.TransactionIds[0]), nil
	}

	if len(resp.TransactionIds) > 1 {
		return nil, fmt.Errorf("there was more than 1 transctionId: %s", resp.TransactionIds)
	}

	return nil, fmt.Errorf("no transactionIds returned from order creation")
}

// CancelOrder impl.
func (k *krakenExchange) CancelOrder(txID *model.TransactionID, pair model.TradingPair) (model.CancelOrderResult, error) {
	if k.isSimulated {
		return model.CancelResultCancelSuccessful, nil
	}

	// we don't actually use the pair for kraken
	resp, e := k.nextAPI().CancelOrder(txID.String())
	if e != nil {
		return model.CancelResultFailed, e
	}

	if resp.Count > 1 {
		log.Printf("warning: count from a cancelled order is greater than 1: %d\n", resp.Count)
	}

	// TODO 2 - need to figure out whether count = 0 could also mean that it is pending cancellation
	if resp.Count == 0 {
		return model.CancelResultFailed, nil
	}
	// resp.Count == 1 here

	if resp.Pending {
		return model.CancelResultPending, nil
	}
	return model.CancelResultCancelSuccessful, nil
}

// GetAccountBalances impl.
func (k *krakenExchange) GetAccountBalances(assetList []model.Asset) (map[model.Asset]model.Number, error) {
	balanceResponse, e := k.nextAPI().Balance()
	if e != nil {
		return nil, e
	}

	m := map[model.Asset]model.Number{}
	for _, a := range assetList {
		krakenAssetString, e := k.assetConverter.ToString(a)
		if e != nil {
			// discard partially built map for now
			return nil, e
		}
		bal := getFieldValue(*balanceResponse, krakenAssetString)
		m[a] = *model.NumberFromFloat(bal, precisionBalances)
	}
	return m, nil
}

func getFieldValue(object krakenapi.BalanceResponse, fieldName string) float64 {
	r := reflect.ValueOf(object)
	f := reflect.Indirect(r).FieldByName(fieldName)
	return f.Interface().(float64)
}

// GetOrderConstraints impl
func (k *krakenExchange) GetOrderConstraints(pair *model.TradingPair) *model.OrderConstraints {
	constraints, ok := krakenPrecisionMatrix[*pair]
	if !ok {
		return nil
	}
	return &constraints
}

// GetAssetConverter impl.
func (k *krakenExchange) GetAssetConverter() *model.AssetConverter {
	return k.assetConverter
}

// GetOpenOrders impl.
func (k *krakenExchange) GetOpenOrders(pairs []*model.TradingPair) (map[model.TradingPair][]model.OpenOrder, error) {
	openOrdersResponse, e := k.nextAPI().OpenOrders(map[string]string{})
	if e != nil {
		return nil, e
	}

	// convert to a map so we can easily search for the existence of a trading pair
	// kraken uses different symbols when fetching open orders!
	pairsMap, e := model.TradingPairs2Strings2(k.assetConverterOpenOrders, "", pairs)
	if e != nil {
		return nil, e
	}

	m := map[model.TradingPair][]model.OpenOrder{}
	for ID, o := range openOrdersResponse.Open {
		// kraken uses different symbols when fetching open orders!
		pair, e := model.TradingPairFromString(3, k.assetConverterOpenOrders, o.Description.AssetPair)
		if e != nil {
			return nil, e
		}

		if _, ok := pairsMap[*pair]; !ok {
			// skip open orders for pairs that were not requested
			continue
		}

		if _, ok := m[*pair]; !ok {
			m[*pair] = []model.OpenOrder{}
		}
		if _, ok := m[model.TradingPair{Base: pair.Quote, Quote: pair.Base}]; ok {
			return nil, fmt.Errorf("open orders are listed with repeated base/quote pairs for %s", *pair)
		}

		orderConstraints := k.GetOrderConstraints(pair)
		m[*pair] = append(m[*pair], model.OpenOrder{
			Order: model.Order{
				Pair:        pair,
				OrderAction: model.OrderActionFromString(o.Description.Type),
				OrderType:   model.OrderTypeFromString(o.Description.OrderType),
				Price:       model.MustNumberFromString(o.Description.PrimaryPrice, orderConstraints.PricePrecision),
				Volume:      model.MustNumberFromString(o.Volume, orderConstraints.VolumePrecision),
				Timestamp:   model.MakeTimestamp(int64(o.OpenTime)),
			},
			ID:             ID,
			StartTime:      model.MakeTimestamp(int64(o.StartTime)),
			ExpireTime:     model.MakeTimestamp(int64(o.ExpireTime)),
			VolumeExecuted: model.NumberFromFloat(o.VolumeExecuted, orderConstraints.VolumePrecision),
		})
	}
	return m, nil
}

// GetOrderBook impl.
func (k *krakenExchange) GetOrderBook(pair *model.TradingPair, maxCount int32) (*model.OrderBook, error) {
	pairStr, e := pair.ToString(k.assetConverter, k.delimiter)
	if e != nil {
		return nil, e
	}

	krakenob, e := k.nextAPI().Depth(pairStr, int(maxCount))
	if e != nil {
		return nil, e
	}

	asks := k.readOrders(krakenob.Asks, pair, model.OrderActionSell)
	bids := k.readOrders(krakenob.Bids, pair, model.OrderActionBuy)
	ob := model.MakeOrderBook(pair, asks, bids)
	return ob, nil
}

func (k *krakenExchange) readOrders(obi []krakenapi.OrderBookItem, pair *model.TradingPair, orderAction model.OrderAction) []model.Order {
	orderConstraints := k.GetOrderConstraints(pair)
	orders := []model.Order{}
	for _, item := range obi {
		orders = append(orders, model.Order{
			Pair:        pair,
			OrderAction: orderAction,
			OrderType:   model.OrderTypeLimit,
			Price:       model.NumberFromFloat(item.Price, orderConstraints.PricePrecision),
			Volume:      model.NumberFromFloat(item.Amount, orderConstraints.VolumePrecision),
			Timestamp:   model.MakeTimestamp(item.Ts),
		})
	}
	return orders
}

// GetTickerPrice impl.
func (k *krakenExchange) GetTickerPrice(pairs []model.TradingPair) (map[model.TradingPair]api.Ticker, error) {
	pairsMap, e := model.TradingPairs2Strings(k.assetConverter, k.delimiter, pairs)
	if e != nil {
		return nil, e
	}

	resp, e := k.nextAPI().Ticker(values(pairsMap)...)
	if e != nil {
		return nil, e
	}

	priceResult := map[model.TradingPair]api.Ticker{}
	for _, p := range pairs {
		orderConstraints := k.GetOrderConstraints(&p)
		pairTickerInfo := resp.GetPairTickerInfo(pairsMap[p])
		priceResult[p] = api.Ticker{
			AskPrice: model.MustNumberFromString(pairTickerInfo.Ask[0], orderConstraints.PricePrecision),
			BidPrice: model.MustNumberFromString(pairTickerInfo.Bid[0], orderConstraints.PricePrecision),
		}
	}

	return priceResult, nil
}

// values gives you the values of a map
// TODO 2 - move to autogenerated generic function
func values(m map[model.TradingPair]string) []string {
	values := []string{}
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// GetTradeHistory impl.
func (k *krakenExchange) GetTradeHistory(pair model.TradingPair, maybeCursorStart interface{}, maybeCursorEnd interface{}) (*api.TradeHistoryResult, error) {
	var mcs *int64
	if maybeCursorStart != nil {
		i := maybeCursorStart.(int64)
		mcs = &i
	}

	var mce *int64
	if maybeCursorEnd != nil {
		i := maybeCursorEnd.(int64)
		mce = &i
	}

	return k.getTradeHistory(pair, mcs, mce)
}

func (k *krakenExchange) getTradeHistory(tradingPair model.TradingPair, maybeCursorStart *int64, maybeCursorEnd *int64) (*api.TradeHistoryResult, error) {
	input := map[string]string{}
	if maybeCursorStart != nil {
		input["start"] = strconv.FormatInt(*maybeCursorStart, 10)
	}
	if maybeCursorEnd != nil {
		input["end"] = strconv.FormatInt(*maybeCursorEnd, 10)
	}

	resp, e := k.nextAPI().Query("TradesHistory", input)
	if e != nil {
		return nil, e
	}
	krakenResp := resp.(map[string]interface{})
	krakenTrades := krakenResp["trades"].(map[string]interface{})

	res := api.TradeHistoryResult{Trades: []model.Trade{}}
	for _, v := range krakenTrades {
		m := v.(map[string]interface{})
		_txid := m["ordertxid"].(string)
		_time := m["time"].(float64)
		ts := model.MakeTimestamp(int64(_time))
		_type := m["type"].(string)
		_ordertype := m["ordertype"].(string)
		_price := m["price"].(string)
		_vol := m["vol"].(string)
		_cost := m["cost"].(string)
		_fee := m["fee"].(string)
		_pair := m["pair"].(string)
		pair, e := model.TradingPairFromString(4, k.assetConverter, _pair)
		if e != nil {
			return nil, e
		}
		orderConstraints := k.GetOrderConstraints(pair)
		// for now use the max precision between price and volume for fee and cost
		feeCostPrecision := orderConstraints.PricePrecision
		if orderConstraints.VolumePrecision > feeCostPrecision {
			feeCostPrecision = orderConstraints.VolumePrecision
		}

		if *pair == tradingPair {
			res.Trades = append(res.Trades, model.Trade{
				Order: model.Order{
					Pair:        pair,
					OrderAction: model.OrderActionFromString(_type),
					OrderType:   model.OrderTypeFromString(_ordertype),
					Price:       model.MustNumberFromString(_price, orderConstraints.PricePrecision),
					Volume:      model.MustNumberFromString(_vol, orderConstraints.VolumePrecision),
					Timestamp:   ts,
				},
				TransactionID: model.MakeTransactionID(_txid),
				Cost:          model.MustNumberFromString(_cost, feeCostPrecision),
				Fee:           model.MustNumberFromString(_fee, feeCostPrecision),
			})
		}
		res.Cursor = _time
	}
	return &res, nil
}

// GetTrades impl.
func (k *krakenExchange) GetTrades(pair *model.TradingPair, maybeCursor interface{}) (*api.TradesResult, error) {
	if maybeCursor != nil {
		mc := maybeCursor.(int64)
		return k.getTrades(pair, &mc)
	}
	return k.getTrades(pair, nil)
}

func (k *krakenExchange) getTrades(pair *model.TradingPair, maybeCursor *int64) (*api.TradesResult, error) {
	pairStr, e := pair.ToString(k.assetConverter, k.delimiter)
	if e != nil {
		return nil, e
	}

	var tradesResp *krakenapi.TradesResponse
	if maybeCursor != nil {
		tradesResp, e = k.nextAPI().Trades(pairStr, *maybeCursor)
	} else {
		tradesResp, e = k.nextAPI().Trades(pairStr, -1)
	}
	if e != nil {
		return nil, e
	}

	orderConstraints := k.GetOrderConstraints(pair)
	tradesResult := &api.TradesResult{
		Cursor: tradesResp.Last,
		Trades: []model.Trade{},
	}
	for _, tInfo := range tradesResp.Trades {
		action, e := getAction(tInfo)
		if e != nil {
			return nil, e
		}
		orderType, e := getOrderType(tInfo)
		if e != nil {
			return nil, e
		}

		tradesResult.Trades = append(tradesResult.Trades, model.Trade{
			Order: model.Order{
				Pair:        pair,
				OrderAction: action,
				OrderType:   orderType,
				Price:       model.NumberFromFloat(tInfo.PriceFloat, orderConstraints.PricePrecision),
				Volume:      model.NumberFromFloat(tInfo.VolumeFloat, orderConstraints.VolumePrecision),
				Timestamp:   model.MakeTimestamp(tInfo.Time),
			},
			// TransactionID unavailable
			// Cost unavailable
			// Fee unavailable
		})
	}
	return tradesResult, nil
}

func getAction(tInfo krakenapi.TradeInfo) (model.OrderAction, error) {
	if tInfo.Buy {
		return model.OrderActionBuy, nil
	} else if tInfo.Sell {
		return model.OrderActionSell, nil
	}

	// return OrderActionBuy as nil value
	return model.OrderActionBuy, errors.New("unidentified trade action")
}

func getOrderType(tInfo krakenapi.TradeInfo) (model.OrderType, error) {
	if tInfo.Market {
		return model.OrderTypeMarket, nil
	} else if tInfo.Limit {
		return model.OrderTypeLimit, nil
	}
	return -1, errors.New("unidentified trade action")
}

// GetWithdrawInfo impl.
func (k *krakenExchange) GetWithdrawInfo(
	asset model.Asset,
	amountToWithdraw *model.Number,
	address string,
) (*api.WithdrawInfo, error) {
	krakenAsset, e := k.assetConverter.ToString(asset)
	if e != nil {
		return nil, e
	}

	withdrawKey, e := k.withdrawKeys.getKey(asset, address)
	if e != nil {
		return nil, e
	}
	resp, e := k.nextAPI().Query(
		"WithdrawInfo",
		map[string]string{
			"asset":  krakenAsset,
			"key":    withdrawKey,
			"amount": amountToWithdraw.AsString(),
		},
	)
	if e != nil {
		return nil, e
	}

	return parseWithdrawInfoResponse(resp, amountToWithdraw)
}

func parseWithdrawInfoResponse(resp interface{}, amountToWithdraw *model.Number) (*api.WithdrawInfo, error) {
	switch m := resp.(type) {
	case map[string]interface{}:
		info, e := parseWithdrawInfo(m)
		if e != nil {
			return nil, e
		}
		if info.limit != nil && info.limit.AsFloat() < amountToWithdraw.AsFloat() {
			return nil, api.MakeErrWithdrawAmountAboveLimit(amountToWithdraw, info.limit)
		}
		if info.fee != nil && info.fee.AsFloat() >= amountToWithdraw.AsFloat() {
			return nil, api.MakeErrWithdrawAmountInvalid(amountToWithdraw, info.fee)
		}

		return &api.WithdrawInfo{AmountToReceive: info.amount}, nil
	default:
		return nil, fmt.Errorf("could not parse response type from WithdrawInfo: %s", reflect.TypeOf(m))
	}
}

type withdrawInfo struct {
	limit  *model.Number
	fee    *model.Number
	amount *model.Number
}

func parseWithdrawInfo(m map[string]interface{}) (*withdrawInfo, error) {
	// limit
	limit, e := networking.ParseNumber(m, "limit", "WithdrawInfo")
	if e != nil {
		return nil, e
	}

	// fee
	fee, e := networking.ParseNumber(m, "fee", "WithdrawInfo")
	if e != nil {
		if !strings.HasPrefix(e.Error(), networking.PrefixFieldNotFound) {
			return nil, e
		}
		// fee may be missing in which case it's null
		fee = nil
	}

	// amount
	amount, e := networking.ParseNumber(m, "amount", "WithdrawInfo")
	if e != nil {
		return nil, e
	}

	return &withdrawInfo{
		limit:  limit,
		fee:    fee,
		amount: amount,
	}, nil
}

// PrepareDeposit impl.
func (k *krakenExchange) PrepareDeposit(asset model.Asset, amount *model.Number) (*api.PrepareDepositResult, error) {
	krakenAsset, e := k.assetConverter.ToString(asset)
	if e != nil {
		return nil, e
	}

	dm, e := k.getDepositMethods(krakenAsset)
	if e != nil {
		return nil, e
	}

	if dm.limit != nil && dm.limit.AsFloat() < amount.AsFloat() {
		return nil, api.MakeErrDepositAmountAboveLimit(amount, dm.limit)
	}

	// get any unused address on the account or generate a new address if no existing unused address
	generateNewAddress := false
	for {
		addressList, e := k.getDepositAddress(krakenAsset, dm.method, generateNewAddress)
		if e != nil {
			if strings.Contains(e.Error(), "EFunding:Too many addresses") {
				return nil, api.MakeErrTooManyDepositAddresses()
			}
			return nil, e
		}
		// TODO 2 - filter addresses that may be "in progress" - save suggested address on account before using and filter using that list
		// discard addresses that have been used up
		addressList = keepOnlyNew(addressList)

		if len(addressList) > 0 {
			earliestAddress := addressList[len(addressList)-1]
			return &api.PrepareDepositResult{
				Fee:      dm.fee,
				Address:  earliestAddress.address,
				ExpireTs: earliestAddress.expireTs,
			}, nil
		}

		// error if we just tried to generate a new address which failed
		if generateNewAddress {
			return nil, fmt.Errorf("attempt to generate a new address failed")
		}

		// retry the loop by attempting to generate a new address
		generateNewAddress = true
	}
}

func keepOnlyNew(addressList []depositAddress) []depositAddress {
	ret := []depositAddress{}
	for _, a := range addressList {
		if a.isNew {
			ret = append(ret, a)
		}
	}
	return ret
}

type depositMethod struct {
	method     string
	limit      *model.Number
	fee        *model.Number
	genAddress bool
}

func (k *krakenExchange) getDepositMethods(asset string) (*depositMethod, error) {
	resp, e := k.nextAPI().Query(
		"DepositMethods",
		map[string]string{"asset": asset},
	)
	if e != nil {
		return nil, e
	}

	switch arr := resp.(type) {
	case []interface{}:
		switch m := arr[0].(type) {
		case map[string]interface{}:
			return parseDepositMethods(m)
		default:
			return nil, fmt.Errorf("could not parse inner response type of returned []interface{} from DepositMethods: %s", reflect.TypeOf(m))
		}
	default:
		return nil, fmt.Errorf("could not parse response type from DepositMethods: %s", reflect.TypeOf(arr))
	}
}

type depositAddress struct {
	address  string
	expireTs int64
	isNew    bool
}

func (k *krakenExchange) getDepositAddress(asset string, method string, genAddress bool) ([]depositAddress, error) {
	input := map[string]string{
		"asset":  asset,
		"method": method,
	}
	if genAddress {
		// only set "new" if it's supposed to be 'true'. If you set it to 'false' then it will be treated as true by Kraken :(
		input["new"] = "true"
	}
	resp, e := k.nextAPI().Query("DepositAddresses", input)
	if e != nil {
		return []depositAddress{}, e
	}

	addressList := []depositAddress{}
	switch arr := resp.(type) {
	case []interface{}:
		for _, elem := range arr {
			switch m := elem.(type) {
			case map[string]interface{}:
				da, e := parseDepositAddress(m)
				if e != nil {
					return []depositAddress{}, e
				}
				addressList = append(addressList, *da)
			default:
				return []depositAddress{}, fmt.Errorf("could not parse inner response type of returned []interface{} from DepositAddresses: %s", reflect.TypeOf(m))
			}
		}
	default:
		return []depositAddress{}, fmt.Errorf("could not parse response type from DepositAddresses: %s", reflect.TypeOf(arr))
	}
	return addressList, nil
}

func parseDepositAddress(m map[string]interface{}) (*depositAddress, error) {
	// address
	address, e := networking.ParseString(m, "address", "DepositAddresses")
	if e != nil {
		return nil, e
	}

	// expiretm
	expireN, e := networking.ParseNumber(m, "expiretm", "DepositAddresses")
	if e != nil {
		return nil, e
	}
	expireTs := int64(expireN.AsFloat())

	// new
	isNew, e := networking.ParseBool(m, "new", "DepositAddresses")
	if e != nil {
		if !strings.HasPrefix(e.Error(), networking.PrefixFieldNotFound) {
			return nil, e
		}
		// new may be missing in which case it's false
		isNew = false
	}

	return &depositAddress{
		address:  address,
		expireTs: expireTs,
		isNew:    isNew,
	}, nil
}

func parseDepositMethods(m map[string]interface{}) (*depositMethod, error) {
	// method
	method, e := networking.ParseString(m, "method", "DepositMethods")
	if e != nil {
		return nil, e
	}

	// limit
	var limit *model.Number
	limB, e := networking.ParseBool(m, "limit", "DepositMethods")
	if e != nil {
		// limit is special as it can be a boolean or a number
		limit, e = networking.ParseNumber(m, "limit", "DepositMethods")
		if e != nil {
			return nil, e
		}
	} else {
		if limB {
			return nil, fmt.Errorf("invalid value for 'limit' as a response from DepositMethods: boolean value of 'limit' should never be 'true' as it should be a number in that case")
		}
		limit = nil
	}

	// fee
	fee, e := networking.ParseNumber(m, "fee", "DepositMethods")
	if e != nil {
		if !strings.HasPrefix(e.Error(), networking.PrefixFieldNotFound) {
			return nil, e
		}
		// fee may be missing in which case it's null
		fee = nil
	}

	// gen-address
	genAddress, e := networking.ParseBool(m, "gen-address", "DepositMethods")
	if e != nil {
		return nil, e
	}

	return &depositMethod{
		method:     method,
		limit:      limit,
		fee:        fee,
		genAddress: genAddress,
	}, nil
}

// WithdrawFunds impl.
func (k *krakenExchange) WithdrawFunds(
	asset model.Asset,
	amountToWithdraw *model.Number,
	address string,
) (*api.WithdrawFunds, error) {
	krakenAsset, e := k.assetConverter.ToString(asset)
	if e != nil {
		return nil, e
	}

	withdrawKey, e := k.withdrawKeys.getKey(asset, address)
	if e != nil {
		return nil, e
	}
	resp, e := k.nextAPI().Query(
		"Withdraw",
		map[string]string{
			"asset":  krakenAsset,
			"key":    withdrawKey,
			"amount": amountToWithdraw.AsString(),
		},
	)
	if e != nil {
		return nil, e
	}

	return parseWithdrawResponse(resp)
}

func parseWithdrawResponse(resp interface{}) (*api.WithdrawFunds, error) {
	switch m := resp.(type) {
	case map[string]interface{}:
		refid, e := networking.ParseString(m, "refid", "Withdraw")
		if e != nil {
			return nil, e
		}
		return &api.WithdrawFunds{
			WithdrawalID: refid,
		}, nil
	default:
		return nil, fmt.Errorf("could not parse response type from Withdraw: %s", reflect.TypeOf(m))
	}
}

// krakenPrecisionMatrix describes the price and volume precision and min base volume for each trading pair
// taken from this URL: https://support.kraken.com/hc/en-us/articles/360001389366-Price-and-volume-decimal-precision
var krakenPrecisionMatrix = map[model.TradingPair]model.OrderConstraints{
	*model.MakeTradingPair(model.XLM, model.USD): *model.MakeOrderConstraints(6, 8, 30.0),
	*model.MakeTradingPair(model.XLM, model.BTC): *model.MakeOrderConstraints(8, 8, 30.0),
	*model.MakeTradingPair(model.BTC, model.USD): *model.MakeOrderConstraints(1, 8, 0.002),
}
