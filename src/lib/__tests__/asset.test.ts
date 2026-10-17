import { describe, expect, test } from 'vitest';

import { AssetMarket, AssetTradeSide, type AssetBinanceStatusResponse, type AssetHoldingInfoResponse, type AssetSnapshotInfoResponse } from '@/models/asset.ts';

import {
    ASSET_TRADE_CSV_TEMPLATE,
    ASSET_DATA_STALE_SECONDS,
    buildAssetHistory,
    formatAssetAge,
    getAssetBinanceSyncState,
    getAssetPriceFreshness,
    filterAssetHistoryByRange,
    filterHoldings,
    getAssetAllocation,
    getHoldingProfitRatio,
    getHoldingTotalProfit,
    summarizeAssetHoldings,
    getDefaultAssetCurrency,
    parseAssetNumber,
    parseAssetTradeRows,
    parseAssetTradeTime,
    parseCsvText,
    splitCryptoPair
} from '@/lib/asset.ts';

const NOON = (y: number, m: number, d: number) => Date.UTC(y, m - 1, d, 12, 0, 0) / 1000;

describe('parseCsvText', () => {
    test('parses rows and skips blank lines', () => {
        expect(parseCsvText('a,b\r\n1,2\r\n\r\n3,4\r\n')).toEqual([['a', 'b'], ['1', '2'], ['3', '4']]);
    });

    test('handles quoted cells with commas and doubled quotes', () => {
        expect(parseCsvText('a,b\n"x, y","say ""hi"""')).toEqual([['a', 'b'], ['x, y', 'say "hi"']]);
    });

    test('detects semicolons and tabs and strips the BOM', () => {
        expect(parseCsvText('﻿a;b\n1;2')).toEqual([['a', 'b'], ['1', '2']]);
        expect(parseCsvText('a\tb\n1\t2')).toEqual([['a', 'b'], ['1', '2']]);
    });

    test('keeps the last row without a trailing newline', () => {
        expect(parseCsvText('a,b\n1,2')).toEqual([['a', 'b'], ['1', '2']]);
    });
});

describe('parseAssetTradeTime', () => {
    test('reads ISO dates as noon UTC', () => {
        expect(parseAssetTradeTime('2026-10-12')).toBe(NOON(2026, 10, 12));
    });

    test('reads day-first dates', () => {
        expect(parseAssetTradeTime('12/10/2026')).toBe(NOON(2026, 10, 12));
        expect(parseAssetTradeTime('12-10-2026')).toBe(NOON(2026, 10, 12));
    });

    test('keeps the time when there is one', () => {
        expect(parseAssetTradeTime('2026-10-12 14:30:05')).toBe(Date.UTC(2026, 9, 12, 14, 30, 5) / 1000);
    });

    test('rejects impossible dates', () => {
        expect(parseAssetTradeTime('2026-02-31')).toBeNull();
        expect(parseAssetTradeTime('2026-13-01')).toBeNull();
        expect(parseAssetTradeTime('yesterday')).toBeNull();
    });
});

describe('parseAssetNumber', () => {
    test('reads plain numbers, separators and trailing units', () => {
        expect(parseAssetNumber('10.40')).toBe(10.4);
        expect(parseAssetNumber('1,234.5')).toBe(1234.5);
        expect(parseAssetNumber('0.00100000BTC')).toBe(0.001);
        expect(parseAssetNumber(' 5 ')).toBe(5);
    });

    test('rejects text', () => {
        expect(parseAssetNumber('abc')).toBeNull();
        expect(parseAssetNumber('')).toBeNull();
        expect(parseAssetNumber('1.2.3')).toBeNull();
    });
});

describe('splitCryptoPair', () => {
    test('splits dollar quoted pairs', () => {
        expect(splitCryptoPair('BTCUSDT')).toEqual({ symbol: 'BTC', quote: 'USDT' });
        expect(splitCryptoPair('eth/usdc')).toEqual({ symbol: 'ETH', quote: 'USDC' });
    });

    test('ignores pairs quoted in other assets', () => {
        expect(splitCryptoPair('ETHBTC')).toBeNull();
        expect(splitCryptoPair('USDT')).toBeNull();
    });
});

