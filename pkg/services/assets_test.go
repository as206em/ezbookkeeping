package services

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

func initializeAssetServiceTest(t *testing.T) core.Context {
	t.Helper()

	config := &settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{
			DatabaseType:          settings.Sqlite3DbType,
			DatabasePath:          filepath.Join(t.TempDir(), "asset-test.db"),
			MaxIdleConnection:     4,
			MaxOpenConnection:     4,
			ConnectionMaxLifeTime: 60,
		},
		UuidGeneratorType: "internal",
		UuidServerId:      0,
		SecretKey:         "test-secret-key",
	}

	settings.SetCurrentConfig(config)

	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, uuid.InitializeUuidGenerator(config))
	t.Cleanup(func() {
		require.NoError(t, datastore.Container.UserStore.Choose(0).Close())
	})
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.AssetTrade)))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.AssetPrice)))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.AssetBinanceConnection)))

	return core.NewNullContext()
}

func buildTestAssetTrade(t *testing.T, input *models.AssetTradeInput) *models.AssetTrade {
	t.Helper()

	trade, err := Assets.BuildTrade(1, input)
	require.NoError(t, err)

	return trade
}

func emaarBuy(quantity float64, price float64, tradeTime int64) *models.AssetTradeInput {
	return &models.AssetTradeInput{
		Market:    models.ASSET_MARKET_DFM,
		Symbol:    "emaar",
		Side:      models.ASSET_TRADE_SIDE_BUY,
		Quantity:  quantity,
		Price:     price,
		Currency:  "aed",
		TradeTime: tradeTime,
	}
}

func TestBuildTrade_NormalizesAndValidates(t *testing.T) {
	trade, err := Assets.BuildTrade(1, emaarBuy(100, 10.5, 1000))
	require.NoError(t, err)
	assert.Equal(t, "EMAAR", trade.Symbol)
	assert.Equal(t, "AED", trade.Currency)
	assert.Equal(t, models.ScaleAssetNumber(100), trade.Quantity)
	assert.NotEmpty(t, trade.ExternalId)

	invalid := emaarBuy(0, 10, 1000)
	_, err = Assets.BuildTrade(1, invalid)
	assert.Equal(t, errs.ErrAssetQuantityInvalid, err)

	invalid = emaarBuy(1, -1, 1000)
	_, err = Assets.BuildTrade(1, invalid)
	assert.Equal(t, errs.ErrAssetPriceInvalid, err)

	invalid = emaarBuy(1, 1, 1000)
	invalid.Market = "NYSE"
	_, err = Assets.BuildTrade(1, invalid)
	assert.Equal(t, errs.ErrAssetMarketInvalid, err)

	invalid = emaarBuy(1, 1, 1000)
	invalid.Currency = "DIRHAM"
	_, err = Assets.BuildTrade(1, invalid)
	assert.Equal(t, errs.ErrAssetCurrencyInvalid, err)

	invalid = emaarBuy(1, 1, 1000)
	invalid.Fee = -2
	_, err = Assets.BuildTrade(1, invalid)
	assert.Equal(t, errs.ErrAssetFeeInvalid, err)
}

func TestImportTrades_SkipsTradesAlreadyImported(t *testing.T) {
	c := initializeAssetServiceTest(t)

	first := []*models.AssetTrade{
		buildTestAssetTrade(t, emaarBuy(100, 10, 1000)),
		buildTestAssetTrade(t, emaarBuy(50, 11, 2000)),
	}
	created, skipped, err := Assets.ImportTrades(c, 1, first)
	require.NoError(t, err)
	assert.Equal(t, 2, created)
	assert.Equal(t, 0, skipped)

	again := []*models.AssetTrade{
		buildTestAssetTrade(t, emaarBuy(100, 10, 1000)),
		buildTestAssetTrade(t, emaarBuy(50, 11, 2000)),
		buildTestAssetTrade(t, emaarBuy(25, 12, 3000)),
	}
	created, skipped, err = Assets.ImportTrades(c, 1, again)
	require.NoError(t, err)
	assert.Equal(t, 1, created)
	assert.Equal(t, 2, skipped)

	trades, err := Assets.GetAllTradesByUid(c, 1)
	require.NoError(t, err)
	assert.Len(t, trades, 3)
}

