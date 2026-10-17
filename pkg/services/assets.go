package services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/assetprices"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// AssetService represents asset trade and price service
type AssetService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize an asset service singleton instance
var (
	Assets = &AssetService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetAllTradesByUid returns all trades of a user ordered by trade time
func (s *AssetService) GetAllTradesByUid(c core.Context, uid int64) ([]*models.AssetTrade, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var trades []*models.AssetTrade
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("trade_time asc, trade_id asc").Find(&trades)

	return trades, err
}

// GetAllPricesByUid returns the latest known prices of a user by market and symbol
func (s *AssetService) GetAllPricesByUid(c core.Context, uid int64) (map[models.AssetMarket]map[string]*models.AssetPrice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var prices []*models.AssetPrice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Find(&prices)

	if err != nil {
		return nil, err
	}

	result := make(map[models.AssetMarket]map[string]*models.AssetPrice)

	for _, price := range prices {
		if result[price.Market] == nil {
			result[price.Market] = make(map[string]*models.AssetPrice)
		}

		result[price.Market][price.Symbol] = price
	}

	return result, nil
}

// GetPositionsByUid returns the holdings of a user and the profit realized by each sell trade
func (s *AssetService) GetPositionsByUid(c core.Context, uid int64) ([]*models.AssetTrade, *models.AssetPositions, error) {
	trades, err := s.GetAllTradesByUid(c, uid)

	if err != nil {
		return nil, nil, err
	}

	prices, err := s.GetAllPricesByUid(c, uid)

	if err != nil {
		return nil, nil, err
	}

	return trades, models.CalculateAssetPositions(trades, prices), nil
}

// BuildTrade validates a trade input and converts it to a database model
func (s *AssetService) BuildTrade(uid int64, input *models.AssetTradeInput) (*models.AssetTrade, error) {
	if !models.IsValidAssetMarket(input.Market) {
		return nil, errs.ErrAssetMarketInvalid
	}

	symbol, ok := models.NormalizeAssetSymbol(input.Market, input.Symbol)

	if !ok {
		return nil, errs.ErrAssetSymbolInvalid
	}

	currency, ok := models.NormalizeAssetCurrency(input.Currency)

	if !ok {
		return nil, errs.ErrAssetCurrencyInvalid
	}

	if input.Side != models.ASSET_TRADE_SIDE_BUY && input.Side != models.ASSET_TRADE_SIDE_SELL {
		return nil, errs.ErrAssetTradeSideInvalid
	}

	if !isValidAssetNumber(input.Quantity) || models.ScaleAssetNumber(input.Quantity) <= 0 {
		return nil, errs.ErrAssetQuantityInvalid
	}

	if !isValidAssetNumber(input.Price) {
		return nil, errs.ErrAssetPriceInvalid
	}

	if !isValidAssetNumber(input.Fee) {
		return nil, errs.ErrAssetFeeInvalid
	}

	if input.TradeTime <= 0 {
		return nil, errs.ErrAssetTradeTimeInvalid
	}

	trade := &models.AssetTrade{
		Uid:        uid,
		Market:     input.Market,
		Symbol:     symbol,
		Side:       input.Side,
		Quantity:   models.ScaleAssetNumber(input.Quantity),
		Price:      models.ScaleAssetNumber(input.Price),
		Fee:        models.ScaleAssetNumber(input.Fee),
		Currency:   currency,
		TradeTime:  input.TradeTime,
		ExternalId: input.ExternalId,
	}

	if len(trade.ExternalId) > 64 {
		return nil, errs.ErrIncompleteOrIncorrectSubmission
	}

	if trade.ExternalId == "" {
		trade.ExternalId = buildAssetTradeFingerprint(trade)
	}

	return trade, nil
}

// CreateTrade saves a new trade to database
func (s *AssetService) CreateTrade(c core.Context, trade *models.AssetTrade) error {
	if trade.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	trade.TradeId = s.GenerateUuid(uuid.UUID_TYPE_ASSET_TRADE)

	if trade.TradeId < 1 {
		return errs.ErrSystemIsBusy
	}

	now := time.Now().Unix()
	trade.Deleted = false
	trade.CreatedUnixTime = now
	trade.UpdatedUnixTime = now

	return s.UserDataDB(trade.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(trade)
		return err
	})
}

