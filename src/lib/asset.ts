import { AssetMarket, AssetTradeSide, type AssetHoldingInfoResponse, type AssetSnapshotInfoResponse, type AssetTradeInput } from '@/models/asset.ts';

export const ASSET_MARKET_NAMES: Record<AssetMarket, string> = {
    [AssetMarket.DFM]: 'DFM',
    [AssetMarket.ADX]: 'ADX',
    [AssetMarket.Crypto]: 'Binance / crypto',
    [AssetMarket.Manual]: 'Manual'
};

export const ASSET_TRADE_CSV_TEMPLATE =
    'date,market,symbol,side,quantity,price,fee\n' +
    '2026-10-12,DFM,EMAAR,buy,1000,10.40,15.60\n' +
    '2026-10-13,ADX,ALDAR,sell,500,7.20,8.00\n' +
    '2026-10-14,CRYPTO,BTC,buy,0.02,109000,3.40\n';

const CRYPTO_QUOTE_ASSETS = ['USDT', 'USDC', 'FDUSD', 'BUSD', 'TUSD', 'USD'];

export interface ParsedAssetTradeRow {
    readonly line: number;
    readonly trade?: AssetTradeInput;
    readonly error?: string;
}

export function getDefaultAssetCurrency(market: AssetMarket, fallbackCurrency: string): string {
    switch (market) {
        case AssetMarket.DFM:
        case AssetMarket.ADX:
            return 'AED';
        case AssetMarket.Crypto:
            return 'USD';
        default:
            return fallbackCurrency;
    }
}

/**
 * Splits CSV text into rows of cells. Handles quoted cells, doubled quotes, CRLF, a leading BOM
 * and comma, semicolon or tab separators (detected from the first line).
 */
export function parseCsvText(text: string): string[][] {
    const content = text.replace(/^﻿/, '');
    const firstLine = content.split(/\r?\n/, 1)[0] ?? '';
    const delimiter = [',', ';', '\t'].map(d => ({ d, count: firstLine.split(d).length - 1 })).sort((a, b) => b.count - a.count)[0]!.d;
    const rows: string[][] = [];
    let row: string[] = [];
    let cell = '';
    let inQuotes = false;

    for (let i = 0; i < content.length; i++) {
        const char = content[i]!;

        if (inQuotes) {
            if (char === '"' && content[i + 1] === '"') {
                cell += '"';
                i++;
            } else if (char === '"') {
                inQuotes = false;
            } else {
                cell += char;
            }
        } else if (char === '"') {
            inQuotes = true;
        } else if (char === delimiter) {
            row.push(cell);
            cell = '';
        } else if (char === '\n' || char === '\r') {
            if (char === '\r' && content[i + 1] === '\n') {
                i++;
            }

            row.push(cell);
            cell = '';

            if (row.some(value => value.trim() !== '')) {
                rows.push(row);
            }

            row = [];
        } else {
            cell += char;
        }
    }

    row.push(cell);

    if (row.some(value => value.trim() !== '')) {
        rows.push(row);
    }

    return rows;
}

function normalizeHeader(header: string): string {
    return header.toLowerCase().replace(/\(.*?\)/g, '').replace(/[^a-z]/g, '');
}

const COLUMN_ALIASES: Record<string, string[]> = {
    date: ['date', 'time', 'datetime', 'tradedate', 'tradetime', 'executiondate'],
    market: ['market', 'exchange', 'platform'],
    symbol: ['symbol', 'ticker', 'pair', 'asset', 'security', 'code'],
    side: ['side', 'type', 'action', 'direction', 'buysell'],
    quantity: ['quantity', 'qty', 'executed', 'amount', 'shares', 'units', 'volume'],
    price: ['price', 'avgprice', 'rate', 'executionprice'],
    fee: ['fee', 'fees', 'commission'],
    currency: ['currency', 'ccy'],
    id: ['id', 'tradeid', 'orderid', 'externalid', 'reference']
};

function findColumns(header: string[]): Record<string, number> {
    const normalized = header.map(normalizeHeader);
    const columns: Record<string, number> = {};

    for (const [field, aliases] of Object.entries(COLUMN_ALIASES)) {
        for (const alias of aliases) {
            const index = normalized.indexOf(alias);

            if (index >= 0 && !Object.values(columns).includes(index)) {
                columns[field] = index;
                break;
            }
        }
    }

    return columns;
}