func TestImportTrades_KeepsIdenticalRowsInsideOneFile(t *testing.T) {
	c := initializeAssetServiceTest(t)

	created, skipped, err := Assets.ImportTrades(c, 1, []*models.AssetTrade{
		buildTestAssetTrade(t, emaarBuy(100, 10, 1000)),
		buildTestAssetTrade(t, emaarBuy(100, 10, 1000)),
	})
	require.NoError(t, err)
	assert.Equal(t, 2, created)
	assert.Equal(t, 0, skipped)

	created, skipped, err = Assets.ImportTrades(c, 1, []*models.AssetTrade{
		buildTestAssetTrade(t, emaarBuy(100, 10, 1000)),
		buildTestAssetTrade(t, emaarBuy(100, 10, 1000)),
	})
	require.NoError(t, err)
	assert.Equal(t, 0, created)
	assert.Equal(t, 2, skipped)
}

func TestImportTrades_UsesExternalIdWhenGiven(t *testing.T) {
	c := initializeAssetServiceTest(t)

	input := emaarBuy(1, 1, 1000)
	input.ExternalId = "binance-123"
	created, _, err := Assets.ImportTrades(c, 1, []*models.AssetTrade{buildTestAssetTrade(t, input)})
	require.NoError(t, err)
	assert.Equal(t, 1, created)

	changed := emaarBuy(2, 3, 5000)
	changed.ExternalId = "binance-123"
	created, skipped, err := Assets.ImportTrades(c, 1, []*models.AssetTrade{buildTestAssetTrade(t, changed)})
	require.NoError(t, err)
	assert.Equal(t, 0, created)
	assert.Equal(t, 1, skipped)
}

func TestImportTrades_IsScopedToUser(t *testing.T) {
	c := initializeAssetServiceTest(t)

	_, _, err := Assets.ImportTrades(c, 1, []*models.AssetTrade{buildTestAssetTrade(t, emaarBuy(100, 10, 1000))})
	require.NoError(t, err)

	created, _, err := Assets.ImportTrades(c, 2, []*models.AssetTrade{buildTestAssetTrade(t, emaarBuy(100, 10, 1000))})
	require.NoError(t, err)
	assert.Equal(t, 1, created)

	trades, err := Assets.GetAllTradesByUid(c, 2)
	require.NoError(t, err)
	assert.Len(t, trades, 1)
}

func TestModifyAndDeleteTrade(t *testing.T) {
	c := initializeAssetServiceTest(t)

	trade := buildTestAssetTrade(t, emaarBuy(100, 10, 1000))
	require.NoError(t, Assets.CreateTrade(c, trade))

	modified := buildTestAssetTrade(t, emaarBuy(150, 10, 1000))
	require.NoError(t, Assets.ModifyTrade(c, 1, trade.TradeId, modified))

	trades, err := Assets.GetAllTradesByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, trades, 1)
	assert.Equal(t, models.ScaleAssetNumber(150), trades[0].Quantity)

	assert.Equal(t, errs.ErrAssetTradeNotFound, Assets.ModifyTrade(c, 1, 999999, modified))
	assert.Equal(t, errs.ErrAssetTradeNotFound, Assets.ModifyTrade(c, 2, trade.TradeId, modified))

	require.NoError(t, Assets.DeleteTrade(c, 1, trade.TradeId))
	assert.Equal(t, errs.ErrAssetTradeNotFound, Assets.DeleteTrade(c, 1, trade.TradeId))

	trades, err = Assets.GetAllTradesByUid(c, 1)
	require.NoError(t, err)
	assert.Len(t, trades, 0)
}

func TestSetPrice_ReplacesPreviousPrice(t *testing.T) {
	c := initializeAssetServiceTest(t)

	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_ADX, Symbol: "ALDAR", Price: models.ScaleAssetNumber(7), Currency: "AED", PriceTime: 100, Source: "manual"}))
	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_ADX, Symbol: "ALDAR", Price: models.ScaleAssetNumber(7.5), Currency: "AED", PriceTime: 200, Source: "manual"}))

	prices, err := Assets.GetAllPricesByUid(c, 1)
	require.NoError(t, err)
	assert.Equal(t, models.ScaleAssetNumber(7.5), prices[models.ASSET_MARKET_ADX]["ALDAR"].Price)
	assert.Len(t, prices[models.ASSET_MARKET_ADX], 1)
}

