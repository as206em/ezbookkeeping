package services

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/binance"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// binanceQuoteAssets are the dollar-like assets whose spot pairs are synced; their trades are recorded in USD
var binanceQuoteAssets = []string{"USDT", "USDC", "FDUSD"}

const binanceTradesPageSize = 1000
const binanceMaxTradePagesPerPair = 50
const binanceExternalIdPrefix = "binance:"

// GetBinanceConnection returns the Binance connection of a user, or nil when not connected
func (s *AssetService) GetBinanceConnection(c core.Context, uid int64) (*models.AssetBinanceConnection, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	connection := &models.AssetBinanceConnection{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Get(connection)

	if err != nil {
		return nil, err
	}

	if !has {
		return nil, nil
	}

	return connection, nil
}

// SaveBinanceConnection stores the (encrypted) API key of a user, replacing the previous one
func (s *AssetService) SaveBinanceConnection(c core.Context, uid int64, credentials binance.Credentials) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	secretKey := s.CurrentConfig().SecretKey
	encryptedKey, err := utils.EncryptSecret(credentials.ApiKey, secretKey)

	if err != nil {
		return err
	}

	encryptedSecret, err := utils.EncryptSecret(credentials.ApiSecret, secretKey)

	if err != nil {
		return err
	}

	hint := credentials.ApiKey

	if len(hint) > 4 {
		hint = hint[len(hint)-4:]
	}

	now := time.Now().Unix()
	connection := &models.AssetBinanceConnection{
		Uid:             uid,
		ApiKey:          encryptedKey,
		ApiSecret:       encryptedSecret,
		ApiKeyHint:      hint,
		CreatedUnixTime: now,
		UpdatedUnixTime: now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=?", uid).Delete(&models.AssetBinanceConnection{})

		if err != nil {
			return err
		}

		_, err = sess.Insert(connection)

		return err
	})
}

// DeleteBinanceConnection removes the API key of a user. Trades already synced are kept.
func (s *AssetService) DeleteBinanceConnection(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=?", uid).Delete(&models.AssetBinanceConnection{})
		return err
	})
}

// ValidateBinanceCredentials checks with Binance that the key works and can only read
func (s *AssetService) ValidateBinanceCredentials(c core.Context, credentials binance.Credentials) error {
	restrictions, err := binance.Container.Current.GetKeyRestrictions(c, credentials)

	if err != nil {
		var apiErr *binance.APIError

		if errors.As(err, &apiErr) && apiErr.Status >= http.StatusBadRequest && apiErr.Status < http.StatusInternalServerError && apiErr.Status != http.StatusTooManyRequests {
			log.Warnf(c, "[asset_binance.ValidateBinanceCredentials] binance rejected the key, code %d", apiErr.Code)
			return errs.ErrAssetBinanceKeyInvalid
		}

		log.Errorf(c, "[asset_binance.ValidateBinanceCredentials] failed to reach binance, because %s", err.Error())
		return errs.ErrFailedToRequestRemoteApi
	}

	if !restrictions.EnableReading {
		return errs.ErrAssetBinanceKeyInvalid
	}

	if restrictions.EnableWithdrawals || restrictions.EnableSpotAndMarginTrading || restrictions.EnableMargin ||
		restrictions.EnableFutures || restrictions.EnableInternalTransfer {
		return errs.ErrAssetBinanceKeyNotReadOnly
	}

	return nil
}

