package api

import "log"

// PriceFeed allows you to fetch the price of a feed
type PriceFeed interface {
	GetPrice() (float64, error)
}

// FeedPair is the struct representing a price feed for a trading pair
type FeedPair struct {
	FeedA PriceFeed
	FeedB PriceFeed
}

// GetCenterPrice fetches the center price from this feed
func (p *FeedPair) GetCenterPrice() (float64, error) {
	pA, err := p.FeedA.GetPrice()
	if err != nil {
		return 0, err
	}

	var pB float64
	pB, err = p.FeedB.GetPrice()
	if err != nil {
		return 0, err
	}

	centerPrice := pA / pB
	log.Printf("feedPair prices: feedA=%.7f, feedB=%.7f; centerPrice=%.7f\n", pA, pB, centerPrice)
	return centerPrice, nil
}
