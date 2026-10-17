package models

import (
	"math"
	"regexp"
	"strings"
)

// AssetMarket represents where an asset is traded or priced
type AssetMarket string

// Asset markets
const (
	ASSET_MARKET_DFM    AssetMarket = "DFM"
	ASSET_MARKET_ADX    AssetMarket = "ADX"
	ASSET_MARKET_CRYPTO AssetMarket = "CRYPTO"
	ASSET_MARKET_MANUAL AssetMarket = "MANUAL"
)

// AssetTradeSide represents whether a trade buys or sells an asset
type AssetTradeSide byte

// Asset trade sides
const (
	ASSET_TRADE_SIDE_BUY  AssetTradeSide = 0
	ASSET_TRADE_SIDE_SELL AssetTradeSide = 1
)

// Asset price sources
const (
	ASSET_PRICE_SOURCE_MANUAL = "manual"
)

// AssetNumberScale is the fixed-point scale of quantities, prices and fees stored in the database
const AssetNumberScale float64 = 1e8

// AssetMaxTradesPerImport is the maximum number of trades accepted by one import request
const AssetMaxTradesPerImport = 5000

var assetSymbolPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]{0,31}$`)
var assetCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// AssetTrade represents a buy or sell of an asset
type AssetTrade struct {
	TradeId         int64          `xorm:"PK"`
	Uid             int64          `xorm:"INDEX(IDX_asset_trade_uid_deleted_trade_time) NOT NULL"`
	Deleted         bool           `xorm:"INDEX(IDX_asset_trade_uid_deleted_trade_time) NOT NULL"`
	Market          AssetMarket    `xorm:"VARCHAR(16) NOT NULL"`
	Symbol          string         `xorm:"VARCHAR(32) NOT NULL"`
	Side            AssetTradeSide `xorm:"TINYINT NOT NULL"`
	Quantity        int64          `xorm:"NOT NULL"`
	Price           int64          `xorm:"NOT NULL"`
	Fee             int64          `xorm:"NOT NULL"`
	Currency        string         `xorm:"VARCHAR(3) NOT NULL"`
	TradeTime       int64          `xorm:"INDEX(IDX_asset_trade_uid_deleted_trade_time) NOT NULL"`
	ExternalId      string         `xorm:"VARCHAR(64) NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// AssetPrice represents the latest known price of an asset
type AssetPrice struct {
	Uid             int64       `xorm:"PK"`
	Market          AssetMarket `xorm:"PK VARCHAR(16)"`
	Symbol          string      `xorm:"PK VARCHAR(32)"`
	Price           int64       `xorm:"NOT NULL"`
	Currency        string      `xorm:"VARCHAR(3) NOT NULL"`
	PriceTime       int64       `xorm:"NOT NULL"`
	Source          string      `xorm:"VARCHAR(32) NOT NULL"`
	UpdatedUnixTime int64
}

// AssetTradeInput represents one trade of an add, modify or import request
type AssetTradeInput struct {
	Market     AssetMarket    `json:"market" binding:"required"`
	Symbol     string         `json:"symbol" binding:"required"`
	Side       AssetTradeSide `json:"side" binding:"min=0,max=1"`
	Quantity   float64        `json:"quantity"`
	Price      float64        `json:"price"`
	Fee        float64        `json:"fee"`
	Currency   string         `json:"currency" binding:"required"`
	TradeTime  int64          `json:"tradeTime" binding:"required,min=1"`
	ExternalId string         `json:"externalId"`
}

// AssetTradeModifyRequest represents all parameters of the asset trade modify request
type AssetTradeModifyRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
	AssetTradeInput
}

// AssetTradeDeleteRequest represents all parameters of the asset trade delete request
type AssetTradeDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// AssetTradeImportRequest represents all parameters of the asset trade import request
type AssetTradeImportRequest struct {
	Trades []*AssetTradeInput `json:"trades" binding:"required,min=1,max=5000,dive"`
}

// AssetPriceSetRequest represents all parameters of the manual asset price request
type AssetPriceSetRequest struct {
	Market   AssetMarket `json:"market" binding:"required"`
	Symbol   string      `json:"symbol" binding:"required"`
	Price    float64     `json:"price"`
	Currency string      `json:"currency" binding:"required"`
}

