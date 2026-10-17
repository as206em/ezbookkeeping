<template>
    <v-row class="align-content-start">
        <v-col cols="12">
            <v-card :class="{ disabled: loading }">
                <template #title>
                    <div class="d-flex align-center flex-wrap gap-2">
                        <span>Assets</span>
                        <v-progress-circular indeterminate size="20" class="ms-3" v-if="loading || refreshing"/>
                        <v-spacer/>
                        <v-btn variant="tonal" :prepend-icon="mdiRefresh" :disabled="loading || refreshing" @click="refreshPrices">Refresh prices</v-btn>
                        <v-btn variant="tonal" :prepend-icon="mdiUpload" :disabled="loading" @click="importTrades">Import CSV</v-btn>
                        <v-btn color="primary" :prepend-icon="mdiPlus" :disabled="loading" @click="addTrade()">Add trade</v-btn>
                    </div>
                </template>

                <v-card-text>
                    <v-row>
                        <v-col cols="12" sm="6" md="3">
                            <div class="text-body-medium text-medium-emphasis">Net worth</div>
                            <div class="text-h5">{{ formatSummaryAmount(netWorth) }}</div>
                            <div class="text-body-medium text-medium-emphasis">Accounts {{ formatSummaryAmount(cashBalance) }} + assets</div>
                        </v-col>
                        <v-col cols="12" sm="6" md="3">
                            <div class="text-body-medium text-medium-emphasis">Assets value</div>
                            <div class="text-h5">{{ formatSummaryAmount(summary.value) }}</div>
                            <div class="text-body-medium text-medium-emphasis" v-if="summary.unpricedHoldingCount">
                                {{ summary.unpricedHoldingCount }} without a price
                            </div>
                        </v-col>
                        <v-col cols="12" sm="6" md="3">
                            <div class="text-body-medium text-medium-emphasis">Unrealized profit</div>
                            <div class="text-h5" :class="profitClass(summary.unrealizedProfit)">{{ formatSummaryAmount(summary.unrealizedProfit, true) }}</div>
                        </v-col>
                        <v-col cols="12" sm="6" md="3">
                            <div class="text-body-medium text-medium-emphasis">Realized profit</div>
                            <div class="text-h5" :class="profitClass(summary.realizedProfit)">{{ formatSummaryAmount(summary.realizedProfit, true) }}</div>
                        </v-col>
                    </v-row>
                    <div class="text-body-medium text-medium-emphasis mt-3" v-if="summary.incomplete">
                        Some amounts could not be converted to {{ defaultCurrency }} because an exchange rate is missing, so totals are partial.
                    </div>
                    <div class="text-body-medium text-medium-emphasis mt-3" v-if="lastPriceTime">
                        Prices as of {{ formatDateTime(lastPriceTime) }}. DFM and crypto refresh automatically twice a day; ADX prices are set by hand.
                    </div>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card :class="{ disabled: loading }">
                <v-card-text class="d-flex align-center flex-wrap gap-3">
                    <div class="flex-grow-1">
                        <div class="font-weight-medium">Binance</div>
                        <div class="text-body-medium text-medium-emphasis" v-if="!binanceStatus.connected">
                            Connect a read-only API key to pull your spot trades automatically, once a day or whenever you press Sync now.
                        </div>
                        <template v-else>
                            <div class="text-body-medium text-medium-emphasis">
                                Connected with the key ending {{ binanceStatus.apiKeyHint }} ·
                                <span v-if="binanceStatus.lastSyncTime">last synced {{ formatDateTime(binanceStatus.lastSyncTime) }}, {{ binanceStatus.lastSyncCreatedCount }} new trade{{ binanceStatus.lastSyncCreatedCount === 1 ? '' : 's' }}</span>
                                <span v-else>not synced yet</span>
                            </div>
                            <div class="text-body-medium text-warning" v-if="binanceStatus.lastSyncError">{{ binanceStatus.lastSyncError }}</div>
                        </template>
                    </div>
                    <template v-if="!binanceStatus.connected">
                        <v-btn variant="tonal" :disabled="syncing" @click="connectBinance">Connect Binance</v-btn>
                    </template>
                    <template v-else>
                        <v-progress-circular indeterminate size="20" v-if="syncing"/>
                        <v-btn variant="tonal" :prepend-icon="mdiSync" :disabled="syncing" @click="syncBinance">Sync now</v-btn>
                        <v-btn variant="text" color="error" :disabled="syncing" @click="disconnectBinance">Disconnect</v-btn>
                    </template>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12">
            <v-card :class="{ disabled: loading }">
                <v-card-text class="d-flex align-center flex-wrap gap-4">
                    <v-btn-toggle mandatory density="comfortable" color="primary" variant="outlined" divided v-model="marketFilter">
                        <v-btn value="">All</v-btn>
                        <v-btn :value="market" :key="market" v-for="market in filterMarkets">{{ ASSET_MARKET_NAMES[market] }}</v-btn>
                    </v-btn-toggle>
                    <v-spacer/>
                    <v-switch hide-details density="compact" color="primary" label="Show sold positions" v-model="showSold"/>
                </v-card-text>

                <v-table class="asset-holding-table" hover>
                    <thead>
                        <tr>
                            <th>Asset</th>
                            <th class="text-end">Quantity</th>
                            <th class="text-end">Avg buy</th>
                            <th class="text-end">Price</th>
                            <th class="text-end">Value</th>
                            <th class="text-end">Profit</th>
                        </tr>
                    </thead>
                    <tbody>
                        <template :key="holdingKey(holding)" v-for="holding in visibleHoldings">
                            <tr class="cursor-pointer" @click="toggleExpanded(holding)">
                                <td>
                                    <div class="d-flex align-center">
                                        <v-icon size="18" class="me-2" :icon="isExpanded(holding) ? mdiChevronDown : mdiChevronRight"/>
                                        <div>
                                            <div>
                                                <span class="font-weight-medium">{{ holding.symbol }}</span>
                                                <v-chip class="ms-2" size="x-small" variant="tonal" v-if="isHoldingSold(holding)">Sold</v-chip>
                                                <v-chip class="ms-2" size="x-small" variant="tonal" color="warning" v-if="holding.oversold">Check history</v-chip>
                                            </div>
                                            <div class="text-body-medium text-medium-emphasis">{{ ASSET_MARKET_NAMES[holding.market] }}</div>
                                        </div>
                                    </div>
                                </td>
                                <template v-if="isHoldingSold(holding)">
                                    <td class="text-end text-medium-emphasis">—</td>
                                    <td class="text-end text-medium-emphasis">—</td>
                                    <td class="text-end text-medium-emphasis">—</td>
                                    <td class="text-end text-medium-emphasis">—</td>
                                </template>
                                <template v-else>
                                    <td class="text-end">{{ formatQuantity(holding.quantity) }}</td>
                                    <td class="text-end">{{ formatPrice(holding.averageCost, holding.currency) }}</td>
                                    <td class="text-end">
                                        <div class="d-inline-flex align-center">
                                            <span v-if="holding.hasPrice">{{ formatPrice(holding.price, holding.priceCurrency || holding.currency) }}</span>
                                            <v-chip size="small" color="warning" variant="tonal" v-else>Set price</v-chip>
                                            <v-btn class="ms-1" density="compact" variant="text" size="small" :icon="mdiPencilOutline"
                                                   aria-label="Set price" @click.stop="editPrice(holding)"/>
                                        </div>
                                        <div class="text-body-medium text-medium-emphasis" v-if="holding.hasPrice">{{ describePrice(holding) }}</div>
                                    </td>
                                    <td class="text-end">{{ holding.hasPrice ? formatPrice(holding.value, holding.priceCurrency || holding.currency, 0) : '—' }}</td>
                                </template>
                                <td class="text-end" :class="profitClass(holdingProfit(holding))">
                                    <template v-if="isHoldingSold(holding) || holding.hasPrice">
                                        <div>{{ formatPrice(holdingProfit(holding), holding.currency, 0, true) }}</div>
                                        <div class="text-body-medium" v-if="getHoldingProfitRatio(holding) !== null">{{ formatRatio(getHoldingProfitRatio(holding)!) }}</div>
                                    </template>
                                    <span class="text-medium-emphasis" v-else>—</span>
                                </td>
                            </tr>
                            <tr class="asset-trade-rows" v-if="isExpanded(holding)">
                                <td colspan="6" class="pa-0">
                                    <v-table density="compact" class="bg-transparent">
                                        <tbody>
                                            <tr :key="trade.id" v-for="trade in tradesOf(holding)">
                                                <td class="ps-10">{{ formatDate(trade.tradeTime) }}</td>
                                                <td>{{ trade.side === AssetTradeSide.Buy ? 'Buy' : 'Sell' }}</td>
                                                <td class="text-end">{{ formatQuantity(trade.quantity) }} @ {{ formatPrice(trade.price, trade.currency) }}</td>
                                                <td class="text-end text-medium-emphasis">{{ trade.fee ? `fee ${formatPrice(trade.fee, trade.currency)}` : '' }}</td>
                                                <td class="text-end" :class="profitClass(trade.realizedProfit ?? 0)">
                                                    {{ trade.realizedProfit !== undefined ? `realized ${formatPrice(trade.realizedProfit, trade.currency, 0, true)}` : '' }}
                                                </td>
                                                <td class="text-end">
                                                    <v-btn density="compact" variant="text" size="small" :icon="mdiPencilOutline" aria-label="Edit trade" @click.stop="editTrade(trade)"/>
                                                    <v-btn density="compact" variant="text" size="small" color="error" :icon="mdiDeleteOutline" aria-label="Delete trade" @click.stop="deleteTrade(trade)"/>
                                                </td>
                                            </tr>
                                        </tbody>
                                    </v-table>
                                    <div class="px-4 py-2 text-body-medium text-medium-emphasis" v-if="holding.oversold">
                                        Sells in this history add up to more than the buys. Earlier buys are probably missing, so profit here may be overstated.
                                    </div>
                                </td>
                            </tr>
                        </template>
                        <tr v-if="!visibleHoldings.length">
                            <td colspan="6" class="text-center py-8">
                                <div class="text-h6" v-if="!allHoldings.length">Start tracking your assets</div>
                                <div class="text-medium-emphasis mb-4" v-if="!allHoldings.length">
                                    Add a trade, import your DFM, ADX or Binance trades from a CSV, or add something like gold as a manual asset.
                                </div>
                                <div class="text-medium-emphasis" v-else>Nothing to show with these filters.</div>
                            </td>
                        </tr>
                    </tbody>
                </v-table>
            </v-card>
        </v-col>
    </v-row>

    <v-dialog width="420" v-model="priceDialog.show">
        <v-card class="dialog-layout-card">
            <v-toolbar color="primary">
                <v-toolbar-title>Set price of {{ priceDialog.symbol }}</v-toolbar-title>
            </v-toolbar>
            <v-card-text class="pa-4">
                <v-text-field type="number" min="0" step="any" autofocus label="Price per unit" :suffix="priceDialog.currency"
                              v-model="priceDialog.price" @keyup.enter="savePrice"/>
                <div class="text-body-medium text-medium-emphasis">
                    This price is used until the next automatic refresh. ADX prices and manual assets are only changed by you.
                </div>
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
                <v-spacer/>
                <v-btn color="gray" @click="priceDialog.show = false">Cancel</v-btn>
                <v-btn color="primary" @click="savePrice">Save price</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <asset-trade-dialog ref="tradeDialog"/>
    <asset-import-dialog ref="importDialog"/>
    <asset-binance-dialog ref="binanceDialog"/>
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar"/>
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import AssetTradeDialog from './AssetTradeDialog.vue';
import AssetImportDialog from './AssetImportDialog.vue';
import AssetBinanceDialog from './AssetBinanceDialog.vue';

