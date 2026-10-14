import { ref } from 'vue';
import { defineStore } from 'pinia';

import type { CategoryBudgetInfoResponse, CategoryBudgetSaveRequest } from '@/models/transaction_category_budget.ts';

import { type EffectiveCategoryBudget, getEffectiveCategoryBudgets } from '@/lib/category_budget.ts';
import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

export const useTransactionCategoryBudgetsStore = defineStore('transactionCategoryBudgets', () => {
    const allBudgets = ref<CategoryBudgetInfoResponse[]>([]);
    const budgetListStateInvalid = ref<boolean>(true);

    function getEffectiveBudgets(yearMonth: number): Record<string, EffectiveCategoryBudget> {
        return getEffectiveCategoryBudgets(allBudgets.value, yearMonth);
    }

    function resetCategoryBudgets(): void {
        allBudgets.value = [];
        budgetListStateInvalid.value = true;
    }

    function loadAllBudgets({ force }: { force?: boolean }): Promise<CategoryBudgetInfoResponse[]> {
        if (!force && !budgetListStateInvalid.value) {
            return Promise.resolve(allBudgets.value);
        }

        return new Promise((resolve, reject) => {
            services.getAllCategoryBudgets().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve budgets' });
                    return;
                }

                allBudgets.value = data.result;
                budgetListStateInvalid.value = false;
                resolve(data.result);
            }).catch(error => {
                logger.error('failed to load budgets', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve budgets' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function saveBudgets(req: CategoryBudgetSaveRequest): Promise<CategoryBudgetInfoResponse[]> {
        return new Promise((resolve, reject) => {
            services.saveCategoryBudgets(req).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save budgets' });
                    return;
                }

                allBudgets.value = data.result;
                budgetListStateInvalid.value = false;
                resolve(data.result);
            }).catch(error => {
                logger.error('failed to save budgets', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to save budgets' });
                } else {
                    reject(error);
                }
            });
        });
    }

    return {
        // states
        allBudgets,
        budgetListStateInvalid,
        // functions
        getEffectiveBudgets,
        resetCategoryBudgets,
        loadAllBudgets,
        saveBudgets
    };
});
