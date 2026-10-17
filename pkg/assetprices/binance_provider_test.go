package assetprices

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

const binanceTickerResponse = `[{"symbol":"BTCUSDT","price":"109200.50000000"},{"symbol":"ETHUSDT","price":"2545.10000000"},{"symbol":"ETHBTC","price":"0.02330000"}]`

func newTestBinanceProvider(t *testing.T, handler http.HandlerFunc) *BinanceProvider {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	provider := newBinanceProvider(server.Client(), server.URL)
	provider.now = func() time.Time { return time.Unix(1792219000, 0) }

	return provider
}

func TestBinanceProvider_ReturnsUsdQuotesForKnownCoins(t *testing.T) {
	provider := newTestBinanceProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(binanceTickerResponse))
	})

	quotes, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_CRYPTO, []string{"BTC", "ETH", "NOSUCH"})

	require.NoError(t, err)
	require.Len(t, quotes, 2)
	assert.Equal(t, "BTC", quotes[0].Symbol)
	assert.Equal(t, 109200.5, quotes[0].Price)
	assert.Equal(t, "USD", quotes[0].Currency)
	assert.Equal(t, int64(1792219000), quotes[0].PriceTime)
	assert.Equal(t, "ETH", quotes[1].Symbol)
	assert.Equal(t, 2545.1, quotes[1].Price)
}

func TestBinanceProvider_FailsOnHttpError(t *testing.T) {
	provider := newTestBinanceProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	quotes, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_CRYPTO, []string{"BTC"})

	assert.Error(t, err)
	assert.Nil(t, quotes)
}

func TestBinanceProvider_FailsOnUnexpectedBody(t *testing.T) {
	provider := newTestBinanceProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":-1003,"msg":"too many requests"}`))
	})

	_, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_CRYPTO, []string{"BTC"})

	assert.Error(t, err)
}

func TestPriceProviderContainer_UsesRegisteredProviderPerMarket(t *testing.T) {
	container := NewPriceProviderContainer()
	dfm := newDubaiFinancialMarketProvider(http.DefaultClient, "")
	binance := newBinanceProvider(http.DefaultClient, "")

	container.Register(dfm)
	container.Register(binance)

	assert.Same(t, dfm, container.GetProvider(models.ASSET_MARKET_DFM))
	assert.Same(t, binance, container.GetProvider(models.ASSET_MARKET_CRYPTO))
	assert.Nil(t, container.GetProvider(models.ASSET_MARKET_ADX), "ADX has no automatic source")
	assert.Nil(t, container.GetProvider(models.ASSET_MARKET_MANUAL))
}

func TestProductionProvidersUseSaneHttpClients(t *testing.T) {
	config := &settings.Config{}

	for _, client := range []*http.Client{NewDubaiFinancialMarketProvider(config).httpClient, NewBinanceProvider(config).httpClient} {
		assert.NotNil(t, client.Transport)
		assert.Equal(t, 30*time.Second, client.Timeout, "timeout is configured in milliseconds")
	}

	assert.Equal(t, "system", priceRequestProxy, "an empty proxy value is treated as a broken proxy address")
}
