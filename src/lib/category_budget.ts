import type { BigDecimal } from '@/core/numeral.ts';
import { CategoryType } from '@/core/category.ts';

import type { Account } from '@/models/account.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import type { TransactionStatisticResponseItem } from '@/models/transaction.ts';
import { type CategoryBudgetInfoResponse, CategoryBudgetPaceType } from '@/models/transaction_category_budget.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';

// Projections before this day of the month are mostly noise, so the budget is assumed to be on pace until then
export const CATEGORY_BUDGET_MIN_PROJECTION_DAYS: number = 5;

export enum CategoryBudgetStatus {
    Ahead = 'ahead',
    OnTrack = 'onTrack',
    AtRisk = 'atRisk',
    Over = 'over',
    WithinBudget = 'withinBudget',
    Reached = 'reached',
    Behind = 'behind',
    Missed = 'missed'
}

export type CategoryBudgetStatusLevel = 'good' | 'neutral' | 'warning' | 'bad';

export const CATEGORY_BUDGET_STATUS_INFO: Record<CategoryBudgetStatus, { name: string; level: CategoryBudgetStatusLevel; order: number }> = {
    [CategoryBudgetStatus.Over]: { name: 'Over budget', level: 'bad', order: 0 },
    [CategoryBudgetStatus.Missed]: { name: 'Missed target', level: 'bad', order: 0 },
    [CategoryBudgetStatus.AtRisk]: { name: 'At risk', level: 'warning', order: 1 },
    [CategoryBudgetStatus.Behind]: { name: 'Behind', level: 'warning', order: 1 },
    [CategoryBudgetStatus.OnTrack]: { name: 'On track', level: 'neutral', order: 2 },
    [CategoryBudgetStatus.WithinBudget]: { name: 'Within budget', level: 'good', order: 3 },
    [CategoryBudgetStatus.Ahead]: { name: 'Ahead', level: 'good', order: 3 },
    [CategoryBudgetStatus.Reached]: { name: 'Target reached', level: 'good', order: 3 }
};

export type BudgetMonthState = 'past' | 'current' | 'future';

export interface YearMonthDayValue {
    readonly year: number;
    readonly month: number; // 1-based
    readonly day: number;
}

export interface BudgetMonthProgress {
    readonly yearMonth: number; // e.g. 202610
    readonly totalDays: number;
    readonly elapsedDays: number;
    readonly daysLeft: number;
    readonly state: BudgetMonthState;
}

export interface EffectiveCategoryBudget {
    readonly amount: number;
    readonly paceType: CategoryBudgetPaceType;
}

export interface CategoryBudgetProgress {
    readonly budget: number;
    readonly actual: number;
    readonly isIncome: boolean;
    readonly paceType: CategoryBudgetPaceType;
    readonly expected: number | null;
    readonly paceDifference: number | null;
    readonly projected: number;
    readonly remaining: number;
    readonly remainingPerDay: number | null;
    readonly usedRatio: number;
    readonly expectedRatio: number | null;
    readonly projectedRatio: number;
    readonly status: CategoryBudgetStatus;
    readonly level: CategoryBudgetStatusLevel;
}

export interface CategoryBudgetReportRow {
    readonly category: TransactionCategory;
    readonly progress: CategoryBudgetProgress;
}

export interface CategoryBudgetUnbudgetedRow {
    readonly category: TransactionCategory;
    readonly actual: number;
}

export interface CategoryBudgetMonthReport {
    readonly month: BudgetMonthProgress;
    readonly isIncome: boolean;
    readonly rows: CategoryBudgetReportRow[];
    readonly unbudgetedRows: CategoryBudgetUnbudgetedRow[];
    readonly total: CategoryBudgetProgress | null;
}

export interface CategoryBudgetHistoryCell {
    readonly yearMonth: number;
    readonly budget: number;
    readonly actual: number;
    readonly ratio: number | null;
    readonly projectedRatio: number | null;
    readonly heatLevel: number; // 0 = no budget, 1 = best, 5 = worst
    readonly state: BudgetMonthState;
}

export interface CategoryBudgetHistoryRow {
    readonly category: TransactionCategory;
    readonly cells: CategoryBudgetHistoryCell[];
    readonly missedCount: number;
    readonly budgetedCount: number;
}

export interface PrimaryCategoryAmounts {
    readonly amounts: Record<string, number>;
    readonly incomplete: boolean;
}

