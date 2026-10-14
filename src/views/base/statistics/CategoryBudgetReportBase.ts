import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionCategoryBudgetsStore } from '@/stores/transactionCategoryBudget.ts';
import { useExchangeRatesStore } from '@/stores/exchangeRates.ts';
import { useOverviewStore } from '@/stores/overview.ts';

import { CategoryType } from '@/core/category.ts';
import { DateRange } from '@/core/datetime.ts';
import { KeywordMatchMode } from '@/core/text.ts';
import { TimezoneTypeForStatistics } from '@/core/timezone.ts';
import { TransactionType } from '@/core/transaction.ts';

import type { TransactionCategory } from '@/models/transaction_category.ts';
import type { TransactionInfoResponse, TransactionStatisticResponseItem } from '@/models/transaction.ts';
import { CategoryBudgetPaceType } from '@/models/transaction_category_budget.ts';

import {
    type CategoryBudgetHistoryRow,
    type CategoryBudgetMonthReport,
    type CategoryBudgetProgress,
    CategoryBudgetStatus,
    type CategoryBudgetStatusLevel,
    type PrimaryCategoryAmounts,
    type YearMonthDayValue,
    CATEGORY_BUDGET_STATUS_INFO,
    buildCategoryBudgetHistory,
    buildCategoryBudgetMonthReport,
    getBudgetMonthProgress,
    getDaysInMonth,
    getTextualYearMonth,
    parseTextualYearMonth,
    shiftYearMonth,
    sumAmountsByPrimaryCategory
} from '@/lib/category_budget.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import { getCurrentDateTime, getYearMonthDayDateTime, getYearMonthFirstUnixTime, getYearMonthLastUnixTime } from '@/lib/datetime.ts';
import services from '@/lib/services.ts';

export const CATEGORY_BUDGET_HISTORY_MONTHS_OPTIONS: number[] = [6, 12];
const CATEGORY_BUDGET_LOADED_MONTHS: number = 12;

export interface CategoryBudgetDailyDetail {
    readonly dailyAmounts: number[];
    readonly transactions: TransactionInfoResponse[];
    readonly incomplete: boolean;
}

export function getTodayYearMonthDay(): YearMonthDayValue {
    const now = getCurrentDateTime();
    const yearMonth = parseTextualYearMonth(now.getGregorianCalendarYearDashMonth()) as number;

    return {
        year: Math.floor(yearMonth / 100),
        month: yearMonth % 100,
        day: now.getGregorianCalendarDay()
    };
}

export function getStatusColor(level: CategoryBudgetStatusLevel): string {
    switch (level) {
        case 'good':
            return 'success';
        case 'warning':
            return 'warning';
        case 'bad':
            return 'error';
        default:
            return 'primary';
    }
}

