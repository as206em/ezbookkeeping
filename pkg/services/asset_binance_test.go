package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/binance"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

type fakeBinanceClient struct {
	restrictions    *binance.KeyRestrictions
	restrictionsErr error
	balances        []string
	balancesErr     error
	trades          map[string][]*binance.Trade // by pair
	tradesErr       map[string]error            // by pair
	requestedPairs  []string
}

func (f *fakeBinanceClient) GetKeyRestrictions(c core.Context, credentials binance.Credentials) (*binance.KeyRestrictions, error) {
	return f.restrictions, f.restrictionsErr
}

func (f *fakeBinanceClient) GetBalanceAssets(c core.Context, credentials binance.Credentials) ([]string, error) {
	return f.balances, f.balancesErr
}

func (f *fakeBinanceClient) GetTrades(c core.Context, credentials binance.Credentials, pair string, fromId int64, limit int) ([]*binance.Trade, error) {
	f.requestedPairs = append(f.requestedPairs, pair)

	if err := f.tradesErr[pair]; err != nil {
		return nil, err
	}

	var result []*binance.Trade

	for _, trade := range f.trades[pair] {
		if trade.Id >= fromId && len(result) < limit {
			result = append(result, trade)
		}
	}

	return result, nil
}

func useFakeBinanceClient(t *testing.T, client *fakeBinanceClient) {
	t.Helper()

	previous := binance.Container.Current
	binance.Container.Current = client
	t.Cleanup(func() { binance.Container.Current = previous })
}

func binanceTrade(pair string, id int64, isBuyer bool, quantity string, price string, commission string, commissionAsset string) *binance.Trade {
	return &binance.Trade{Id: id, Symbol: pair, Price: price, Quantity: quantity, Commission: commission, CommissionAsset: commissionAsset, Time: 1759478400000 + id*1000, IsBuyer: isBuyer}
}

var testBinanceCredentials = binance.Credentials{ApiKey: "AKIA-test-key-1234", ApiSecret: "super-secret-value"}

func TestMapBinanceTrade_FeeInQuoteAssetIsTheFee(t *testing.T) {
	input, ok := mapBinanceTrade(binanceTrade("BTCUSDT", 7, true, "0.02", "109200.5", "3.4", "USDT"), "BTC", "USDT")

	require.True(t, ok)
	assert.Equal(t, models.ASSET_MARKET_CRYPTO, input.Market)
	assert.Equal(t, "BTC", input.Symbol)
	assert.Equal(t, models.ASSET_TRADE_SIDE_BUY, input.Side)
	assert.Equal(t, 0.02, input.Quantity)
	assert.Equal(t, 109200.5, input.Price)
	assert.Equal(t, 3.4, input.Fee)
	assert.Equal(t, "USD", input.Currency)
	assert.Equal(t, int64(1759478407), input.TradeTime)
	assert.Equal(t, "binance:BTCUSDT:7", input.ExternalId)
}

func TestMapBinanceTrade_FeeInBoughtCoinReducesQuantity(t *testing.T) {
	input, ok := mapBinanceTrade(binanceTrade("ETHUSDT", 1, true, "1", "2500", "0.001", "ETH"), "ETH", "USDT")

	require.True(t, ok)
	assert.InDelta(t, 0.999, input.Quantity, 1e-12)
	assert.Equal(t, 0.0, input.Fee)
}

func TestMapBinanceTrade_FeeInOtherAssetIsIgnored(t *testing.T) {
	input, ok := mapBinanceTrade(binanceTrade("ETHUSDT", 1, false, "2", "2500", "0.0005", "BNB"), "ETH", "USDT")

	require.True(t, ok)
	assert.Equal(t, models.ASSET_TRADE_SIDE_SELL, input.Side)
	assert.Equal(t, 2.0, input.Quantity)
	assert.Equal(t, 0.0, input.Fee)
}

