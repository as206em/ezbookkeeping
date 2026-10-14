<template>
    <v-card class="h-100 d-flex flex-column" :class="{ disabled: loading }">
        <template #title>
            <div class="d-flex align-center">
                <span class="text-title-medium">{{ title || (isIncome ? 'Income Targets' : 'Category Budgets') }}</span>
                <v-spacer/>
                <span class="text-body-small text-medium-emphasis">{{ getMonthShortName(report.month.yearMonth) }}</span>
            </div>
        </template>

        <v-card-text class="py-0 flex-grow-1 overflow-y-auto">
            <template v-if="report.total">
                <div class="d-flex align-baseline mt-1">
                    <span class="text-headline-small">{{ displayAmount(report.total.actual) }}</span>
                    <span class="text-body-medium text-medium-emphasis ms-2">of {{ displayAmount(report.total.budget) }}</span>
                </div>
                <category-budget-bar class="mt-3" :progress="report.total" :height="8"/>
                <div class="text-body-small text-medium-emphasis mt-2 mb-1">
                    {{ attentionCount ? `${attentionCount} of ${report.rows.length} categories need attention` : 'All categories on track' }}
                </div>

                <router-link class="category-budget-widget-row link-no-color" :key="row.category.id" v-for="row in displayRows"
                             :to="getTransactionListUrl(row.category.id, report.month.yearMonth, isIncome)">
                    <div class="d-flex align-center">
                        <ItemIcon size="20px" :icon-type="getCategoryIconType(row.category.iconType)"
                                  :icon-id="row.category.icon" :color="row.category.color" />
                        <span class="ms-2 text-truncate">{{ row.category.name }}</span>
                        <v-spacer/>
                        <v-chip size="x-small" variant="tonal" :color="getStatusColor(row.progress.level)">{{ getStatusName(row.progress.status) }}</v-chip>
                    </div>
                    <category-budget-bar class="mt-2" :progress="row.progress" :height="6" :show-projection="false"/>
                    <div class="d-flex text-body-small mt-1">
                        <span class="text-medium-emphasis text-truncate">{{ showAmount ? getStatusNote(row.progress, false) : getStatusName(row.progress.status) }}</span>
                        <v-spacer/>
                        <span class="text-no-wrap ms-2">{{ displayAmount(row.progress.actual) }} / {{ displayAmount(row.progress.budget) }}</span>
                    </div>
                </router-link>
            </template>

            <div class="text-medium-emphasis text-center pt-4" v-else-if="!loading">
                <div>{{ isIncome ? 'No income targets set.' : 'No budgets set.' }}</div>
                <router-link to="/user/settings/budgets">Set budgets</router-link>
            </div>
            <div v-else>
                <v-skeleton-loader class="skeleton-no-margin py-3" type="text" :key="idx" :loading="true" v-for="idx in itemCount"></v-skeleton-loader>
            </div>
        </v-card-text>

        <v-card-actions class="px-4" v-if="report.total">
            <span class="text-body-small text-medium-emphasis" v-if="report.rows.length > displayRows.length">+{{ report.rows.length - displayRows.length }} more</span>
            <v-spacer/>
            <v-btn density="comfortable" variant="text" color="primary" to="/statistics/budget">View all</v-btn>
        </v-card-actions>
    </v-card>
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
const { defaultCurrency, formatBudgetAmount, getStatusName, getStatusNote, getStatusColor, getMonthShortName, getTransactionListUrl } = useCategoryBudgetDisplay();
const { isIncome, showAmount, report, attentionCount, displayRows } = useCategoryBudgetWidgetBase(props);

function displayAmount(amount: number): string {
    if (!showAmount.value) {
        return formatAmountToLocalizedNumeralsWithCurrency(DISPLAY_HIDDEN_AMOUNT, defaultCurrency.value);
    }

    return formatBudgetAmount(amount);
}
</script>

<style>
.category-budget-widget-row {
    display: block;
    padding: 10px 0;
    text-decoration: none;
    border-top: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}
</style>
