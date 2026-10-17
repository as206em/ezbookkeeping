<template>
    <v-chart autoresize class="asset-history-chart" :option="chartOptions" :update-options="{ notMerge: true }"/>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useTheme } from 'vuetify';

import type { AssetHistoryPoint } from '@/lib/asset.ts';

const props = defineProps<{
    points: AssetHistoryPoint[];
    formatAmount: (amount: number) => string;
}>();

const theme = useTheme();

const chartOptions = computed<object>(() => {
    const colors = theme.current.value.colors;
    const textColor = colors['on-surface'] as string;
    const valueColor = colors['primary'] as string;

    return {
        animation: false,
        grid: { left: 8, right: 16, top: 36, bottom: 8, containLabel: true },
        legend: {
            top: 0,
            textStyle: { color: textColor }
        },
        tooltip: {
            trigger: 'axis',
            valueFormatter: (value: number | null) => value === null || value === undefined ? '-' : props.formatAmount(value)
        },
        xAxis: {
            type: 'category',
            boundaryGap: false,
            data: props.points.map(point => point.date),
            axisLabel: { color: textColor },
            axisLine: { lineStyle: { color: textColor, opacity: 0.3 } }
        },
        yAxis: {
            type: 'value',
            scale: true,
            axisLabel: { color: textColor, formatter: (value: number) => props.formatAmount(value) },
            splitLine: { lineStyle: { color: textColor, opacity: 0.1 } }
        },
        series: [
            {
                name: 'Value',
                type: 'line',
                data: props.points.map(point => Math.round(point.value)),
                symbol: props.points.length < 15 ? 'circle' : 'none',
                symbolSize: 6,
                lineStyle: { color: valueColor, width: 2.5 },
                itemStyle: { color: valueColor },
                areaStyle: { color: valueColor, opacity: 0.12 }
            },
            {
                name: 'Invested',
                type: 'line',
                data: props.points.map(point => Math.round(point.costBasis)),
                symbol: 'none',
                lineStyle: { color: textColor, width: 1.5, type: 'dashed', opacity: 0.55 },
                itemStyle: { color: textColor }
            }
        ]
    };
});
</script>

<style>
.asset-history-chart {
    width: 100%;
    height: 260px;
}
</style>
