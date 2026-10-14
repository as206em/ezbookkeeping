<template>
    <main-page-layout>
        <template #nav-items>
            <div class="mb-2">
                <btn-vertical-group :disabled="loading" :buttons="[
                    { name: tt('Expense'), value: CategoryType.Expense },
                    { name: tt('Income'), value: CategoryType.Income }
                ]" v-model="categoryType" />
            </div>
            <v-divider class="my-2" />
            <li class="nav-link" :key="view.value" v-for="view in allViews">
                <a class="d-flex align-center cursor-pointer my-1"
                   :class="{ 'router-link-active router-link-exact-active': activeView === view.value, 'disabled': loading }"
                   @click="activeView = view.value">
                    <v-icon class="nav-item-icon" :icon="view.icon"/>
                    <span class="nav-item-title text-truncate">{{ view.name }}</span>
                </a>
            </li>
            <v-divider class="my-2" />
            <li class="nav-link">
                <router-link to="/user/settings/budgets">
                    <v-icon class="nav-item-icon" :icon="mdiCogOutline"/>
                    <span class="nav-item-title">Edit budgets</span>
                </router-link>
            </li>
        </template>

        <template #content>
            <v-card min-height="600">
                <template #title>
                    <div class="title-and-toolbar d-flex align-center flex-wrap ga-2">
                        <span>{{ isIncome ? 'Income Targets' : 'Budgets' }}</span>
                        <v-btn-group class="ms-2" color="default" density="comfortable" variant="outlined" divided>
                            <v-btn class="button-icon-with-direction" :icon="mdiArrowLeft"
                                   :disabled="loading" @click="changeMonth(-1)"/>
                            <v-btn :disabled="loading">{{ getMonthDisplayName(selectedYearMonth) }}</v-btn>
                            <v-btn class="button-icon-with-direction" :icon="mdiArrowRight"
                                   :disabled="loading || !canShiftToNextMonth" @click="changeMonth(1)"/>
                        </v-btn-group>
                        <v-btn density="compact" color="default" variant="text"
                               class="ms-1" :icon="true" :loading="loading" @click="reloadReport(true)">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer/>
                        <v-btn-toggle density="compact" variant="outlined" mandatory divided
                                      :disabled="loading" v-model="historyMonthCount"
                                      v-if="activeView === 'history'">
                            <v-btn :key="months" :value="months" v-for="months in CATEGORY_BUDGET_HISTORY_MONTHS_OPTIONS">{{ months }} months</v-btn>
                        </v-btn-toggle>
                    </div>
                </template>

                <v-card-text v-if="!loading && !hasAnyBudget">
                    <div class="d-flex flex-column align-center text-center py-12 ga-3">
                        <v-icon :icon="mdiPiggyBankOutline" size="48" class="text-medium-emphasis"/>
                        <div class="text-title-large">No budgets yet</div>
                        <div class="text-medium-emphasis budget-empty-text">
                            Set a monthly budget for your primary expense categories, or a target for income categories,
                            to see how close you are to each one.
                        </div>
                        <v-btn color="primary" to="/user/settings/budgets">Set budgets</v-btn>
                    </div>
                </v-card-text>

                <v-card-text v-else>
                    <div class="budget-summary" v-if="report.total && activeView !== 'history'">
                        <div :key="item.title" v-for="item in summaryItems">
                            <div class="text-body-small text-medium-emphasis">{{ item.title }}</div>
                            <div class="text-title-large font-weight-medium" :class="item.className">{{ item.value }}</div>
                        </div>
                        <div class="budget-summary-bar">
                            <category-budget-bar :progress="report.total" :height="10"/>
                            <div class="text-body-small text-medium-emphasis mt-2">{{ monthProgressText }}</div>
                        </div>
                    </div>
                    <v-alert type="warning" variant="tonal" density="compact" class="mb-4" v-if="incomplete">
                        Some amounts could not be converted to {{ defaultCurrency }} and are left out.
                    </v-alert>

                    <div v-if="loading && !report.rows.length">
                        <v-skeleton-loader type="text" class="my-4" :key="idx" v-for="idx in 6"/>
                    </div>

                    <div class="text-center text-medium-emphasis py-8" v-else-if="!report.rows.length && activeView !== 'history'">
                        No {{ isIncome ? 'income targets' : 'budgets' }} apply to {{ getMonthDisplayName(selectedYearMonth) }}.
                        <router-link to="/user/settings/budgets">Set them in Budgets settings.</router-link>
                    </div>

                    <div class="budget-rows" v-else-if="activeView === 'bars'">
                        <div class="budget-row" :key="row.category.id" v-for="row in report.rows"
                             role="button" tabindex="0"
                             @click="openDetail(row)" @keydown.enter="openDetail(row)" @keydown.space.prevent="openDetail(row)">
                            <div class="budget-row-name">
                                <ItemIcon size="24px" :icon-type="getCategoryIconType(row.category.iconType)"
                                          :icon-id="row.category.icon" :color="row.category.color" />
                                <span class="ms-3 text-truncate">{{ row.category.name }}</span>
                                <v-chip class="ms-2" size="x-small" variant="outlined" v-if="row.progress.paceType === CategoryBudgetPaceType.Fixed">Fixed</v-chip>
                            </div>
                            <div class="budget-row-bar">
                                <category-budget-bar :progress="row.progress" :show-projection="report.month.state === 'current'"/>
                            </div>
                            <div class="budget-row-amount">
                                <span class="font-weight-medium">{{ formatBudgetAmount(row.progress.actual) }}</span>
                                <span class="text-medium-emphasis"> of {{ formatBudgetAmount(row.progress.budget) }}</span>
                            </div>
                            <div class="budget-row-status">
                                <v-chip size="small" variant="tonal" :color="getStatusColor(row.progress.level)">{{ getStatusName(row.progress.status) }}</v-chip>
                                <span class="text-body-small text-medium-emphasis">{{ getStatusNote(row.progress, report.month.state === 'past') }}</span>
                            </div>
                        </div>
                        <div class="budget-legend text-body-small text-medium-emphasis">
                            <span><i class="budget-legend-swatch budget-legend-fill"></i>{{ isIncome ? 'Received' : 'Spent' }} so far</span>
                            <span v-if="report.month.state === 'current'"><i class="budget-legend-swatch budget-legend-projected"></i>Projected month-end</span>
                            <span><i class="budget-legend-tick"></i>{{ isIncome ? 'Target' : 'Budget' }}</span>
                            <span v-if="report.month.state === 'current'"><i class="budget-legend-tick budget-legend-pace"></i>Even pace for today</span>
                            <span>Click a category for details</span>
                        </div>
                    </div>

                    <div class="budget-table-wrapper" v-else-if="activeView === 'table'">
                        <v-table density="comfortable" class="budget-table">
                            <thead>
                                <tr>
                                    <th>{{ tt('Primary Category') }}</th>
                                    <th class="text-end">{{ isIncome ? 'Target' : 'Budget' }}</th>
                                    <th class="text-end">{{ isIncome ? 'Received' : 'Spent' }}</th>
                                    <th class="text-end">Used</th>
                                    <th class="text-end" v-if="report.month.state === 'current'">Pace today</th>
                                    <th class="text-end" v-if="report.month.state === 'current'">vs. pace</th>
                                    <th class="text-end" v-if="report.month.state === 'current'">Projected</th>
                                    <th class="text-end">{{ isIncome ? 'Still to come' : 'Left' }}</th>
                                    <th class="text-end" v-if="!isIncome && report.month.state === 'current'">Left / day</th>
                                    <th>Status</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr class="cursor-pointer" :key="row.category.id" v-for="row in report.rows" @click="openDetail(row)">
                                    <td>
                                        <div class="d-flex align-center">
                                            <ItemIcon size="20px" :icon-type="getCategoryIconType(row.category.iconType)"
                                                      :icon-id="row.category.icon" :color="row.category.color" />
                                            <span class="ms-2">{{ row.category.name }}</span>
                                        </div>
                                    </td>
                                    <td class="text-end">{{ formatBudgetAmount(row.progress.budget) }}</td>
                                    <td class="text-end">{{ formatBudgetAmount(row.progress.actual) }}</td>
                                    <td class="text-end">{{ formatBudgetPercent(row.progress.usedRatio) }}</td>
                                    <td class="text-end" v-if="report.month.state === 'current'">
                                        <span class="text-medium-emphasis" v-if="row.progress.expected === null">Fixed</span>
                                        <span v-else>{{ formatBudgetAmount(row.progress.expected) }}</span>
                                    </td>
                                    <td class="text-end" :class="getPaceDifferenceClass(row.progress.paceDifference)" v-if="report.month.state === 'current'">
                                        {{ row.progress.paceDifference === null ? '-' : formatBudgetAmount(row.progress.paceDifference, true) }}
                                    </td>
                                    <td class="text-end" v-if="report.month.state === 'current'">{{ formatBudgetAmount(row.progress.projected) }}</td>
                                    <td class="text-end" :class="{ 'text-error': !isIncome && row.progress.remaining < 0 }">{{ formatBudgetAmount(row.progress.remaining) }}</td>
                                    <td class="text-end" v-if="!isIncome && report.month.state === 'current'">
                                        {{ row.progress.remainingPerDay === null ? '-' : formatBudgetAmount(row.progress.remainingPerDay) }}
                                    </td>
                                    <td><v-chip size="small" variant="tonal" :color="getStatusColor(row.progress.level)">{{ getStatusName(row.progress.status) }}</v-chip></td>
                                </tr>
                            </tbody>
                            <tfoot v-if="report.total">
                                <tr class="font-weight-bold">
                                    <td>Total</td>
                                    <td class="text-end">{{ formatBudgetAmount(report.total.budget) }}</td>
                                    <td class="text-end">{{ formatBudgetAmount(report.total.actual) }}</td>
                                    <td class="text-end">{{ formatBudgetPercent(report.total.usedRatio) }}</td>
                                    <td class="text-end" v-if="report.month.state === 'current'">{{ formatBudgetAmount(report.total.expected ?? 0) }}</td>
                                    <td class="text-end" :class="getPaceDifferenceClass(report.total.paceDifference)" v-if="report.month.state === 'current'">
                                        {{ report.total.paceDifference === null ? '-' : formatBudgetAmount(report.total.paceDifference, true) }}
                                    </td>
                                    <td class="text-end" v-if="report.month.state === 'current'">{{ formatBudgetAmount(report.total.projected) }}</td>
                                    <td class="text-end">{{ formatBudgetAmount(report.total.remaining) }}</td>
                                    <td class="text-end" v-if="!isIncome && report.month.state === 'current'">
                                        {{ report.total.remainingPerDay === null ? '-' : formatBudgetAmount(report.total.remainingPerDay) }}
                                    </td>
                                    <td></td>
                                </tr>
                            </tfoot>
                        </v-table>
                    </div>

                    <template v-else-if="activeView === 'history'">
                        <div class="text-center text-medium-emphasis py-8" v-if="!history.length">
                            No {{ isIncome ? 'income targets' : 'budgets' }} applied in these months.
                        </div>
                        <div class="budget-table-wrapper" v-else>
                            <table class="budget-heatmap">
                                <thead>
                                    <tr>
                                        <th class="text-start">{{ tt('Primary Category') }}</th>
                                        <th :key="yearMonth" v-for="yearMonth in historyYearMonths">{{ getMonthShortName(yearMonth) }}</th>
                                        <th>{{ isIncome ? 'Missed' : 'Over' }}</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr :key="historyRow.category.id" v-for="historyRow in history">
                                        <td class="text-start text-no-wrap">
                                            <div class="d-flex align-center">
                                                <ItemIcon size="20px" :icon-type="getCategoryIconType(historyRow.category.iconType)"
                                                          :icon-id="historyRow.category.icon" :color="historyRow.category.color" />
                                                <span class="ms-2">{{ historyRow.category.name }}</span>
                                            </div>
                                        </td>
                                        <td :key="cell.yearMonth" v-for="cell in historyRow.cells">
                                            <div class="budget-heat-cell" :class="[`budget-heat-${cell.heatLevel}`, { 'budget-heat-current': cell.state === 'current' }]">
                                                <template v-if="cell.ratio !== null">
                                                    <div>{{ formatBudgetPercent(cell.ratio) }}</div>
                                                    <div class="text-caption text-medium-emphasis" v-if="cell.projectedRatio !== null">→ {{ formatBudgetPercent(cell.projectedRatio) }}</div>
                                                </template>
                                                <span class="text-medium-emphasis" v-else>-</span>
                                                <v-tooltip activator="parent" v-if="cell.ratio !== null">
                                                    {{ getMonthDisplayName(cell.yearMonth) }}: {{ formatBudgetAmount(cell.actual) }} of {{ formatBudgetAmount(cell.budget) }}
                                                </v-tooltip>
                                            </div>
                                        </td>
                                        <td class="text-medium-emphasis text-no-wrap">{{ historyRow.missedCount }} of {{ historyRow.budgetedCount }}</td>
                                    </tr>
                                </tbody>
                            </table>
                            <div class="budget-legend text-body-small text-medium-emphasis">
                                <span :key="item.level" v-for="item in heatLegend"><i class="budget-legend-swatch" :class="`budget-heat-${item.level}`"></i>{{ item.text }}</span>
                                <span>This month shows used so far → projected</span>
                            </div>
                        </div>
                    </template>

                    <template v-if="report.unbudgetedRows.length && activeView !== 'history'">
                        <div class="text-overline text-medium-emphasis mt-6">Not budgeted</div>
                        <div class="budget-unbudgeted-row" :key="row.category.id" v-for="row in report.unbudgetedRows">
                            <ItemIcon size="20px" :icon-type="getCategoryIconType(row.category.iconType)"
                                      :icon-id="row.category.icon" :color="row.category.color" />
                            <router-link class="ms-2 link-no-color" :to="getTransactionListUrl(row.category.id, selectedYearMonth, isIncome)">{{ row.category.name }}</router-link>
                            <v-spacer/>
                            <span>{{ formatBudgetAmount(row.actual) }}</span>
                        </div>
                    </template>
                </v-card-text>
            </v-card>
        </template>
    </main-page-layout>

    <category-budget-detail-dialog v-model:show="showDetail" :row="detailRow" :month="report.month" :is-income="isIncome"
                                   @error="onError"/>
    <snack-bar ref="snackbar"/>
