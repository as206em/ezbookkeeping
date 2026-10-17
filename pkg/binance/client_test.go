package binance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
)

var testCredentials = Credentials{ApiKey: "test-key", ApiSecret: "test-secret"}

func newTestClient(t *testing.T, handler http.HandlerFunc) *HttpClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := newHttpClient(server.Client(), server.URL)
	client.now = func() time.Time { return time.UnixMilli(1792220000123) }

	return client
}

func TestHttpClient_SignsRequestsAndSendsApiKeyHeader(t *testing.T) {
	var request *http.Request

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		request = r
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := client.GetTrades(core.NewNullContext(), testCredentials, "BTCUSDT", 5, 1000)
	require.NoError(t, err)

	assert.Equal(t, "/api/v3/myTrades", request.URL.Path)
	assert.Equal(t, "test-key", request.Header.Get("X-MBX-APIKEY"))

	query := request.URL.Query()
	assert.Equal(t, "BTCUSDT", query.Get("symbol"))
	assert.Equal(t, "5", query.Get("fromId"))
	assert.Equal(t, "1000", query.Get("limit"))
	assert.Equal(t, "1792220000123", query.Get("timestamp"))
	assert.Equal(t, "10000", query.Get("recvWindow"))

	rawQuery := request.URL.RawQuery
	signedPart := rawQuery[:strings.LastIndex(rawQuery, "&signature=")]
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte(signedPart))
	assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), query.Get("signature"))
	assert.NotContains(t, rawQuery, "test-secret", "the secret must never be sent")
}

func TestHttpClient_ParsesTrades(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"symbol":"BTCUSDT","id":28457,"price":"109200.50","qty":"0.02000000","quoteQty":"2184.01","commission":"3.40","commissionAsset":"USDT","time":1759478400000,"isBuyer":true}]`))
	})

	trades, err := client.GetTrades(core.NewNullContext(), testCredentials, "BTCUSDT", 0, 1000)

	require.NoError(t, err)
	require.Len(t, trades, 1)
	assert.Equal(t, int64(28457), trades[0].Id)
	assert.Equal(t, "109200.50", trades[0].Price)
	assert.Equal(t, "USDT", trades[0].CommissionAsset)
	assert.True(t, trades[0].IsBuyer)
	assert.Equal(t, int64(1759478400000), trades[0].Time)
}

func TestHttpClient_ReturnsBinanceErrorCode(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":-1121,"msg":"Invalid symbol."}`))
	})

	_, err := client.GetTrades(core.NewNullContext(), testCredentials, "NOPEUSDT", 0, 1000)

	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, ErrorCodeInvalidSymbol, apiErr.Code)
	assert.Equal(t, http.StatusBadRequest, apiErr.Status)
}

func TestHttpClient_GetBalanceAssetsKeepsOnlyNonZeroBalances(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v3/account", r.URL.Path)
		_, _ = w.Write([]byte(`{"balances":[{"asset":"BTC","free":"0.42000000","locked":"0.00000000"},{"asset":"ETH","free":"0.00000000","locked":"1.50000000"},{"asset":"XRP","free":"0.00000000","locked":"0.00000000"}]}`))
	})

	assets, err := client.GetBalanceAssets(core.NewNullContext(), testCredentials)

	require.NoError(t, err)
	assert.Equal(t, []string{"BTC", "ETH"}, assets)
}

func TestHttpClient_ParsesKeyRestrictions(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sapi/v1/account/apiRestrictions", r.URL.Path)
		_, _ = w.Write([]byte(`{"ipRestrict":true,"enableReading":true,"enableWithdrawals":false,"enableSpotAndMarginTrading":false,"enableMargin":false,"enableFutures":false}`))
	})

	restrictions, err := client.GetKeyRestrictions(core.NewNullContext(), testCredentials)

	require.NoError(t, err)
	assert.True(t, restrictions.EnableReading)
	assert.False(t, restrictions.EnableWithdrawals)
	assert.False(t, restrictions.EnableSpotAndMarginTrading)
}