export function useCategoryBudgetDisplay() {
    const {
        formatAmountToLocalizedNumeralsWithCurrency,
        formatPercentToLocalizedNumerals,
        formatDateTimeToGregorianLikeLongYearMonth,
        formatDateTimeToGregorianLikeShortMonth
    } = useI18n();

    const userStore = useUserStore();

    const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);

    function formatBudgetAmount(amount: number, withSign?: boolean): string {
        const rounded = Math.round(amount);
        const text = formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(Math.abs(rounded)), defaultCurrency.value);

        if (withSign && rounded > 0) {
            return '+' + text;
        } else if (rounded < 0) {
            return '-' + text;
        }

        return text;
    }

    function formatBudgetPercent(ratio: number): string {
        return formatPercentToLocalizedNumerals(ratio * 100, 0, '<1');
    }

    function getStatusName(status: CategoryBudgetStatus): string {
        return CATEGORY_BUDGET_STATUS_INFO[status].name;
    }

    function getStatusNote(progress: CategoryBudgetProgress, completed: boolean): string {
        const fixed = progress.paceType === CategoryBudgetPaceType.Fixed;

        if (progress.isIncome) {
            if (Math.round(progress.actual - progress.budget) === 0) {
                return 'Target met';
            } else if (progress.actual > progress.budget) {
                return `${formatBudgetAmount(progress.actual - progress.budget)} above target`;
            } else if (completed) {
                return `${formatBudgetAmount(progress.remaining)} short of target`;
            } else if (fixed) {
                return progress.actual === 0 ? 'Not received yet' : `${formatBudgetAmount(progress.remaining)} still to come`;
            } else if (progress.projected < progress.budget * 0.95 && progress.paceDifference !== null && progress.paceDifference < 0) {
                return `On pace to fall short by ${formatBudgetAmount(progress.budget - progress.projected)}`;
            } else if (progress.paceDifference !== null && progress.paceDifference > 0) {
                return `${formatBudgetAmount(progress.paceDifference)} ahead of pace`;
            }

            return `${formatBudgetAmount(progress.remaining)} still to come`;
        }

        if (progress.actual > progress.budget) {
            return `${formatBudgetAmount(progress.actual - progress.budget)} over budget`;
        } else if (completed) {
            return `${formatBudgetAmount(progress.remaining)} left unspent`;
        } else if (fixed) {
            return progress.actual === 0 ? 'Bill not paid yet' : `${formatBudgetAmount(progress.remaining)} left`;
        } else if (progress.projected > progress.budget * 1.05 && progress.status === CategoryBudgetStatus.AtRisk) {
            return `On pace to go over by ${formatBudgetAmount(progress.projected - progress.budget)}`;
        } else if (progress.status === CategoryBudgetStatus.AtRisk) {
            return `Only ${formatBudgetAmount(progress.remaining)} left`;
        } else if (progress.paceDifference !== null && progress.paceDifference < 0 && progress.status === CategoryBudgetStatus.Ahead) {
            return `${formatBudgetAmount(-progress.paceDifference)} below pace`;
        }

        return `${formatBudgetAmount(progress.remaining)} left`;
    }

    function getMonthDisplayName(yearMonth: number): string {
        return formatDateTimeToGregorianLikeLongYearMonth(getYearMonthDayDateTime(Math.floor(yearMonth / 100), yearMonth % 100, 1));
    }

    function getMonthShortName(yearMonth: number): string {
        return formatDateTimeToGregorianLikeShortMonth(getYearMonthDayDateTime(Math.floor(yearMonth / 100), yearMonth % 100, 1));
    }

    function getTransactionListUrl(categoryId: string, yearMonth: number, isIncome: boolean): string {
        const yearMonthObject = { year: Math.floor(yearMonth / 100), month1base: yearMonth % 100 };
        const minTime = getYearMonthFirstUnixTime(yearMonthObject);
        const maxTime = getYearMonthLastUnixTime(yearMonthObject);
        const type = isIncome ? TransactionType.Income : TransactionType.Expense;

        return `/transaction/list?type=${type}&dateType=${DateRange.Custom.type}&minTime=${minTime}&maxTime=${maxTime}&categoryIds=${categoryId}`;
    }

    return {
        defaultCurrency,
        formatBudgetAmount,
        formatBudgetPercent,
        getStatusName,
        getStatusNote,
        getStatusColor,
        getMonthDisplayName,
        getMonthShortName,
        getTransactionListUrl
    };
}

