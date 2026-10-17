package errs

import "net/http"

// Error codes related to assets
var (
	ErrAssetTradeNotFound         = NewNormalError(NormalSubcategoryAsset, 0, http.StatusBadRequest, "asset trade not found")
	ErrAssetMarketInvalid         = NewNormalError(NormalSubcategoryAsset, 1, http.StatusBadRequest, "asset market is invalid")
	ErrAssetSymbolInvalid         = NewNormalError(NormalSubcategoryAsset, 2, http.StatusBadRequest, "asset symbol is invalid")
	ErrAssetTradeSideInvalid      = NewNormalError(NormalSubcategoryAsset, 3, http.StatusBadRequest, "asset trade side is invalid")
	ErrAssetQuantityInvalid       = NewNormalError(NormalSubcategoryAsset, 4, http.StatusBadRequest, "asset quantity is invalid")
	ErrAssetPriceInvalid          = NewNormalError(NormalSubcategoryAsset, 5, http.StatusBadRequest, "asset price is invalid")
	ErrAssetFeeInvalid            = NewNormalError(NormalSubcategoryAsset, 6, http.StatusBadRequest, "asset fee is invalid")
	ErrAssetCurrencyInvalid       = NewNormalError(NormalSubcategoryAsset, 7, http.StatusBadRequest, "asset currency is invalid")
	ErrAssetTradeTimeInvalid      = NewNormalError(NormalSubcategoryAsset, 8, http.StatusBadRequest, "asset trade time is invalid")
	ErrAssetImportTooManyTrades   = NewNormalError(NormalSubcategoryAsset, 9, http.StatusBadRequest, "too many trades in one import")
	ErrAssetNoTradesToImport      = NewNormalError(NormalSubcategoryAsset, 10, http.StatusBadRequest, "no trades to import")
	ErrAssetSymbolHasNoTrades     = NewNormalError(NormalSubcategoryAsset, 11, http.StatusBadRequest, "asset has no trades")
	ErrAssetPriceRefreshUnsupport = NewNormalError(NormalSubcategoryAsset, 12, http.StatusBadRequest, "there is no automatic price source for this market")
)
