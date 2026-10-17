<template>
    <v-dialog persistent width="640" v-model="showState">
        <v-card class="dialog-layout-card">
            <v-toolbar color="primary">
                <v-toolbar-title>Import trades from CSV</v-toolbar-title>
            </v-toolbar>

            <v-card-text class="pa-4">
                <div class="text-body-medium text-medium-emphasis mb-4">
                    Works for DFM and ADX trades you copy into a file, and for the Binance trade history export.
                    Importing the same file twice is safe: trades already in your list are skipped.
                </div>

                <v-file-input accept=".csv,.txt,text/csv" label="CSV file" prepend-icon="" :prepend-inner-icon="mdiFileDelimitedOutline"
                              :disabled="importing" :show-size="true" v-model="file" @update:model-value="onFileChange" />

                <div class="mb-2">
                    <v-btn variant="text" size="small" :prepend-icon="mdiDownloadOutline" @click="downloadTemplate">Download template</v-btn>
                </div>

                <template v-if="parsed.length">
                    <div class="d-flex flex-wrap gap-4 my-3">
                        <v-chip color="success" variant="tonal">{{ validTrades.length }} ready to import</v-chip>
                        <v-chip color="error" variant="tonal" v-if="errorRows.length">{{ errorRows.length }} need a fix</v-chip>
                    </div>

                    <v-table density="compact" class="mb-3" v-if="validTrades.length">
                        <thead>
                            <tr><th>Date</th><th>Asset</th><th>Side</th><th class="text-end">Quantity</th><th class="text-end">Price</th></tr>
                        </thead>
                        <tbody>
                            <tr :key="row.line" v-for="row in previewRows">
                                <td>{{ formatDate(row.trade!.tradeTime) }}</td>
                                <td>{{ row.trade!.symbol }} <span class="text-medium-emphasis">{{ row.trade!.market }}</span></td>
                                <td>{{ row.trade!.side === AssetTradeSide.Buy ? 'Buy' : 'Sell' }}</td>
                                <td class="text-end">{{ row.trade!.quantity }}</td>
                                <td class="text-end">{{ row.trade!.price }} {{ row.trade!.currency }}</td>
                            </tr>
                        </tbody>
                    </v-table>
                    <div class="text-body-medium text-medium-emphasis mb-3" v-if="validTrades.length > previewRows.length">
                        and {{ validTrades.length - previewRows.length }} more
                    </div>

                    <v-alert type="warning" variant="tonal" density="compact" class="mb-2" v-if="errorRows.length">
                        <div :key="row.line" v-for="row in errorRows.slice(0, 8)">Line {{ row.line }}: {{ row.error }}</div>
                        <div v-if="errorRows.length > 8">and {{ errorRows.length - 8 }} more</div>
                        <div class="mt-1">Rows with a problem are left out. Fix the file and import it again to add them.</div>
                    </v-alert>
                </template>

                <v-alert type="error" variant="tonal" density="compact" class="mt-3" v-if="errorMessage">{{ errorMessage }}</v-alert>
            </v-card-text>

            <v-card-actions class="px-4 pb-4">
                <v-spacer />
                <v-btn color="gray" :disabled="importing" @click="cancel">Cancel</v-btn>
                <v-btn color="primary" :disabled="importing" @click="runImport">
                    {{ validTrades.length ? `Import ${validTrades.length} trade${validTrades.length === 1 ? '' : 's'}` : 'Import' }}
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

import { useAssetsStore } from '@/stores/asset.ts';

import { AssetTradeSide, type AssetTradeImportResponse } from '@/models/asset.ts';

import { ASSET_TRADE_CSV_TEMPLATE, parseAssetTradeRows, parseCsvText, type ParsedAssetTradeRow } from '@/lib/asset.ts';

import { mdiDownloadOutline, mdiFileDelimitedOutline } from '@mdi/js';

const MAX_TRADES_PER_REQUEST = 5000;
const PREVIEW_ROW_COUNT = 6;

const assetsStore = useAssetsStore();

let resolveFunc: ((result: AssetTradeImportResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;
let fallbackCurrency = 'AED';

const showState = ref<boolean>(false);
const importing = ref<boolean>(false);
const errorMessage = ref<string>('');
const file = ref<File | File[] | null>(null);
const parsed = ref<ParsedAssetTradeRow[]>([]);

const validTrades = computed(() => parsed.value.filter(row => row.trade).map(row => row.trade!));
const errorRows = computed(() => parsed.value.filter(row => row.error));
const previewRows = computed(() => parsed.value.filter(row => row.trade).slice(0, PREVIEW_ROW_COUNT));

function formatDate(unixTime: number): string {
    return new Date(unixTime * 1000).toISOString().slice(0, 10);
}

function open(defaultCurrency: string): Promise<AssetTradeImportResponse> {
    fallbackCurrency = defaultCurrency;
    file.value = null;
    parsed.value = [];
    errorMessage.value = '';
    importing.value = false;
    showState.value = true;

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function onFileChange(): void {
    const selected = Array.isArray(file.value) ? file.value[0] : file.value;

    parsed.value = [];
    errorMessage.value = '';

    if (!selected) {
        return;
    }

    selected.text().then(text => {
        parsed.value = parseAssetTradeRows(parseCsvText(text), fallbackCurrency);

        if (!parsed.value.length) {
            errorMessage.value = 'This file has no rows';
        }
    }).catch(() => {
        errorMessage.value = 'Unable to read this file';
    });
}

function downloadTemplate(): void {
    const url = URL.createObjectURL(new Blob([ASSET_TRADE_CSV_TEMPLATE], { type: 'text/csv' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'asset-trades-template.csv';
    link.click();
    URL.revokeObjectURL(url);
}

function runImport(): void {
    if (!validTrades.value.length) {
        errorMessage.value = parsed.value.length ? 'No valid rows to import. Fix the problems listed above.' : 'Choose a CSV file first';
        return;
    }

    if (validTrades.value.length > MAX_TRADES_PER_REQUEST) {
        errorMessage.value = `A file can have at most ${MAX_TRADES_PER_REQUEST} trades. Split it and import in parts.`;
        return;
    }

    errorMessage.value = '';
    importing.value = true;

    assetsStore.importTrades(validTrades.value).then(result => {
        importing.value = false;
        showState.value = false;
        resolveFunc?.(result);
    }).catch((error: { message?: string; error?: { errorMessage?: string } }) => {
        importing.value = false;
        errorMessage.value = error.error?.errorMessage || error.message || 'Unable to import these trades';
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