/** Parses a date such as 2026-10-12, 2026-10-12 14:30:05, 12/10/2026 or 12-10-2026 (day first) as UTC unix seconds. Dates without a time count as noon. */
export function parseAssetTradeTime(text: string): number | null {
    const value = text.trim();
    let match = /^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})(?:[T\s]+(\d{1,2}):(\d{2})(?::(\d{2}))?)?/.exec(value);
    let year: number, month: number, day: number, rest: (string | undefined)[];

    if (match) {
        [year, month, day] = [Number(match[1]), Number(match[2]), Number(match[3])];
        rest = [match[4], match[5], match[6]];
    } else {
        match = /^(\d{1,2})[-/.](\d{1,2})[-/.](\d{4})(?:[T\s]+(\d{1,2}):(\d{2})(?::(\d{2}))?)?/.exec(value);

        if (!match) {
            return null;
        }

        [day, month, year] = [Number(match[1]), Number(match[2]), Number(match[3])];
        rest = [match[4], match[5], match[6]];
    }

    if (month < 1 || month > 12 || day < 1 || day > 31) {
        return null;
    }

    const hasTime = rest[0] !== undefined;
    const time = Date.UTC(year, month - 1, day, hasTime ? Number(rest[0]) : 12, hasTime ? Number(rest[1]) : 0, hasTime ? Number(rest[2] ?? 0) : 0);
    const date = new Date(time);

    if (date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) {
        return null;
    }

    return Math.floor(time / 1000);
}

/** Parses a number that may contain thousand separators and a trailing unit such as 0.001BTC. Returns null when there is no number. */
export function parseAssetNumber(text: string): number | null {
    const match = /^\s*(-?\d[\d,]*(?:\.\d+)?|-?\.\d+)\s*[A-Za-z]*\s*$/.exec(text);

    if (!match) {
        return null;
    }

    const value = Number(match[1]!.replace(/,/g, ''));

    return Number.isFinite(value) ? value : null;
}

function parseMarket(text: string): AssetMarket | null {
    switch (text.trim().toUpperCase()) {
        case 'DFM':
        case 'DUBAI':
            return AssetMarket.DFM;
        case 'ADX':
        case 'ADSM':
        case 'ABU DHABI':
            return AssetMarket.ADX;
        case 'CRYPTO':
        case 'BINANCE':
            return AssetMarket.Crypto;
        case 'MANUAL':
            return AssetMarket.Manual;
        default:
            return null;
    }
}

function parseSide(text: string): AssetTradeSide | null {
    switch (text.trim().toLowerCase()) {
        case 'buy':
        case 'b':
        case 'bought':
            return AssetTradeSide.Buy;
        case 'sell':
        case 's':
        case 'sold':
            return AssetTradeSide.Sell;
        default:
            return null;
    }
}

/** Returns the coin of a Binance pair such as BTCUSDT and the quote asset it is priced in, or null when the pair is not quoted in a dollar asset. */
export function splitCryptoPair(pair: string): { symbol: string; quote: string } | null {
    const value = pair.trim().toUpperCase().replace(/[/_-]/g, '');

    for (const quote of CRYPTO_QUOTE_ASSETS) {
        if (value.length > quote.length && value.endsWith(quote)) {
            return { symbol: value.slice(0, value.length - quote.length), quote };
        }
    }

    return null;
}

/**
 * Converts CSV rows (the first row is the header) to trades. Every row gets either a trade or an error, so the
 * user can fix the file or import the valid rows. A Binance trade history export works as is: the pair column
 * (BTCUSDT) sets the market and symbol, and a fee is only counted when it is paid in the quote asset.
 */