// ImportTrades saves the trades that are not in the database yet and returns how many were created and skipped.
// A trade counts as already imported when the user has a trade with the same external id (or the same fingerprint of
// market, symbol, side, quantity, price, fee and time), so importing the same file twice is safe. Identical rows inside
// one file are real separate fills and are all kept.
func (s *AssetService) ImportTrades(c core.Context, uid int64, trades []*models.AssetTrade) (int, int, error) {
	if uid <= 0 {
		return 0, 0, errs.ErrUserIdInvalid
	}

	if len(trades) < 1 {
		return 0, 0, errs.ErrAssetNoTradesToImport
	}

	created := 0
	skipped := 0

	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var existing []*models.AssetTrade
		err := sess.Cols("external_id").Where("uid=? AND deleted=?", uid, false).Find(&existing)

		if err != nil {
			return err
		}

		known := make(map[string]bool, len(existing)+len(trades))

		for _, trade := range existing {
			known[trade.ExternalId] = true
		}

		newTrades := make([]*models.AssetTrade, 0, len(trades))
		fingerprintCounts := make(map[string]int)

		for _, trade := range trades {
			if strings.HasPrefix(trade.ExternalId, assetFingerprintPrefix) {
				fingerprintCounts[trade.ExternalId]++

				if fingerprintCounts[trade.ExternalId] > 1 {
					trade.ExternalId = fmt.Sprintf("%s#%d", trade.ExternalId, fingerprintCounts[trade.ExternalId])
				}
			}

			if known[trade.ExternalId] {
				skipped++
				continue
			}

			known[trade.ExternalId] = true
			newTrades = append(newTrades, trade)
		}

		if len(newTrades) < 1 {
			return nil
		}

		tradeIds := s.GenerateUuids(uuid.UUID_TYPE_ASSET_TRADE, uint16(len(newTrades)))

		if len(tradeIds) != len(newTrades) {
			return errs.ErrSystemIsBusy
		}

		now := time.Now().Unix()

		for i, trade := range newTrades {
			trade.TradeId = tradeIds[i]
			trade.Uid = uid
			trade.Deleted = false
			trade.CreatedUnixTime = now
			trade.UpdatedUnixTime = now
		}

		_, err = sess.Insert(newTrades)

		if err == nil {
			created = len(newTrades)
		}

		return err
	})

	return created, skipped, err
}

// ModifyTrade replaces the details of an existing trade
func (s *AssetService) ModifyTrade(c core.Context, uid int64, tradeId int64, updated *models.AssetTrade) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		existing := &models.AssetTrade{}
		has, err := sess.Where("uid=? AND trade_id=? AND deleted=?", uid, tradeId, false).Get(existing)

		if err != nil {
			return err
		}

		if !has {
			return errs.ErrAssetTradeNotFound
		}

		updated.TradeId = tradeId
		updated.Uid = uid
		updated.Deleted = false
		updated.CreatedUnixTime = existing.CreatedUnixTime
		updated.UpdatedUnixTime = time.Now().Unix()

		_, err = sess.ID(tradeId).AllCols().Update(updated)

		return err
	})
}

// DeleteTrade soft-deletes a trade
func (s *AssetService) DeleteTrade(c core.Context, uid int64, tradeId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deleted, err := sess.Cols("deleted", "deleted_unix_time").Where("uid=? AND trade_id=? AND deleted=?", uid, tradeId, false).Update(&models.AssetTrade{
			Deleted:         true,
			DeletedUnixTime: now,
		})

		if err != nil {
			return err
		}

		if deleted < 1 {
			return errs.ErrAssetTradeNotFound
		}

		return nil
	})
}

// DeleteAllTradesAndPrices deletes all trades and prices of a user
func (s *AssetService) DeleteAllTradesAndPrices(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=?", uid).Delete(&models.AssetTrade{})

		if err != nil {
			return err
		}

		_, err = sess.Where("uid=?", uid).Delete(&models.AssetPrice{})

		return err
	})
}

// SetPrice stores a price for an asset, replacing the previous one
func (s *AssetService) SetPrice(c core.Context, price *models.AssetPrice) error {
	if price.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	price.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(price.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=? AND market=? AND symbol=?", price.Uid, price.Market, price.Symbol).Delete(&models.AssetPrice{})

		if err != nil {
			return err
		}

		_, err = sess.Insert(price)

		return err
	})
}

