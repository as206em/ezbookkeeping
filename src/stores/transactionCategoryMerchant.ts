import { ref } from 'vue';
import { defineStore } from 'pinia';

import type { TransactionCategoryMerchantInfoResponse } from '@/models/transaction_category_merchant.ts';

import { getMerchantKey } from '@/lib/category_merchant.ts';
import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

export const useTransactionCategoryMerchantsStore = defineStore('transactionCategoryMerchants', () => {
    const allMerchants = ref<TransactionCategoryMerchantInfoResponse[]>([]);
    const merchantListStateInvalid = ref<boolean>(true);

    function getMerchantsByCategoryId(categoryId: string): TransactionCategoryMerchantInfoResponse[] {
        return allMerchants.value.filter(merchant => merchant.categoryId === categoryId);
    }

    function getCategoryIdByMerchant(merchant: string): string | undefined {
        const key = getMerchantKey(merchant);
        return allMerchants.value.find(item => getMerchantKey(item.merchant) === key)?.categoryId;
    }

    function resetCategoryMerchants(): void {
        allMerchants.value = [];
        merchantListStateInvalid.value = true;
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    function toRejection(error: any, defaultMessage: string): unknown {
        if (error.response && error.response.data && error.response.data.errorMessage) {
            return { error: error.response.data };
        } else if (!error.processed) {
            return { message: defaultMessage };
        }

        return error;
    }

    function loadAllMerchants({ force }: { force?: boolean }): Promise<TransactionCategoryMerchantInfoResponse[]> {
        if (!force && !merchantListStateInvalid.value) {
            return Promise.resolve(allMerchants.value);
        }

        return services.getAllCategoryMerchants().then(response => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                throw { message: 'Unable to retrieve merchant rules' };
            }

            allMerchants.value = data.result;
            merchantListStateInvalid.value = false;
            return data.result;
        }).catch(error => {
            logger.error('failed to load merchant rules', error);
            throw toRejection(error, 'Unable to retrieve merchant rules');
        });
    }

    function addMerchant({ categoryId, merchant }: { categoryId: string, merchant: string }): Promise<TransactionCategoryMerchantInfoResponse> {
        return services.addCategoryMerchant({ categoryId, merchant }).then(response => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                throw { message: 'Unable to save merchant rule' };
            }

            const key = getMerchantKey(data.result.merchant);
            allMerchants.value = allMerchants.value.filter(item => getMerchantKey(item.merchant) !== key).concat([data.result]);
            return data.result;
        }).catch(error => {
            logger.error('failed to save merchant rule', error);
            throw toRejection(error, 'Unable to save merchant rule');
        });
    }

    function deleteMerchant({ merchant }: { merchant: string }): Promise<boolean> {
        return services.deleteCategoryMerchant({ merchant }).then(response => {
            const data = response.data;

            if (!data || !data.success) {
                throw { message: 'Unable to delete merchant rule' };
            }

            const key = getMerchantKey(merchant);
            allMerchants.value = allMerchants.value.filter(item => getMerchantKey(item.merchant) !== key);
            return true;
        }).catch(error => {
            logger.error('failed to delete merchant rule', error);
            throw toRejection(error, 'Unable to delete merchant rule');
        });
    }

    return {
        // states
        allMerchants,
        // functions
        getMerchantsByCategoryId,
        getCategoryIdByMerchant,
        resetCategoryMerchants,
        loadAllMerchants,
        addMerchant,
        deleteMerchant
    };
});