</template>

<script setup lang="ts">
import SnackBar from '@/components/desktop/SnackBar.vue';
import CategoryBudgetBar from '@/components/common/CategoryBudgetBar.vue';
import CategoryBudgetDetailDialog from '@/views/desktop/statistics/budget/CategoryBudgetDetailDialog.vue';

import { ref, computed, watch, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import {
    CATEGORY_BUDGET_HISTORY_MONTHS_OPTIONS,
    useCategoryBudgetDisplay,
    useCategoryBudgetReportBase
} from '@/views/base/statistics/CategoryBudgetReportBase.ts';

import { CategoryType } from '@/core/category.ts';
import { CategoryBudgetPaceType } from '@/models/transaction_category_budget.ts';

import type { CategoryBudgetReportRow } from '@/lib/category_budget.ts';
import { getCategoryIconType } from '@/lib/icon.ts';

import {
    mdiArrowLeft,
    mdiArrowRight,
    mdiRefresh,
    mdiCogOutline,
    mdiPiggyBankOutline,
    mdiChartTimelineVariant,
    mdiTableLarge,
    mdiViewGridOutline
} from '@mdi/js';

type SnackBarType = InstanceType<typeof SnackBar>;
type BudgetView = 'bars' | 'table' | 'history';

const { tt } = useI18n();
const {
    defaultCurrency,
    formatBudgetAmount,
    formatBudgetPercent,
    getStatusName,
    getStatusNote,
    getStatusColor,
    getMonthDisplayName,
    getMonthShortName,
    getTransactionListUrl
} = useCategoryBudgetDisplay();
const {
    selectedYearMonth,
    categoryType,
    historyMonthCount,
    loading,
    isIncome,
    canShiftToNextMonth,
    hasAnyBudget,
    report,
    history,
    historyYearMonths,
    incomplete,
    reload,
    shiftMonth
} = useCategoryBudgetReportBase();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const activeView = ref<BudgetView>('bars');
const showDetail = ref<boolean>(false);
const detailRow = ref<CategoryBudgetReportRow | null>(null);

const allViews: { name: string; value: BudgetView; icon: string }[] = [
    { name: 'Budget vs. actual', value: 'bars', icon: mdiChartTimelineVariant },
    { name: 'Table', value: 'table', icon: mdiTableLarge },
    { name: 'History', value: 'history', icon: mdiViewGridOutline }
];

const heatLegend = computed<{ level: number; text: string }[]>(() => isIncome.value ? [
    { level: 1, text: 'Reached' },
    { level: 3, text: '90-100%' },
    { level: 4, text: '70-90%' },
    { level: 5, text: 'Under 70%' }
] : [
    { level: 1, text: 'Under 70%' },
    { level: 2, text: '70-90%' },
    { level: 3, text: '90-100%' },
    { level: 4, text: '100-115%' },
    { level: 5, text: 'Over 115%' }
]);

const summaryItems = computed<{ title: string; value: string; className?: string }[]>(() => {
    const total = report.value.total;

    if (!total) {
        return [];
    }

    const items: { title: string; value: string; className?: string }[] = [
        { title: isIncome.value ? 'Total target' : 'Total budget', value: formatBudgetAmount(total.budget) },
        { title: isIncome.value ? 'Received' : 'Spent', value: `${formatBudgetAmount(total.actual)} (${formatBudgetPercent(total.usedRatio)})` },
        { title: isIncome.value ? 'Still to come' : 'Left', value: formatBudgetAmount(total.remaining), className: !isIncome.value && total.remaining < 0 ? 'text-error' : undefined }
    ];

    if (report.value.month.state === 'current') {
        items.push({ title: 'Projected', value: formatBudgetAmount(total.projected) });

        if (total.paceDifference !== null) {
            items.push({ title: 'vs. pace', value: formatBudgetAmount(total.paceDifference, true), className: getPaceDifferenceClass(total.paceDifference) });
        }
    }

    return items;
});

const monthProgressText = computed<string>(() => {
    const month = report.value.month;

    if (month.state === 'past') {
        return 'Month complete';
    }

    return `Day ${month.elapsedDays} of ${month.totalDays} · ${formatBudgetPercent(month.elapsedDays / month.totalDays)} of the month elapsed · ${month.daysLeft} days left`;
});

function getPaceDifferenceClass(paceDifference: number | null): string {
    if (paceDifference === null || Math.round(paceDifference) === 0) {
        return '';
    }

    const isBad = isIncome.value ? paceDifference < 0 : paceDifference > 0;
    return isBad ? 'text-error' : 'text-success';
}

function openDetail(row: CategoryBudgetReportRow): void {
    detailRow.value = row;
    showDetail.value = true;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function onError(error: any): void {
    if (!error?.processed) {
        snackbar.value?.showError(error);
    }
}

function reloadReport(force: boolean): void {
    reload(force).then(() => {
        if (force) {
            snackbar.value?.showMessage('Data has been updated');
        }
    }).catch(onError);
}

function changeMonth(months: number): void {
    shiftMonth(months).catch(onError);
}

watch(report, () => {
    if (detailRow.value) {
        detailRow.value = report.value.rows.find(row => row.category.id === detailRow.value?.category.id) ?? detailRow.value;
    }
});

reloadReport(false);
</script>

<style>
.budget-empty-text {
    max-width: 52ch;
}

.budget-summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 12px 24px;
    padding-bottom: 16px;
    margin-bottom: 8px;
    border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}

.budget-summary .budget-summary-bar {
    grid-column: 1 / -1;
}

.budget-rows {
    container-type: inline-size;
}

.budget-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 2fr) 210px 240px;
    grid-template-areas: "name bar amount status";
    align-items: center;
    gap: 8px 20px;
    padding: 12px 8px;
    border-radius: 8px;
    border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    cursor: pointer;
}

