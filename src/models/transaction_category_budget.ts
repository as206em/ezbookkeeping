export enum CategoryBudgetPaceType {
    Even = 0,
    Fixed = 1
}

export interface CategoryBudgetInfoResponse {
    readonly categoryId: string;
    readonly startYearMonth: string; // e.g. 2026-10
    readonly amount: number;
    readonly paceType: CategoryBudgetPaceType;
}

export interface CategoryBudgetSaveItem {
    readonly categoryId: string;
    readonly amount: number;
    readonly paceType: CategoryBudgetPaceType;
}

export interface CategoryBudgetSaveRequest {
    readonly startYearMonth: string; // e.g. 2026-10
    readonly budgets: CategoryBudgetSaveItem[];
}