export function loadCategoryBudgetDailyDetail(categoryId: string, yearMonth: number, income: boolean): Promise<CategoryBudgetDailyDetail> {
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();
    const exchangeRatesStore = useExchangeRatesStore();
    const defaultCurrency = userStore.currentUserDefaultCurrency;

    return services.getAllTransactionsByMonth({
        year: Math.floor(yearMonth / 100),
        month: yearMonth % 100,
        type: income ? TransactionType.Income : TransactionType.Expense,
        categoryIds: categoryId,
        accountIds: '',
        tagFilter: '',
        amountFilter: '',
        keyword: '',
        matchMode: KeywordMatchMode.Default.type
    }).then(response => {
        const data = response.data;

        if (!data || !data.success || !data.result) {
            throw { message: 'Unable to retrieve transaction list' };
        }

        const dailyAmounts: number[] = new Array(getDaysInMonth(yearMonth)).fill(0);
        let detailIncomplete = false;

        for (const transaction of data.result.items) {
            const account = accountsStore.allAccountsMap[transaction.sourceAccountId];

            if (!account) {
                continue;
            }

            let amount = transaction.sourceAmount;

            if (account.currency !== defaultCurrency) {
                const exchanged = exchangeRatesStore.getExchangedAmount(parseBigDecimal(amount), account.currency, defaultCurrency);

                if (!exchanged) {
                    detailIncomplete = true;
                    continue;
                }

                amount = exchanged.truncate().toDoubleNumber();
            }

            const day = new Date(transaction.time * 1000).getDate();

            if (day >= 1 && day <= dailyAmounts.length) {
                dailyAmounts[day - 1] = (dailyAmounts[day - 1] ?? 0) + amount;
            }
        }

        return {
            dailyAmounts: dailyAmounts,
            transactions: data.result.items,
            incomplete: detailIncomplete
        };
    });
}

