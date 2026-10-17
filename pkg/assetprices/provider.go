package assetprices

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// priceRequestTimeout is the timeout of one request to a price source, in milliseconds
const priceRequestTimeout uint32 = 30000

// priceRequestProxy makes price sources use the proxy of the system environment, like the other outgoing requests of the app
const priceRequestProxy = "system"

// Quote represents the latest price of one asset
type Quote struct {
	Symbol    string
	Price     float64
	Currency  string
	PriceTime int64
}

// PriceProvider defines the structure of an asset price source for one or more markets.
// A market without a registered provider has no automatic price and relies on manually entered prices.
type PriceProvider interface {
	// Name returns the name stored as the source of the prices it provides
	Name() string

	// Markets returns the markets this provider can price
	Markets() []models.AssetMarket

	// GetLatestQuotes returns the latest quotes of the symbols in the market.
	// Symbols the source does not know are left out of the result instead of failing the whole call.
	GetLatestQuotes(c core.Context, market models.AssetMarket, symbols []string) ([]*Quote, error)
}
