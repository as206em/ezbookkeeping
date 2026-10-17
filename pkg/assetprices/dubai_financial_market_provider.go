package assetprices

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

const dubaiFinancialMarketWidgetDataUrl = "https://api2.dfm.ae/web/widgets/v1/data"
const dubaiFinancialMarketHistoryDays = 10
const dubaiFinancialMarketDateLayout = "02/01/2006"

// dubaiFinancialMarketCloseHour is the hour (UAE time) a trading day's closing price is stamped with, so the day reads
// the same in every time zone instead of rolling back to the previous day for viewers west of the UAE
const dubaiFinancialMarketCloseHour = 15

// DubaiFinancialMarketProvider reads delayed DFM closing prices from the daily summary data behind dfm.ae company pages.
// It uses an unofficial endpoint, so any change by DFM can break it; prices can then be entered manually.
type DubaiFinancialMarketProvider struct {
	httpClient *http.Client
	dataUrl    string
	location   *time.Location
	now        func() time.Time
}

type dubaiFinancialMarketDailyRow struct {
	Date    string `json:"Date"`
	Last    string `json:"Last"`
	Closing string `json:"Closing"`
}

// NewDubaiFinancialMarketProvider creates a new DFM price provider
func NewDubaiFinancialMarketProvider(config *settings.Config) *DubaiFinancialMarketProvider {
	return newDubaiFinancialMarketProvider(httpclient.NewHttpClient(priceRequestTimeout, priceRequestProxy, false, core.GetOutgoingUserAgent(), config.EnableDebugLog), dubaiFinancialMarketWidgetDataUrl)
}

func newDubaiFinancialMarketProvider(client *http.Client, dataUrl string) *DubaiFinancialMarketProvider {
	location, err := time.LoadLocation("Asia/Dubai")

	if err != nil {
		location = time.FixedZone("GST", 4*60*60)
	}

	return &DubaiFinancialMarketProvider{
		httpClient: client,
		dataUrl:    dataUrl,
		location:   location,
		now:        time.Now,
	}
}

// Name returns the name of the price source
func (p *DubaiFinancialMarketProvider) Name() string {
	return "dfm.ae"
}

// Markets returns the markets this provider can price
func (p *DubaiFinancialMarketProvider) Markets() []models.AssetMarket {
	return []models.AssetMarket{models.ASSET_MARKET_DFM}
}

// GetLatestQuotes returns the latest DFM prices of the symbols in AED
func (p *DubaiFinancialMarketProvider) GetLatestQuotes(c core.Context, market models.AssetMarket, symbols []string) ([]*Quote, error) {
	quotes := make([]*Quote, 0, len(symbols))
	var firstErr error

	for _, symbol := range symbols {
		quote, err := p.getLatestQuote(c, symbol)

		if err != nil {
			log.Warnf(c, "[dubai_financial_market_provider.GetLatestQuotes] failed to get price of \"%s\", because %s", symbol, err.Error())

			if firstErr == nil {
				firstErr = err
			}

			continue
		}

		if quote != nil {
			quotes = append(quotes, quote)
		}
	}

	if len(quotes) == 0 && firstErr != nil {
		return nil, errs.ErrFailedToRequestRemoteApi
	}

	return quotes, nil
}

func (p *DubaiFinancialMarketProvider) getLatestQuote(c core.Context, symbol string) (*Quote, error) {
	today := p.now().In(p.location)
	form := url.Values{}
	form.Set("Command", "SearchProfileDailyTab")
	form.Set("Language", "en")
	form.Set("Company", strings.ToLower(symbol))
	form.Set("Period", "custom")
	form.Set("fromDate", today.AddDate(0, 0, -dubaiFinancialMarketHistoryDays).Format(dubaiFinancialMarketDateLayout))
	form.Set("toDate", today.Format(dubaiFinancialMarketDateLayout))

	req, err := http.NewRequest("POST", p.dataUrl, strings.NewReader(form.Encode()))

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(httpclient.CustomHttpResponseLog(c, func(data []byte) {
		log.Debugf(c, "[dubai_financial_market_provider.getLatestQuote] response of \"%s\" is %s", symbol, data)
	}))

	resp, err := p.httpClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response code is %d", resp.StatusCode)
	}

	return p.parseLatestQuote(symbol, body)
}

// parseLatestQuote returns the quote of the newest row, or nil when the symbol has no trading days in the requested range
func (p *DubaiFinancialMarketProvider) parseLatestQuote(symbol string, body []byte) (*Quote, error) {
	var rows []*dubaiFinancialMarketDailyRow
	err := json.Unmarshal(body, &rows)

	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		price, ok := parseDubaiFinancialMarketNumber(row.Last)

		if !ok {
			price, ok = parseDubaiFinancialMarketNumber(row.Closing)
		}

		if !ok || price <= 0 {
			continue
		}

		tradingDay, err := time.ParseInLocation("02-01-2006", row.Date, p.location)

		if err != nil {
			return nil, err
		}

		tradingDay = tradingDay.Add(dubaiFinancialMarketCloseHour * time.Hour)

		return &Quote{
			Symbol:    symbol,
			Price:     price,
			Currency:  "AED",
			PriceTime: tradingDay.Unix(),
		}, nil
	}

	return nil, nil
}

func parseDubaiFinancialMarketNumber(text string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(text), ",", ""), 64)

	return value, err == nil
}
