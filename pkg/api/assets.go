package api

import (
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/binance"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

// AssetsApi represents asset api
type AssetsApi struct {
	assets *services.AssetService
}

// Initialize an asset api singleton instance
var (
	Assets = &AssetsApi{
		assets: services.Assets,
	}
)

// TradeListHandler returns all trades of current user with the profit realized by each sell
func (a *AssetsApi) TradeListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	trades, positions, err := a.assets.GetPositionsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[assets.TradeListHandler] failed to get trades for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	responses := make([]*models.AssetTradeInfoResponse, len(trades))

	for i, trade := range trades {
		var realized *float64

		if profit, exists := positions.RealizedProfit[trade.TradeId]; exists {
			realized = &profit
		}

		responses[i] = trade.ToAssetTradeInfoResponse(realized)
	}

	return responses, nil
}

// HoldingListHandler returns the holdings of current user calculated from all trades
func (a *AssetsApi) HoldingListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	_, positions, err := a.assets.GetPositionsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[assets.HoldingListHandler] failed to get holdings for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if positions.Holdings == nil {
		return []*models.AssetHoldingInfoResponse{}, nil
	}

	return positions.Holdings, nil
}

// TradeAddHandler saves a new trade of current user
func (a *AssetsApi) TradeAddHandler(c *core.WebContext) (any, *errs.Error) {
	var input models.AssetTradeInput
	err := c.ShouldBindJSON(&input)

	if err != nil {
		log.Warnf(c, "[assets.TradeAddHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	trade, err := a.assets.BuildTrade(uid, &input)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	err = a.assets.CreateTrade(c, trade)

	if err != nil {
		log.Errorf(c, "[assets.TradeAddHandler] failed to create trade for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[assets.TradeAddHandler] user \"uid:%d\" has added trade \"id:%d\"", uid, trade.TradeId)

	return trade.ToAssetTradeInfoResponse(nil), nil
}

// TradeModifyHandler replaces the details of a trade of current user
func (a *AssetsApi) TradeModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.AssetTradeModifyRequest
	err := c.ShouldBindJSON(&req)

	if err != nil {
		log.Warnf(c, "[assets.TradeModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	trade, err := a.assets.BuildTrade(uid, &req.AssetTradeInput)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	err = a.assets.ModifyTrade(c, uid, req.Id, trade)

	if err != nil {
		log.Errorf(c, "[assets.TradeModifyHandler] failed to modify trade \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[assets.TradeModifyHandler] user \"uid:%d\" has modified trade \"id:%d\"", uid, req.Id)

	return trade.ToAssetTradeInfoResponse(nil), nil
}

// TradeDeleteHandler deletes a trade of current user
func (a *AssetsApi) TradeDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.AssetTradeDeleteRequest
	err := c.ShouldBindJSON(&req)

	if err != nil {
		log.Warnf(c, "[assets.TradeDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.assets.DeleteTrade(c, uid, req.Id)

	if err != nil {
		log.Errorf(c, "[assets.TradeDeleteHandler] failed to delete trade \"id:%d\" for user \"uid:%d\", because %s", req.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[assets.TradeDeleteHandler] user \"uid:%d\" has deleted trade \"id:%d\"", uid, req.Id)

	return true, nil
}

// TradeImportHandler saves the new trades of an imported file for current user and skips the ones already imported
func (a *AssetsApi) TradeImportHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.AssetTradeImportRequest
	err := c.ShouldBindJSON(&req)

	if err != nil {
		log.Warnf(c, "[assets.TradeImportHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if len(req.Trades) > models.AssetMaxTradesPerImport {
		return nil, errs.ErrAssetImportTooManyTrades
	}

	uid := c.GetCurrentUid()
	trades := make([]*models.AssetTrade, len(req.Trades))

	for i, input := range req.Trades {
		if input == nil {
			return nil, errs.ErrIncompleteOrIncorrectSubmission
		}

		trades[i], err = a.assets.BuildTrade(uid, input)

		if err != nil {
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}
	}

	created, skipped, err := a.assets.ImportTrades(c, uid, trades)

	if err != nil {
		log.Errorf(c, "[assets.TradeImportHandler] failed to import trades for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[assets.TradeImportHandler] user \"uid:%d\" has imported %d trades and skipped %d", uid, created, skipped)

	return &models.AssetTradeImportResponse{CreatedCount: created, SkippedCount: skipped}, nil
}

// PriceSetHandler stores a manually entered price of an asset of current user
func (a *AssetsApi) PriceSetHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.AssetPriceSetRequest
	err := c.ShouldBindJSON(&req)

	if err != nil {
		log.Warnf(c, "[assets.PriceSetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if !models.IsValidAssetMarket(req.Market) {
		return nil, errs.ErrAssetMarketInvalid
	}

	symbol, ok := models.NormalizeAssetSymbol(req.Market, req.Symbol)

	if !ok {
		return nil, errs.ErrAssetSymbolInvalid
	}

	currency, ok := models.NormalizeAssetCurrency(req.Currency)

	if !ok {
		return nil, errs.ErrAssetCurrencyInvalid
	}

	if req.Price <= 0 || req.Price > 1e12 {
		return nil, errs.ErrAssetPriceInvalid
	}

	uid := c.GetCurrentUid()
	err = a.assets.SetPriceAndSnapshot(c, &models.AssetPrice{
		Uid:       uid,
		Market:    req.Market,
		Symbol:    symbol,
		Price:     models.ScaleAssetNumber(req.Price),
		Currency:  currency,
		PriceTime: time.Now().Unix(),
		Source:    models.ASSET_PRICE_SOURCE_MANUAL,
	})

	if err != nil {
		log.Errorf(c, "[assets.PriceSetHandler] failed to set price for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}

// PriceRefreshHandler fetches the latest prices of the held assets of current user
func (a *AssetsApi) PriceRefreshHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	response, err := a.assets.RefreshPrices(c, uid)

	if err != nil {
		log.Errorf(c, "[assets.PriceRefreshHandler] failed to refresh prices for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return response, nil
}

// BinanceStatusHandler returns whether current user has connected Binance and when it last synced
func (a *AssetsApi) BinanceStatusHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	connection, err := a.assets.GetBinanceConnection(c, uid)

	if err != nil {
		log.Errorf(c, "[assets.BinanceStatusHandler] failed to get binance connection for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return connection.ToAssetBinanceStatusResponse(), nil
}

// BinanceConnectHandler checks and stores a read-only Binance API key of current user
func (a *AssetsApi) BinanceConnectHandler(c *core.WebContext) (any, *errs.Error) {
	var req models.AssetBinanceConnectRequest
	err := c.ShouldBindJSON(&req)

	if err != nil {
		log.Warnf(c, "[assets.BinanceConnectHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	credentials := binance.Credentials{ApiKey: strings.TrimSpace(req.ApiKey), ApiSecret: strings.TrimSpace(req.ApiSecret)}

	if credentials.ApiKey == "" || credentials.ApiSecret == "" {
		return nil, errs.ErrAssetBinanceCredentialsEmpty
	}

	uid := c.GetCurrentUid()
	err = a.assets.ValidateBinanceCredentials(c, credentials)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	err = a.assets.SaveBinanceConnection(c, uid, credentials)

	if err != nil {
		log.Errorf(c, "[assets.BinanceConnectHandler] failed to save binance connection for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[assets.BinanceConnectHandler] user \"uid:%d\" has connected binance", uid)

	connection, err := a.assets.GetBinanceConnection(c, uid)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return connection.ToAssetBinanceStatusResponse(), nil
}

// BinanceSyncHandler pulls the new Binance spot trades of current user
func (a *AssetsApi) BinanceSyncHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	response, err := a.assets.SyncBinanceTrades(c, uid)

	if err != nil {
		log.Errorf(c, "[assets.BinanceSyncHandler] failed to sync binance for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[assets.BinanceSyncHandler] user \"uid:%d\" synced binance: %d new, %d skipped", uid, response.CreatedCount, response.SkippedCount)

	return response, nil
}

// BinanceDisconnectHandler removes the Binance API key of current user, keeping the trades already synced
func (a *AssetsApi) BinanceDisconnectHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	err := a.assets.DeleteBinanceConnection(c, uid)

	if err != nil {
		log.Errorf(c, "[assets.BinanceDisconnectHandler] failed to delete binance connection for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[assets.BinanceDisconnectHandler] user \"uid:%d\" has disconnected binance", uid)

	return true, nil
}

// SnapshotListHandler returns the daily history of what the holdings of current user were worth
func (a *AssetsApi) SnapshotListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	snapshots, err := a.assets.GetSnapshotsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[assets.SnapshotListHandler] failed to get snapshots for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	responses := make([]*models.AssetSnapshotInfoResponse, len(snapshots))

	for i, snapshot := range snapshots {
		responses[i] = snapshot.ToAssetSnapshotInfoResponse()
	}

	return responses, nil
}