import { ref, reactive, computed, onMounted, useTemplateRef } from 'vue';

import { useAccountsStore } from '@/stores/account.ts';
import { useAssetsStore } from '@/stores/asset.ts';
import { useExchangeRatesStore } from '@/stores/exchangeRates.ts';
import { useUserStore } from '@/stores/user.ts';

import { AssetMarket, AssetTradeSide, type AssetHoldingInfoResponse, type AssetTradeInfoResponse } from '@/models/asset.ts';

import {
    ASSET_MARKET_NAMES,
    filterHoldings,
    getHoldingProfitRatio,
    getHoldingTotalProfit,
    isHoldingSold,
    summarizeAssetHoldings
} from '@/lib/asset.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';

import { mdiChevronDown, mdiChevronRight, mdiDeleteOutline, mdiPencilOutline, mdiPlus, mdiRefresh, mdiSync, mdiUpload } from '@mdi/js';

type SnackBarType = InstanceType<typeof SnackBar>;
type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type TradeDialogType = InstanceType<typeof AssetTradeDialog>;
type ImportDialogType = InstanceType<typeof AssetImportDialog>;
type BinanceDialogType = InstanceType<typeof AssetBinanceDialog>;

const filterMarkets = [AssetMarket.DFM, AssetMarket.ADX, AssetMarket.Crypto, AssetMarket.Manual];