export function parseAssetTradeRows(rows: string[][], fallbackCurrency: string): ParsedAssetTradeRow[] {
    if (rows.length < 1) {
        return [];
    }

    const columns = findColumns(rows[0]!);
    const results: ParsedAssetTradeRow[] = [];

    if (columns['date'] === undefined || columns['symbol'] === undefined || columns['side'] === undefined
        || columns['quantity'] === undefined || columns['price'] === undefined) {
        return [{ line: 1, error: 'The header needs the columns date, symbol, side, quantity and price (market and fee are optional)' }];
    }

    for (let i = 1; i < rows.length; i++) {
        const row = rows[i]!;
        const line = i + 1;
        const cell = (field: string): string => columns[field] === undefined ? '' : (row[columns[field]!] ?? '').trim();

        const tradeTime = parseAssetTradeTime(cell('date'));

        if (tradeTime === null) {
            results.push({ line, error: `Date "${cell('date')}" is not recognised (use YYYY-MM-DD)` });
            continue;
        }

        const side = parseSide(cell('side'));

        if (side === null) {
            results.push({ line, error: `Side "${cell('side')}" must be buy or sell` });
            continue;
        }

        let market = cell('market') === '' ? null : parseMarket(cell('market'));
        let symbol = cell('symbol').toUpperCase();
        let feeIsInQuoteAsset = true;
        const pair = splitCryptoPair(symbol);

        if (cell('market') !== '' && market === null) {
            results.push({ line, error: `Market "${cell('market')}" must be DFM, ADX or CRYPTO` });
            continue;
        }

        if (market === null || market === AssetMarket.Crypto) {
            if (pair) {
                market = AssetMarket.Crypto;
                symbol = pair.symbol;
                const feeUnit = /[A-Za-z]+$/.exec(cell('fee'))?.[0]?.toUpperCase();
                feeIsInQuoteAsset = !feeUnit || feeUnit === pair.quote;
            }
        }

        if (market === null) {
            results.push({ line, error: 'Market is missing (use DFM, ADX or CRYPTO)' });
            continue;
        }

        if (!/^[A-Z0-9][A-Z0-9._-]{0,31}$/.test(symbol)) {
            results.push({ line, error: `Symbol "${cell('symbol')}" is not valid` });
            continue;
        }

        const quantity = parseAssetNumber(cell('quantity'));

        if (quantity === null || quantity <= 0) {
            results.push({ line, error: `Quantity "${cell('quantity')}" must be a positive number` });
            continue;
        }

        const price = parseAssetNumber(cell('price'));

        if (price === null || price < 0) {
            results.push({ line, error: `Price "${cell('price')}" must be a number` });
            continue;
        }

        let fee = 0;

        if (cell('fee') !== '' && feeIsInQuoteAsset) {
            const parsedFee = parseAssetNumber(cell('fee'));

            if (parsedFee === null || parsedFee < 0) {
                results.push({ line, error: `Fee "${cell('fee')}" must be a number` });
                continue;
            }

            fee = parsedFee;
        }

        const currency = (cell('currency') || getDefaultAssetCurrency(market, fallbackCurrency)).toUpperCase();

        if (!/^[A-Z]{3}$/.test(currency)) {
            results.push({ line, error: `Currency "${cell('currency')}" must be a 3-letter code` });
            continue;
        }

        results.push({
            line,
            trade: {
                market,
                symbol,
                side,
                quantity,
                price,
                fee,
                currency,
                tradeTime,
                ...(cell('id') ? { externalId: cell('id').slice(0, 64) } : {})
            }
        });
    }

    return results;
}

export type AssetAmountConverter = (amount: number, currency: string) => number | null;

export interface AssetSummary {
    readonly value: number;
    readonly unrealizedProfit: number;
    readonly realizedProfit: number;
    readonly pricedHoldingCount: number;
    readonly unpricedHoldingCount: number;
    /** true when some amount could not be converted to the target currency, so the totals are partial */
    readonly incomplete: boolean;
}

export function isHoldingSold(holding: AssetHoldingInfoResponse): boolean {
    return holding.quantity <= 0;
}

export function getHoldingTotalProfit(holding: AssetHoldingInfoResponse): number {
    return holding.unrealizedProfit + holding.realizedProfit;
}

/** Total profit (unrealized + realized) as a ratio of everything ever bought, or null if nothing was bought */
export function getHoldingProfitRatio(holding: AssetHoldingInfoResponse): number | null {
    return holding.totalBought > 0 ? getHoldingTotalProfit(holding) / holding.totalBought : null;
}

export function filterHoldings(holdings: AssetHoldingInfoResponse[], showSold: boolean, market: AssetMarket | ''): AssetHoldingInfoResponse[] {
    return holdings
        .filter(holding => (showSold || !isHoldingSold(holding)) && (!market || holding.market === market))
        .sort((a, b) => {
            if (isHoldingSold(a) !== isHoldingSold(b)) {
                return isHoldingSold(a) ? 1 : -1;
            }

            return b.value - a.value || a.symbol.localeCompare(b.symbol);
        });
}