func TestMapBinanceTrade_RejectsUnusableTrades(t *testing.T) {
	_, ok := mapBinanceTrade(binanceTrade("ETHUSDT", 1, true, "oops", "2500", "0", "ETH"), "ETH", "USDT")
	assert.False(t, ok)

	_, ok = mapBinanceTrade(binanceTrade("ETHUSDT", 1, true, "0.001", "2500", "0.001", "ETH"), "ETH", "USDT")
	assert.False(t, ok, "a trade whose whole quantity went to the fee leaves nothing")
}

func TestGetBinanceBaseAssets_CombinesBalancesAndKnownSymbolsWithoutQuoteAssets(t *testing.T) {
	existing := []*models.AssetTrade{
		{Market: models.ASSET_MARKET_CRYPTO, Symbol: "ETH"},
		{Market: models.ASSET_MARKET_CRYPTO, Symbol: "BTC"},
		{Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR"},
	}

	assert.Equal(t, []string{"BTC", "ETH", "SOL"}, getBinanceBaseAssets([]string{"btc", "SOL", "USDT", "FDUSD", "BTC"}, existing))
}

func TestValidateBinanceCredentials(t *testing.T) {
	c := initializeAssetServiceTest(t)
	readOnly := &binance.KeyRestrictions{EnableReading: true}

	useFakeBinanceClient(t, &fakeBinanceClient{restrictions: readOnly})
	assert.NoError(t, Assets.ValidateBinanceCredentials(c, testBinanceCredentials))

	for name, restrictions := range map[string]*binance.KeyRestrictions{
		"withdrawals": {EnableReading: true, EnableWithdrawals: true},
		"trading":     {EnableReading: true, EnableSpotAndMarginTrading: true},
		"margin":      {EnableReading: true, EnableMargin: true},
		"futures":     {EnableReading: true, EnableFutures: true},
		"transfers":   {EnableReading: true, EnableInternalTransfer: true},
	} {
		useFakeBinanceClient(t, &fakeBinanceClient{restrictions: restrictions})
		assert.Equal(t, errs.ErrAssetBinanceKeyNotReadOnly, Assets.ValidateBinanceCredentials(c, testBinanceCredentials), name)
	}

	useFakeBinanceClient(t, &fakeBinanceClient{restrictions: &binance.KeyRestrictions{}})
	assert.Equal(t, errs.ErrAssetBinanceKeyInvalid, Assets.ValidateBinanceCredentials(c, testBinanceCredentials), "reading disabled")

	useFakeBinanceClient(t, &fakeBinanceClient{restrictionsErr: &binance.APIError{Status: http.StatusUnauthorized, Code: -2015, Message: "Invalid API-key"}})
	assert.Equal(t, errs.ErrAssetBinanceKeyInvalid, Assets.ValidateBinanceCredentials(c, testBinanceCredentials))

	useFakeBinanceClient(t, &fakeBinanceClient{restrictionsErr: &binance.APIError{Status: http.StatusTooManyRequests}})
	assert.Equal(t, errs.ErrFailedToRequestRemoteApi, Assets.ValidateBinanceCredentials(c, testBinanceCredentials), "rate limits are not a bad key")

	useFakeBinanceClient(t, &fakeBinanceClient{restrictionsErr: errors.New("connection refused")})
	assert.Equal(t, errs.ErrFailedToRequestRemoteApi, Assets.ValidateBinanceCredentials(c, testBinanceCredentials))
}

func TestSaveBinanceConnection_StoresEncryptedAndNeverExposesSecrets(t *testing.T) {
	c := initializeAssetServiceTest(t)

	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))

	connection, err := Assets.GetBinanceConnection(c, 1)
	require.NoError(t, err)
	require.NotNil(t, connection)

	assert.NotContains(t, connection.ApiKey, "AKIA-test-key")
	assert.NotContains(t, connection.ApiSecret, "super-secret")
	assert.Equal(t, "1234", connection.ApiKeyHint)

	status, err := json.Marshal(connection.ToAssetBinanceStatusResponse())
	require.NoError(t, err)
	assert.Contains(t, string(status), `"connected":true`)
	assert.Contains(t, string(status), `"apiKeyHint":"1234"`)
	assert.NotContains(t, string(status), "AKIA")
	assert.NotContains(t, string(status), "secret")

	disconnected, err := json.Marshal((*models.AssetBinanceConnection)(nil).ToAssetBinanceStatusResponse())
	require.NoError(t, err)
	assert.Contains(t, string(disconnected), `"connected":false`)

	other, err := Assets.GetBinanceConnection(c, 2)
	require.NoError(t, err)
	assert.Nil(t, other, "connections are per user")
}

