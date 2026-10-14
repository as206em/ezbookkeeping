<template>
    <v-dialog width="1000" :persistent="false" v-model="showState">
        <v-card class="pa-2 pa-sm-4 pa-md-6" v-if="row">
            <template #title>
                <div class="d-flex align-center flex-wrap ga-2">
                    <ItemIcon size="28px" :icon-type="getCategoryIconType(row.category.iconType)"
                              :icon-id="row.category.icon" :color="row.category.color" />
                    <span class="ms-1">{{ row.category.name }}</span>
                    <span class="text-medium-emphasis text-body-large ms-1">{{ getMonthDisplayName(month.yearMonth) }}</span>
                    <v-spacer/>
                    <v-chip size="small" variant="tonal" :color="getStatusColor(row.progress.level)">{{ getStatusName(row.progress.status) }}</v-chip>
                </div>
            </template>

            <v-card-text>
                <v-row>
                    <v-col cols="12" md="8">
                        <div class="category-budget-detail-chart-placeholder d-flex align-center justify-center" v-if="loading">
                            <v-progress-circular indeterminate/>
                        </div>
                        <category-budget-progress-chart :daily-amounts="detail.dailyAmounts"
                                                        :budget="row.progress.budget"
                                                        :pace-type="row.progress.paceType"
                                                        :elapsed-days="month.elapsedDays"
                                                        :projected="row.progress.projected"
                                                        :level="row.progress.level"
                                                        :show-projection="month.state === 'current'"
                                                        v-else-if="detail.dailyAmounts.length" />
                    </v-col>
                    <v-col cols="12" md="4">
                        <v-list density="compact" class="py-0">
                            <v-list-item class="px-0" :key="fact.title" v-for="fact in facts">
                                <div class="d-flex align-center">
                                    <span class="text-medium-emphasis">{{ fact.title }}</span>
                                    <v-spacer/>
                                    <span class="font-weight-medium" :class="fact.className">{{ fact.value }}</span>
                                </div>
                            </v-list-item>
                        </v-list>
                        <div class="text-body-medium text-medium-emphasis mt-3">{{ getStatusNote(row.progress, month.state === 'past') }}.</div>
                        <div class="text-body-small text-warning mt-2" v-if="detail.incomplete">Some amounts could not be converted to {{ defaultCurrency }} and are left out.</div>
                    </v-col>
                </v-row>

                <div class="d-flex align-center mt-6 mb-2">
                    <span class="text-title-medium">{{ tt('Transaction List') }}</span>
                    <v-spacer/>
                    <v-btn density="comfortable" variant="text" color="primary"
                           :to="getTransactionListUrl(row.category.id, month.yearMonth, isIncome)">Open in transaction list</v-btn>
                </div>
                <v-table density="compact" class="category-budget-detail-transactions">
                    <thead>
                        <tr>
                            <th>{{ tt('Date') }}</th>
                            <th>{{ tt('Category') }}</th>
                            <th>{{ tt('Description') }}</th>
                            <th>{{ tt('Account') }}</th>
                            <th class="text-end">{{ tt('Amount') }}</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr :key="transaction.id" v-for="transaction in sortedTransactions">
                            <td class="text-no-wrap">{{ formatTransactionDate(transaction) }}</td>
                            <td class="text-no-wrap">{{ getCategoryName(transaction.categoryId) }}</td>
                            <td class="category-budget-detail-comment">{{ transaction.comment }}</td>
                            <td class="text-no-wrap">{{ getAccountName(transaction.sourceAccountId) }}</td>
                            <td class="text-end text-no-wrap">{{ formatTransactionAmount(transaction) }}</td>
                        </tr>
                        <tr v-if="!loading && !sortedTransactions.length">
                            <td colspan="5" class="text-center text-medium-emphasis">{{ tt('No data') }}</td>
                        </tr>
                    </tbody>
                </v-table>
            </v-card-text>

            <v-card-actions>
                <v-spacer/>
                <v-btn color="secondary" variant="tonal" @click="showState = false">{{ tt('Close') }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import CategoryBudgetProgressChart from '@/components/desktop/CategoryBudgetProgressChart.vue';

import { ref, computed, watch } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import {
    type CategoryBudgetDailyDetail,
    useCategoryBudgetDisplay,
    loadCategoryBudgetDailyDetail
} from '@/views/base/statistics/CategoryBudgetReportBase.ts';

import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';

import type { TransactionInfoResponse } from '@/models/transaction.ts';

import type { BudgetMonthProgress, CategoryBudgetReportRow } from '@/lib/category_budget.ts';
import { getCategoryIconType } from '@/lib/icon.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

const props = defineProps<{
    show: boolean;
    row: CategoryBudgetReportRow | null;
    month: BudgetMonthProgress;
    isIncome: boolean;
}>();

const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'error', error: unknown): void;
}>();

