<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ disabled: loading || saving }" :back-link="tt('Back')"/>
            <f7-nav-title title="Budgets"/>
            <f7-nav-right :class="{ disabled: loading || saving }">
                <f7-link :text="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-block class="margin-vertical-half">
            Monthly budgets for expense categories and monthly targets for income categories, in {{ defaultCurrency }}.
            Leave a category empty to stop tracking it. Use "Fixed bill" for rent, insurance or salary.
        </f7-block>

        <f7-list strong inset dividers :class="{ disabled: loading || saving }">
            <f7-list-input type="select" label="Apply from" :value="startYearMonth"
                           @input="startYearMonth = parseInt($event.target.value, 10)">
                <option :value="option.value" :key="option.value" v-for="option in startMonthOptions">{{ option.title }}</option>
            </f7-list-input>
        </f7-list>
        <f7-block class="margin-vertical-half">
            Saving replaces budgets from {{ getMonthDisplayName(startYearMonth) }} onward. Earlier months keep their budgets.
        </f7-block>

        <template :key="section.type" v-for="section in sections">
            <f7-block-title>{{ section.title }} · {{ formatBudgetAmount(getSectionTotal(section.type)) }}</f7-block-title>
            <f7-list strong inset dividers :class="{ disabled: loading || saving }">
                <template :key="category.id" v-for="category in section.categories">
                    <f7-list-input type="text" inputmode="decimal" placeholder="No budget"
                                   :label="category.name" :info="getAverageInfo(section.type, category.id)"
                                   :value="amountTexts[category.id] ?? ''"
                                   @input="amountTexts[category.id] = $event.target.value">
                        <template #media>
                            <ItemIcon :icon-type="getCategoryIconType(category.iconType)" :icon-id="category.icon" :color="category.color" />
                        </template>
                    </f7-list-input>
                    <f7-list-input type="select" label="Pacing" :value="paceTypes[category.id] ?? CategoryBudgetPaceType.Even"
                                   @input="paceTypes[category.id] = parseInt($event.target.value, 10)"
                                   v-if="getAmount(category.id) > 0">
                        <option :value="CategoryBudgetPaceType.Even">Spread evenly</option>
                        <option :value="CategoryBudgetPaceType.Fixed">Fixed bill</option>
                    </f7-list-input>
                </template>
                <f7-list-item :title="tt('No available category')" v-if="!section.categories.length"></f7-list-item>
            </f7-list>
        </template>

        <f7-list strong inset :class="{ disabled: loading || saving }">
            <f7-list-button @click="fillFromAverages">Fill empty from {{ CATEGORY_BUDGET_AVERAGE_MONTHS }}-month average</f7-list-button>
            <f7-list-button @click="save">Save Budgets</f7-list-button>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useCategoryBudgetDisplay, useCategoryBudgetAverageBase, getTodayYearMonthDay, CATEGORY_BUDGET_AVERAGE_MONTHS } from '@/views/base/statistics/CategoryBudgetReportBase.ts';

import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionCategoryBudgetsStore } from '@/stores/transactionCategoryBudget.ts';

import { CategoryType } from '@/core/category.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import { CategoryBudgetPaceType } from '@/models/transaction_category_budget.ts';

import { getTextualYearMonth, shiftYearMonth } from '@/lib/category_budget.ts';
import { getCategoryIconType } from '@/lib/icon.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';

const props = defineProps<{
    f7router: Router.Router;
}>();

const { tt, parseAmountFromLocalizedNumerals, formatAmountToLocalizedNumeralsWithoutDigitGrouping } = useI18n();
const { routeBackOnError, showToast } = useI18nUIComponents();
const { defaultCurrency, formatBudgetAmount, getMonthDisplayName } = useCategoryBudgetDisplay();
const { averageAmounts, loadAverageAmounts } = useCategoryBudgetAverageBase();

const accountsStore = useAccountsStore();
const transactionCategoriesStore = useTransactionCategoriesStore();
const budgetsStore = useTransactionCategoryBudgetsStore();

