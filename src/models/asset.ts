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
