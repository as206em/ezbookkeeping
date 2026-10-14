<template>
    <v-row>
        <v-col cols="12">
            <v-card :class="{ disabled: loading || saving }">
                <template #title>
                    <div class="d-flex align-center">
                        <span>Monthly Budgets</span>
                        <v-progress-circular indeterminate size="20" class="ms-3" v-if="loading || saving"/>
                    </div>
                </template>

                <v-card-text>
                    <v-alert type="info" variant="tonal" class="mb-5">
                        Set a monthly budget for each primary expense category and a monthly target for each primary income category.
                        Amounts are in your default currency ({{ defaultCurrency }}). Leave a category empty to stop tracking it.
                        "Fixed bill" suits rent, insurance or salary, which arrive once a month, so they are not judged against an even daily pace.
                    </v-alert>

                    <v-row align="center">
                        <v-col cols="12" md="5">
                            <v-select
                                item-title="title"
                                item-value="value"
                                label="Apply from"
                                :disabled="loading || saving"
                                :items="startMonthOptions"
                                v-model="startYearMonth"
                            />
                        </v-col>
                        <v-col cols="12" md="7" class="text-body-medium text-medium-emphasis">
                            Saving replaces budgets from {{ getMonthDisplayName(startYearMonth) }} onward.
                            Earlier months keep the budgets they had, so past reports stay accurate.
                        </v-col>
                    </v-row>
                </v-card-text>
            </v-card>
        </v-col>

        <v-col cols="12" :key="section.type" v-for="section in sections">
            <v-card :class="{ disabled: loading || saving }">
                <template #title>
                    <div class="d-flex align-center flex-wrap">
                        <span>{{ section.title }}</span>
                        <v-spacer/>
                        <span class="text-body-medium text-medium-emphasis">Total {{ formatBudgetAmount(getSectionTotal(section.type)) }}</span>
                    </div>
                </template>

                <v-table class="budget-setting-table">
                    <thead>
                        <tr>
                            <th>{{ tt('Primary Category') }}</th>
                            <th class="budget-amount-column">{{ section.amountTitle }}</th>
                            <th class="budget-pace-column">Pacing</th>
                            <th class="text-end">{{ averageMonthsTitle }}</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr :key="category.id" v-for="category in section.categories">
                            <td>
                                <div class="d-flex align-center">
                                    <ItemIcon size="24px" :icon-type="getCategoryIconType(category.iconType)"
                                              :icon-id="category.icon" :color="category.color" />
                                    <span class="ms-3">{{ category.name }}</span>
                                    <v-chip class="ms-2" size="x-small" variant="tonal" v-if="!category.visible">Hidden</v-chip>
                                </div>
                            </td>
                            <td class="budget-amount-column">
                                <amount-input density="compact" variant="outlined" class="my-2"
                                              :currency="defaultCurrency" :disabled="loading || saving"
                                              :persistent-placeholder="true" placeholder="No budget"
                                              v-model="getFormItem(category.id).amount" />
                            </td>
                            <td class="budget-pace-column">
                                <v-select density="compact" variant="outlined" class="my-2" hide-details
                                          item-title="title" item-value="value"
                                          :disabled="loading || saving || !getFormItem(category.id).amount"
                                          :items="paceTypeOptions"
                                          v-model="getFormItem(category.id).paceType" />
                            </td>
                            <td class="text-end text-medium-emphasis">
                                {{ averageAmounts[section.type]?.[category.id] ? formatBudgetAmount(averageAmounts[section.type]?.[category.id] ?? 0) : '-' }}
                            </td>
                        </tr>
                        <tr v-if="!section.categories.length">
                            <td colspan="4" class="text-center text-medium-emphasis">{{ tt('No available category') }}</td>
                        </tr>
                    </tbody>
                </v-table>
            </v-card>
        </v-col>

        <v-col cols="12" class="d-flex flex-wrap ga-4">
            <v-btn color="primary" size="large" :disabled="loading || saving" @click="save">
                Save Budgets
                <v-progress-circular indeterminate size="22" class="ms-2" v-if="saving"/>
            </v-btn>
            <v-btn color="default" variant="tonal" size="large" :disabled="loading || saving" @click="fillFromAverages">
                Fill empty from {{ CATEGORY_BUDGET_AVERAGE_MONTHS }}-month average
            </v-btn>
            <v-btn color="default" variant="text" size="large" to="/statistics/budget" :disabled="loading || saving">
                View budget report
            </v-btn>
        </v-col>
    </v-row>

    <snack-bar ref="snackbar"/>
</template>

<script setup lang="ts">
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useCategoryBudgetDisplay, useCategoryBudgetAverageBase, getTodayYearMonthDay, CATEGORY_BUDGET_AVERAGE_MONTHS } from '@/views/base/statistics/CategoryBudgetReportBase.ts';

import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionCategoryBudgetsStore } from '@/stores/transactionCategoryBudget.ts';

import { CategoryType } from '@/core/category.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import { CategoryBudgetPaceType } from '@/models/transaction_category_budget.ts';

