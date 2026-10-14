import { describe, expect, test } from 'vitest';

import { CategoryType } from '@/core/category.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import { CategoryBudgetPaceType } from '@/models/transaction_category_budget.ts';

import {
    CategoryBudgetStatus,
    buildCategoryBudgetHistory,
    buildCategoryBudgetMonthReport,
    calculateCategoryBudgetProgress,
    getBudgetMonthProgress,
    getEffectiveCategoryBudgets,
    shiftYearMonth
} from '@/lib/category_budget.ts';

const TODAY = { year: 2026, month: 10, day: 14 };
const OCTOBER = getBudgetMonthProgress(202610, TODAY);
const EVEN = (amount: number) => ({ amount, paceType: CategoryBudgetPaceType.Even });
const FIXED = (amount: number) => ({ amount, paceType: CategoryBudgetPaceType.Fixed });

function category(id: string, displayOrder: number): TransactionCategory {
    return { id, name: id, parentId: '0', type: CategoryType.Expense, displayOrder } as TransactionCategory;
}

describe('year months', () => {
    test('shifts across years', () => {
        expect(shiftYearMonth(202601, -1)).toBe(202512);
        expect(shiftYearMonth(202611, 3)).toBe(202702);
    });

    test('describes past, current and future months', () => {
        expect(OCTOBER).toMatchObject({ totalDays: 31, elapsedDays: 14, daysLeft: 17, state: 'current' });
        expect(getBudgetMonthProgress(202609, TODAY)).toMatchObject({ elapsedDays: 30, daysLeft: 0, state: 'past' });
        expect(getBudgetMonthProgress(202611, TODAY)).toMatchObject({ elapsedDays: 0, state: 'future' });
    });
});

describe('getEffectiveCategoryBudgets', () => {
    const history = [
        { categoryId: 'food', startYearMonth: '2026-05', amount: 90000, paceType: CategoryBudgetPaceType.Even },
        { categoryId: 'rent', startYearMonth: '2026-05', amount: 220000, paceType: CategoryBudgetPaceType.Fixed },
        { categoryId: 'food', startYearMonth: '2026-08', amount: 100000, paceType: CategoryBudgetPaceType.Even },
        { categoryId: 'rent', startYearMonth: '2026-09', amount: 0, paceType: CategoryBudgetPaceType.Fixed }
    ];

    test('uses the latest budget that started on or before the month', () => {
        expect(getEffectiveCategoryBudgets(history, 202604)).toEqual({});
        expect(getEffectiveCategoryBudgets(history, 202607)).toEqual({ food: EVEN(90000), rent: FIXED(220000) });
        expect(getEffectiveCategoryBudgets(history, 202610)).toEqual({ food: EVEN(100000) });
    });
});

