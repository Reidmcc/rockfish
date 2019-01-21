package simplepath

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	sq "github.com/Masterminds/squirrel"
	"github.com/stellar/go/services/horizon/internal/db2/core"
	"github.com/stellar/go/xdr"
)

// ErrNotEnough represents an error that occurs when pricing a trade on an
// orderbook.  This error occurs when the orderbook cannot fulfill the
// requested amount.
var ErrNotEnough = errors.New("not enough depth")

// orderbook represents a one-way orderbook that is selling you a specific asset (ob.Selling)
type orderBook struct {
	Selling xdr.Asset // the offers are selling this asset
	Buying  xdr.Asset // the offers are buying this asset
	Q       *core.Q
}

// CostToConsumeLiquidity returns the buyingAmount (ob.Buying) needed to consume the sellingAmount (ob.Selling)
func (ob *orderBook) CostToConsumeLiquidity(sellingAmount xdr.Int64) (xdr.Int64, error) {
	// load orderbook from core's db
	sql, e := ob.query()
	if e != nil {
		return 0, e
	}
	rows, e := ob.Q.Query(sql)
	if e != nil {
		return 0, e
	}
	defer rows.Close()

	// remaining is the units of ob.Selling that we want to consume
	remaining := int64(sellingAmount)
	var buyingAmount int64
	for rows.Next() {
		// load data from the row
		var offerAmount, pricen, priced, offerid int64
		e = rows.Scan(&offerAmount, &pricen, &priced, &offerid)
		if e != nil {
			return 0, e
		}

		buyingUnitsExtracted, sellingUnitsExtracted, e := convertToBuyingUnits(offerAmount, remaining, pricen, priced)
		if e != nil {
			return 0, e
		}
		// overflow check
		if willAddOverflow(buyingAmount, buyingUnitsExtracted) {
			return xdr.Int64(0), fmt.Errorf("adding these two values will cause an integer overflow: %d, %d", buyingAmount, buyingUnitsExtracted)
		}
		buyingAmount += buyingUnitsExtracted
		remaining -= sellingUnitsExtracted

		// check if we got all the units we wanted
		if remaining <= 0 {
			return xdr.Int64(buyingAmount), nil
		}
	}
	return 0, ErrNotEnough
}

func willAddOverflow(a int64, b int64) bool {
	return a > math.MaxInt64-b
}

func (ob *orderBook) query() (sq.SelectBuilder, error) {
	var (
		// selling/buying types
		st, bt xdr.AssetType
		// selling/buying codes
		sc, bc string
		// selling/buying issuers
		si, bi string
	)
	e := ob.Selling.Extract(&st, &sc, &si)
	if e != nil {
		return sq.SelectBuilder{}, e
	}
	e = ob.Buying.Extract(&bt, &bc, &bi)
	if e != nil {
		return sq.SelectBuilder{}, e
	}

	sql := sq.
		Select("amount", "pricen", "priced", "offerid").
		From("offers").
		Where(sq.Eq{
			"sellingassettype":               st,
			"COALESCE(sellingassetcode, '')": sc,
			"COALESCE(sellingissuer, '')":    si}).
		Where(sq.Eq{
			"buyingassettype":               bt,
			"COALESCE(buyingassetcode, '')": bc,
			"COALESCE(buyingissuer, '')":    bi}).
		OrderBy("price ASC")
	return sql, nil
}

// convertToBuyingUnits uses special rounding logic to multiply the amount by the price and returns (buyingUnits, sellingUnits) that can be taken from the offer
/*
	offerSellingBound = (offer.price.n > offer.price.d)
		? offer.amount : ceil(floor(offer.amount * offer.price) / offer.price)
	pathPaymentAmountBought = min(offerSellingBound, pathPaymentBuyingBound)
	pathPaymentAmountSold = ceil(pathPaymentAmountBought * offer.price)

	offer.amount = amount selling
	offerSellingBound = roundingCorrectedOffer
	pathPaymentBuyingBound = needed
	pathPaymentAmountBought = what we are consuming from offer
	pathPaymentAmountSold = amount we are giving to the buyer
	Sell units = pathPaymentAmountSold and buy units = pathPaymentAmountBought

	this is how we do floor and ceiling in stellar-core:
	https://github.com/stellar/stellar-core/blob/9af27ef4e20b66f38ab148d52ba7904e74fe502f/src/util/types.cpp#L201
*/
func convertToBuyingUnits(sellingOfferAmount int64, sellingUnitsNeeded int64, pricen int64, priced int64) (int64, int64, error) {
	var e error
	// offerSellingBound
	result := sellingOfferAmount
	if pricen <= priced {
		result, e = mulFractionRoundDown(sellingOfferAmount, pricen, priced)
		if e != nil {
			return 0, 0, e
		}
		result, e = mulFractionRoundUp(result, priced, pricen)
		if e != nil {
			return 0, 0, e
		}
	}

	// pathPaymentAmountBought
	result = min(result, sellingUnitsNeeded)
	sellingUnitsExtracted := result

	// pathPaymentAmountSold
	result, e = mulFractionRoundUp(result, pricen, priced)
	if e != nil {
		return 0, 0, e
	}

	return result, sellingUnitsExtracted, nil
}

// mulFractionRoundDown sets x = (x * n) / d, which is a round-down operation
// see https://github.com/stellar/stellar-core/blob/9af27ef4e20b66f38ab148d52ba7904e74fe502f/src/util/types.cpp#L201
func mulFractionRoundDown(x int64, n int64, d int64) (int64, error) {
	var bn, bd big.Int
	bn.SetInt64(n)
	bd.SetInt64(d)
	var r big.Int

	r.SetInt64(x)
	r.Mul(&r, &bn)
	r.Quo(&r, &bd)

	return toInt64Checked(r)
}

// mulFractionRoundUp sets x = ((x * n) + d - 1) / d, which is a round-up operation
// see https://github.com/stellar/stellar-core/blob/9af27ef4e20b66f38ab148d52ba7904e74fe502f/src/util/types.cpp#L201
func mulFractionRoundUp(x int64, n int64, d int64) (int64, error) {
	var bn, bd big.Int
	bn.SetInt64(n)
	bd.SetInt64(d)
	var one big.Int
	one.SetInt64(1)
	var r big.Int

	r.SetInt64(x)
	r.Mul(&r, &bn)
	r.Add(&r, &bd)
	r.Sub(&r, &one)
	r.Quo(&r, &bd)

	return toInt64Checked(r)
}

// min impl for int64
func min(x int64, y int64) int64 {
	if x <= y {
		return x
	}
	return y
}

func toInt64Checked(x big.Int) (int64, error) {
	if x.IsInt64() {
		return x.Int64(), nil
	}
	return 0, fmt.Errorf("cannot convert big.Int value to int64")
}
