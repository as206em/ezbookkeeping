package models

import (
	"sort"
)

// assetQuantityEpsilon is the smallest quantity (in fixed-point units) still considered held
const assetQuantityEpsilon int64 = 1

type assetPositionKey struct {
	market AssetMarket
	symbol string
}

type assetPosition struct {
	holding  *AssetHoldingInfoResponse
	quantity int64
}

// AssetPositions represents the holdings of a user and the profit realized by each sell trade
type AssetPositions struct {
	Holdings       []*AssetHoldingInfoResponse
	RealizedProfit map[int64]float64
}

// CalculateAssetPositions replays all trades with the average cost method.
// A buy adds quantity price + fee to the cost basis. A sell removes quantity at the current average cost and
// realizes quantity * (price - average cost) - fee. Selling more than is held realizes the excess at zero cost
// and marks the holding as oversold, which usually means earlier trades are missing from the history.
func CalculateAssetPositions(trades []*AssetTrade, prices map[AssetMarket]map[string]*AssetPrice) *AssetPositions {
	sorted := make([]*AssetTrade, len(trades))
	copy(sorted, trades)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TradeTime != sorted[j].TradeTime {
			return sorted[i].TradeTime < sorted[j].TradeTime
		}

		return sorted[i].TradeId < sorted[j].TradeId
	})

	positions := make(map[assetPositionKey]*assetPosition)
	order := make([]assetPositionKey, 0)
	result := &AssetPositions{RealizedProfit: make(map[int64]float64)}

	for _, trade := range sorted {
		key := assetPositionKey{market: trade.Market, symbol: trade.Symbol}
		position := positions[key]

		if position == nil {
			position = &assetPosition{holding: &AssetHoldingInfoResponse{
				Market:   trade.Market,
				Symbol:   trade.Symbol,
				Currency: trade.Currency,
			}}
			positions[key] = position
			order = append(order, key)
		}

		holding := position.holding
		holding.TradeCount++
		holding.LastTradeTime = trade.TradeTime
		quantity := UnscaleAssetNumber(trade.Quantity)
		price := UnscaleAssetNumber(trade.Price)
		fee := UnscaleAssetNumber(trade.Fee)

		if trade.Side == ASSET_TRADE_SIDE_BUY {
			cost := quantity*price + fee
			holding.CostBasis += cost
			holding.TotalBought += cost
			position.quantity += trade.Quantity
			continue
		}

		averageCost := 0.0

		if position.quantity > 0 {
			averageCost = holding.CostBasis / UnscaleAssetNumber(position.quantity)
		}

		sellQuantity := trade.Quantity

		if sellQuantity > position.quantity {
			sellQuantity = position.quantity
			holding.Oversold = true
		}

		costOfSold := averageCost * UnscaleAssetNumber(sellQuantity)
		realized := quantity*price - fee - costOfSold
		holding.RealizedProfit += realized
		result.RealizedProfit[trade.TradeId] = realized
		holding.CostBasis -= costOfSold
		position.quantity -= sellQuantity
	}

	for _, key := range order {
		position := positions[key]
		holding := position.holding

		if position.quantity < assetQuantityEpsilon {
			position.quantity = 0
			holding.CostBasis = 0
		}

		holding.Quantity = UnscaleAssetNumber(position.quantity)

		if position.quantity > 0 {
			holding.AverageCost = holding.CostBasis / holding.Quantity
		}

		if price := prices[key.market][key.symbol]; price != nil {
			holding.HasPrice = true
			holding.Price = UnscaleAssetNumber(price.Price)
			holding.PriceCurrency = price.Currency
			holding.PriceTime = price.PriceTime
			holding.PriceUpdatedTime = price.UpdatedUnixTime
			holding.PriceSource = price.Source

			if position.quantity > 0 {
				holding.Value = holding.Quantity * holding.Price
				holding.UnrealizedGain = holding.Value - holding.CostBasis
			}
		}

		result.Holdings = append(result.Holdings, holding)
	}

	return result
}
