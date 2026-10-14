<template>
    <v-chart autoresize class="category-budget-progress-chart" :option="chartOptions" :update-options="{ notMerge: true }"/>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useTheme } from 'vuetify';

import { useCategoryBudgetDisplay } from '@/views/base/statistics/CategoryBudgetReportBase.ts';

import type { CategoryBudgetStatusLevel } from '@/lib/category_budget.ts';
import { CATEGORY_BUDGET_MIN_PROJECTION_DAYS } from '@/lib/category_budget.ts';
import { CategoryBudgetPaceType } from '@/models/transaction_category_budget.ts';

const props = defineProps<{
    dailyAmounts: number[];
    budget: number;
    paceType: CategoryBudgetPaceType;
    elapsedDays: number;
    projected: number;
    level: CategoryBudgetStatusLevel;
    showProjection: boolean;
}>();

const theme = useTheme();
const { formatBudgetAmount } = useCategoryBudgetDisplay();

const LEVEL_COLORS: Record<CategoryBudgetStatusLevel, string> = {
    good: '#34c759',
    neutral: '#c67e48',
    warning: '#ff9500',
    bad: '#ff3b30'
};

const chartOptions = computed<object>(() => {
    const colors = theme.current.value.colors;
    const textColor = colors['on-surface'] as string;
    const lineColor = LEVEL_COLORS[props.level];
    const totalDays = props.dailyAmounts.length;
    const days: string[] = [];
    const actual: (number | null)[] = [];
    const pace: number[] = [];
    const budget: number[] = [];
    const projection: (number | null)[] = [];
    let cumulative = 0;

    for (let i = 0; i < totalDays; i++) {
        cumulative += props.dailyAmounts[i] ?? 0;
        days.push((i + 1).toString());
        actual.push(i < props.elapsedDays ? cumulative : null);
        pace.push(props.budget * (i + 1) / totalDays);
        budget.push(props.budget);
    }

    const canProject = props.showProjection && props.paceType === CategoryBudgetPaceType.Even && props.elapsedDays >= CATEGORY_BUDGET_MIN_PROJECTION_DAYS && props.elapsedDays < totalDays;

    if (canProject) {
        const start = actual[props.elapsedDays - 1] ?? 0;

        for (let i = 0; i < totalDays; i++) {
            if (i < props.elapsedDays - 1) {
                projection.push(null);
            } else {
                projection.push(start + (props.projected - start) * (i - props.elapsedDays + 1) / (totalDays - props.elapsedDays));
            }
        }
    }

    const series: object[] = [
        {
            name: 'Budget',
            type: 'line',
            data: budget,
            symbol: 'none',
            silent: true,
            lineStyle: { color: textColor, width: 1.5, opacity: 0.7 },
            itemStyle: { color: textColor }
        },
        {
            name: 'Actual',
            type: 'line',
            data: actual,
            symbol: 'none',
            lineStyle: { color: lineColor, width: 2.5 },
            itemStyle: { color: lineColor },
            areaStyle: { color: lineColor, opacity: 0.12 }
        }
    ];

    if (props.paceType === CategoryBudgetPaceType.Even) {
        series.push({
            name: 'Even pace',
            type: 'line',
            data: pace,
            symbol: 'none',
            lineStyle: { color: textColor, width: 1, type: 'dashed', opacity: 0.45 },
            itemStyle: { color: textColor }
        });
    }

    if (canProject) {
        series.push({
            name: 'Projected',
            type: 'line',
            data: projection,
            symbol: 'none',
            lineStyle: { color: lineColor, width: 2, type: 'dotted' },
            itemStyle: { color: lineColor }
        });
    }

    return {
        animation: false,
        grid: { left: 8, right: 16, top: 36, bottom: 8, containLabel: true },
        legend: {
            top: 0,
            textStyle: { color: textColor }
        },
        tooltip: {
            trigger: 'axis',
            valueFormatter: (value: number | null) => value === null || value === undefined ? '-' : formatBudgetAmount(value)
        },
        xAxis: {
            type: 'category',
            boundaryGap: false,
            data: days,
            axisLabel: { color: textColor, interval: 6 },
            axisLine: { lineStyle: { color: textColor, opacity: 0.3 } }
        },
        yAxis: {
            type: 'value',
            axisLabel: { color: textColor, formatter: (value: number) => formatBudgetAmount(value) },
            splitLine: { lineStyle: { color: textColor, opacity: 0.08 } }
        },
        series: series
    };
});
</script>

<style>
.category-budget-progress-chart {
    width: 100%;
    height: 300px;
}
</style>
