<template>
    <f7-page ptr @ptr:refresh="onRefresh" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ disabled: loading }" :back-link="tt('Back')"/>
            <f7-nav-title :title="isIncome ? 'Income Targets' : 'Budgets'"/>
            <f7-nav-right>
                <f7-link icon-f7="gear_alt" href="/settings/budgets"></f7-link>
            </f7-nav-right>
            <f7-subnavbar>
                <f7-segmented strong :class="{ disabled: loading }">
                    <f7-button :text="tt('Expense')" :active="categoryType === CategoryType.Expense" @click="categoryType = CategoryType.Expense"></f7-button>
                    <f7-button :text="tt('Income')" :active="categoryType === CategoryType.Income" @click="categoryType = CategoryType.Income"></f7-button>
                </f7-segmented>
            </f7-subnavbar>
        </f7-navbar>

        <div class="budget-mobile-month-bar" :class="{ disabled: loading }">
            <f7-link icon-f7="chevron_left" @click="changeMonth(-1)"></f7-link>
            <span>{{ getMonthDisplayName(selectedYearMonth) }}</span>
            <f7-link icon-f7="chevron_right" :class="{ disabled: !canShiftToNextMonth }" @click="changeMonth(1)"></f7-link>
        </div>

        <template v-if="!loading && !hasAnyBudget">
            <f7-block class="text-align-center">
                <p>Set a monthly budget for your expense categories, or a target for income categories, to see how close you are to each one.</p>
                <f7-button fill href="/settings/budgets">Set budgets</f7-button>
            </f7-block>
        </template>

        <template v-else>
            <f7-block class="margin-vertical-half">
                <f7-segmented>
                    <f7-button text="Categories" :active="activeView === 'list'" @click="activeView = 'list'"></f7-button>
                    <f7-button text="History" :active="activeView === 'history'" @click="activeView = 'history'"></f7-button>
                </f7-segmented>
            </f7-block>

            <template v-if="activeView === 'list'">
                <f7-card class="budget-mobile-hero" v-if="report.total">
                    <f7-card-content>
                        <div class="budget-mobile-hero-label">{{ isIncome ? 'Still to come' : (report.month.state === 'past' ? 'Left unspent' : 'Left to spend') }}</div>
                        <div class="budget-mobile-hero-amount" :class="{ 'text-color-red': !isIncome && report.total.remaining < 0 }">{{ formatBudgetAmount(report.total.remaining) }}</div>
                        <div class="budget-mobile-hero-sub">
                            <span v-if="!isIncome && report.month.state === 'current' && report.total.remainingPerDay !== null">
                                {{ formatBudgetAmount(report.total.remainingPerDay) }} per day for {{ report.month.daysLeft }} days
                            </span>
                            <span v-else>{{ formatBudgetAmount(report.total.actual) }} of {{ formatBudgetAmount(report.total.budget) }}</span>
                        </div>
                        <category-budget-bar class="margin-top" :progress="report.total" :height="8"/>
                        <div class="budget-mobile-hero-sub margin-top-half">
                            {{ attentionCount ? `${attentionCount} of ${report.rows.length} categories need attention` : 'All categories on track' }}
                        </div>
                    </f7-card-content>
                </f7-card>

                <f7-list strong inset dividers class="budget-mobile-list" :class="{ 'skeleton-text': loading }" v-if="loading && !report.rows.length">
                    <f7-list-item title="Category" after="0.00 USD" :key="idx" v-for="idx in 5"></f7-list-item>
                </f7-list>
                <f7-block class="text-align-center" v-else-if="!report.rows.length">
                    No {{ isIncome ? 'income targets' : 'budgets' }} apply to this month.
                </f7-block>
                <f7-list strong inset dividers class="budget-mobile-list" v-else>
                    <f7-list-item :key="row.category.id" :title="row.category.name"
                                  :link="getTransactionListUrl(row.category.id, selectedYearMonth, isIncome)"
                                  v-for="row in report.rows">
                        <template #media>
                            <ItemIcon :icon-type="getCategoryIconType(row.category.iconType)" :icon-id="row.category.icon" :color="row.category.color" />
                        </template>
                        <template #after>
                            <span :class="{ 'text-color-red': !isIncome && row.progress.remaining < 0 }">{{ formatBudgetAmount(row.progress.remaining) }}</span>
                        </template>
                        <template #footer>
                            <category-budget-bar class="margin-top-half margin-bottom-half" :progress="row.progress" :height="6"
                                                 :show-projection="report.month.state === 'current'"/>
                            <span :class="`budget-mobile-status-${row.progress.level}`">{{ getStatusName(row.progress.status) }}</span>
                            <span v-if="getRowSubtitle(row)"> · {{ getRowSubtitle(row) }}</span>
                        </template>
                    </f7-list-item>
                </f7-list>

                <template v-if="report.unbudgetedRows.length">
                    <f7-block-title>Not budgeted</f7-block-title>
                    <f7-list strong inset dividers>
                        <f7-list-item :key="row.category.id" :title="row.category.name" :after="formatBudgetAmount(row.actual)"
                                      :link="getTransactionListUrl(row.category.id, selectedYearMonth, isIncome)"
                                      v-for="row in report.unbudgetedRows">
                            <template #media>
                                <ItemIcon :icon-type="getCategoryIconType(row.category.iconType)" :icon-id="row.category.icon" :color="row.category.color" />
                            </template>
                        </f7-list-item>
                    </f7-list>
                </template>
            </template>

            <template v-else>
                <f7-block class="text-align-center" v-if="!history.length">
                    No {{ isIncome ? 'income targets' : 'budgets' }} applied in these months.
                </f7-block>
                <f7-card v-else>
                    <f7-card-content class="budget-mobile-heatmap-wrapper">
                        <table class="budget-mobile-heatmap">
                            <thead>
                                <tr>
                                    <th></th>
                                    <th :key="yearMonth" v-for="yearMonth in historyYearMonths">{{ getMonthShortName(yearMonth) }}</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr :key="historyRow.category.id" v-for="historyRow in history">
                                    <td class="budget-mobile-heatmap-name">{{ historyRow.category.name }}</td>
                                    <td :key="cell.yearMonth" v-for="cell in historyRow.cells">
                                        <div class="budget-mobile-heat-cell" :class="[`budget-mobile-heat-${cell.heatLevel}`, { 'budget-mobile-heat-current': cell.state === 'current' }]">
                                            {{ cell.ratio === null ? '-' : formatBudgetPercent(cell.ratio) }}
                                        </div>
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </f7-card-content>
                </f7-card>
                <f7-block class="budget-mobile-hero-sub">
                    Percent of each month's {{ isIncome ? 'target received' : 'budget used' }}. Green is good, red is {{ isIncome ? 'short of target' : 'over budget' }}.
                </f7-block>
            </template>
        </template>
    </f7-page>