// SyncBinanceTrades pulls the spot trades of the user's Binance account and imports the new ones.
// It checks the dollar-quoted pairs of every coin the account holds plus every crypto symbol the user already
// has trades for, so coins sold completely before the first sync are only found if they were imported from a CSV.
// Trades are deduplicated by their Binance trade id, so syncing again only adds new trades.
func (s *AssetService) SyncBinanceTrades(c core.Context, uid int64) (*models.AssetBinanceSyncResponse, error) {
	connection, err := s.GetBinanceConnection(c, uid)

	if err != nil {
		return nil, err
	}

	if connection == nil {
		return nil, errs.ErrAssetBinanceNotConnected
	}

	secretKey := s.CurrentConfig().SecretKey
	apiKey, err := utils.DecryptSecret(connection.ApiKey, secretKey)

	if err != nil {
		return nil, err
	}

	apiSecret, err := utils.DecryptSecret(connection.ApiSecret, secretKey)

	if err != nil {
		return nil, err
	}

	credentials := binance.Credentials{ApiKey: apiKey, ApiSecret: apiSecret}
	response, syncErr := s.syncBinanceTrades(c, uid, credentials)

	if syncErr != nil {
		log.Errorf(c, "[asset_binance.SyncBinanceTrades] sync failed for user \"uid:%d\", because %s", uid, syncErr.Error())
		response.Error = describeBinanceSyncError(syncErr)
	}

	s.saveBinanceSyncResult(c, uid, response)

	return response, nil
}

func (s *AssetService) syncBinanceTrades(c core.Context, uid int64, credentials binance.Credentials) (*models.AssetBinanceSyncResponse, error) {
	response := &models.AssetBinanceSyncResponse{}
	client := binance.Container.Current

	assets, err := client.GetBalanceAssets(c, credentials)

	if err != nil {
		return response, err
	}

	existingTrades, err := s.GetAllTradesByUid(c, uid)

	if err != nil {
		return response, err
	}

	bases := getBinanceBaseAssets(assets, existingTrades)
	var trades []*models.AssetTrade
	var fetchErr error

pairs:
	for _, base := range bases {
		for _, quote := range binanceQuoteAssets {
			response.PairsChecked++
			pairTrades, err := fetchBinancePairTrades(c, client, credentials, base, quote)

			if err != nil {
				fetchErr = err
				break pairs
			}

			for _, binanceTrade := range pairTrades {
				input, ok := mapBinanceTrade(binanceTrade, base, quote)

				if !ok {
					continue
				}

				trade, err := s.BuildTrade(uid, input)

				if err != nil {
					log.Warnf(c, "[asset_binance.syncBinanceTrades] skipped trade \"%s\", because %s", input.ExternalId, err.Error())
					continue
				}

				trades = append(trades, trade)
			}
		}
	}

	// trades fetched before an error are still imported, so a rate limit midway does not lose the work done
	if len(trades) > 0 {
		created, skipped, err := s.ImportTrades(c, uid, trades)

		if err != nil {
			return response, err
		}

		response.CreatedCount = created
		response.SkippedCount = skipped
	}

	return response, fetchErr
}

// getBinanceBaseAssets returns the coins to check: held coins plus crypto symbols the user already trades, without the quote assets
func getBinanceBaseAssets(balanceAssets []string, existingTrades []*models.AssetTrade) []string {
	isQuote := make(map[string]bool, len(binanceQuoteAssets))

	for _, quote := range binanceQuoteAssets {
		isQuote[quote] = true
	}

	seen := make(map[string]bool)
	var bases []string

	add := func(asset string) {
		asset = strings.ToUpper(strings.TrimSpace(asset))

		if asset != "" && !isQuote[asset] && !seen[asset] {
			seen[asset] = true
			bases = append(bases, asset)
		}
	}

	for _, asset := range balanceAssets {
		add(asset)
	}

	for _, trade := range existingTrades {
		if trade.Market == models.ASSET_MARKET_CRYPTO {
			add(trade.Symbol)
		}
	}

	sort.Strings(bases)

	return bases
}

// fetchBinancePairTrades returns all trades of one pair, paging by trade id. A pair Binance does not list has no trades.
func fetchBinancePairTrades(c core.Context, client binance.Client, credentials binance.Credentials, base string, quote string) ([]*binance.Trade, error) {
	pair := base + quote
	var all []*binance.Trade
	var fromId int64

	for page := 0; page < binanceMaxTradePagesPerPair; page++ {
		trades, err := client.GetTrades(c, credentials, pair, fromId, binanceTradesPageSize)

		if err != nil {
			var apiErr *binance.APIError

			if errors.As(err, &apiErr) && apiErr.Code == binance.ErrorCodeInvalidSymbol {
				return all, nil
			}

			return all, err
		}

		all = append(all, trades...)

		if len(trades) < binanceTradesPageSize {
			return all, nil
		}

		fromId = trades[len(trades)-1].Id + 1
	}

	return all, nil
}