const accountsStore = useAccountsStore();
const assetsStore = useAssetsStore();
const exchangeRatesStore = useExchangeRatesStore();
const userStore = useUserStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');
const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const tradeDialog = useTemplateRef<TradeDialogType>('tradeDialog');
const importDialog = useTemplateRef<ImportDialogType>('importDialog');
const binanceDialog = useTemplateRef<BinanceDialogType>('binanceDialog');

const loading = ref<boolean>(true);
const refreshing = ref<boolean>(false);
const syncing = ref<boolean>(false);
const showSold = ref<boolean>(false);
const marketFilter = ref<AssetMarket | ''>('');
const expanded = reactive<Record<string, boolean>>({});
const priceDialog = reactive({ show: false, market: AssetMarket.DFM as AssetMarket, symbol: '', currency: 'AED', price: '' as string | number });

const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);
const allHoldings = computed(() => assetsStore.allHoldings);
const binanceStatus = computed(() => assetsStore.binanceStatus);
const visibleHoldings = computed(() => filterHoldings(allHoldings.value, showSold.value, marketFilter.value));

// Amounts in the app are whole minor units (cents), so values are converted to cents, exchanged, and shown back in whole units.
function toDefaultCurrency(amount: number, currency: string): number | null {
    if (currency === defaultCurrency.value) {
        return amount;
    }

    const exchanged = exchangeRatesStore.getExchangedAmount(parseBigDecimal(Math.round(amount * 100)), currency, defaultCurrency.value);

    return exchanged ? exchanged.toDoubleNumber() / 100 : null;
}

