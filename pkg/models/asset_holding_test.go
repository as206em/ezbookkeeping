package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestTrade(id int64, market AssetMarket, symbol string, side AssetTradeSide, quantity float64, price float64, fee float64, tradeTime int64) *AssetTrade {
	return &AssetTrade{
		TradeId:   id,
		Market:    market,
		Symbol:    symbol,
		Side:      side,
		Quantity:  ScaleAssetNumber(quantity),
		Price:     ScaleAssetNumber(price),
		Fee:       ScaleAssetNumber(fee),
		Currency:  "AED",
		TradeTime: tradeTime,
	}
}

func holdingOf(positions *AssetPositions, symbol string) *AssetHoldingInfoResponse {
	for _, holding := range positions.Holdings {
		if holding.Symbol == symbol {
			return holding
		}
	}

	return nil
}

func TestCalculateAssetPositions_AverageCostOfSeveralBuys(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(1, ASSET_MARKET_DFM, "EMAAR", ASSET_TRADE_SIDE_BUY, 4000, 9.2, 0, 100),
		newTestTrade(2, ASSET_MARKET_DFM, "EMAAR", ASSET_TRADE_SIDE_BUY, 1000, 10.4, 0, 200),
	}

	positions := CalculateAssetPositions(trades, nil)
	holding := holdingOf(positions, "EMAAR")

	assert.Equal(t, 5000.0, holding.Quantity)
	assert.InDelta(t, 9.44, holding.AverageCost, 1e-9)
	assert.InDelta(t, 47200.0, holding.CostBasis, 1e-6)
	assert.Equal(t, 0.0, holding.RealizedProfit)
	assert.False(t, holding.HasPrice)
}

func TestCalculateAssetPositions_FeesAreAddedToCostAndSubtractedFromProceeds(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(1, ASSET_MARKET_DFM, "EMAAR", ASSET_TRADE_SIDE_BUY, 100, 10, 5, 100),
		newTestTrade(2, ASSET_MARKET_DFM, "EMAAR", ASSET_TRADE_SIDE_SELL, 100, 12, 6, 200),
	}

	positions := CalculateAssetPositions(trades, nil)
	holding := holdingOf(positions, "EMAAR")

	assert.Equal(t, 0.0, holding.Quantity)
	assert.Equal(t, 0.0, holding.CostBasis)
	assert.InDelta(t, 189.0, holding.RealizedProfit, 1e-9)
	assert.InDelta(t, 189.0, positions.RealizedProfit[2], 1e-9)
}

func TestCalculateAssetPositions_PartialSellKeepsAverageCost(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(1, ASSET_MARKET_CRYPTO, "ETH", ASSET_TRADE_SIDE_BUY, 3.6, 8200, 0, 100),
		newTestTrade(2, ASSET_MARKET_CRYPTO, "ETH", ASSET_TRADE_SIDE_SELL, 0.5, 9310, 0, 200),
	}

	positions := CalculateAssetPositions(trades, nil)
	holding := holdingOf(positions, "ETH")

	assert.InDelta(t, 3.1, holding.Quantity, 1e-9)
	assert.InDelta(t, 8200.0, holding.AverageCost, 1e-6)
	assert.InDelta(t, 555.0, holding.RealizedProfit, 1e-6)
	assert.InDelta(t, 25420.0, holding.CostBasis, 1e-6)
}

func TestCalculateAssetPositions_AverageCostChangesOnlyOnBuys(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(1, ASSET_MARKET_DFM, "A", ASSET_TRADE_SIDE_BUY, 100, 10, 0, 100),
		newTestTrade(2, ASSET_MARKET_DFM, "A", ASSET_TRADE_SIDE_SELL, 50, 20, 0, 200),
		newTestTrade(3, ASSET_MARKET_DFM, "A", ASSET_TRADE_SIDE_BUY, 50, 30, 0, 300),
	}

	holding := holdingOf(CalculateAssetPositions(trades, nil), "A")

	assert.Equal(t, 100.0, holding.Quantity)
	assert.InDelta(t, 20.0, holding.AverageCost, 1e-9)
	assert.InDelta(t, 500.0, holding.RealizedProfit, 1e-9)
}

func TestCalculateAssetPositions_TradesAreReplayedInTimeOrder(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(2, ASSET_MARKET_DFM, "A", ASSET_TRADE_SIDE_SELL, 10, 15, 0, 200),
		newTestTrade(1, ASSET_MARKET_DFM, "A", ASSET_TRADE_SIDE_BUY, 10, 10, 0, 100),
	}

	positions := CalculateAssetPositions(trades, nil)

	assert.InDelta(t, 50.0, holdingOf(positions, "A").RealizedProfit, 1e-9)
	assert.False(t, holdingOf(positions, "A").Oversold)
}