// AssetTradeInfoResponse represents a view-object of an asset trade
type AssetTradeInfoResponse struct {
	Id             int64          `json:"id,string"`
	Market         AssetMarket    `json:"market"`
	Symbol         string         `json:"symbol"`
	Side           AssetTradeSide `json:"side"`
	Quantity       float64        `json:"quantity"`
	Price          float64        `json:"price"`
	Fee            float64        `json:"fee"`
	Currency       string         `json:"currency"`
	TradeTime      int64          `json:"tradeTime"`
	RealizedProfit *float64       `json:"realizedProfit,omitempty"`
}

// AssetTradeImportResponse represents the result of an asset trade import
type AssetTradeImportResponse struct {
	CreatedCount int `json:"createdCount"`
	SkippedCount int `json:"skippedCount"`
}

// AssetHoldingInfoResponse represents one asset position calculated from all trades of the asset
type AssetHoldingInfoResponse struct {
	Market         AssetMarket `json:"market"`
	Symbol         string      `json:"symbol"`
	Currency       string      `json:"currency"`
	Quantity       float64     `json:"quantity"`
	AverageCost    float64     `json:"averageCost"`
	CostBasis      float64     `json:"costBasis"`
	TotalBought    float64     `json:"totalBought"`
	RealizedProfit float64     `json:"realizedProfit"`
	HasPrice       bool        `json:"hasPrice"`
	Price          float64     `json:"price"`
	PriceCurrency  string      `json:"priceCurrency,omitempty"`
	PriceTime      int64       `json:"priceTime"`
	PriceSource    string      `json:"priceSource,omitempty"`
	Value          float64     `json:"value"`
	UnrealizedGain float64     `json:"unrealizedProfit"`
	TradeCount     int         `json:"tradeCount"`
	LastTradeTime  int64       `json:"lastTradeTime"`
	Oversold       bool        `json:"oversold"`
}

// AssetPriceRefreshMarketResult represents the result of refreshing the prices of one market
type AssetPriceRefreshMarketResult struct {
	Market  AssetMarket `json:"market"`
	Updated int         `json:"updated"`
	Failed  []string    `json:"failed,omitempty"`
	Message string      `json:"message,omitempty"`
}

// AssetPriceRefreshResponse represents the result of an asset price refresh
type AssetPriceRefreshResponse struct {
	Markets []*AssetPriceRefreshMarketResult `json:"markets"`
}

// IsValidAssetMarket returns whether the market is supported
func IsValidAssetMarket(market AssetMarket) bool {
	switch market {
	case ASSET_MARKET_DFM, ASSET_MARKET_ADX, ASSET_MARKET_CRYPTO, ASSET_MARKET_MANUAL:
		return true
	default:
		return false
	}
}

// NormalizeAssetSymbol returns the canonical form of a symbol in the market, or false if it is invalid
func NormalizeAssetSymbol(market AssetMarket, symbol string) (string, bool) {
	symbol = strings.TrimSpace(symbol)

	if market == ASSET_MARKET_MANUAL {
		return symbol, symbol != "" && len([]rune(symbol)) <= 32
	}

	symbol = strings.ToUpper(symbol)

	return symbol, assetSymbolPattern.MatchString(symbol)
}

// NormalizeAssetCurrency returns the canonical form of a currency code, or false if it is invalid
func NormalizeAssetCurrency(currency string) (string, bool) {
	currency = strings.ToUpper(strings.TrimSpace(currency))

	return currency, assetCurrencyPattern.MatchString(currency)
}

// ScaleAssetNumber converts a decimal number to its fixed-point database representation
func ScaleAssetNumber(value float64) int64 {
	return int64(math.Round(value * AssetNumberScale))
}

// UnscaleAssetNumber converts a fixed-point database number to a decimal number
func UnscaleAssetNumber(value int64) float64 {
	return float64(value) / AssetNumberScale
}

// ToAssetTradeInfoResponse returns a view-object according to database model
func (t *AssetTrade) ToAssetTradeInfoResponse(realizedProfit *float64) *AssetTradeInfoResponse {
	return &AssetTradeInfoResponse{
		Id:             t.TradeId,
		Market:         t.Market,
		Symbol:         t.Symbol,
		Side:           t.Side,
		Quantity:       UnscaleAssetNumber(t.Quantity),
		Price:          UnscaleAssetNumber(t.Price),
		Fee:            UnscaleAssetNumber(t.Fee),
		Currency:       t.Currency,
		TradeTime:      t.TradeTime,
		RealizedProfit: realizedProfit,
	}
}