</template>

<script setup lang="ts">
import CategoryBudgetBar from '@/components/common/CategoryBudgetBar.vue';

import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import { useCategoryBudgetDisplay, useCategoryBudgetReportBase } from '@/views/base/statistics/CategoryBudgetReportBase.ts';

import { CategoryType } from '@/core/category.ts';
import { CategoryBudgetPaceType } from '@/models/transaction_category_budget.ts';

import { type CategoryBudgetReportRow, CategoryBudgetStatus } from '@/lib/category_budget.ts';
import { getCategoryIconType } from '@/lib/icon.ts';

const props = defineProps<{
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { routeBackOnError, showToast } = useI18nUIComponents();
const {
    formatBudgetAmount,
    formatBudgetPercent,
    getStatusName,
    getStatusNote,
    getMonthDisplayName,
    getMonthShortName,
    getTransactionListUrl
} = useCategoryBudgetDisplay();
const {
    selectedYearMonth,
    categoryType,
    loading,
    isIncome,
    canShiftToNextMonth,
    hasAnyBudget,
    report,
    history,
    historyYearMonths,
    reload,
    shiftMonth
} = useCategoryBudgetReportBase();

const activeView = ref<'list' | 'history'>('list');
const loadingError = ref<unknown | null>(null);

const attentionCount = computed<number>(() => report.value.rows.filter(row => row.progress.level === 'bad' || row.progress.level === 'warning').length);

function getRowSubtitle(row: CategoryBudgetReportRow): string {
    const progress = row.progress;

    if (!isIncome.value && report.value.month.state === 'current' && progress.paceType === CategoryBudgetPaceType.Even
        && progress.remainingPerDay !== null && progress.status !== CategoryBudgetStatus.Over) {
        return `${formatBudgetAmount(progress.remainingPerDay)}/day left`;
    }

    if (progress.status === CategoryBudgetStatus.Over || progress.status === CategoryBudgetStatus.Reached) {
        return '';
    }

    return getStatusNote(progress, report.value.month.state === 'past');
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function handleError(error: any): void {
    if (!error?.processed) {
        showToast(error?.message || error);
    }
}

function changeMonth(months: number): void {
    shiftMonth(months).catch(handleError);
}

function onRefresh(done: () => void): void {
    reload(true).then(() => done()).catch(error => {
        done();
        handleError(error);
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

reload(false).catch(error => {
    loadingError.value = error;
    handleError(error);
});
</script>

<style>
.budget-mobile-month-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 16px 0;
    font-weight: 600;
}

.budget-mobile-hero-label {
    font-size: 13px;
    opacity: 0.7;
}

.budget-mobile-hero-amount {
    font-size: 28px;
    font-weight: 700;
    margin-top: 2px;
}

.budget-mobile-hero-sub {
    font-size: 13px;
    opacity: 0.7;
}

.budget-mobile-list .item-footer {
    width: 100%;
}

.budget-mobile-status-good {
    color: #34c759;
}

.budget-mobile-status-warning {
    color: #ff9500;
}

.budget-mobile-status-bad {
    color: #ff3b30;
}

.budget-mobile-heatmap-wrapper {
    overflow-x: auto;
}

.budget-mobile-heatmap {
    border-collapse: separate;
    border-spacing: 3px;
    font-size: 12px;
    font-variant-numeric: tabular-nums;
}

.budget-mobile-heatmap th {
    font-weight: 500;
    opacity: 0.7;
}

.budget-mobile-heatmap-name {
    max-width: 110px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.budget-mobile-heat-cell {
    min-width: 44px;
    padding: 6px 2px;
    text-align: center;
    border-radius: 4px;
}

.budget-mobile-heat-current {
    outline: 1.5px dashed #c67e48;
    outline-offset: -1.5px;
}

.budget-mobile-heat-0 {
    background-color: rgba(127, 127, 127, 0.08);
}

.budget-mobile-heat-1 {
    background-color: rgba(52, 199, 89, 0.32);
}

.budget-mobile-heat-2 {
    background-color: rgba(52, 199, 89, 0.15);
}

.budget-mobile-heat-3 {
    background-color: rgba(255, 149, 0, 0.22);
}

.budget-mobile-heat-4 {
    background-color: rgba(255, 59, 48, 0.22);
}

.budget-mobile-heat-5 {
    background-color: rgba(255, 59, 48, 0.42);
}
</style>