func TestSaveBinanceConnection_ReplacesPreviousKey(t *testing.T) {
	c := initializeAssetServiceTest(t)

	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))
	require.NoError(t, Assets.SaveBinanceConnection(c, 1, binance.Credentials{ApiKey: "second-key-9999", ApiSecret: "second-secret"}))

	connection, err := Assets.GetBinanceConnection(c, 1)
	require.NoError(t, err)
	assert.Equal(t, "9999", connection.ApiKeyHint)
}

func TestSyncBinanceTrades_ImportsNewTradesAndSkipsOnesAlreadySynced(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))

	fake := &fakeBinanceClient{
		balances: []string{"BTC", "USDT"},
		trades: map[string][]*binance.Trade{
			"BTCUSDT": {
				binanceTrade("BTCUSDT", 1, true, "0.30", "81000", "0", "USDT"),
				binanceTrade("BTCUSDT", 2, true, "0.12", "103000", "3.4", "USDT"),
			},
		},
		tradesErr: map[string]error{
			"BTCUSDC":  &binance.APIError{Status: http.StatusBadRequest, Code: binance.ErrorCodeInvalidSymbol},
			"BTCFDUSD": &binance.APIError{Status: http.StatusBadRequest, Code: binance.ErrorCodeInvalidSymbol},
		},
	}
	useFakeBinanceClient(t, fake)

	response, err := Assets.SyncBinanceTrades(c, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, response.CreatedCount)
	assert.Equal(t, 0, response.SkippedCount)
	assert.Equal(t, 3, response.PairsChecked)
	assert.Empty(t, response.Error)
	assert.Equal(t, []string{"BTCUSDT", "BTCUSDC", "BTCFDUSD"}, fake.requestedPairs)

	again, err := Assets.SyncBinanceTrades(c, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, again.CreatedCount)
	assert.Equal(t, 2, again.SkippedCount)

	_, positions, err := Assets.GetPositionsByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, positions.Holdings, 1)
	assert.InDelta(t, 0.42, positions.Holdings[0].Quantity, 1e-9)

	connection, err := Assets.GetBinanceConnection(c, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, connection.LastSyncCreatedCount)
	assert.NotZero(t, connection.LastSyncUnixTime)
	assert.Empty(t, connection.LastSyncError)
}

func TestSyncBinanceTrades_PagesThroughLongHistories(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))

	var many []*binance.Trade

	for id := int64(1); id <= 1005; id++ {
		many = append(many, binanceTrade("BTCUSDT", id, true, "0.001", "100000", "0", "USDT"))
	}

	useFakeBinanceClient(t, &fakeBinanceClient{
		balances:  []string{"BTC"},
		trades:    map[string][]*binance.Trade{"BTCUSDT": many},
		tradesErr: map[string]error{"BTCUSDC": &binance.APIError{Code: binance.ErrorCodeInvalidSymbol}, "BTCFDUSD": &binance.APIError{Code: binance.ErrorCodeInvalidSymbol}},
	})

	response, err := Assets.SyncBinanceTrades(c, 1)
	require.NoError(t, err)
	assert.Equal(t, 1005, response.CreatedCount)
}

