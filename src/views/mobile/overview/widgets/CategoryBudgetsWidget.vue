<template>
    <f7-list strong inset dividers class="overview-widget-list no-margin-top margin-bottom" :class="{ 'skeleton-text': loading }">
        <f7-list-item group-title>
            <small>{{ title || (isIncome ? 'Income Targets' : 'Category Budgets') }}</small>
        </f7-list-item>
        <f7-list-item link="/statistic/budget" v-if="report.total">
            <div class="category-budget-mobile-widget-total">
                <div class="display-flex justify-content-space-between align-items-baseline">
                    <span>{{ isIncome ? 'Still to come' : 'Left to spend' }}</span>
                    <span class="category-budget-mobile-widget-amount">{{ displayAmount(Math.max(report.total.remaining, 0)) }}</span>
                </div>
                <category-budget-bar class="margin-top-half" :progress="report.total" :height="8"/>
                <div class="category-budget-mobile-widget-note">
                    {{ attentionCount ? `${attentionCount} of ${report.rows.length} categories need attention` : 'All categories on track' }}
                </div>
            </div>
        </f7-list-item>
        <f7-list-item :key="row.category.id" :title="row.category.name"
                      :after="displayAmount(row.progress.remaining)"
                      :link="getTransactionListUrl(row.category.id, report.month.yearMonth, isIncome)"
                      v-for="row in displayRows">
            <template #media>
                <ItemIcon :icon-type="getCategoryIconType(row.category.iconType)" :icon-id="row.category.icon" :color="row.category.color" />
            </template>
            <template #footer>
                <category-budget-bar class="margin-top-half margin-bottom-half" :progress="row.progress" :height="6" :show-projection="false"/>
                <span :class="`category-budget-mobile-status-${row.progress.level}`">{{ getStatusName(row.progress.status) }}</span>
                <span v-if="showAmount"> · {{ getStatusNote(row.progress, false) }}</span>
            </template>
        </f7-list-item>
        <template v-if="loading && !report.rows.length">
            <f7-list-item title="Category" after="0.00 USD" :key="idx" v-for="idx in itemCount">
                <template #media>
                    <f7-icon f7="app_fill"></f7-icon>
                </template>
            </f7-list-item>
        </template>
        <f7-list-item :title="isIncome ? 'No income targets set' : 'No budgets set'" link="/settings/budgets"
                      v-else-if="!loading && !report.rows.length"></f7-list-item>
    </f7-list>
</template>

<script setup lang="ts">
import CategoryBudgetBar from '@/components/common/CategoryBudgetBar.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useCategoryBudgetDisplay, useCategoryBudgetWidgetBase } from '@/views/base/statistics/CategoryBudgetReportBase.ts';

import type { CategoryType } from '@/core/category.ts';
import { DISPLAY_HIDDEN_AMOUNT } from '@/consts/numeral.ts';

import { getCategoryIconType } from '@/lib/icon.ts';

const props = defineProps<{
    loading: boolean;
    title?: string;
    categoryType: CategoryType;
    itemCount: number;
}>();

const { formatAmountToLocalizedNumeralsWithCurrency } = useI18n();
const { defaultCurrency, formatBudgetAmount, getStatusName, getStatusNote, getTransactionListUrl } = useCategoryBudgetDisplay();
const { isIncome, showAmount, report, attentionCount, displayRows } = useCategoryBudgetWidgetBase(props);

function displayAmount(amount: number): string {
    if (!showAmount.value) {
        return formatAmountToLocalizedNumeralsWithCurrency(DISPLAY_HIDDEN_AMOUNT, defaultCurrency.value);
    }

    return formatBudgetAmount(amount);
}
</script>

<style>
.category-budget-mobile-widget-total {
    width: 100%;
    padding: 4px 0;
}

.category-budget-mobile-widget-amount {
    font-size: 1.4em;
    font-weight: 600;
}

.category-budget-mobile-widget-note {
    margin-top: 6px;
    font-size: 12px;
    opacity: 0.7;
}

.category-budget-mobile-status-good {
    color: #34c759;
}

.category-budget-mobile-status-warning {
    color: #ff9500;
}

.category-budget-mobile-status-bad {
    color: #ff3b30;
}
</style>