export function useCategoryBudgetReportBase() {
    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();
    const transactionCategoriesStore = useTransactionCategoriesStore();
    const budgetsStore = useTransactionCategoryBudgetsStore();
    const exchangeRatesStore = useExchangeRatesStore();

    const today = ref<YearMonthDayValue>(getTodayYearMonthDay());
    const currentYearMonth = computed<number>(() => today.value.year * 100 + today.value.month);
    const selectedYearMonth = ref<number>(currentYearMonth.value);
    const categoryType = ref<CategoryType>(CategoryType.Expense);
    const historyMonthCount = ref<number>(CATEGORY_BUDGET_HISTORY_MONTHS_OPTIONS[0] as number);
    const loading = ref<boolean>(false);
    const monthlyStatisticItems = ref<Record<number, TransactionStatisticResponseItem[]>>({});
    const loadedEndYearMonth = ref<number>(0);

    const isIncome = computed<boolean>(() => categoryType.value === CategoryType.Income);
    const defaultCurrency = computed<string>(() => userStore.currentUserDefaultCurrency);
    const isSelectedCurrentMonth = computed<boolean>(() => selectedYearMonth.value === currentYearMonth.value);
    const canShiftToNextMonth = computed<boolean>(() => selectedYearMonth.value < currentYearMonth.value);

    const primaryCategories = computed<TransactionCategory[]>(() => transactionCategoriesStore.allTransactionCategories[categoryType.value] ?? []);

    const hasAnyBudget = computed<boolean>(() => budgetsStore.allBudgets.some(budget => budget.amount > 0));

    const monthlyAmounts = computed<Record<number, PrimaryCategoryAmounts>>(() => {
        const result: Record<number, PrimaryCategoryAmounts> = {};

        for (const [yearMonth, items] of Object.entries(monthlyStatisticItems.value)) {
            result[parseInt(yearMonth, 10)] = sumAmountsByPrimaryCategory(
                items,
                categoryType.value,
                transactionCategoriesStore.allTransactionCategoriesMap,
                accountsStore.allAccountsMap,
                defaultCurrency.value,
                (amount, fromCurrency) => exchangeRatesStore.getExchangedAmount(amount, fromCurrency, defaultCurrency.value)
            );
        }

        return result;
    });

    const report = computed<CategoryBudgetMonthReport>(() => buildCategoryBudgetMonthReport(
        primaryCategories.value,
        budgetsStore.getEffectiveBudgets(selectedYearMonth.value),
        monthlyAmounts.value[selectedYearMonth.value]?.amounts ?? {},
        isIncome.value,
        getBudgetMonthProgress(selectedYearMonth.value, today.value)
    ));

    const historyYearMonths = computed<number[]>(() => {
        const yearMonths: number[] = [];

        for (let i = historyMonthCount.value - 1; i >= 0; i--) {
            yearMonths.push(shiftYearMonth(selectedYearMonth.value, -i));
        }

        return yearMonths;
    });

    const history = computed<CategoryBudgetHistoryRow[]>(() => {
        const amounts: Record<number, Record<string, number>> = {};

        for (const [yearMonth, item] of Object.entries(monthlyAmounts.value)) {
            amounts[parseInt(yearMonth, 10)] = item.amounts;
        }

        return buildCategoryBudgetHistory(primaryCategories.value, budgetsStore.allBudgets, amounts, historyYearMonths.value, isIncome.value, today.value);
    });

    const incomplete = computed<boolean>(() => !!monthlyAmounts.value[selectedYearMonth.value]?.incomplete);

    function loadMonthlyStatistics(force: boolean): Promise<unknown> {
        const endYearMonth = selectedYearMonth.value;
        const startYearMonth = shiftYearMonth(endYearMonth, -(CATEGORY_BUDGET_LOADED_MONTHS - 1));

        if (!force && loadedEndYearMonth.value === endYearMonth) {
            return Promise.resolve();
        }

        return services.getTransactionStatisticsTrends({
            startYearMonth: getTextualYearMonth(startYearMonth),
            endYearMonth: getTextualYearMonth(endYearMonth),
            tagFilter: '',
            keyword: '',
            matchMode: KeywordMatchMode.Default.type,
            useTransactionTimezone: settingsStore.appSettings.statistics.defaultTimezoneType === TimezoneTypeForStatistics.TransactionTimezone.type
        }).then(response => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                throw { message: 'Unable to retrieve transaction statistics' };
            }

            const items: Record<number, TransactionStatisticResponseItem[]> = {};

            for (const monthItem of data.result) {
                items[monthItem.year * 100 + monthItem.month] = monthItem.items;
            }

            monthlyStatisticItems.value = items;
            loadedEndYearMonth.value = endYearMonth;
        });
    }

    function reload(force: boolean): Promise<unknown> {
        loading.value = true;
        today.value = getTodayYearMonthDay();

        return Promise.all([
            accountsStore.loadAllAccounts({ force: false }),
            transactionCategoriesStore.loadAllCategories({ force: false }),
            budgetsStore.loadAllBudgets({ force: force })
        ]).then(() => loadMonthlyStatistics(force)).then(result => {
            loading.value = false;
            return result;
        }).catch(error => {
            loading.value = false;
            throw error;
        });
    }

    function shiftMonth(months: number): Promise<unknown> {
        const target = shiftYearMonth(selectedYearMonth.value, months);

        if (target > currentYearMonth.value) {
            return Promise.resolve();
        }

        selectedYearMonth.value = target;
        return reload(false);
    }

    return {
        // states
        today,
        selectedYearMonth,
        categoryType,
        historyMonthCount,
        loading,
        // computed states
        currentYearMonth,
        isIncome,
        isSelectedCurrentMonth,
        canShiftToNextMonth,
        primaryCategories,
        hasAnyBudget,
        report,
        history,
        historyYearMonths,
        incomplete,
        // functions
        reload,
        shiftMonth,
        loadCategoryDailyDetail: loadCategoryBudgetDailyDetail
    };
}

export const CATEGORY_BUDGET_AVERAGE_MONTHS: number = 3;

