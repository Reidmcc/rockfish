package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon/base"
)

// Common Utilities needed by various bots

// Native is the string representing the type for the native lumen asset
const Native = "native"

// NativeAsset represents the native asset
var NativeAsset = horizon.Asset{Type: Native}

// SdexPrecision defines the number of decimals used in SDEX
const SdexPrecision int8 = 7

// ByPrice implements sort.Interface for []horizon.Offer based on the price
type ByPrice []horizon.Offer

func (a ByPrice) Len() int      { return len(a) }
func (a ByPrice) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByPrice) Less(i, j int) bool {
	return PriceAsFloat(a[i].Price) < PriceAsFloat(a[j].Price)
}

// PriceAsFloat converts a string price to a float price
func PriceAsFloat(price string) float64 {
	p, err := strconv.ParseFloat(price, 64)
	if err != nil {
		log.Printf("Error parsing price: %s | %s\n", price, err)
		return 0
	}
	return p
}

// AmountStringAsFloat converts a string amount to a float amount
func AmountStringAsFloat(amount string) float64 {
	if amount == "" {
		return 0
	}
	p, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		log.Printf("Error parsing amount: %s | %s\n", amount, err)
		return 0
	}
	return p
}

// GetPrice gets the price from an offer
func GetPrice(offer horizon.Offer) float64 {
	if int64(offer.PriceR.D) == 0 {
		return 0.0
	}
	return PriceAsFloat(big.NewRat(int64(offer.PriceR.N), int64(offer.PriceR.D)).FloatString(10))
}

// GetInvertedPrice gets the inverted price from an offer
func GetInvertedPrice(offer horizon.Offer) float64 {
	if int64(offer.PriceR.N) == 0 {
		return 0.0
	}
	return PriceAsFloat(big.NewRat(int64(offer.PriceR.D), int64(offer.PriceR.N)).FloatString(10))
}

// Asset2Asset is a Boyz2Men cover band on the blockchain
func Asset2Asset(Asset horizon.Asset) build.Asset {
	a := build.Asset{}

	a.Code = Asset.Code
	a.Issuer = Asset.Issuer
	if Asset.Type == Native {
		a.Native = true
	}
	return a
}

// Asset2Asset2 converts a build.Asset to a horizon.Asset
func Asset2Asset2(Asset build.Asset) horizon.Asset {
	a := horizon.Asset{}

	a.Code = Asset.Code
	a.Issuer = Asset.Issuer
	if Asset.Native {
		a.Type = Native
	} else if len(a.Code) > 4 {
		a.Type = "credit_alphanum12"
	} else {
		a.Type = "credit_alphanum4"
	}
	return a
}

// Asset2String converts a horizon.Asset to a string representation, using "native" for the native XLM
func Asset2String(asset horizon.Asset) string {
	if asset.Type == Native {
		return Native
	}
	return fmt.Sprintf("%s:%s", asset.Code, asset.Issuer)
}

// Asset2CodeString extracts the code out of a horizon.Asset
func Asset2CodeString(asset horizon.Asset) string {
	if asset.Type == Native {
		return "XLM"
	}
	return asset.Code
}

// String2Asset converts a code:issuer to a horizon.Asset
func String2Asset(code string, issuer string) horizon.Asset {
	if code == "XLM" {
		return Asset2Asset2(build.NativeAsset())
	}
	return Asset2Asset2(build.CreditAsset(code, issuer))
}

// LoadAllOffers loads all the offers for a given account
func LoadAllOffers(account string, api *horizon.Client) (offersRet []horizon.Offer, err error) {
	// get what orders are outstanding now
	offersPage, err := api.LoadAccountOffers(account)
	if err != nil {
		log.Printf("Can't load offers: %s\n", err)
		return
	}
	offersRet = offersPage.Embedded.Records
	for len(offersPage.Embedded.Records) > 0 {
		offersPage, err = api.LoadAccountOffers(account, horizon.At(offersPage.Links.Next.Href))
		if err != nil {
			log.Printf("Can't load offers: %s\n", err)
			return
		}
		offersRet = append(offersRet, offersPage.Embedded.Records...)
	}
	return
}

// FilterOffers filters out the offers into selling and buying, where sellOffers sells the sellAsset and buyOffers buys the sellAsset
func FilterOffers(offers []horizon.Offer, sellAsset horizon.Asset, buyAsset horizon.Asset) (sellOffers []horizon.Offer, buyOffers []horizon.Offer) {
	for _, offer := range offers {
		if offer.Selling == sellAsset {
			if offer.Buying == buyAsset {
				sellOffers = append(sellOffers, offer)
			}
		} else if offer.Selling == buyAsset {
			if offer.Buying == sellAsset {
				buyOffers = append(buyOffers, offer)
			}
		}
	}
	return
}

// ParseSecret returns the address from the secret
func ParseSecret(secret string) (*string, error) {
	if secret == "" {
		return nil, nil
	}

	sourceKP, err := keypair.Parse(secret)
	if err != nil {
		return nil, err
	}

	address := sourceKP.Address()
	return &address, nil
}

// ParseNetwork checks the horizon url and returns the test network if it contains "test"
func ParseNetwork(horizonURL string) build.Network {
	if strings.Contains(horizonURL, "test") {
		return build.TestNetwork
	}
	return build.PublicNetwork
}

// GetJSON is a helper method to get json from a URL
func GetJSON(client http.Client, url string, target interface{}) error {
	r, err := client.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

// GetCreditBalance is a drop-in for the function in the GoSDK, we want it to return nil if there's no balance (as opposed to "0")
func GetCreditBalance(a horizon.Account, code string, issuer string) *string {
	for _, balance := range a.Balances {
		if balance.Asset.Code == code && balance.Asset.Issuer == issuer {
			return &balance.Balance
		}
	}
	return nil
}

// AssetsEqual is a convenience method to compare horizon.Asset and base.Asset because they are not type aliased
func AssetsEqual(baseAsset base.Asset, horizonAsset horizon.Asset) bool {
	return horizonAsset.Type == baseAsset.Type &&
		horizonAsset.Code == baseAsset.Code &&
		horizonAsset.Issuer == baseAsset.Issuer
}

// CheckFetchFloat tries to fetch and then cast the value for the provided key
func CheckFetchFloat(m map[string]interface{}, key string) (float64, error) {
	v, ok := m[key]
	if !ok {
		return 0.0, fmt.Errorf("'%s' field not in map", key)
	}

	f, ok := v.(float64)
	if !ok {
		return 0.0, fmt.Errorf("unable to cast '%s' field to float64, value: %v", key, v)
	}

	return f, nil
}

// CheckedString returns "<nil>" if the object is nil, otherwise calls the String() function on the object
func CheckedString(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", v)
}