func TestSyncBinanceTrades_AlsoChecksCoinsAlreadyInTheUsersHistory(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))

	csvTrade := buildTestAssetTrade(t, &models.AssetTradeInput{Market: models.ASSET_MARKET_CRYPTO, Symbol: "SOL", Side: models.ASSET_TRADE_SIDE_BUY, Quantity: 5, Price: 100, Currency: "USD", TradeTime: 1000})
	_, _, err := Assets.ImportTrades(c, 1, []*models.AssetTrade{csvTrade})
	require.NoError(t, err)

	fake := &fakeBinanceClient{balances: []string{"BTC"}, trades: map[string][]*binance.Trade{}}
	useFakeBinanceClient(t, fake)

	_, err = Assets.SyncBinanceTrades(c, 1)
	require.NoError(t, err)
	assert.Contains(t, fake.requestedPairs, "SOLUSDT")
	assert.Contains(t, fake.requestedPairs, "BTCUSDT")
}

func TestSyncBinanceTrades_KeepsTradesFetchedBeforeARateLimit(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))

	useFakeBinanceClient(t, &fakeBinanceClient{
		balances: []string{"BTC", "ETH"},
		trades:   map[string][]*binance.Trade{"BTCUSDT": {binanceTrade("BTCUSDT", 1, true, "0.1", "100000", "0", "USDT")}},
		tradesErr: map[string]error{
			"BTCUSDC":  &binance.APIError{Code: binance.ErrorCodeInvalidSymbol},
			"BTCFDUSD": &binance.APIError{Code: binance.ErrorCodeInvalidSymbol},
			"ETHUSDT":  &binance.APIError{Status: http.StatusTooManyRequests, Code: -1003, Message: "Too many requests"},
		},
	})

	response, err := Assets.SyncBinanceTrades(c, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, response.CreatedCount)
	assert.Contains(t, response.Error, "slow down")

	connection, err := Assets.GetBinanceConnection(c, 1)
	require.NoError(t, err)
	assert.Contains(t, connection.LastSyncError, "slow down")
	assert.NotContains(t, strings.ToLower(fmt.Sprint(response)), "super-secret")
}

func TestSyncBinanceTrades_ReportsRejectedKeyWithoutLeakingDetails(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))

	useFakeBinanceClient(t, &fakeBinanceClient{balancesErr: &binance.APIError{Status: http.StatusUnauthorized, Code: -2015, Message: "Invalid API-key, IP, or permissions for action."}})

	response, err := Assets.SyncBinanceTrades(c, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, response.CreatedCount)
	assert.Contains(t, response.Error, "rejected the API key")
}

func TestSyncBinanceTrades_NeedsAConnection(t *testing.T) {
	c := initializeAssetServiceTest(t)

	_, err := Assets.SyncBinanceTrades(c, 1)

	assert.Equal(t, errs.ErrAssetBinanceNotConnected, err)
}

func TestDeleteBinanceConnection_KeepsSyncedTrades(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))
	require.NoError(t, Assets.CreateTrade(c, buildTestAssetTrade(t, emaarBuy(1, 1, 1000))))

	require.NoError(t, Assets.DeleteBinanceConnection(c, 1))

	connection, err := Assets.GetBinanceConnection(c, 1)
	require.NoError(t, err)
	assert.Nil(t, connection)

	trades, err := Assets.GetAllTradesByUid(c, 1)
	require.NoError(t, err)
	assert.Len(t, trades, 1)
}

func TestDeleteAllTradesAndPrices_AlsoRemovesBinanceConnection(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SaveBinanceConnection(c, 1, testBinanceCredentials))

	require.NoError(t, Assets.DeleteAllTradesAndPrices(c, 1))

	connection, err := Assets.GetBinanceConnection(c, 1)
	require.NoError(t, err)
	assert.Nil(t, connection)
}