const today = getTodayYearMonthDay();
const currentYearMonth = today.year * 100 + today.month;

const loading = ref<boolean>(true);
const saving = ref<boolean>(false);
const loadingError = ref<unknown | null>(null);
const startYearMonth = ref<number>(currentYearMonth);
const amountTexts = ref<Record<string, string>>({});
const paceTypes = ref<Record<string, CategoryBudgetPaceType>>({});

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
        categories: getBudgetableCategories(CategoryType.Expense)
    },
    {
        type: CategoryType.Income,
        title: tt('Income Categories'),
        categories: getBudgetableCategories(CategoryType.Income)
    }
]);

function getBudgetableCategories(type: CategoryType): TransactionCategory[] {
    return (transactionCategoriesStore.allTransactionCategories[type] ?? [])
        .filter(category => category.visible || getAmount(category.id) > 0);
}

function getAmount(categoryId: string): number {
    const text = amountTexts.value[categoryId];

    if (!text || !text.trim()) {
        return 0;
    }

    const amount = parseAmountFromLocalizedNumerals(text.trim());
    return Number.isFinite(amount) ? amount : 0;
}

function getSectionTotal(type: CategoryType): number {
    let total = 0;

    for (const category of transactionCategoriesStore.allTransactionCategories[type] ?? []) {
        total += Math.max(getAmount(category.id), 0);
    }

    return total;
}

function getAverageInfo(type: CategoryType, categoryId: string): string | undefined {
    const average = averageAmounts.value[type]?.[categoryId];
    return average ? `${CATEGORY_BUDGET_AVERAGE_MONTHS}-month average: ${formatBudgetAmount(average)}` : undefined;
}

function toAmountText(amount: number): string {
    return formatAmountToLocalizedNumeralsWithoutDigitGrouping(parseBigDecimal(amount));
}

function applyBudgets(): void {
    const effective = budgetsStore.getEffectiveBudgets(currentYearMonth);
    const texts: Record<string, string> = {};
    const paces: Record<string, CategoryBudgetPaceType> = {};

    for (const [categoryId, budget] of Object.entries(effective)) {
        texts[categoryId] = toAmountText(budget.amount);
        paces[categoryId] = budget.paceType;
    }

    amountTexts.value = texts;
    paceTypes.value = paces;
}

function fillFromAverages(): void {
    let filled = 0;

    for (const section of sections.value) {
        for (const category of section.categories) {
            const average = averageAmounts.value[section.type]?.[category.id];

            if (getAmount(category.id) <= 0 && average && average > 0) {
                amountTexts.value[category.id] = toAmountText(Math.ceil(average / 100) * 100);
                filled++;
            }
        }
    }

    showToast(filled ? `Filled ${filled} categories` : 'No empty category has recent spending');
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

        loadAverageAmounts().catch(error => {
            if (!error.processed) {
                showToast(error.message || error);
            }
        });
    }).catch(error => {
        loading.value = false;
        loadingError.value = error;

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function save(): void {
    const budgets: { categoryId: string, amount: number, paceType: CategoryBudgetPaceType }[] = [];

    for (const type of [CategoryType.Expense, CategoryType.Income]) {
        for (const category of transactionCategoriesStore.allTransactionCategories[type] ?? []) {
            const amount = getAmount(category.id);

            if (amount < 0) {
                showToast(`The budget of ${category.name} cannot be negative`);
                return;
            }

            if (amount > 0) {
                budgets.push({
                    categoryId: category.id,
                    amount: amount,
                    paceType: paceTypes.value[category.id] ?? CategoryBudgetPaceType.Even
                });
            }
        }
    }

    saving.value = true;
    showLoading(() => saving.value);

    budgetsStore.saveBudgets({
        startYearMonth: getTextualYearMonth(startYearMonth.value),
        budgets: budgets
    }).then(() => {
        saving.value = false;
        hideLoading();
        showToast('Budgets saved');
    }).catch(error => {
        saving.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

load();
</script>