describe('getDefaultAssetCurrency', () => {
    test('uses the market currency and falls back for manual assets', () => {
        expect(getDefaultAssetCurrency(AssetMarket.DFM, 'EUR')).toBe('AED');
        expect(getDefaultAssetCurrency(AssetMarket.ADX, 'EUR')).toBe('AED');
        expect(getDefaultAssetCurrency(AssetMarket.Crypto, 'AED')).toBe('USD');
        expect(getDefaultAssetCurrency(AssetMarket.Manual, 'EUR')).toBe('EUR');
    });
});

describe('parseAssetTradeRows', () => {
    test('imports the downloadable template without errors', () => {
        const results = parseAssetTradeRows(parseCsvText(ASSET_TRADE_CSV_TEMPLATE), 'AED');

        expect(results).toHaveLength(3);
        expect(results.every(r => r.trade && !r.error)).toBe(true);
        expect(results[0]!.trade).toEqual({
            market: AssetMarket.DFM, symbol: 'EMAAR', side: AssetTradeSide.Buy,
            quantity: 1000, price: 10.4, fee: 15.6, currency: 'AED', tradeTime: NOON(2026, 10, 12)
        });
        expect(results[1]!.trade!.side).toBe(AssetTradeSide.Sell);
        expect(results[2]!.trade!.currency).toBe('USD');
    });

    test('reads a Binance trade history export', () => {
        const csv = 'Date(UTC),Pair,Side,Price,Executed,Amount,Fee\n' +
            '2026-09-28 10:15:00,BTCUSDT,BUY,109200.5,0.02000000BTC,2184.01USDT,3.40USDT\n' +
            '2026-10-03 08:00:00,ETHUSDT,SELL,2545.1,0.5ETH,1272.55USDT,0.0005BNB\n';
        const results = parseAssetTradeRows(parseCsvText(csv), 'AED');

        expect(results[0]!.trade).toMatchObject({ market: AssetMarket.Crypto, symbol: 'BTC', side: AssetTradeSide.Buy, quantity: 0.02, price: 109200.5, fee: 3.4, currency: 'USD' });
        expect(results[1]!.trade).toMatchObject({ market: AssetMarket.Crypto, symbol: 'ETH', side: AssetTradeSide.Sell, quantity: 0.5, fee: 0 });
    });

    test('passes a trade id through for de-duplication', () => {
        const rows = parseCsvText('date,market,symbol,side,quantity,price,id\n2026-10-12,DFM,EMAAR,buy,10,1,T-77');

        expect(parseAssetTradeRows(rows, 'AED')[0]!.trade!.externalId).toBe('T-77');
    });

    test('reports the line of each bad row and keeps the good ones', () => {
        const rows = parseCsvText(
            'date,market,symbol,side,quantity,price\n' +
            '2026-10-12,DFM,EMAAR,buy,10,1\n' +
            'not a date,DFM,EMAAR,buy,10,1\n' +
            '2026-10-12,DFM,EMAAR,hold,10,1\n' +
            '2026-10-12,DFM,EMAAR,buy,0,1\n' +
            '2026-10-12,NYSE,AAPL,buy,1,1\n' +
            '2026-10-12,,EMAAR,buy,1,1\n'
        );
        const results = parseAssetTradeRows(rows, 'AED');

        expect(results.map(r => r.line)).toEqual([2, 3, 4, 5, 6, 7]);
        expect(results[0]!.trade).toBeDefined();
        expect(results.slice(1).every(r => !r.trade && r.error)).toBe(true);
    });

    test('explains a missing column', () => {
        const results = parseAssetTradeRows(parseCsvText('date,symbol\n2026-10-12,EMAAR'), 'AED');

        expect(results).toHaveLength(1);
        expect(results[0]!.error).toContain('header');
    });

    test('returns nothing for an empty file', () => {
        expect(parseAssetTradeRows([], 'AED')).toEqual([]);
    });
});