func TestRefreshPrices_UsesProvidersAndReportsMarketsWithoutOne(t *testing.T) {
	c := initializeAssetServiceTest(t)

	_, _, err := Assets.ImportTrades(c, 1, []*models.AssetTrade{
		buildTestAssetTrade(t, emaarBuy(100, 10, 1000)),
		buildTestAssetTrade(t, &models.AssetTradeInput{Market: models.ASSET_MARKET_ADX, Symbol: "ALDAR", Side: models.ASSET_TRADE_SIDE_BUY, Quantity: 5, Price: 6, Currency: "AED", TradeTime: 1000}),
		buildTestAssetTrade(t, &models.AssetTradeInput{Market: models.ASSET_MARKET_DFM, Symbol: "GONE", Side: models.ASSET_TRADE_SIDE_BUY, Quantity: 5, Price: 6, Currency: "AED", TradeTime: 1000}),
		buildTestAssetTrade(t, &models.AssetTradeInput{Market: models.ASSET_MARKET_DFM, Symbol: "SOLD", Side: models.ASSET_TRADE_SIDE_BUY, Quantity: 5, Price: 6, Currency: "AED", TradeTime: 1000}),
		buildTestAssetTrade(t, &models.AssetTradeInput{Market: models.ASSET_MARKET_DFM, Symbol: "SOLD", Side: models.ASSET_TRADE_SIDE_SELL, Quantity: 5, Price: 7, Currency: "AED", TradeTime: 2000}),
	})
	require.NoError(t, err)

	provider := &fakeAssetPriceProvider{quotes: map[string]float64{"EMAAR": 11.46}}
	registerFakeAssetPriceProvider(t, provider)

	response, err := Assets.RefreshPrices(c, 1)
	require.NoError(t, err)
	require.Len(t, response.Markets, 2)

	assert.Equal(t, models.ASSET_MARKET_ADX, response.Markets[0].Market)
	assert.Equal(t, 0, response.Markets[0].Updated)
	assert.Equal(t, []string{"ALDAR"}, response.Markets[0].Failed)
	assert.NotEmpty(t, response.Markets[0].Message)

	assert.Equal(t, models.ASSET_MARKET_DFM, response.Markets[1].Market)
	assert.Equal(t, 1, response.Markets[1].Updated)
	assert.Equal(t, []string{"GONE"}, response.Markets[1].Failed)

	assert.ElementsMatch(t, []string{"EMAAR", "GONE"}, provider.requested, "sold holdings are not refreshed")

	prices, err := Assets.GetAllPricesByUid(c, 1)
	require.NoError(t, err)
	assert.Equal(t, models.ScaleAssetNumber(11.46), prices[models.ASSET_MARKET_DFM]["EMAAR"].Price)
	assert.Equal(t, "fake.test", prices[models.ASSET_MARKET_DFM]["EMAAR"].Source)
}

func TestRefreshPrices_KeepsManualPriceWhenSourceFails(t *testing.T) {
	c := initializeAssetServiceTest(t)

	_, _, err := Assets.ImportTrades(c, 1, []*models.AssetTrade{buildTestAssetTrade(t, emaarBuy(100, 10, 1000))})
	require.NoError(t, err)
	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR", Price: models.ScaleAssetNumber(11), Currency: "AED", PriceTime: 100, Source: models.ASSET_PRICE_SOURCE_MANUAL}))

	registerFakeAssetPriceProvider(t, &fakeAssetPriceProvider{err: errs.ErrFailedToRequestRemoteApi})

	response, err := Assets.RefreshPrices(c, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, response.Markets[0].Updated)
	assert.NotEmpty(t, response.Markets[0].Message)

	prices, err := Assets.GetAllPricesByUid(c, 1)
	require.NoError(t, err)
	assert.Equal(t, models.ScaleAssetNumber(11), prices[models.ASSET_MARKET_DFM]["EMAAR"].Price)
	assert.Equal(t, models.ASSET_PRICE_SOURCE_MANUAL, prices[models.ASSET_MARKET_DFM]["EMAAR"].Source)
}

func TestDeleteAllTradesAndPrices(t *testing.T) {
	c := initializeAssetServiceTest(t)

	require.NoError(t, Assets.CreateTrade(c, buildTestAssetTrade(t, emaarBuy(100, 10, 1000))))
	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR", Price: 1, Currency: "AED", Source: "manual"}))

	require.NoError(t, Assets.DeleteAllTradesAndPrices(c, 1))

	trades, err := Assets.GetAllTradesByUid(c, 1)
	require.NoError(t, err)
	assert.Len(t, trades, 0)

	prices, err := Assets.GetAllPricesByUid(c, 1)
	require.NoError(t, err)
	assert.Len(t, prices, 0)
}