// RefreshPrices fetches the latest prices of all currently held assets of a user from the registered price providers.
// Markets without a provider are reported but never fail the refresh; their prices stay as entered manually.
func (s *AssetService) RefreshPrices(c core.Context, uid int64) (*models.AssetPriceRefreshResponse, error) {
	_, positions, err := s.GetPositionsByUid(c, uid)

	if err != nil {
		return nil, err
	}

	symbolsByMarket := make(map[models.AssetMarket][]string)

	for _, holding := range positions.Holdings {
		if holding.Quantity > 0 && holding.Market != models.ASSET_MARKET_MANUAL {
			symbolsByMarket[holding.Market] = append(symbolsByMarket[holding.Market], holding.Symbol)
		}
	}

	markets := make([]models.AssetMarket, 0, len(symbolsByMarket))

	for market := range symbolsByMarket {
		markets = append(markets, market)
	}

	sort.Slice(markets, func(i, j int) bool { return markets[i] < markets[j] })

	response := &models.AssetPriceRefreshResponse{Markets: make([]*models.AssetPriceRefreshMarketResult, 0, len(markets))}

	for _, market := range markets {
		response.Markets = append(response.Markets, s.refreshMarketPrices(c, uid, market, symbolsByMarket[market]))
	}

	return response, nil
}

func (s *AssetService) refreshMarketPrices(c core.Context, uid int64, market models.AssetMarket, symbols []string) *models.AssetPriceRefreshMarketResult {
	result := &models.AssetPriceRefreshMarketResult{Market: market}
	provider := assetprices.Container.GetProvider(market)

	if provider == nil {
		result.Failed = symbols
		result.Message = "No automatic price source for this market. Set prices manually."
		return result
	}

	quotes, err := provider.GetLatestQuotes(c, market, symbols)

	if err != nil {
		log.Errorf(c, "[assets.refreshMarketPrices] failed to get %s prices for user \"uid:%d\", because %s", market, uid, err.Error())
		result.Failed = symbols
		result.Message = "Could not reach the price source. Try again later or set prices manually."
		return result
	}

	found := make(map[string]bool, len(quotes))

	for _, quote := range quotes {
		err = s.SetPrice(c, &models.AssetPrice{
			Uid:       uid,
			Market:    market,
			Symbol:    quote.Symbol,
			Price:     models.ScaleAssetNumber(quote.Price),
			Currency:  quote.Currency,
			PriceTime: quote.PriceTime,
			Source:    provider.Name(),
		})

		if err != nil {
			log.Errorf(c, "[assets.refreshMarketPrices] failed to save %s price of \"%s\" for user \"uid:%d\", because %s", market, quote.Symbol, uid, err.Error())
			continue
		}

		found[quote.Symbol] = true
		result.Updated++
	}

	for _, symbol := range symbols {
		if !found[symbol] {
			result.Failed = append(result.Failed, symbol)
		}
	}

	return result
}

// RefreshPricesOfAllUsers refreshes the prices of every user that has held assets
func (s *AssetService) RefreshPricesOfAllUsers(c core.Context) error {
	for i := 0; i < s.UserDataDBCount(); i++ {
		var trades []*models.AssetTrade
		err := s.UserDataDBByIndex(i).NewSession(c).Distinct("uid").Cols("uid").Where("deleted=?", false).Find(&trades)

		if err != nil {
			return err
		}

		for _, trade := range trades {
			_, err = s.RefreshPrices(c, trade.Uid)

			if err != nil {
				log.Errorf(c, "[assets.RefreshPricesOfAllUsers] failed to refresh prices for user \"uid:%d\", because %s", trade.Uid, err.Error())
			}
		}
	}

	return nil
}

const assetFingerprintPrefix = "fp:"

func isValidAssetNumber(value float64) bool {
	return !math.IsNaN(value) && value >= 0 && value <= 1e12
}

func buildAssetTradeFingerprint(trade *models.AssetTrade) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%d|%d|%d|%d", trade.Market, trade.Symbol, trade.Side, trade.Quantity, trade.Price, trade.Fee, trade.TradeTime)))

	return assetFingerprintPrefix + hex.EncodeToString(sum[:])[:32]
}
