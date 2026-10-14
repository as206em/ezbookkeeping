<template>
    <div class="category-budget-bar" :style="{ height: `${height}px` }" role="img" :aria-label="ariaLabel">
        <div class="category-budget-bar-over-zone" :style="{ left: `${budgetPosition}%` }" v-if="!progress.isIncome"></div>
        <div class="category-budget-bar-projected" :style="{ width: `${projectedPosition}%` }" v-if="showProjection"></div>
        <div class="category-budget-bar-fill" :class="`category-budget-bar-${progress.level}`" :style="{ width: `${actualPosition}%` }"></div>
        <div class="category-budget-bar-tick category-budget-bar-pace" :style="{ left: `${pacePosition}%` }" v-if="showPace && pacePosition !== null"></div>
        <div class="category-budget-bar-tick category-budget-bar-budget" :style="{ left: `${budgetPosition}%` }"></div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import type { CategoryBudgetProgress } from '@/lib/category_budget.ts';

// the bar spans 0% to 130% of the budget, so overspending stays visible
const BAR_SCALE = 1.3;

const props = withDefaults(defineProps<{
    progress: CategoryBudgetProgress;
    height?: number;
    showPace?: boolean;
    showProjection?: boolean;
}>(), {
    height: 12,
    showPace: true,
    showProjection: true
});

function getPosition(ratio: number): number {
    return Math.min(Math.max(ratio, 0), BAR_SCALE) / BAR_SCALE * 100;
}

const budgetPosition = computed<number>(() => getPosition(1));
const actualPosition = computed<number>(() => getPosition(props.progress.usedRatio));
const projectedPosition = computed<number>(() => getPosition(props.progress.projectedRatio));
const pacePosition = computed<number | null>(() => props.progress.expectedRatio === null ? null : getPosition(props.progress.expectedRatio));
const ariaLabel = computed<string>(() => `${Math.round(props.progress.usedRatio * 100)}% of budget used`);
</script>

<style>
.category-budget-bar {
    position: relative;
    width: 100%;
    border-radius: 6px;
    background-color: rgba(127, 127, 127, 0.16);
}

.category-budget-bar-over-zone {
    position: absolute;
    top: 0;
    bottom: 0;
    right: 0;
    border-radius: 0 6px 6px 0;
    background: repeating-linear-gradient(135deg, transparent 0 4px, rgba(255, 59, 48, 0.16) 4px 6px);
}

.category-budget-bar-projected,
.category-budget-bar-fill {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    border-radius: 6px;
    transition: width 0.25s ease;
}

.category-budget-bar-projected {
    background-color: rgba(127, 127, 127, 0.3);
}

.category-budget-bar-good {
    background-color: #34c759;
}

.category-budget-bar-neutral {
    background-color: #c67e48;
}

.category-budget-bar-warning {
    background-color: #ff9500;
}

.category-budget-bar-bad {
    background-color: #ff3b30;
}

.category-budget-bar-tick {
    position: absolute;
    width: 2px;
    border-radius: 1px;
    transform: translateX(-1px);
    background-color: currentColor;
}

.category-budget-bar-budget {
    top: -4px;
    bottom: -4px;
}

.category-budget-bar-pace {
    top: -2px;
    bottom: -2px;
    opacity: 0.4;
}

@media (prefers-reduced-motion: reduce) {
    .category-budget-bar-projected,
    .category-budget-bar-fill {
        transition: none;
    }
}
</style>
