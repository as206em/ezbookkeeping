package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/httpclient"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

const baseUrl = "https://api.binance.com"
const requestTimeout uint32 = 30000 // milliseconds
const requestProxy = "system"
const recvWindow = "10000"

// ErrorCodeInvalidSymbol is returned by Binance when a trading pair does not exist
const ErrorCodeInvalidSymbol = -1121

// Credentials represents a Binance API key pair
type Credentials struct {
	ApiKey    string
	ApiSecret string
}

// KeyRestrictions represents what a Binance API key is allowed to do
type KeyRestrictions struct {
	EnableReading              bool `json:"enableReading"`
	EnableWithdrawals          bool `json:"enableWithdrawals"`
	EnableInternalTransfer     bool `json:"enableInternalTransfer"`
	EnableSpotAndMarginTrading bool `json:"enableSpotAndMarginTrading"`
	EnableMargin               bool `json:"enableMargin"`
	EnableFutures              bool `json:"enableFutures"`
}

// Trade represents one executed spot trade of the account
type Trade struct {
	Id              int64  `json:"id"`
	Symbol          string `json:"symbol"`
	Price           string `json:"price"`
	Quantity        string `json:"qty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Time            int64  `json:"time"` // milliseconds
	IsBuyer         bool   `json:"isBuyer"`
}

// APIError represents an error response of Binance
type APIError struct {
	Status  int
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

// Error returns the error text
func (e *APIError) Error() string {
	return fmt.Sprintf("binance api error (http %d, code %d): %s", e.Status, e.Code, e.Message)
}

// Client defines the structure of a Binance read-only API client
type Client interface {
	// GetKeyRestrictions returns what the API key is allowed to do
	GetKeyRestrictions(c core.Context, credentials Credentials) (*KeyRestrictions, error)

	// GetBalanceAssets returns the assets the account currently holds (free or locked)
	GetBalanceAssets(c core.Context, credentials Credentials) ([]string, error)

	// GetTrades returns up to limit trades of the pair with an id of at least fromId, oldest first
	GetTrades(c core.Context, credentials Credentials, pair string, fromId int64, limit int) ([]*Trade, error)
}

// HttpClient is the Client backed by Binance's REST API
type HttpClient struct {
	httpClient *http.Client
	baseUrl    string
	now        func() time.Time
}

// ClientContainer contains the current Binance client
type ClientContainer struct {
	Current Client
}

// Initialize a Binance client container singleton instance
var (
	Container = &ClientContainer{}
)

// InitializeClient sets the current client to the real Binance API client
func InitializeClient(config *settings.Config) {
	Container.Current = newHttpClient(httpclient.NewHttpClient(requestTimeout, requestProxy, false, core.GetOutgoingUserAgent(), false), baseUrl)
}

func newHttpClient(client *http.Client, apiBaseUrl string) *HttpClient {
	return &HttpClient{
		httpClient: client,
		baseUrl:    apiBaseUrl,
		now:        time.Now,
	}
}

// GetKeyRestrictions returns what the API key is allowed to do
func (b *HttpClient) GetKeyRestrictions(c core.Context, credentials Credentials) (*KeyRestrictions, error) {
	body, err := b.signedGet(c, credentials, "/sapi/v1/account/apiRestrictions", url.Values{})

	if err != nil {
		return nil, err
	}

	restrictions := &KeyRestrictions{}

	if err = json.Unmarshal(body, restrictions); err != nil {
		return nil, err
	}

	return restrictions, nil
}

// GetBalanceAssets returns the assets the account currently holds (free or locked)
func (b *HttpClient) GetBalanceAssets(c core.Context, credentials Credentials) ([]string, error) {
	body, err := b.signedGet(c, credentials, "/api/v3/account", url.Values{"omitZeroBalances": {"true"}})

	if err != nil {
		return nil, err
	}

	var account struct {
		Balances []struct {
			Asset  string `json:"asset"`
			Free   string `json:"free"`
			Locked string `json:"locked"`
		} `json:"balances"`
	}

	if err = json.Unmarshal(body, &account); err != nil {
		return nil, err
	}

	assets := make([]string, 0, len(account.Balances))

	for _, balance := range account.Balances {
		free, _ := strconv.ParseFloat(balance.Free, 64)
		locked, _ := strconv.ParseFloat(balance.Locked, 64)

		if free > 0 || locked > 0 {
			assets = append(assets, balance.Asset)
		}
	}

	return assets, nil
}

// GetTrades returns up to limit trades of the pair with an id of at least fromId, oldest first
func (b *HttpClient) GetTrades(c core.Context, credentials Credentials, pair string, fromId int64, limit int) ([]*Trade, error) {
	body, err := b.signedGet(c, credentials, "/api/v3/myTrades", url.Values{
		"symbol": {pair},
		"fromId": {strconv.FormatInt(fromId, 10)},
		"limit":  {strconv.Itoa(limit)},
	})

	if err != nil {
		return nil, err
	}

	var trades []*Trade

	if err = json.Unmarshal(body, &trades); err != nil {
		return nil, err
	}

	return trades, nil
}

func (b *HttpClient) signedGet(c core.Context, credentials Credentials, path string, params url.Values) ([]byte, error) {
	params.Set("timestamp", strconv.FormatInt(b.now().UnixMilli(), 10))
	params.Set("recvWindow", recvWindow)

	query := params.Encode()
	req, err := http.NewRequest("GET", b.baseUrl+path+"?"+query+"&signature="+sign(credentials.ApiSecret, query), nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", credentials.ApiKey)

	resp, err := b.httpClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{Status: resp.StatusCode}
		_ = json.Unmarshal(body, apiErr)

		return nil, apiErr
	}

	return body, nil
}

func sign(secret string, query string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(query))

	return hex.EncodeToString(mac.Sum(nil))
}