/** Adds up values and profits of holdings in the currency the converter returns. Holdings without a price count as unpriced and add no value. */
export function summarizeAssetHoldings(holdings: AssetHoldingInfoResponse[], convert: AssetAmountConverter): AssetSummary {
    let value = 0;
    let unrealizedProfit = 0;
    let realizedProfit = 0;
    let pricedHoldingCount = 0;
    let unpricedHoldingCount = 0;
    let incomplete = false;

    for (const holding of holdings) {
        const priceCurrency = holding.priceCurrency || holding.currency;
        const converted = (amount: number, currency: string): number => {
            const result = convert(amount, currency);

            if (result === null) {
                incomplete = true;
                return 0;
            }

            return result;
        };

        realizedProfit += converted(holding.realizedProfit, holding.currency);

        if (isHoldingSold(holding)) {
            continue;
        }

        if (!holding.hasPrice) {
            unpricedHoldingCount++;
            continue;
        }

        pricedHoldingCount++;
        value += converted(holding.value, priceCurrency);
        unrealizedProfit += converted(holding.unrealizedProfit, priceCurrency);
    }

    return { value, unrealizedProfit, realizedProfit, pricedHoldingCount, unpricedHoldingCount, incomplete };
}

export interface AssetHistoryPoint {
    readonly date: string; // yyyy-mm-dd
    readonly value: number;
    readonly costBasis: number;
}

export interface AssetHistory {
    readonly points: AssetHistoryPoint[];
    /** true when some amounts could not be converted to the target currency, so those days are understated */
    readonly incomplete: boolean;
}

export const ASSET_HISTORY_RANGES: { label: string; days: number | null }[] = [
    { label: '1M', days: 30 },
    { label: '6M', days: 182 },
    { label: '1Y', days: 365 },
    { label: 'All', days: null }
];

/** Adds the snapshots of each day (one per market and currency) in the currency the converter returns, oldest day first. */
export function buildAssetHistory(snapshots: AssetSnapshotInfoResponse[], convert: AssetAmountConverter): AssetHistory {
    const days = new Map<string, { value: number; costBasis: number }>();
    let incomplete = false;

    for (const snapshot of snapshots) {
        const value = convert(snapshot.value, snapshot.currency);
        const costBasis = convert(snapshot.costBasis, snapshot.currency);

        if (value === null || costBasis === null) {
            incomplete = true;
        }

        const day = days.get(snapshot.date) ?? { value: 0, costBasis: 0 };
        day.value += value ?? 0;
        day.costBasis += costBasis ?? 0;
        days.set(snapshot.date, day);
    }

    const points = [...days.entries()]
        .sort((a, b) => a[0].localeCompare(b[0]))
        .map(([date, day]) => ({ date, value: day.value, costBasis: day.costBasis }));

    return { points, incomplete };
}

/** Keeps the points of the last `days` days up to today (yyyy-mm-dd); null keeps everything. */
export function filterAssetHistoryByRange(points: AssetHistoryPoint[], days: number | null, today: string): AssetHistoryPoint[] {
    if (days === null) {
        return points;
    }

    const start = new Date(`${today}T00:00:00Z`);
    start.setUTCDate(start.getUTCDate() - days);
    const startText = start.toISOString().slice(0, 10);

    return points.filter(point => point.date >= startText);
}

export type AssetAllocationKey = 'cash' | 'stocks' | 'crypto' | 'other';

export interface AssetAllocationItem {
    readonly key: AssetAllocationKey;
    readonly label: string;
    readonly value: number;
    /** share of the total, 0 to 1 */
    readonly share: number;
}

const ALLOCATION_LABELS: Record<AssetAllocationKey, string> = {
    cash: 'Cash',
    stocks: 'Stocks',
    crypto: 'Crypto',
    other: 'Other'
};

/** Splits cash and priced holdings into cash, stocks (DFM and ADX), crypto and other (manual assets). Empty groups are left out. */
export function getAssetAllocation(holdings: AssetHoldingInfoResponse[], cash: number, convert: AssetAmountConverter): AssetAllocationItem[] {
    const totals: Record<AssetAllocationKey, number> = { cash: Math.max(cash, 0), stocks: 0, crypto: 0, other: 0 };

    for (const holding of holdings) {
        if (isHoldingSold(holding) || !holding.hasPrice) {
            continue;
        }

        const value = convert(holding.value, holding.priceCurrency || holding.currency);

        if (value === null || value <= 0) {
            continue;
        }

        if (holding.market === AssetMarket.DFM || holding.market === AssetMarket.ADX) {
            totals.stocks += value;
        } else if (holding.market === AssetMarket.Crypto) {
            totals.crypto += value;
        } else {
            totals.other += value;
        }
    }

    const total = totals.cash + totals.stocks + totals.crypto + totals.other;

    if (total <= 0) {
        return [];
    }

    return (Object.keys(totals) as AssetAllocationKey[])
        .filter(key => totals[key] > 0)
        .map(key => ({ key, label: ALLOCATION_LABELS[key], value: totals[key], share: totals[key] / total }));
}