export type ExchangeAmountFunction = (amount: BigDecimal, fromCurrency: string) => BigDecimal | null;

export function getNumericYearMonth(year: number, month: number): number {
    return year * 100 + month;
}

export function getTextualYearMonth(yearMonth: number): string {
    const year = Math.floor(yearMonth / 100);
    const month = yearMonth % 100;
    return `${year.toString().padStart(4, '0')}-${month.toString().padStart(2, '0')}`;
}

export function parseTextualYearMonth(textualYearMonth: string): number | null {
    const parts = textualYearMonth.split('-');

    if (parts.length !== 2) {
        return null;
    }

    const year = parseInt(parts[0] as string, 10);
    const month = parseInt(parts[1] as string, 10);

    if (!Number.isFinite(year) || !Number.isFinite(month) || month < 1 || month > 12) {
        return null;
    }

    return getNumericYearMonth(year, month);
}

export function shiftYearMonth(yearMonth: number, months: number): number {
    const index = Math.floor(yearMonth / 100) * 12 + (yearMonth % 100 - 1) + months;
    return getNumericYearMonth(Math.floor(index / 12), index % 12 + 1);
}

export function getDaysInMonth(yearMonth: number): number {
    return new Date(Math.floor(yearMonth / 100), yearMonth % 100, 0).getDate();
}

export function getBudgetMonthProgress(yearMonth: number, today: YearMonthDayValue): BudgetMonthProgress {
    const totalDays = getDaysInMonth(yearMonth);
    const currentYearMonth = getNumericYearMonth(today.year, today.month);
    let state: BudgetMonthState = 'current';
    let elapsedDays = Math.min(Math.max(today.day, 1), totalDays);

    if (yearMonth < currentYearMonth) {
        state = 'past';
        elapsedDays = totalDays;
    } else if (yearMonth > currentYearMonth) {
        state = 'future';
        elapsedDays = 0;
    }

    return {
        yearMonth: yearMonth,
        totalDays: totalDays,
        elapsedDays: elapsedDays,
        daysLeft: totalDays - elapsedDays,
        state: state
    };
}

// Returns the budget of each category that applies in the given month. Categories without a budget are omitted.
export function getEffectiveCategoryBudgets(budgets: CategoryBudgetInfoResponse[], yearMonth: number): Record<string, EffectiveCategoryBudget> {
    const latest: Record<string, { startYearMonth: number; budget: CategoryBudgetInfoResponse }> = {};

    for (const budget of budgets) {
        const startYearMonth = parseTextualYearMonth(budget.startYearMonth);

        if (startYearMonth === null || startYearMonth > yearMonth) {
            continue;
        }

        const existing = latest[budget.categoryId];

        if (!existing || existing.startYearMonth < startYearMonth) {
            latest[budget.categoryId] = { startYearMonth, budget };
        }
    }

    const result: Record<string, EffectiveCategoryBudget> = {};

    for (const [categoryId, item] of Object.entries(latest)) {
        if (item.budget.amount > 0) {
            result[categoryId] = {
                amount: item.budget.amount,
                paceType: item.budget.paceType
            };
        }
    }

    return result;
}

function determineStatus(isIncome: boolean, paceType: CategoryBudgetPaceType, month: BudgetMonthProgress, budget: number, actual: number, expected: number | null, projected: number): CategoryBudgetStatus {
    const completed = month.state === 'past' || month.daysLeft <= 0;
    const canProject = month.elapsedDays >= CATEGORY_BUDGET_MIN_PROJECTION_DAYS;

    if (isIncome) {
        if (actual >= budget) {
            return CategoryBudgetStatus.Reached;
        } else if (completed) {
            return CategoryBudgetStatus.Missed;
        } else if (paceType === CategoryBudgetPaceType.Fixed || expected === null) {
            return CategoryBudgetStatus.OnTrack;
        } else if (canProject && projected < budget * 0.95) {
            return CategoryBudgetStatus.Behind;
        } else if (actual > expected * 1.15) {
            return CategoryBudgetStatus.Ahead;
        }

        return CategoryBudgetStatus.OnTrack;
    }

    if (actual > budget) {
        return CategoryBudgetStatus.Over;
    } else if (completed) {
        return actual < budget * 0.85 ? CategoryBudgetStatus.WithinBudget : CategoryBudgetStatus.OnTrack;
    } else if (paceType === CategoryBudgetPaceType.Fixed || expected === null) {
        return CategoryBudgetStatus.OnTrack;
    } else if ((canProject && projected > budget * 1.05) || actual >= budget * 0.9) {
        return CategoryBudgetStatus.AtRisk;
    } else if (actual < expected * 0.85) {
        return CategoryBudgetStatus.Ahead;
    }

    return CategoryBudgetStatus.OnTrack;
}