function holding(overrides: Partial<AssetHoldingInfoResponse>): AssetHoldingInfoResponse {
    return {
        market: AssetMarket.DFM, symbol: 'X', currency: 'AED', quantity: 10, averageCost: 1, costBasis: 10, totalBought: 10,
        realizedProfit: 0, hasPrice: true, price: 2, priceCurrency: 'AED', priceTime: 0, priceUpdatedTime: 0, value: 20, unrealizedProfit: 10,
        tradeCount: 1, lastTradeTime: 0, oversold: false, ...overrides
    };
}

describe('holding profit', () => {
    test('adds unrealized and realized profit and divides by everything bought', () => {
        const h = holding({ unrealizedProfit: 30, realizedProfit: 20, totalBought: 200 });

        expect(getHoldingTotalProfit(h)).toBe(50);
        expect(getHoldingProfitRatio(h)).toBe(0.25);
    });

    test('has no ratio when nothing was bought', () => {
        expect(getHoldingProfitRatio(holding({ totalBought: 0 }))).toBeNull();
    });
});

describe('filterHoldings', () => {
    const held = holding({ symbol: 'HELD', value: 100 });
    const bigger = holding({ symbol: 'BIG', value: 500 });
    const sold = holding({ symbol: 'SOLD', quantity: 0, value: 0, realizedProfit: 5 });
    const crypto = holding({ symbol: 'BTC', market: AssetMarket.Crypto, value: 300 });
    const all = [sold, held, crypto, bigger];

    test('hides sold positions by default and orders by value', () => {
        expect(filterHoldings(all, false, '').map(h => h.symbol)).toEqual(['BIG', 'BTC', 'HELD']);
    });

    test('shows sold positions last when asked', () => {
        expect(filterHoldings(all, true, '').map(h => h.symbol)).toEqual(['BIG', 'BTC', 'HELD', 'SOLD']);
    });

    test('filters by market', () => {
        expect(filterHoldings(all, true, AssetMarket.Crypto).map(h => h.symbol)).toEqual(['BTC']);
    });
});

describe('summarizeAssetHoldings', () => {
    const toAed = (amount: number, currency: string) => currency === 'USD' ? amount * 3.6725 : currency === 'AED' ? amount : null;

    test('converts every holding to the target currency', () => {
        const summary = summarizeAssetHoldings([
            holding({ value: 100, unrealizedProfit: 10 }),
            holding({ market: AssetMarket.Crypto, currency: 'USD', priceCurrency: 'USD', value: 100, unrealizedProfit: 20, realizedProfit: 10 })
        ], toAed);

        expect(summary.value).toBeCloseTo(100 + 367.25);
        expect(summary.unrealizedProfit).toBeCloseTo(10 + 73.45);
        expect(summary.realizedProfit).toBeCloseTo(36.725);
        expect(summary.pricedHoldingCount).toBe(2);
        expect(summary.incomplete).toBe(false);
    });

    test('counts sold realized profit but no value, and unpriced holdings separately', () => {
        const summary = summarizeAssetHoldings([
            holding({ quantity: 0, value: 0, unrealizedProfit: 0, realizedProfit: 40 }),
            holding({ hasPrice: false, value: 0, unrealizedProfit: 0 })
        ], toAed);

        expect(summary.value).toBe(0);
        expect(summary.realizedProfit).toBe(40);
        expect(summary.unpricedHoldingCount).toBe(1);
        expect(summary.pricedHoldingCount).toBe(0);
    });

    test('flags totals as incomplete when a currency cannot be converted', () => {
        const summary = summarizeAssetHoldings([holding({ currency: 'XYZ', priceCurrency: 'XYZ', value: 50 })], toAed);

        expect(summary.incomplete).toBe(true);
        expect(summary.value).toBe(0);
    });
});

function snap(date: string, market: AssetMarket, currency: string, value: number, costBasis: number): AssetSnapshotInfoResponse {
    return { date, market, currency, value, costBasis };
}