// mapBinanceTrade converts a Binance spot trade to a trade input in USD.
// A fee paid in the quote asset is the trade fee. A fee paid in the coin bought reduces the quantity received.
// Fees paid in other assets (such as BNB) are not counted because they cannot be priced here.
func mapBinanceTrade(trade *binance.Trade, base string, quote string) (*models.AssetTradeInput, bool) {
	quantity, err := strconv.ParseFloat(trade.Quantity, 64)

	if err != nil {
		return nil, false
	}

	price, err := strconv.ParseFloat(trade.Price, 64)

	if err != nil {
		return nil, false
	}

	commission, _ := strconv.ParseFloat(trade.Commission, 64)
	fee := 0.0
	side := models.ASSET_TRADE_SIDE_SELL

	if trade.IsBuyer {
		side = models.ASSET_TRADE_SIDE_BUY
	}

	switch strings.ToUpper(trade.CommissionAsset) {
	case quote:
		fee = commission
	case base:
		if trade.IsBuyer {
			quantity -= commission
		}
	}

	if quantity <= 0 || price < 0 || fee < 0 {
		return nil, false
	}

	return &models.AssetTradeInput{
		Market:     models.ASSET_MARKET_CRYPTO,
		Symbol:     base,
		Side:       side,
		Quantity:   quantity,
		Price:      price,
		Fee:        fee,
		Currency:   "USD",
		TradeTime:  trade.Time / 1000,
		ExternalId: fmt.Sprintf("%s%s%s:%d", binanceExternalIdPrefix, base, quote, trade.Id),
	}, true
}

func describeBinanceSyncError(err error) string {
	var apiErr *binance.APIError

	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == http.StatusTooManyRequests || apiErr.Status == 418:
			return "Binance asked to slow down. Part of your trades may be missing, try again in a few minutes."
		case apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden || apiErr.Code == -2015 || apiErr.Code == -2014:
			return "Binance rejected the API key. Check that it is still valid and its IP restriction allows this server."
		}
	}

	return "Could not finish syncing with Binance. Try again later."
}

func (s *AssetService) saveBinanceSyncResult(c core.Context, uid int64, response *models.AssetBinanceSyncResponse) {
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Cols("last_sync_unix_time", "last_sync_created_count", "last_sync_error", "updated_unix_time").Where("uid=?", uid).Update(&models.AssetBinanceConnection{
			LastSyncUnixTime:     time.Now().Unix(),
			LastSyncCreatedCount: response.CreatedCount,
			LastSyncError:        response.Error,
			UpdatedUnixTime:      time.Now().Unix(),
		})

		return err
	})

	if err != nil {
		log.Errorf(c, "[asset_binance.saveBinanceSyncResult] failed to save sync result for user \"uid:%d\", because %s", uid, err.Error())
	}
}

// SyncBinanceTradesOfAllUsers syncs every user that has connected Binance
func (s *AssetService) SyncBinanceTradesOfAllUsers(c core.Context) error {
	for i := 0; i < s.UserDataDBCount(); i++ {
		var connections []*models.AssetBinanceConnection
		err := s.UserDataDBByIndex(i).NewSession(c).Cols("uid").Find(&connections)

		if err != nil {
			return err
		}

		for _, connection := range connections {
			_, err = s.SyncBinanceTrades(c, connection.Uid)

			if err != nil {
				log.Errorf(c, "[asset_binance.SyncBinanceTradesOfAllUsers] failed to sync user \"uid:%d\", because %s", connection.Uid, err.Error())
			}
		}
	}

	return nil
}