const summary = computed(() => summarizeAssetHoldings(allHoldings.value, toDefaultCurrency));

const cashBalance = computed<number>(() => {
    const netAssets = accountsStore.getNetAssets(true);
    const value = typeof netAssets === 'object' && 'value' in netAssets ? netAssets.value : netAssets;

    return typeof value === 'object' && 'toDoubleNumber' in value ? value.toDoubleNumber() / 100 : 0;
});

const netWorth = computed<number>(() => cashBalance.value + summary.value.value);

const lastPriceTime = computed<number>(() => allHoldings.value.reduce((latest, holding) => holding.hasPrice && !isHoldingSold(holding) ? Math.max(latest, holding.priceTime) : latest, 0));

function holdingKey(holding: AssetHoldingInfoResponse): string {
    return `${holding.market}:${holding.symbol}`;
}

function isExpanded(holding: AssetHoldingInfoResponse): boolean {
    return !!expanded[holdingKey(holding)];
}

function toggleExpanded(holding: AssetHoldingInfoResponse): void {
    expanded[holdingKey(holding)] = !expanded[holdingKey(holding)];
}

function tradesOf(holding: AssetHoldingInfoResponse): AssetTradeInfoResponse[] {
    return assetsStore.allTrades
        .filter(trade => trade.market === holding.market && trade.symbol === holding.symbol)
        .sort((a, b) => b.tradeTime - a.tradeTime);
}

function holdingProfit(holding: AssetHoldingInfoResponse): number {
    return getHoldingTotalProfit(holding);
}

function profitClass(amount: number): string {
    return amount > 0 ? 'text-success' : amount < 0 ? 'text-error' : '';
}

function formatNumber(value: number, maxFractionDigits: number): string {
    return new Intl.NumberFormat(undefined, { maximumFractionDigits: maxFractionDigits }).format(value);
}

function formatQuantity(value: number): string {
    return formatNumber(value, 8);
}

function formatPrice(value: number, currency: string, fractionDigits?: number, withSign?: boolean): string {
    const digits = fractionDigits ?? (Math.abs(value) >= 1000 ? 0 : 2);
    const text = new Intl.NumberFormat(undefined, {
        style: 'currency', currency, minimumFractionDigits: digits, maximumFractionDigits: Math.max(digits, Math.abs(value) < 1000 && fractionDigits === undefined ? 4 : digits)
    }).format(Math.abs(value));

    if (value < 0) {
        return '-' + text;
    }

    return withSign && value > 0 ? '+' + text : text;
}

function formatSummaryAmount(value: number, withSign?: boolean): string {
    return formatPrice(value, defaultCurrency.value, 0, withSign);
}

function formatRatio(ratio: number): string {
    return (ratio > 0 ? '+' : '') + (ratio * 100).toFixed(1) + '%';
}

function formatDate(unixTime: number): string {
    return new Date(unixTime * 1000).toISOString().slice(0, 10);
}

function formatDateTime(unixTime: number): string {
    return new Date(unixTime * 1000).toLocaleString();
}

function describePrice(holding: AssetHoldingInfoResponse): string {
    const source = holding.priceSource === 'manual' ? 'set by you' : holding.priceSource;

    return `${source ?? ''} · ${formatDate(holding.priceTime)}`;
}