const { tt, formatAmountToLocalizedNumeralsWithCurrency, formatDateTimeToShortMonthDay } = useI18n();
const {
    defaultCurrency,
    formatBudgetAmount,
    formatBudgetPercent,
    getStatusName,
    getStatusNote,
    getStatusColor,
    getMonthDisplayName,
    getTransactionListUrl
} = useCategoryBudgetDisplay();

const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();

const loading = ref<boolean>(false);
const detail = ref<CategoryBudgetDailyDetail>({ dailyAmounts: [], transactions: [], incomplete: false });

const showState = computed<boolean>({
    get: () => props.show,
    set: (value) => emit('update:show', value)
});

const sortedTransactions = computed<TransactionInfoResponse[]>(() => [...detail.value.transactions].sort((a, b) => b.time - a.time));

const facts = computed<{ title: string; value: string; className?: string }[]>(() => {
    if (!props.row) {
        return [];
    }

    const progress = props.row.progress;
    const result: { title: string; value: string; className?: string }[] = [
        { title: props.isIncome ? 'Target' : 'Budget', value: formatBudgetAmount(progress.budget) },
        { title: props.isIncome ? 'Received' : 'Spent', value: `${formatBudgetAmount(progress.actual)} (${formatBudgetPercent(progress.usedRatio)})` }
    ];

    if (props.month.state === 'current') {
        result.push({
            title: 'Pace for today',
            value: progress.expected === null ? 'Fixed bill' : formatBudgetAmount(progress.expected)
        });

        if (progress.paceDifference !== null) {
            const isBad = props.isIncome ? progress.paceDifference < 0 : progress.paceDifference > 0;
            result.push({
                title: 'vs. pace',
                value: formatBudgetAmount(progress.paceDifference, true),
                className: isBad ? 'text-error' : 'text-success'
            });
        }

        result.push({ title: 'Projected month-end', value: formatBudgetAmount(progress.projected) });
    }

    result.push({
        title: props.isIncome ? 'Still to come' : 'Left',
        value: formatBudgetAmount(progress.remaining),
        className: !props.isIncome && progress.remaining < 0 ? 'text-error' : undefined
    });

    if (!props.isIncome && props.month.state === 'current' && progress.remainingPerDay !== null) {
        result.push({ title: 'Left per day', value: formatBudgetAmount(progress.remainingPerDay) });
    }

    return result;
});

function getCategoryName(categoryId: string): string {
    return transactionCategoriesStore.allTransactionCategoriesMap[categoryId]?.name ?? '';
}

function getAccountName(accountId: string): string {
    return accountsStore.allAccountsMap[accountId]?.name ?? '';
}

function formatTransactionDate(transaction: TransactionInfoResponse): string {
    return formatDateTimeToShortMonthDay(parseDateTimeFromUnixTime(transaction.time));
}

function formatTransactionAmount(transaction: TransactionInfoResponse): string {
    const account = accountsStore.allAccountsMap[transaction.sourceAccountId];
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(transaction.sourceAmount), account?.currency ?? defaultCurrency.value);
}

function load(): void {
    if (!props.row) {
        return;
    }

    loading.value = true;
    detail.value = { dailyAmounts: [], transactions: [], incomplete: false };

    loadCategoryBudgetDailyDetail(props.row.category.id, props.month.yearMonth, props.isIncome).then(result => {
        detail.value = result;
        loading.value = false;
    }).catch(error => {
        loading.value = false;
        emit('error', error);
    });
}

watch(() => [props.show, props.row?.category.id, props.month.yearMonth], () => {
    if (props.show) {
        load();
    }
});
</script>

<style>
.category-budget-detail-chart-placeholder {
    height: 300px;
}

.category-budget-detail-transactions .category-budget-detail-comment {
    max-width: 320px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
</style>