.budget-row:hover {
    background-color: rgba(var(--v-theme-on-surface), 0.04);
}

.budget-row:focus-visible {
    outline: 2px solid rgb(var(--v-theme-primary));
    outline-offset: 1px;
}

.budget-row-name {
    grid-area: name;
    display: flex;
    align-items: center;
    min-width: 0;
}

.budget-row-bar {
    grid-area: bar;
}

.budget-row-amount {
    grid-area: amount;
    white-space: nowrap;
    text-align: end;
    font-variant-numeric: tabular-nums;
}

.budget-row-status {
    grid-area: status;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    min-width: 0;
}

@container (max-width: 760px) {
    .budget-row {
        grid-template-columns: minmax(0, 1fr) auto;
        grid-template-areas: "name amount" "bar bar" "status status";
    }

    .budget-row .budget-row-status {
        flex-direction: row;
        align-items: center;
        gap: 8px;
    }
}

.budget-legend {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 18px;
    margin-top: 14px;
}

.budget-legend-swatch,
.budget-legend-tick {
    display: inline-block;
    vertical-align: middle;
    margin-inline-end: 6px;
}

.budget-legend-swatch {
    width: 16px;
    height: 8px;
    border-radius: 4px;
}

.budget-legend-fill {
    background-color: #c67e48;
}

