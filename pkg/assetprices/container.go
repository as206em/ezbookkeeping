package assetprices

import (
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// PriceProviderContainer contains the price providers by market
type PriceProviderContainer struct {
	providers map[models.AssetMarket]PriceProvider
}

// Initialize a price provider container singleton instance
var (
	Container = NewPriceProviderContainer()
)

// NewPriceProviderContainer creates an empty price provider container
func NewPriceProviderContainer() *PriceProviderContainer {
	return &PriceProviderContainer{
		providers: make(map[models.AssetMarket]PriceProvider),
	}
}

// InitializePriceProviders registers the built-in price providers.
// To use another price source for a market, register a different PriceProvider for it here.
func InitializePriceProviders(config *settings.Config) {
	Container.Register(NewDubaiFinancialMarketProvider(config))
	Container.Register(NewBinanceProvider(config))
}

// Register sets the provider of every market it supports, replacing any earlier provider of those markets
func (c *PriceProviderContainer) Register(provider PriceProvider) {
	for _, market := range provider.Markets() {
		c.providers[market] = provider
	}
}

// GetProvider returns the provider of the market, or nil when the market has no automatic price source
func (c *PriceProviderContainer) GetProvider(market models.AssetMarket) PriceProvider {
	return c.providers[market]
}
