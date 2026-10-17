import { ref } from 'vue';
import { defineStore } from 'pinia';

import type {
    AssetHoldingInfoResponse,
    AssetPriceRefreshResponse,
    AssetPriceSetRequest,
    AssetTradeImportResponse,
    AssetTradeInfoResponse,
    AssetTradeInput,
    AssetTradeModifyRequest
} from '@/models/asset.ts';

import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

function rejectWith(error: { processed?: boolean; response?: { data?: { errorMessage?: string } } }, fallbackMessage: string, reject: (reason: unknown) => void): void {
    logger.error(fallbackMessage, error);

    if (error.response && error.response.data && error.response.data.errorMessage) {
        reject({ error: error.response.data });
    } else if (!error.processed) {
        reject({ message: fallbackMessage });
    } else {
        reject(error);
    }
}

export const useAssetsStore = defineStore('assets', () => {
    const allHoldings = ref<AssetHoldingInfoResponse[]>([]);
    const allTrades = ref<AssetTradeInfoResponse[]>([]);

    function resetAssets(): void {
        allHoldings.value = [];
        allTrades.value = [];
    }

    function loadAssets(): Promise<void> {
        return new Promise((resolve, reject) => {
            Promise.all([
                services.getAllAssetHoldings(),
                services.getAllAssetTrades()
            ]).then(([holdingsResponse, tradesResponse]) => {
                const holdings = holdingsResponse.data;
                const trades = tradesResponse.data;

                if (!holdings || !holdings.success || !holdings.result || !trades || !trades.success || !trades.result) {
                    reject({ message: 'Unable to retrieve assets' });
                    return;
                }

                allHoldings.value = holdings.result;
                allTrades.value = trades.result;
                resolve();
            }).catch(error => rejectWith(error, 'Unable to retrieve assets', reject));
        });
    }

    function addTrade(req: AssetTradeInput): Promise<AssetTradeInfoResponse> {
        return new Promise((resolve, reject) => {
            services.addAssetTrade(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to add this trade' });
                    return;
                }

                resolve(data.result);
            }).catch(error => rejectWith(error, 'Unable to add this trade', reject));
        });
    }

    function modifyTrade(req: AssetTradeModifyRequest): Promise<AssetTradeInfoResponse> {
        return new Promise((resolve, reject) => {
            services.modifyAssetTrade(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save this trade' });
                    return;
                }

                resolve(data.result);
            }).catch(error => rejectWith(error, 'Unable to save this trade', reject));
        });
    }

    function deleteTrade(id: string): Promise<void> {
        return new Promise((resolve, reject) => {
            services.deleteAssetTrade({ id }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete this trade' });
                    return;
                }

                resolve();
            }).catch(error => rejectWith(error, 'Unable to delete this trade', reject));
        });
    }

    function importTrades(trades: AssetTradeInput[]): Promise<AssetTradeImportResponse> {
        return new Promise((resolve, reject) => {
            services.importAssetTrades({ trades }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to import these trades' });
                    return;
                }

                resolve(data.result);
            }).catch(error => rejectWith(error, 'Unable to import these trades', reject));
        });
    }

    function setPrice(req: AssetPriceSetRequest): Promise<void> {
        return new Promise((resolve, reject) => {
            services.setAssetPrice(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save this price' });
                    return;
                }

                resolve();
            }).catch(error => rejectWith(error, 'Unable to save this price', reject));
        });
    }

    function refreshPrices(): Promise<AssetPriceRefreshResponse> {
        return new Promise((resolve, reject) => {
            services.refreshAssetPrices().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to refresh prices' });
                    return;
                }

                resolve(data.result);
            }).catch(error => rejectWith(error, 'Unable to refresh prices', reject));
        });
    }

    return {
        // states
        allHoldings,
        allTrades,
        // functions
        resetAssets,
        loadAssets,
        addTrade,
        modifyTrade,
        deleteTrade,
        importTrades,
        setPrice,
        refreshPrices
    };
});