function buildProgress(isIncome: boolean, paceType: CategoryBudgetPaceType, month: BudgetMonthProgress, budget: number, actual: number, expected: number | null, projected: number): CategoryBudgetProgress {
    const status = determineStatus(isIncome, paceType, month, budget, actual, expected, projected);
    const remaining = budget - actual;

    return {
        budget: budget,
        actual: actual,
        isIncome: isIncome,
        paceType: paceType,
        expected: expected,
        paceDifference: expected === null ? null : actual - expected,
        projected: projected,
        remaining: remaining,
        remainingPerDay: month.daysLeft > 0 && remaining > 0 ? remaining / month.daysLeft : null,
        usedRatio: budget > 0 ? actual / budget : 0,
        expectedRatio: expected === null || budget <= 0 ? null : expected / budget,
        projectedRatio: budget > 0 ? projected / budget : 0,
        status: status,
        level: CATEGORY_BUDGET_STATUS_INFO[status].level
    };
}

export function calculateCategoryBudgetProgress(budget: EffectiveCategoryBudget, actual: number, isIncome: boolean, month: BudgetMonthProgress): CategoryBudgetProgress {
    const amount = budget.amount;
    const elapsedRatio = month.totalDays > 0 ? month.elapsedDays / month.totalDays : 0;
    const completed = month.state === 'past' || month.daysLeft <= 0;
    let expected: number | null = null;
    let projected: number;

    if (budget.paceType === CategoryBudgetPaceType.Even) {
        expected = amount * elapsedRatio;
    }

    if (completed) {
        projected = actual;
    } else if (budget.paceType === CategoryBudgetPaceType.Fixed || month.elapsedDays < CATEGORY_BUDGET_MIN_PROJECTION_DAYS || elapsedRatio <= 0) {
        projected = Math.max(actual, amount);
    } else {
        projected = actual / elapsedRatio;
    }

    return buildProgress(isIncome, budget.paceType, month, amount, actual, expected, projected);
}

export function calculateTotalCategoryBudgetProgress(rows: CategoryBudgetReportRow[], isIncome: boolean, month: BudgetMonthProgress): CategoryBudgetProgress | null {
    if (!rows.length) {
        return null;
    }

    let budget = 0;
    let actual = 0;
    let expected = 0;
    let projected = 0;

    for (const row of rows) {
        budget += row.progress.budget;
        actual += row.progress.actual;
        projected += row.progress.projected;
        // fixed bills have no pace, so they count as exactly on pace in the total
        expected += row.progress.expected ?? Math.min(row.progress.actual, row.progress.budget);
    }

    return buildProgress(isIncome, CategoryBudgetPaceType.Even, month, budget, actual, expected, projected);
}

function compareCategories(a: TransactionCategory, b: TransactionCategory): number {
    return a.displayOrder - b.displayOrder;
}

export function buildCategoryBudgetMonthReport(primaryCategories: TransactionCategory[], budgets: Record<string, EffectiveCategoryBudget>, amounts: Record<string, number>, isIncome: boolean, month: BudgetMonthProgress): CategoryBudgetMonthReport {
    const rows: CategoryBudgetReportRow[] = [];
    const unbudgetedRows: CategoryBudgetUnbudgetedRow[] = [];

    for (const category of [...primaryCategories].sort(compareCategories)) {
        const budget = budgets[category.id];
        const actual = amounts[category.id] ?? 0;

        if (budget) {
            rows.push({
                category: category,
                progress: calculateCategoryBudgetProgress(budget, actual, isIncome, month)
            });
        } else if (actual !== 0) {
            unbudgetedRows.push({
                category: category,
                actual: actual
            });
        }
    }

    rows.sort((a, b) => {
        const orderA = CATEGORY_BUDGET_STATUS_INFO[a.progress.status].order;
        const orderB = CATEGORY_BUDGET_STATUS_INFO[b.progress.status].order;

        if (orderA !== orderB) {
            return orderA - orderB;
        }

        return isIncome ? a.progress.usedRatio - b.progress.usedRatio : b.progress.usedRatio - a.progress.usedRatio;
    });

    unbudgetedRows.sort((a, b) => b.actual - a.actual);

    return {
        month: month,
        isIncome: isIncome,
        rows: rows,
        unbudgetedRows: unbudgetedRows,
        total: calculateTotalCategoryBudgetProgress(rows, isIncome, month)
    };
}

