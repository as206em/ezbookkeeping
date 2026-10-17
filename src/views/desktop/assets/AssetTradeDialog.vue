<template>
    <v-dialog persistent width="520" v-model="showState">
        <v-card class="dialog-layout-card">
            <v-toolbar color="primary">
                <v-toolbar-title>{{ dialogTitle }}</v-toolbar-title>
            </v-toolbar>

            <v-card-text class="pa-4">
                <v-row>
                    <v-col cols="12" sm="6">
                        <v-select label="Where" item-title="title" item-value="value" :disabled="saving || isEditing"
                                  :items="marketOptions" v-model="form.market" />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field persistent-placeholder :disabled="saving || isEditing"
                                      :label="isManual ? 'Asset name' : 'Symbol'"
                                      :placeholder="isManual ? 'Gold (physical)' : symbolPlaceholder"
                                      v-model="form.symbol" />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-btn-toggle mandatory divided density="comfortable" color="primary" class="w-100" :disabled="saving" v-model="form.side">
                            <v-btn class="flex-grow-1" :value="AssetTradeSide.Buy">Buy</v-btn>
                            <v-btn class="flex-grow-1" :value="AssetTradeSide.Sell">Sell</v-btn>
                        </v-btn-toggle>
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field type="date" label="Date" :disabled="saving" v-model="form.date" />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field type="number" min="0" step="any" persistent-placeholder :disabled="saving"
                                      :label="isManual ? 'Quantity (use 1)' : 'Quantity'" v-model="form.quantity" />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field type="number" min="0" step="any" persistent-placeholder :disabled="saving"
                                      :label="isManual ? 'Value / cost' : 'Price per unit'" :suffix="form.currency" v-model="form.price" />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field type="number" min="0" step="any" persistent-placeholder :disabled="saving"
                                      label="Fee" placeholder="0" :suffix="form.currency" v-model="form.fee" />
                    </v-col>
                    <v-col cols="12" sm="6">
                        <v-text-field maxlength="3" persistent-placeholder :disabled="saving"
                                      label="Currency" v-model="form.currency" />
                    </v-col>
                </v-row>
                <div class="text-body-medium text-medium-emphasis" v-if="isManual">
                    Manual assets (gold, property, funds) have no live price. Add one with quantity 1 and its value, then update the value from the holdings list.
                </div>
                <v-alert type="error" variant="tonal" density="compact" class="mt-3" v-if="errorMessage">{{ errorMessage }}</v-alert>
            </v-card-text>

            <v-card-actions class="px-4 pb-4">
                <v-spacer />
                <v-btn color="gray" :disabled="saving" @click="cancel">Cancel</v-btn>
                <v-btn color="primary" :disabled="saving" @click="save">{{ isEditing ? 'Save trade' : 'Add trade' }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue';

import { useAssetsStore } from '@/stores/asset.ts';

import { AssetMarket, AssetTradeSide, type AssetTradeInfoResponse } from '@/models/asset.ts';

import { ASSET_MARKET_NAMES, getDefaultAssetCurrency, parseAssetTradeTime } from '@/lib/asset.ts';

const assetsStore = useAssetsStore();

let resolveFunc: (() => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;
let fallbackCurrency = 'AED';
let editingTrade: AssetTradeInfoResponse | null = null;

const showState = ref<boolean>(false);
const saving = ref<boolean>(false);
const errorMessage = ref<string>('');
const isEditing = ref<boolean>(false);

const form = reactive({
    market: AssetMarket.DFM as AssetMarket,
    symbol: '',
    side: AssetTradeSide.Buy as AssetTradeSide,
    date: '',
    quantity: '' as string | number,
    price: '' as string | number,
    fee: '' as string | number,
    currency: 'AED'
});

const marketOptions = [AssetMarket.DFM, AssetMarket.ADX, AssetMarket.Crypto, AssetMarket.Manual]
    .map(market => ({ title: ASSET_MARKET_NAMES[market], value: market }));

const isManual = computed<boolean>(() => form.market === AssetMarket.Manual);
const dialogTitle = computed<string>(() => isEditing.value ? 'Edit trade' : 'Add trade');
const symbolPlaceholder = computed<string>(() => form.market === AssetMarket.Crypto ? 'BTC' : 'EMAAR');

watch(() => form.market, (market, oldMarket) => {
    if (!isEditing.value && form.currency === getDefaultAssetCurrency(oldMarket, fallbackCurrency)) {
        form.currency = getDefaultAssetCurrency(market, fallbackCurrency);
    }
});

function toDateText(unixTime: number): string {
    return new Date(unixTime * 1000).toISOString().slice(0, 10);
}

function open(trade: AssetTradeInfoResponse | null, defaultCurrency: string, market?: AssetMarket, symbol?: string): Promise<void> {
    fallbackCurrency = defaultCurrency;
    editingTrade = trade;
    isEditing.value = !!trade;
    errorMessage.value = '';
    saving.value = false;

    if (trade) {
        Object.assign(form, {
            market: trade.market, symbol: trade.symbol, side: trade.side, date: toDateText(trade.tradeTime),
            quantity: trade.quantity, price: trade.price, fee: trade.fee || '', currency: trade.currency
        });
    } else {
        const newMarket = market ?? AssetMarket.DFM;

        Object.assign(form, {
            market: newMarket, symbol: symbol ?? '', side: AssetTradeSide.Buy, date: toDateText(Math.floor(Date.now() / 1000)),
            quantity: '', price: '', fee: '', currency: getDefaultAssetCurrency(newMarket, defaultCurrency)
        });
    }

    showState.value = true;

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    const quantity = Number(form.quantity);
    const price = Number(form.price);
    const fee = form.fee === '' ? 0 : Number(form.fee);

    if (!form.symbol.trim()) {
        errorMessage.value = isManual.value ? 'Enter a name for this asset' : 'Enter a symbol';
        return;
    }

    if (!(quantity > 0)) {
        errorMessage.value = 'Quantity must be greater than 0';
        return;
    }

    if (form.price === '' || !(price >= 0)) {
        errorMessage.value = 'Enter a price';
        return;
    }

    if (!(fee >= 0)) {
        errorMessage.value = 'Fee cannot be negative';
        return;
    }

    if (!/^[A-Za-z]{3}$/.test(form.currency)) {
        errorMessage.value = 'Currency must be a 3-letter code, e.g. AED';
        return;
    }

    const dateChanged = !editingTrade || toDateText(editingTrade.tradeTime) !== form.date;
    const tradeTime = dateChanged ? parseAssetTradeTime(form.date) : editingTrade!.tradeTime;

    if (tradeTime === null) {
        errorMessage.value = 'Pick a date';
        return;
    }

    const trade = {
        market: form.market,
        symbol: form.symbol.trim(),
        side: form.side,
        quantity,
        price,
        fee,
        currency: form.currency.toUpperCase(),
        tradeTime
    };

    errorMessage.value = '';
    saving.value = true;

    const request = editingTrade
        ? assetsStore.modifyTrade({ ...trade, id: editingTrade.id })
        : assetsStore.addTrade(trade);

    request.then(() => {
        if (isManual.value && form.side === AssetTradeSide.Buy) {
            return assetsStore.setPrice({ market: trade.market, symbol: trade.symbol, price: trade.price, currency: trade.currency });
        }

        return Promise.resolve();
    }).then(() => {
        saving.value = false;
        showState.value = false;
        resolveFunc?.();
    }).catch((error: { message?: string; error?: { errorMessage?: string } }) => {
        saving.value = false;
        errorMessage.value = error.error?.errorMessage || error.message || 'Unable to save this trade';
    });
}

function cancel(): void {
    showState.value = false;
    rejectFunc?.();
}

defineExpose({
    open
});
</script>