describe('buildAssetHistory', () => {
    const toAed = (amount: number, currency: string) => currency === 'USD' ? amount * 3.6725 : currency === 'AED' ? amount : null;

    test('adds the markets of each day in the target currency, oldest day first', () => {
        const history = buildAssetHistory([
            snap('2026-10-17', AssetMarket.DFM, 'AED', 1100, 1000),
            snap('2026-10-16', AssetMarket.DFM, 'AED', 1000, 1000),
            snap('2026-10-17', AssetMarket.Crypto, 'USD', 100, 80)
        ], toAed);

        expect(history.incomplete).toBe(false);
        expect(history.points.map(p => p.date)).toEqual(['2026-10-16', '2026-10-17']);
        expect(history.points[0]).toEqual({ date: '2026-10-16', value: 1000, costBasis: 1000 });
        expect(history.points[1]!.value).toBeCloseTo(1100 + 367.25);
        expect(history.points[1]!.costBasis).toBeCloseTo(1000 + 293.8);
    });

    test('flags days with an unconvertible currency instead of hiding them', () => {
        const history = buildAssetHistory([snap('2026-10-17', AssetMarket.DFM, 'AED', 500, 400), snap('2026-10-17', AssetMarket.Manual, 'XYZ', 99, 99)], toAed);

        expect(history.incomplete).toBe(true);
        expect(history.points).toEqual([{ date: '2026-10-17', value: 500, costBasis: 400 }]);
    });

    test('is empty without snapshots', () => {
        expect(buildAssetHistory([], toAed)).toEqual({ points: [], incomplete: false });
    });
});

describe('filterAssetHistoryByRange', () => {
    const points = ['2026-01-01', '2026-09-01', '2026-10-01', '2026-10-17'].map(date => ({ date, value: 1, costBasis: 1 }));

    test('keeps the last N days up to today', () => {
        expect(filterAssetHistoryByRange(points, 30, '2026-10-17').map(p => p.date)).toEqual(['2026-10-01', '2026-10-17']);
        expect(filterAssetHistoryByRange(points, 182, '2026-10-17').map(p => p.date)).toEqual(['2026-09-01', '2026-10-01', '2026-10-17']);
    });

    test('keeps everything for All', () => {
        expect(filterAssetHistoryByRange(points, null, '2026-10-17')).toHaveLength(4);
    });
});

describe('getAssetAllocation', () => {
    const toAed = (amount: number, currency: string) => currency === 'USD' ? amount * 4 : currency === 'AED' ? amount : null;

    test('splits cash, stocks, crypto and manual assets into shares of the total', () => {
        const items = getAssetAllocation([
            holding({ market: AssetMarket.DFM, value: 100 }),
            holding({ market: AssetMarket.ADX, value: 100 }),
            holding({ market: AssetMarket.Crypto, currency: 'USD', priceCurrency: 'USD', value: 50 }),
            holding({ market: AssetMarket.Manual, value: 100 })
        ], 400, toAed);

        expect(items.map(i => [i.key, i.value])).toEqual([['cash', 400], ['stocks', 200], ['crypto', 200], ['other', 100]]);
        expect(items.map(i => i.share)).toEqual([400 / 900, 200 / 900, 200 / 900, 100 / 900]);
        expect(items.reduce((sum, i) => sum + i.share, 0)).toBeCloseTo(1);
    });

    test('leaves out empty groups, sold and unpriced holdings and negative cash', () => {
        const items = getAssetAllocation([
            holding({ market: AssetMarket.DFM, value: 100 }),
            holding({ market: AssetMarket.Crypto, quantity: 0, value: 0 }),
            holding({ market: AssetMarket.Manual, hasPrice: false, value: 0 })
        ], -50, toAed);

        expect(items.map(i => i.key)).toEqual(['stocks']);
        expect(items[0]!.share).toBe(1);
    });

    test('is empty when there is nothing', () => {
        expect(getAssetAllocation([], 0, toAed)).toEqual([]);
    });
});

describe('formatAssetAge', () => {
    test('uses the largest whole unit', () => {
        expect(formatAssetAge(5)).toBe('just now');
        expect(formatAssetAge(60)).toBe('1 minute ago');
        expect(formatAssetAge(59 * 60)).toBe('59 minutes ago');
        expect(formatAssetAge(3600)).toBe('1 hour ago');
        expect(formatAssetAge(5 * 3600 + 1800)).toBe('5 hours ago');
        expect(formatAssetAge(86400)).toBe('1 day ago');
        expect(formatAssetAge(3 * 86400 + 7200)).toBe('3 days ago');
    });
});