describe('calculateCategoryBudgetProgress for expenses', () => {
    test('is ahead when well below pace', () => {
        const progress = calculateCategoryBudgetProgress(EVEN(90000), 20000, false, OCTOBER);
        expect(progress.status).toBe(CategoryBudgetStatus.Ahead);
        expect(progress.expected).toBeCloseTo(90000 * 14 / 31);
        expect(progress.remainingPerDay).toBeCloseTo(70000 / 17);
    });

    test('is on track near pace', () => {
        expect(calculateCategoryBudgetProgress(EVEN(90000), 40000, false, OCTOBER).status).toBe(CategoryBudgetStatus.OnTrack);
    });

    test('is at risk when projected to go over', () => {
        const progress = calculateCategoryBudgetProgress(EVEN(90000), 48000, false, OCTOBER);
        expect(progress.projected).toBeCloseTo(48000 * 31 / 14);
        expect(progress.status).toBe(CategoryBudgetStatus.AtRisk);
    });

    test('is over once the budget is exceeded', () => {
        expect(calculateCategoryBudgetProgress(EVEN(10000), 14000, false, OCTOBER).status).toBe(CategoryBudgetStatus.Over);
    });

    test('does not flag a fixed bill paid early', () => {
        const progress = calculateCategoryBudgetProgress(FIXED(220000), 180000, false, getBudgetMonthProgress(202610, { year: 2026, month: 10, day: 2 }));
        expect(progress.status).toBe(CategoryBudgetStatus.OnTrack);
        expect(progress.expected).toBeNull();
        expect(progress.projected).toBe(220000);
    });

    test('does not project during the first days of the month', () => {
        const earlyMonth = getBudgetMonthProgress(202610, { year: 2026, month: 10, day: 2 });
        expect(calculateCategoryBudgetProgress(EVEN(90000), 8000, false, earlyMonth).status).not.toBe(CategoryBudgetStatus.AtRisk);
    });

    test('reports completed months as within budget or over', () => {
        const september = getBudgetMonthProgress(202609, TODAY);
        expect(calculateCategoryBudgetProgress(EVEN(90000), 50000, false, september).status).toBe(CategoryBudgetStatus.WithinBudget);
        expect(calculateCategoryBudgetProgress(EVEN(90000), 85000, false, september).status).toBe(CategoryBudgetStatus.OnTrack);
        expect(calculateCategoryBudgetProgress(EVEN(90000), 95000, false, september).status).toBe(CategoryBudgetStatus.Over);
    });
});

describe('calculateCategoryBudgetProgress for income', () => {
    test('follows the income target', () => {
        expect(calculateCategoryBudgetProgress(EVEN(500000), 510000, true, OCTOBER).status).toBe(CategoryBudgetStatus.Reached);
        expect(calculateCategoryBudgetProgress(EVEN(500000), 100000, true, OCTOBER).status).toBe(CategoryBudgetStatus.Behind);
        expect(calculateCategoryBudgetProgress(EVEN(500000), 300000, true, OCTOBER).status).toBe(CategoryBudgetStatus.Ahead);
        expect(calculateCategoryBudgetProgress(FIXED(500000), 0, true, OCTOBER).status).toBe(CategoryBudgetStatus.OnTrack);
        expect(calculateCategoryBudgetProgress(FIXED(500000), 0, true, getBudgetMonthProgress(202609, TODAY)).status).toBe(CategoryBudgetStatus.Missed);
    });
});

describe('buildCategoryBudgetMonthReport', () => {
    test('sorts by severity, separates unbudgeted spending and totals the budgets', () => {
        const report = buildCategoryBudgetMonthReport(
            [category('food', 1), category('gifts', 2), category('misc', 3), category('edu', 4)],
            { food: EVEN(90000), gifts: EVEN(10000), edu: EVEN(15000) },
            { food: 40000, gifts: 14000, misc: 5000 },
            false,
            OCTOBER
        );

        expect(report.rows.map(row => row.category.id)).toEqual(['gifts', 'food', 'edu']);
        expect(report.unbudgetedRows.map(row => row.category.id)).toEqual(['misc']);
        expect(report.total?.budget).toBe(115000);
        expect(report.total?.actual).toBe(54000);
    });
});

describe('buildCategoryBudgetHistory', () => {
    test('uses the budget that applied in each month', () => {
        const history = [
            { categoryId: 'food', startYearMonth: '2026-08', amount: 100000, paceType: CategoryBudgetPaceType.Even },
            { categoryId: 'food', startYearMonth: '2026-09', amount: 50000, paceType: CategoryBudgetPaceType.Even }
        ];
        const rows = buildCategoryBudgetHistory(
            [category('food', 1), category('misc', 2)],
            history,
            { 202607: { food: 1 }, 202608: { food: 80000 }, 202609: { food: 60000 } },
            [202607, 202608, 202609],
            false,
            TODAY
        );

        expect(rows).toHaveLength(1);
        expect(rows[0]?.cells.map(cell => cell.ratio)).toEqual([null, 0.8, 1.2]);
        expect(rows[0]?.missedCount).toBe(1);
        expect(rows[0]?.budgetedCount).toBe(2);
    });
});
