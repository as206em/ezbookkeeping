export enum AssetMarket {
    DFM = 'DFM',
    ADX = 'ADX',
    Crypto = 'CRYPTO',
    Manual = 'MANUAL'
}

export enum AssetTradeSide {
    Buy = 0,
    Sell = 1
}

export interface AssetTradeInput {
    readonly market: AssetMarket;
    readonly symbol: string;
    readonly side: AssetTradeSide;
    readonly quantity: number;
    readonly price: number;
    readonly fee: number;
    readonly currency: string;
    readonly tradeTime: number; // unix seconds
    readonly externalId?: string;
}

export interface AssetTradeModifyRequest extends AssetTradeInput {
    readonly id: string;
}

export interface AssetTradeInfoResponse extends AssetTradeInput {
    readonly id: string;
    readonly realizedProfit?: number;
}

export interface AssetTradeImportRequest {
    readonly trades: AssetTradeInput[];
}

export interface AssetTradeImportResponse {
    readonly createdCount: number;
    readonly skippedCount: number;
}

export interface AssetPriceSetRequest {
    readonly market: AssetMarket;
    readonly symbol: string;
    readonly price: number;
    readonly currency: string;
}

export interface AssetHoldingInfoResponse {
    readonly market: AssetMarket;
    readonly symbol: string;
    readonly currency: string;
    readonly quantity: number;
    readonly averageCost: number;
    readonly costBasis: number;
    readonly totalBought: number;
    readonly realizedProfit: number;
    readonly hasPrice: boolean;
    readonly price: number;
    readonly priceCurrency?: string;
    readonly priceTime: number;
    readonly priceUpdatedTime: number; // unix seconds when the price was fetched or entered
    readonly priceSource?: string;
    readonly value: number;
    readonly unrealizedProfit: number;
    readonly tradeCount: number;
    readonly lastTradeTime: number;
    readonly oversold: boolean;
}

export interface AssetPriceRefreshMarketResult {
    readonly market: AssetMarket;
    readonly updated: number;
    readonly failed?: string[];
    readonly message?: string;
}

export interface AssetPriceRefreshResponse {
    readonly markets: AssetPriceRefreshMarketResult[];
}

export interface AssetBinanceStatusResponse {
    readonly connected: boolean;
    readonly apiKeyHint?: string;
    readonly lastSyncTime: number; // unix seconds, 0 when never synced
    readonly lastSyncCreatedCount: number;
    readonly lastSyncError?: string;
}

export interface AssetBinanceConnectRequest {
    readonly apiKey: string;
    readonly apiSecret: string;
}

export interface AssetBinanceSyncResponse {
    readonly createdCount: number;
    readonly skippedCount: number;
    readonly pairsChecked: number;
    readonly error?: string;
}

export interface AssetSnapshotInfoResponse {
    readonly date: string; // yyyy-mm-dd
    readonly market: AssetMarket;
    readonly currency: string;
    readonly value: number;
    readonly costBasis: number;
}