function load(): void {
    loading.value = true;

    Promise.all([
        accountsStore.loadAllAccounts({ force: false }),
        exchangeRatesStore.getLatestExchangeRates({ silent: true, force: false }),
        assetsStore.loadAssets(),
        assetsStore.loadBinanceStatus()
    ]).then(() => {
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function reloadAssets(): Promise<void> {
    return assetsStore.loadAssets().catch(error => {
        snackbar.value?.showError(error);
    });
}

function addTrade(market?: AssetMarket, symbol?: string): void {
    tradeDialog.value?.open(null, defaultCurrency.value, market, symbol).then(() => {
        snackbar.value?.showMessage('Trade added');
        return reloadAssets();
    }).catch(() => {});
}

function editTrade(trade: AssetTradeInfoResponse): void {
    tradeDialog.value?.open(trade, defaultCurrency.value).then(() => {
        snackbar.value?.showMessage('Trade saved');
        return reloadAssets();
    }).catch(() => {});
}

function deleteTrade(trade: AssetTradeInfoResponse): void {
    confirmDialog.value?.open('Delete this trade? Your holdings and profit will be recalculated.', { color: 'error' }).then(() => {
        return assetsStore.deleteTrade(trade.id).then(() => {
            snackbar.value?.showMessage('Trade deleted');
            return reloadAssets();
        });
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function importTrades(): void {
    importDialog.value?.open(defaultCurrency.value).then(result => {
        snackbar.value?.showMessage(result.skippedCount
            ? `Imported ${result.createdCount} trades, skipped ${result.skippedCount} already in your list`
            : `Imported ${result.createdCount} trades`);
        return reloadAssets();
    }).catch(() => {});
}

function editPrice(holding: AssetHoldingInfoResponse): void {
    priceDialog.market = holding.market;
    priceDialog.symbol = holding.symbol;
    priceDialog.currency = holding.priceCurrency || holding.currency;
    priceDialog.price = holding.hasPrice ? holding.price : '';
    priceDialog.show = true;
}

function savePrice(): void {
    const price = Number(priceDialog.price);

    if (priceDialog.price === '' || !(price > 0)) {
        snackbar.value?.showMessage('Enter a price greater than 0');
        return;
    }

    assetsStore.setPrice({ market: priceDialog.market, symbol: priceDialog.symbol, price, currency: priceDialog.currency }).then(() => {
        priceDialog.show = false;
        snackbar.value?.showMessage('Price saved');
        return reloadAssets();
    }).catch(error => {
        snackbar.value?.showError(error);
    });
}

function connectBinance(): void {
    binanceDialog.value?.open().then(() => {
        snackbar.value?.showMessage('Binance connected. Reading your trade history…');
        syncBinance();
    }).catch(() => {});
}

function syncBinance(): void {
    syncing.value = true;

    assetsStore.syncBinance().then(result => {
        if (result.error) {
            snackbar.value?.showMessage(result.createdCount
                ? `Added ${result.createdCount} trades, but the sync did not finish. ${result.error}`
                : result.error);
        } else {
            snackbar.value?.showMessage(result.createdCount
                ? `Added ${result.createdCount} new Binance trade${result.createdCount === 1 ? '' : 's'}`
                : 'No new Binance trades');
        }

        return Promise.all([reloadAssets(), assetsStore.loadBinanceStatus()]);
    }).catch(error => {
        snackbar.value?.showError(error);
    }).finally(() => {
        syncing.value = false;
    });
}

function disconnectBinance(): void {
    confirmDialog.value?.open('Disconnect Binance? Trades already synced stay in your list, new ones will no longer be pulled.', { color: 'error' }).then(() => {
        return assetsStore.disconnectBinance().then(() => {
            snackbar.value?.showMessage('Binance disconnected');
        });
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function refreshPrices(): void {
    refreshing.value = true;

    assetsStore.refreshPrices().then(result => {
        const updated = result.markets.reduce((sum, market) => sum + market.updated, 0);
        const problems = result.markets.filter(market => market.failed && market.failed.length);
        const details = problems.map(market => `${ASSET_MARKET_NAMES[market.market]}: ${market.failed!.join(', ')}`).join('; ');

        snackbar.value?.showMessage(problems.length
            ? `Updated ${updated} prices. Could not update ${details}. Set those prices by hand.`
            : (updated ? `Updated ${updated} prices` : 'Nothing to refresh'));

        return reloadAssets();
    }).catch(error => {
        snackbar.value?.showError(error);
    }).finally(() => {
        refreshing.value = false;
    });
}

onMounted(load);
</script>

<style>
.asset-holding-table .asset-trade-rows > td {
    background: rgba(var(--v-theme-on-surface), 0.03);
}
</style>
