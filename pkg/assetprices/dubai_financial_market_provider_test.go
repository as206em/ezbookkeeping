package assetprices

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

const dfmEmaarResponse = `[{"Date":"16-10-2026","Open":"11.40","High":"11.56","Low":"11.40","Trades":"1,775","Volume":"16,041,860","Value":"184,107,423.20","Last":"11.46","Previous":"11.40","Closing":"11.46","Change":"0.06","ChangePercentage":"0.53%","LastTradeTime":"16-10-2026"},` +
	`{"Date":"15-10-2026","Open":"11.38","High":"11.42","Low":"11.32","Trades":"1,387","Volume":"13,494,233","Value":"153,639,827.70","Last":"11.40","Previous":"11.40","Closing":"11.40","Change":"0.00","ChangePercentage":"0.00%","LastTradeTime":"15-10-2026"}]`

func newTestDubaiFinancialMarketProvider(t *testing.T, handler http.HandlerFunc) *DubaiFinancialMarketProvider {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	provider := newDubaiFinancialMarketProvider(server.Client(), server.URL)
	provider.now = func() time.Time { return time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC) }

	return provider
}

func TestDubaiFinancialMarketProvider_ReturnsLatestRowAsAedQuote(t *testing.T) {
	provider := newTestDubaiFinancialMarketProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(dfmEmaarResponse))
	})

	quotes, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_DFM, []string{"EMAAR"})

	require.NoError(t, err)
	require.Len(t, quotes, 1)
	assert.Equal(t, "EMAAR", quotes[0].Symbol)
	assert.Equal(t, 11.46, quotes[0].Price)
	assert.Equal(t, "AED", quotes[0].Currency)
	assert.Equal(t, time.Date(2026, 10, 16, 15, 0, 0, 0, time.FixedZone("GST", 4*3600)).Unix(), quotes[0].PriceTime)
	assert.Equal(t, "2026-10-16", time.Unix(quotes[0].PriceTime, 0).UTC().Format("2006-01-02"), "the trading day must not shift when shown in UTC")
}

func TestDubaiFinancialMarketProvider_SendsCompanyAndDateRange(t *testing.T) {
	var form url.Values
	var method, contentType string

	provider := newTestDubaiFinancialMarketProvider(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ = url.ParseQuery(string(body))
		method = r.Method
		contentType = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte(dfmEmaarResponse))
	})

	_, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_DFM, []string{"EMAAR"})

	require.NoError(t, err)
	assert.Equal(t, "POST", method)
	assert.Equal(t, "application/x-www-form-urlencoded", contentType)
	assert.Equal(t, "SearchProfileDailyTab", form.Get("Command"))
	assert.Equal(t, "emaar", form.Get("Company"))
	assert.Equal(t, "custom", form.Get("Period"))
	assert.Equal(t, "17/10/2026", form.Get("toDate"))
	assert.Equal(t, "07/10/2026", form.Get("fromDate"))
}

func TestDubaiFinancialMarketProvider_LeavesOutSymbolsWithoutTradingDays(t *testing.T) {
	provider := newTestDubaiFinancialMarketProvider(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))

		if form.Get("Company") == "emaar" {
			_, _ = w.Write([]byte(dfmEmaarResponse))
			return
		}

		_, _ = w.Write([]byte(`[]`))
	})

	quotes, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_DFM, []string{"EMAAR", "NOSUCH"})

	require.NoError(t, err)
	require.Len(t, quotes, 1)
	assert.Equal(t, "EMAAR", quotes[0].Symbol)
}

func TestDubaiFinancialMarketProvider_FallsBackToClosingAndReadsThousandSeparators(t *testing.T) {
	provider := newTestDubaiFinancialMarketProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"Date":"16-10-2026","Last":"","Closing":"1,234.50"}]`))
	})

	quotes, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_DFM, []string{"BIG"})

	require.NoError(t, err)
	require.Len(t, quotes, 1)
	assert.Equal(t, 1234.5, quotes[0].Price)
}

func TestDubaiFinancialMarketProvider_FailsWhenEverySymbolFails(t *testing.T) {
	provider := newTestDubaiFinancialMarketProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	quotes, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_DFM, []string{"EMAAR"})

	assert.Error(t, err)
	assert.Nil(t, quotes)
}

func TestDubaiFinancialMarketProvider_FailsOnUnexpectedBody(t *testing.T) {
	provider := newTestDubaiFinancialMarketProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>maintenance</html>`))
	})

	_, err := provider.GetLatestQuotes(core.NewNullContext(), models.ASSET_MARKET_DFM, []string{"EMAAR"})

	assert.Error(t, err)
}

func TestDubaiFinancialMarketProvider_Markets(t *testing.T) {
	provider := newDubaiFinancialMarketProvider(http.DefaultClient, "")

	assert.Equal(t, []models.AssetMarket{models.ASSET_MARKET_DFM}, provider.Markets())
	assert.Equal(t, "dfm.ae", provider.Name())
}