.budget-legend-projected {
    background-color: rgba(127, 127, 127, 0.3);
}

.budget-legend-tick {
    width: 2px;
    height: 12px;
    background-color: currentColor;
}

.budget-legend-pace {
    opacity: 0.4;
}

.budget-table-wrapper {
    overflow-x: auto;
}

.budget-table {
    min-width: 900px;
}

.budget-table td,
.budget-table th {
    white-space: nowrap;
}

.budget-heatmap {
    width: 100%;
    min-width: 720px;
    border-collapse: separate;
    border-spacing: 3px;
    font-variant-numeric: tabular-nums;
}

.budget-heatmap th {
    font-weight: 500;
    font-size: 0.8rem;
    opacity: 0.7;
    padding: 4px;
    text-align: center;
}

.budget-heatmap th.text-start {
    text-align: start;
}

.budget-heatmap td {
    text-align: center;
    padding: 0;
}

.budget-heat-cell {
    border-radius: 4px;
    padding: 6px 4px;
    font-size: 0.8rem;
    min-height: 34px;
}

.budget-heat-current {
    outline: 1.5px dashed rgb(var(--v-theme-primary));
    outline-offset: -1.5px;
}

.budget-heat-0 {
    background-color: rgba(127, 127, 127, 0.08);
}

.budget-heat-1 {
    background-color: rgba(52, 199, 89, 0.32);
}

.budget-heat-2 {
    background-color: rgba(52, 199, 89, 0.15);
}

.budget-heat-3 {
    background-color: rgba(255, 149, 0, 0.22);
}

.budget-heat-4 {
    background-color: rgba(255, 59, 48, 0.22);
}

.budget-heat-5 {
    background-color: rgba(255, 59, 48, 0.42);
}

.budget-unbudgeted-row {
    display: flex;
    align-items: center;
    padding: 8px;
    border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}
</style>
