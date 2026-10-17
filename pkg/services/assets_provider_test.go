package services

import (
	"testing"

	"github.com/mayswind/ezbookkeeping/pkg/assetprices"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

type fakeAssetPriceProvider struct {
	quotes    map[string]float64
	err       error
	requested []string
}

func (p *fakeAssetPriceProvider) Name() string {
	return "fake.test"
}

func (p *fakeAssetPriceProvider) Markets() []models.AssetMarket {
	return []models.AssetMarket{models.ASSET_MARKET_DFM}
}

func (p *fakeAssetPriceProvider) GetLatestQuotes(c core.Context, market models.AssetMarket, symbols []string) ([]*assetprices.Quote, error) {
	p.requested = append(p.requested, symbols...)

	if p.err != nil {
		return nil, p.err
	}

	var quotes []*assetprices.Quote

	for _, symbol := range symbols {
		if price, exists := p.quotes[symbol]; exists {
			quotes = append(quotes, &assetprices.Quote{Symbol: symbol, Price: price, Currency: "AED", PriceTime: 1000})
		}
	}

	return quotes, nil
}

// registerFakeAssetPriceProvider replaces the DFM provider for the duration of the test and leaves ADX without one
func registerFakeAssetPriceProvider(t *testing.T, provider assetprices.PriceProvider) {
	t.Helper()

	previous := assetprices.Container
	assetprices.Container = assetprices.NewPriceProviderContainer()
	assetprices.Container.Register(provider)
	t.Cleanup(func() { assetprices.Container = previous })
}