import { getTextualYearMonth, shiftYearMonth } from '@/lib/category_budget.ts';
import { getCategoryIconType } from '@/lib/icon.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

interface BudgetFormItem {
    amount: number;
    paceType: CategoryBudgetPaceType;
}

const { tt } = useI18n();
const { defaultCurrency, formatBudgetAmount, getMonthDisplayName } = useCategoryBudgetDisplay();
const { averageAmounts, loadAverageAmounts } = useCategoryBudgetAverageBase();

const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();
const budgetsStore = useTransactionCategoryBudgetsStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

const today = getTodayYearMonthDay();
const currentYearMonth = today.year * 100 + today.month;

const loading = ref<boolean>(true);
const saving = ref<boolean>(false);
const startYearMonth = ref<number>(currentYearMonth);
const form = ref<Record<string, BudgetFormItem>>({});

const averageMonthsTitle = `${CATEGORY_BUDGET_AVERAGE_MONTHS}-month average`;

const paceTypeOptions = [
    { title: 'Spread evenly', value: CategoryBudgetPaceType.Even },
    { title: 'Fixed bill', value: CategoryBudgetPaceType.Fixed }
];

const startMonthOptions = computed(() => {
    const options: { title: string, value: number }[] = [];

    for (let i = 0; i < 12; i++) {
        const yearMonth = shiftYearMonth(currentYearMonth, -i);
        options.push({
            title: getMonthDisplayName(yearMonth) + (i === 0 ? ' (this month)' : ''),
            value: yearMonth
        });
    }

    return options;
});

const sections = computed(() => [
    {
        type: CategoryType.Expense,
        title: tt('Expense Categories'),
        amountTitle: 'Monthly budget',
        categories: getBudgetableCategories(CategoryType.Expense)
    },
    {
        type: CategoryType.Income,
        title: tt('Income Categories'),
        amountTitle: 'Monthly target',
        categories: getBudgetableCategories(CategoryType.Income)
    }
]);

function getBudgetableCategories(type: CategoryType): TransactionCategory[] {
    return (transactionCategoriesStore.allTransactionCategories[type] ?? [])
        .filter(category => category.visible || !!form.value[category.id]?.amount);
}

function getFormItem(categoryId: string): BudgetFormItem {
    let item = form.value[categoryId];

    if (!item) {
        item = { amount: 0, paceType: CategoryBudgetPaceType.Even };
        form.value[categoryId] = item;
    }

    return item;
}

function getSectionTotal(type: CategoryType): number {
    let total = 0;

    for (const category of transactionCategoriesStore.allTransactionCategories[type] ?? []) {
        total += form.value[category.id]?.amount ?? 0;
    }

    return total;
}

function applyBudgets(): void {
    const effective = budgetsStore.getEffectiveBudgets(currentYearMonth);
    const newForm: Record<string, BudgetFormItem> = {};

    for (const [categoryId, budget] of Object.entries(effective)) {
        newForm[categoryId] = {
            amount: budget.amount,
            paceType: budget.paceType
        };
    }

    form.value = newForm;
}

function fillFromAverages(): void {
    let filled = 0;

    for (const section of sections.value) {
        for (const category of section.categories) {
            const average = averageAmounts.value[section.type]?.[category.id];
            const item = getFormItem(category.id);

            if (!item.amount && average && average > 0) {
                item.amount = Math.ceil(average / 100) * 100;
                filled++;
            }
        }
    }

    snackbar.value?.showMessage(filled ? `Filled ${filled} categories from the last ${CATEGORY_BUDGET_AVERAGE_MONTHS} months` : 'No empty category has spending in the last months');
}

function load(): void {
    loading.value = true;

    Promise.all([
        accountsStore.loadAllAccounts({ force: false }),
        transactionCategoriesStore.loadAllCategories({ force: false }),
        budgetsStore.loadAllBudgets({ force: true })
    ]).then(() => {
        applyBudgets();
        loading.value = false;
        return loadAverageAmounts();
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function save(): void {
    const budgets: { categoryId: string, amount: number, paceType: CategoryBudgetPaceType }[] = [];

    for (const type of [CategoryType.Expense, CategoryType.Income]) {
        for (const category of transactionCategoriesStore.allTransactionCategories[type] ?? []) {
            const item = form.value[category.id];

            if (item && item.amount > 0) {
                budgets.push({
                    categoryId: category.id,
                    amount: item.amount,
                    paceType: item.paceType
                });
            }
        }
    }

    saving.value = true;

    budgetsStore.saveBudgets({
        startYearMonth: getTextualYearMonth(startYearMonth.value),
        budgets: budgets
    }).then(() => {
        saving.value = false;
        snackbar.value?.showMessage('Budgets saved');
    }).catch(error => {
        saving.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

load();
</script>

<style>
.budget-setting-table .budget-amount-column {
    width: 240px;
    min-width: 200px;
}

.budget-setting-table .budget-pace-column {
    width: 200px;
    min-width: 170px;
}
</style>
