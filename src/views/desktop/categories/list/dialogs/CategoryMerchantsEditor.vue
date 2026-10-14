<template>
    <div>
        <div class="text-body-medium mb-1">Always use this category for</div>
        <div class="text-body-small text-medium-emphasis mb-3">
            Bank SMS transactions from these merchants are put in this category without asking the AI. A merchant can belong to only one category.
        </div>
        <div class="d-flex flex-wrap ga-2 mb-3" v-if="merchants.length">
            <v-chip closable size="small" :key="merchant.merchant" :disabled="disabled || updating"
                    v-for="merchant in merchants" @click:close="remove(merchant.merchant)">
                {{ merchant.merchant }}
            </v-chip>
        </div>
        <div class="text-body-small text-medium-emphasis mb-3" v-else-if="!loading">No merchants yet.</div>
        <div class="d-flex align-center ga-2">
            <v-text-field density="compact" hide-details persistent-placeholder
                          placeholder="Merchant name, e.g. Carrefour"
                          :maxlength="CATEGORY_MERCHANT_MAX_LENGTH"
                          :disabled="disabled || updating"
                          v-model="newMerchant"
                          @keydown.enter.prevent="add" />
            <v-btn variant="tonal" :disabled="disabled || updating || !newMerchant.trim()" @click="add">Add</v-btn>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

import { useTransactionCategoryMerchantsStore } from '@/stores/transactionCategoryMerchant.ts';

import type { TransactionCategoryMerchantInfoResponse } from '@/models/transaction_category_merchant.ts';

import { CATEGORY_MERCHANT_MAX_LENGTH, normalizeMerchantName } from '@/lib/category_merchant.ts';

const props = defineProps<{
    categoryId: string;
    disabled?: boolean;
}>();

const emit = defineEmits<{
    (e: 'error', error: unknown): void;
    (e: 'message', message: string): void;
}>();

const merchantsStore = useTransactionCategoryMerchantsStore();

const loading = ref<boolean>(true);
const updating = ref<boolean>(false);
const newMerchant = ref<string>('');

const merchants = computed<TransactionCategoryMerchantInfoResponse[]>(() => merchantsStore.getMerchantsByCategoryId(props.categoryId)
    .slice().sort((a, b) => a.merchant.localeCompare(b.merchant)));

function add(): void {
    const merchant = normalizeMerchantName(newMerchant.value);

    if (!merchant) {
        return;
    }

    const previousCategoryId = merchantsStore.getCategoryIdByMerchant(merchant);
    updating.value = true;

    merchantsStore.addMerchant({ categoryId: props.categoryId, merchant }).then(() => {
        updating.value = false;
        newMerchant.value = '';

        if (previousCategoryId && previousCategoryId !== props.categoryId) {
            emit('message', `${merchant} was moved to this category`);
        }
    }).catch(error => {
        updating.value = false;
        emit('error', error);
    });
}

function remove(merchant: string): void {
    updating.value = true;

    merchantsStore.deleteMerchant({ merchant }).then(() => {
        updating.value = false;
    }).catch(error => {
        updating.value = false;
        emit('error', error);
    });
}

merchantsStore.loadAllMerchants({ force: true }).then(() => {
    loading.value = false;
}).catch(error => {
    loading.value = false;
    emit('error', error);
});
</script>