// Loads the monthly average of each primary category over the last full months, used as budget suggestions
export function useCategoryBudgetAverageBase() {
    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();
    const transactionCategoriesStore = useTransactionCategoriesStore();
    const exchangeRatesStore = useExchangeRatesStore();

    const averageAmounts = ref<Partial<Record<CategoryType, Record<string, number>>>>({});

    function loadAverageAmounts(): Promise<unknown> {
        const today = getTodayYearMonthDay();
        const currentYearMonth = today.year * 100 + today.month;
        const endYearMonth = shiftYearMonth(currentYearMonth, -1);
        const startYearMonth = shiftYearMonth(currentYearMonth, -CATEGORY_BUDGET_AVERAGE_MONTHS);
        const defaultCurrency = userStore.currentUserDefaultCurrency;

        return services.getTransactionStatisticsTrends({
            startYearMonth: getTextualYearMonth(startYearMonth),
            endYearMonth: getTextualYearMonth(endYearMonth),
            tagFilter: '',
            keyword: '',
            matchMode: KeywordMatchMode.Default.type,
            useTransactionTimezone: settingsStore.appSettings.statistics.defaultTimezoneType === TimezoneTypeForStatistics.TransactionTimezone.type
        }).then(response => {
            const data = response.data;

            if (!data || !data.success || !data.result) {
                throw { message: 'Unable to retrieve transaction statistics' };
            }

            const items: TransactionStatisticResponseItem[] = [];

            for (const monthItem of data.result) {
                items.push(...monthItem.items);
            }

            const result: Partial<Record<CategoryType, Record<string, number>>> = {};

            for (const type of [CategoryType.Expense, CategoryType.Income]) {
                const totals = sumAmountsByPrimaryCategory(
                    items,
                    type,
                    transactionCategoriesStore.allTransactionCategoriesMap,
                    accountsStore.allAccountsMap,
                    defaultCurrency,
                    (amount, fromCurrency) => exchangeRatesStore.getExchangedAmount(amount, fromCurrency, defaultCurrency)
                );
                const averages: Record<string, number> = {};

                for (const [categoryId, total] of Object.entries(totals.amounts)) {
                    averages[categoryId] = Math.round(total / CATEGORY_BUDGET_AVERAGE_MONTHS);
                }

                result[type] = averages;
            }

            averageAmounts.value = result;
        });
    }

    return {
        averageAmounts,
        loadAverageAmounts
    };
}

export interface CategoryBudgetWidgetProps {
    categoryType: CategoryType;
    itemCount: number;
}

// Builds this month's budget report for overview widgets from the overview store's statistics
export function useCategoryBudgetWidgetBase(props: CategoryBudgetWidgetProps) {
    const settingsStore = useSettingsStore();
    const userStore = useUserStore();
    const accountsStore = useAccountsStore();
    const transactionCategoriesStore = useTransactionCategoriesStore();
    const budgetsStore = useTransactionCategoryBudgetsStore();
    const exchangeRatesStore = useExchangeRatesStore();
    const overviewStore = useOverviewStore();

    const isIncome = computed<boolean>(() => props.categoryType === CategoryType.Income);
    const showAmount = computed<boolean>(() => settingsStore.appSettings.showAmountInHomePage);

    const report = computed<CategoryBudgetMonthReport>(() => {
        const today = getTodayYearMonthDay();
        const yearMonth = today.year * 100 + today.month;
        const defaultCurrency = userStore.currentUserDefaultCurrency;
        const response = overviewStore.transactionCategoryStatisticsData[DateRange.ThisMonth.type];
        const amounts = sumAmountsByPrimaryCategory(
            response?.items ?? [],
            props.categoryType,
            transactionCategoriesStore.allTransactionCategoriesMap,
            accountsStore.allAccountsMap,
            defaultCurrency,
            (amount, fromCurrency) => exchangeRatesStore.getExchangedAmount(amount, fromCurrency, defaultCurrency)
        );

        return buildCategoryBudgetMonthReport(
            transactionCategoriesStore.allTransactionCategories[props.categoryType] ?? [],
            budgetsStore.getEffectiveBudgets(yearMonth),
            amounts.amounts,
            isIncome.value,
            getBudgetMonthProgress(yearMonth, today)
        );
    });

    const attentionCount = computed<number>(() => report.value.rows.filter(row => row.progress.level === 'bad' || row.progress.level === 'warning').length);
    const displayRows = computed(() => report.value.rows.slice(0, props.itemCount));

    return {
        isIncome,
        showAmount,
        report,
        attentionCount,
        displayRows
    };
}
