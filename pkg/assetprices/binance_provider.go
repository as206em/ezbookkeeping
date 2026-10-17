package assetprices

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/httpclient"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

const binanceTickerPriceUrl = "https://api.binance.com/api/v3/ticker/price"
const binanceQuoteAsset = "USDT"

// BinanceProvider reads spot prices from Binance's public ticker endpoint (no API key needed).
// Coins are priced in USDT, which is treated as USD.
type BinanceProvider struct {
	httpClient *http.Client
	tickerUrl  string
	now        func() time.Time
}

type binanceTickerPrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

// NewBinanceProvider creates a new Binance price provider
func NewBinanceProvider(config *settings.Config) *BinanceProvider {
	return newBinanceProvider(httpclient.NewHttpClient(priceRequestTimeout, priceRequestProxy, false, core.GetOutgoingUserAgent(), config.EnableDebugLog), binanceTickerPriceUrl)
}

func newBinanceProvider(client *http.Client, tickerUrl string) *BinanceProvider {
	return &BinanceProvider{
		httpClient: client,
		tickerUrl:  tickerUrl,
		now:        time.Now,
	}
}

// Name returns the name of the price source
func (p *BinanceProvider) Name() string {
	return "binance.com"
}

// Markets returns the markets this provider can price
func (p *BinanceProvider) Markets() []models.AssetMarket {
	return []models.AssetMarket{models.ASSET_MARKET_CRYPTO}
}

// GetLatestQuotes returns the latest USD prices of the coins
func (p *BinanceProvider) GetLatestQuotes(c core.Context, market models.AssetMarket, symbols []string) ([]*Quote, error) {
	req, err := http.NewRequest("GET", p.tickerUrl, nil)

	if err != nil {
		log.Errorf(c, "[binance_provider.GetLatestQuotes] failed to build request, because %s", err.Error())
		return nil, errs.ErrFailedToRequestRemoteApi
	}

	req = req.WithContext(httpclient.CustomHttpResponseLog(c, func(data []byte) {
		log.Debugf(c, "[binance_provider.GetLatestQuotes] response is %s", data)
	}))

	resp, err := p.httpClient.Do(req)

	if err != nil {
		log.Errorf(c, "[binance_provider.GetLatestQuotes] failed to request prices, because %s", err.Error())
		return nil, errs.ErrFailedToRequestRemoteApi
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil || resp.StatusCode != http.StatusOK {
		log.Errorf(c, "[binance_provider.GetLatestQuotes] failed to get prices, response code is %d", resp.StatusCode)
		return nil, errs.ErrFailedToRequestRemoteApi
	}

	quotes, err := p.parseQuotes(symbols, body)

	if err != nil {
		log.Errorf(c, "[binance_provider.GetLatestQuotes] failed to parse prices, because %s", err.Error())
		return nil, errs.ErrFailedToRequestRemoteApi
	}

	return quotes, nil
}

func (p *BinanceProvider) parseQuotes(symbols []string, body []byte) ([]*Quote, error) {
	var tickers []*binanceTickerPrice
	err := json.Unmarshal(body, &tickers)

	if err != nil {
		return nil, err
	}

	pricesByPair := make(map[string]string, len(tickers))

	for _, ticker := range tickers {
		pricesByPair[ticker.Symbol] = ticker.Price
	}

	now := p.now().Unix()
	quotes := make([]*Quote, 0, len(symbols))

	for _, symbol := range symbols {
		priceText, exists := pricesByPair[binanceTradingPair(symbol)]

		if !exists {
			continue
		}

		price, err := strconv.ParseFloat(priceText, 64)

		if err != nil || price <= 0 {
			return nil, fmt.Errorf("price of \"%s\" is invalid", symbol)
		}

		quotes = append(quotes, &Quote{
			Symbol:    symbol,
			Price:     price,
			Currency:  "USD",
			PriceTime: now,
		})
	}

	return quotes, nil
}

func binanceTradingPair(symbol string) string {
	return strings.ToUpper(symbol) + binanceQuoteAsset
}