// 1 is the best outcome and 5 the worst; expense budgets are better when less is used, income targets when more is earned
export function getCategoryBudgetHeatLevel(ratio: number, isIncome: boolean): number {
    if (isIncome) {
        if (ratio >= 1) {
            return 1;
        } else if (ratio >= 0.9) {
            return 3;
        } else if (ratio >= 0.7) {
            return 4;
        }

        return 5;
    }

    if (ratio < 0.7) {
        return 1;
    } else if (ratio < 0.9) {
        return 2;
    } else if (ratio <= 1) {
        return 3;
    } else if (ratio <= 1.15) {
        return 4;
    }

    return 5;
}

export function buildCategoryBudgetHistory(primaryCategories: TransactionCategory[], budgetHistory: CategoryBudgetInfoResponse[], monthlyAmounts: Record<number, Record<string, number>>, yearMonths: number[], isIncome: boolean, today: YearMonthDayValue): CategoryBudgetHistoryRow[] {
    const budgetsByMonth: Record<number, Record<string, EffectiveCategoryBudget>> = {};

    for (const yearMonth of yearMonths) {
        budgetsByMonth[yearMonth] = getEffectiveCategoryBudgets(budgetHistory, yearMonth);
    }

    const rows: CategoryBudgetHistoryRow[] = [];

    for (const category of [...primaryCategories].sort(compareCategories)) {
        const cells: CategoryBudgetHistoryCell[] = [];
        let missedCount = 0;
        let budgetedCount = 0;

        for (const yearMonth of yearMonths) {
            const budget = budgetsByMonth[yearMonth]?.[category.id];
            const actual = monthlyAmounts[yearMonth]?.[category.id] ?? 0;
            const month = getBudgetMonthProgress(yearMonth, today);

            if (!budget) {
                cells.push({ yearMonth, budget: 0, actual, ratio: null, projectedRatio: null, heatLevel: 0, state: month.state });
                continue;
            }

            const progress = calculateCategoryBudgetProgress(budget, actual, isIncome, month);
            const ratio = progress.usedRatio;
            budgetedCount++;

            if (progress.status === CategoryBudgetStatus.Over || progress.status === CategoryBudgetStatus.Missed) {
                missedCount++;
            }

            cells.push({
                yearMonth: yearMonth,
                budget: budget.amount,
                actual: actual,
                ratio: ratio,
                projectedRatio: month.state === 'current' ? progress.projectedRatio : null,
                heatLevel: getCategoryBudgetHeatLevel(month.state === 'current' ? progress.projectedRatio : ratio, isIncome),
                state: month.state
            });
        }

        if (budgetedCount > 0) {
            rows.push({ category, cells, missedCount, budgetedCount });
        }
    }

    return rows;
}

// Sums income or expense statistic items into their primary categories, converted to the default currency (in cents)
export function sumAmountsByPrimaryCategory(items: TransactionStatisticResponseItem[], categoryType: CategoryType, allCategoriesMap: Record<string, TransactionCategory>, allAccountsMap: Record<string, Account>, defaultCurrency: string, exchangeAmount: ExchangeAmountFunction): PrimaryCategoryAmounts {
    const amounts: Record<string, number> = {};
    let incomplete = false;

    for (const item of items) {
        if (item.relatedAccountId && item.relatedAccountId !== '0') {
            continue;
        }

        const category = allCategoriesMap[item.categoryId];
        const account = allAccountsMap[item.accountId];

        if (!category || !account || category.type !== categoryType) {
            continue;
        }

        const primaryCategoryId = category.parentId && category.parentId !== '0' ? category.parentId : category.id;
        let amount: BigDecimal | null = parseBigDecimal(item.amount);

        if (account.currency !== defaultCurrency) {
            amount = exchangeAmount(amount, account.currency);

            if (!amount) {
                incomplete = true;
                continue;
            }

            amount = amount.truncate();
        }

        amounts[primaryCategoryId] = (amounts[primaryCategoryId] ?? 0) + amount.toDoubleNumber();
    }

    return { amounts, incomplete };
}