describe('getAssetPriceFreshness', () => {
    const NOW = 1_800_000_000;
    const fetched = (hoursAgo: number) => NOW - hoursAgo * 3600;

    test('reports the newest automatic refresh and no warnings when everything is recent', () => {
        const result = getAssetPriceFreshness([
            holding({ symbol: 'EMAAR', priceSource: 'dfm.ae', priceUpdatedTime: fetched(5) }),
            holding({ symbol: 'BTC', market: AssetMarket.Crypto, priceSource: 'binance.com', priceUpdatedTime: fetched(2) })
        ], NOW);

        expect(result).toEqual({ lastRefreshTime: fetched(2), staleAutomatic: [], staleManual: [] });
    });

    test('flags automatic prices fetched more than a day ago, by fetch time and not trading day', () => {
        const result = getAssetPriceFreshness([
            holding({ symbol: 'EMAAR', priceSource: 'dfm.ae', priceTime: NOW - 3 * 86400, priceUpdatedTime: fetched(2) }),
            holding({ symbol: 'ETH', market: AssetMarket.Crypto, priceSource: 'binance.com', priceUpdatedTime: fetched(30) })
        ], NOW);

        expect(result.staleAutomatic).toEqual(['ETH']);
        expect(result.lastRefreshTime).toBe(fetched(2));
    });

    test('a day exactly is still fine', () => {
        expect(getAssetPriceFreshness([holding({ priceSource: 'dfm.ae', priceUpdatedTime: NOW - ASSET_DATA_STALE_SECONDS })], NOW).staleAutomatic).toEqual([]);
    });

    test('flags prices set by hand but not the value of manual assets', () => {
        const result = getAssetPriceFreshness([
            holding({ symbol: 'ADNOCDIST', market: AssetMarket.ADX, priceSource: 'manual', priceUpdatedTime: fetched(50) }),
            holding({ symbol: 'Gold', market: AssetMarket.Manual, priceSource: 'manual', priceUpdatedTime: fetched(5000) }),
            holding({ symbol: 'FRESH', market: AssetMarket.ADX, priceSource: 'manual', priceUpdatedTime: fetched(1) })
        ], NOW);

        expect(result.staleManual).toEqual(['ADNOCDIST']);
        expect(result.lastRefreshTime).toBeNull();
    });

    test('ignores sold and unpriced holdings', () => {
        const result = getAssetPriceFreshness([
            holding({ quantity: 0, priceSource: 'dfm.ae', priceUpdatedTime: 1 }),
            holding({ hasPrice: false, priceSource: undefined, priceUpdatedTime: 0 })
        ], NOW);

        expect(result).toEqual({ lastRefreshTime: null, staleAutomatic: [], staleManual: [] });
    });
});

describe('getAssetBinanceSyncState', () => {
    const NOW = 1_800_000_000;
    const status = (overrides: Partial<AssetBinanceStatusResponse>): AssetBinanceStatusResponse => ({ connected: true, lastSyncTime: NOW - 3600, lastSyncCreatedCount: 0, ...overrides });

    test('covers each state', () => {
        expect(getAssetBinanceSyncState({ connected: false, lastSyncTime: 0, lastSyncCreatedCount: 0 }, NOW)).toBe('not-connected');
        expect(getAssetBinanceSyncState(status({ lastSyncTime: 0 }), NOW)).toBe('never-synced');
        expect(getAssetBinanceSyncState(status({ lastSyncError: 'Binance rejected the API key' }), NOW)).toBe('failed');
        expect(getAssetBinanceSyncState(status({ lastSyncTime: NOW - ASSET_DATA_STALE_SECONDS - 1 }), NOW)).toBe('stale');
        expect(getAssetBinanceSyncState(status({}), NOW)).toBe('ok');
    });

    test('a failed sync outranks an old one', () => {
        expect(getAssetBinanceSyncState(status({ lastSyncTime: NOW - 5 * 86400, lastSyncError: 'x' }), NOW)).toBe('failed');
    });
});
