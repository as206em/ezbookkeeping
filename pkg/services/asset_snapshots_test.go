package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestGetSnapshotDate_UsesUtcDay(t *testing.T) {
	assert.Equal(t, int32(20261017), GetSnapshotDate(time.Date(2026, 10, 17, 23, 30, 0, 0, time.UTC)))
	assert.Equal(t, int32(20261017), GetSnapshotDate(time.Date(2026, 10, 18, 2, 0, 0, 0, time.FixedZone("GST", 4*3600))))
}

func TestSaveSnapshot_RecordsTodayAndReplacesItOnChange(t *testing.T) {
	c := initializeAssetServiceTest(t)
	today := GetSnapshotDate(time.Now())

	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR", Price: models.ScaleAssetNumber(10), Currency: "AED", Source: "manual"}))
	require.NoError(t, Assets.CreateTrade(c, buildTestAssetTrade(t, emaarBuy(100, 8, 1000))))

	snapshots, err := Assets.GetSnapshotsByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Equal(t, today, snapshots[0].SnapshotDate)
	assert.Equal(t, int64(100000), snapshots[0].Value, "100 shares at 10 AED, in cents")
	assert.Equal(t, int64(80000), snapshots[0].CostBasis)

	require.NoError(t, Assets.SetPriceAndSnapshot(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR", Price: models.ScaleAssetNumber(12), Currency: "AED", Source: "manual"}))

	snapshots, err = Assets.GetSnapshotsByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, snapshots, 1, "today's record is replaced, not duplicated")
	assert.Equal(t, int64(120000), snapshots[0].Value)
}

func TestSaveSnapshot_KeepsEarlierDaysAndOrdersOldestFirst(t *testing.T) {
	c := initializeAssetServiceTest(t)

	for _, date := range []int32{20260301, 20260101, 20260201} {
		_, err := datastore.Container.UserDataStore.Choose(1).NewSession(c).Insert(&models.AssetSnapshot{Uid: 1, SnapshotDate: date, Market: models.ASSET_MARKET_DFM, Currency: "AED", Value: 100, CostBasis: 90})
		require.NoError(t, err)
	}

	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR", Price: models.ScaleAssetNumber(10), Currency: "AED", Source: "manual"}))
	require.NoError(t, Assets.CreateTrade(c, buildTestAssetTrade(t, emaarBuy(1, 1, 1000))))

	snapshots, err := Assets.GetSnapshotsByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, snapshots, 4)
	assert.Equal(t, []int32{20260101, 20260201, 20260301, GetSnapshotDate(time.Now())}, []int32{snapshots[0].SnapshotDate, snapshots[1].SnapshotDate, snapshots[2].SnapshotDate, snapshots[3].SnapshotDate})
}

func TestSaveSnapshot_SoldEverythingRemovesTodaysRowOnly(t *testing.T) {
	c := initializeAssetServiceTest(t)

	_, err := datastore.Container.UserDataStore.Choose(1).NewSession(c).Insert(&models.AssetSnapshot{Uid: 1, SnapshotDate: 20260101, Market: models.ASSET_MARKET_DFM, Currency: "AED", Value: 100, CostBasis: 90})
	require.NoError(t, err)
	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR", Price: models.ScaleAssetNumber(10), Currency: "AED", Source: "manual"}))

	buy := buildTestAssetTrade(t, emaarBuy(10, 8, 1000))
	require.NoError(t, Assets.CreateTrade(c, buy))

	sell := emaarBuy(10, 9, 2000)
	sell.Side = models.ASSET_TRADE_SIDE_SELL
	require.NoError(t, Assets.CreateTrade(c, buildTestAssetTrade(t, sell)))

	snapshots, err := Assets.GetSnapshotsByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Equal(t, int32(20260101), snapshots[0].SnapshotDate)
}

func TestSnapshots_FollowImportDeleteAndRefresh(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR", Price: models.ScaleAssetNumber(10), Currency: "AED", Source: "manual"}))

	_, _, err := Assets.ImportTrades(c, 1, []*models.AssetTrade{buildTestAssetTrade(t, emaarBuy(100, 8, 1000))})
	require.NoError(t, err)

	snapshots, err := Assets.GetSnapshotsByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)

	trades, err := Assets.GetAllTradesByUid(c, 1)
	require.NoError(t, err)
	require.NoError(t, Assets.DeleteTrade(c, 1, trades[0].TradeId))

	snapshots, err = Assets.GetSnapshotsByUid(c, 1)
	require.NoError(t, err)
	assert.Len(t, snapshots, 0, "deleting the only trade clears today's snapshot")

	registerFakeAssetPriceProvider(t, &fakeAssetPriceProvider{quotes: map[string]float64{"EMAAR": 11.46}})
	require.NoError(t, Assets.CreateTrade(c, buildTestAssetTrade(t, emaarBuy(100, 8, 1000))))
	_, err = Assets.RefreshPrices(c, 1)
	require.NoError(t, err)

	snapshots, err = Assets.GetSnapshotsByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	assert.Equal(t, int64(114600), snapshots[0].Value)
}

func TestSnapshots_DeleteAllRemovesThemAndTheyAreScopedToUser(t *testing.T) {
	c := initializeAssetServiceTest(t)
	require.NoError(t, Assets.SetPrice(c, &models.AssetPrice{Uid: 1, Market: models.ASSET_MARKET_DFM, Symbol: "EMAAR", Price: models.ScaleAssetNumber(10), Currency: "AED", Source: "manual"}))
	require.NoError(t, Assets.CreateTrade(c, buildTestAssetTrade(t, emaarBuy(1, 1, 1000))))

	other, err := Assets.GetSnapshotsByUid(c, 2)
	require.NoError(t, err)
	assert.Len(t, other, 0)

	require.NoError(t, Assets.DeleteAllTradesAndPrices(c, 1))

	snapshots, err := Assets.GetSnapshotsByUid(c, 1)
	require.NoError(t, err)
	assert.Len(t, snapshots, 0)
}