func TestCalculateAssetPositions_SellingMoreThanHeldIsFlagged(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(1, ASSET_MARKET_CRYPTO, "BTC", ASSET_TRADE_SIDE_BUY, 1, 100, 0, 100),
		newTestTrade(2, ASSET_MARKET_CRYPTO, "BTC", ASSET_TRADE_SIDE_SELL, 2, 150, 0, 200),
	}

	holding := holdingOf(CalculateAssetPositions(trades, nil), "BTC")

	assert.True(t, holding.Oversold)
	assert.Equal(t, 0.0, holding.Quantity)
	assert.InDelta(t, 200.0, holding.RealizedProfit, 1e-9)
}

func TestCalculateAssetPositions_ValueAndUnrealizedProfitUseLatestPrice(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(1, ASSET_MARKET_DFM, "EMAAR", ASSET_TRADE_SIDE_BUY, 1000, 10, 0, 100),
	}
	prices := map[AssetMarket]map[string]*AssetPrice{
		ASSET_MARKET_DFM: {"EMAAR": {Market: ASSET_MARKET_DFM, Symbol: "EMAAR", Price: ScaleAssetNumber(11.46), Currency: "AED", PriceTime: 500, Source: "dfm.ae"}},
	}

	holding := holdingOf(CalculateAssetPositions(trades, prices), "EMAAR")

	assert.True(t, holding.HasPrice)
	assert.InDelta(t, 11460.0, holding.Value, 1e-6)
	assert.InDelta(t, 1460.0, holding.UnrealizedGain, 1e-6)
	assert.Equal(t, "dfm.ae", holding.PriceSource)
	assert.Equal(t, int64(500), holding.PriceTime)
}

func TestCalculateAssetPositions_SoldHoldingHasNoValue(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(1, ASSET_MARKET_DFM, "ALDAR", ASSET_TRADE_SIDE_BUY, 3000, 6.1, 0, 100),
		newTestTrade(2, ASSET_MARKET_DFM, "ALDAR", ASSET_TRADE_SIDE_SELL, 3000, 7.2, 0, 200),
	}
	prices := map[AssetMarket]map[string]*AssetPrice{
		ASSET_MARKET_DFM: {"ALDAR": {Price: ScaleAssetNumber(7.3)}},
	}

	holding := holdingOf(CalculateAssetPositions(trades, prices), "ALDAR")

	assert.Equal(t, 0.0, holding.Quantity)
	assert.Equal(t, 0.0, holding.Value)
	assert.Equal(t, 0.0, holding.UnrealizedGain)
	assert.InDelta(t, 3300.0, holding.RealizedProfit, 1e-6)
	assert.InDelta(t, 18300.0, holding.TotalBought, 1e-6)
}

func TestCalculateAssetPositions_SameSymbolInDifferentMarketsIsSeparate(t *testing.T) {
	trades := []*AssetTrade{
		newTestTrade(1, ASSET_MARKET_DFM, "ABC", ASSET_TRADE_SIDE_BUY, 10, 1, 0, 100),
		newTestTrade(2, ASSET_MARKET_ADX, "ABC", ASSET_TRADE_SIDE_BUY, 20, 2, 0, 100),
	}

	assert.Len(t, CalculateAssetPositions(trades, nil).Holdings, 2)
}

func TestNormalizeAssetSymbol(t *testing.T) {
	symbol, ok := NormalizeAssetSymbol(ASSET_MARKET_DFM, " emaar ")
	assert.True(t, ok)
	assert.Equal(t, "EMAAR", symbol)

	_, ok = NormalizeAssetSymbol(ASSET_MARKET_DFM, "bad symbol!")
	assert.False(t, ok)

	_, ok = NormalizeAssetSymbol(ASSET_MARKET_CRYPTO, "")
	assert.False(t, ok)

	symbol, ok = NormalizeAssetSymbol(ASSET_MARKET_MANUAL, " Gold (physical) ")
	assert.True(t, ok)
	assert.Equal(t, "Gold (physical)", symbol)
}

func TestScaleAssetNumber_RoundTrips(t *testing.T) {
	assert.Equal(t, int64(42000000), ScaleAssetNumber(0.42))
	assert.Equal(t, 0.42, UnscaleAssetNumber(ScaleAssetNumber(0.42)))
	assert.Equal(t, 401000.0, UnscaleAssetNumber(ScaleAssetNumber(401000)))
}
