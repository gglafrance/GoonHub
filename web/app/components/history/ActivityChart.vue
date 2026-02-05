<script setup lang="ts">
import type { ChartActivityCount } from '~/types/watch';

const props = defineProps<{
    counts: ChartActivityCount[];
    rangeDays: number;
    isLoading: boolean;
}>();

const emit = defineEmits<{
    barClick: [dateKey: string];
}>();

const containerRef = ref<HTMLDivElement | null>(null);
const containerWidth = ref(600);
const hoveredIndex = ref<number | null>(null);
const tooltipX = ref(0);
const tooltipY = ref(0);

const chartHeight = 120;
const chartPadding = { top: 8, bottom: 4, left: 0, right: 0 };
const innerHeight = chartHeight - chartPadding.top - chartPadding.bottom;

const toLocalDateKey = (d: Date) =>
    d.getFullYear() +
    '-' +
    String(d.getMonth() + 1).padStart(2, '0') +
    '-' +
    String(d.getDate()).padStart(2, '0');

// Fill gaps: create a continuous day range with zero-count days filled in
const filledCounts = computed(() => {
    if (props.counts.length === 0) return [];

    const countMap = new Map<string, number>();
    for (const c of props.counts) {
        countMap.set(c.dateKey, c.count);
    }

    const days: { dateKey: string; count: number }[] = [];
    const effectiveRange = props.rangeDays > 0 ? props.rangeDays : 365;
    const now = new Date();
    const start = new Date(now);
    start.setDate(start.getDate() - effectiveRange + 1);

    for (let d = new Date(start); d <= now; d.setDate(d.getDate() + 1)) {
        const key = toLocalDateKey(d);
        days.push({ dateKey: key, count: countMap.get(key) || 0 });
    }

    return days;
});

const maxCount = computed(() => {
    let max = 0;
    for (const d of filledCounts.value) {
        if (d.count > max) max = d.count;
    }
    return max || 1;
});

const barData = computed(() => {
    const data = filledCounts.value;
    if (data.length === 0) return [];

    const w = containerWidth.value - chartPadding.left - chartPadding.right;
    const gap = 1;
    const barWidth = Math.max(1, (w - gap * (data.length - 1)) / data.length);

    return data.map((d, i) => {
        const barHeight = (d.count / maxCount.value) * innerHeight;
        return {
            x: chartPadding.left + i * (barWidth + gap),
            y: chartPadding.top + innerHeight - barHeight,
            width: barWidth,
            height: Math.max(d.count > 0 ? 2 : 0, barHeight),
            dateKey: d.dateKey,
            count: d.count,
        };
    });
});

const hoveredBar = computed(() => {
    if (hoveredIndex.value === null) return null;
    return barData.value[hoveredIndex.value] || null;
});

const formatTooltipDate = (dateKey: string) => {
    const d = new Date(dateKey + 'T00:00:00');
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
};

const handleBarHover = (index: number, event: MouseEvent) => {
    hoveredIndex.value = index;
    if (containerRef.value) {
        const rect = containerRef.value.getBoundingClientRect();
        tooltipX.value = event.clientX - rect.left;
        tooltipY.value = event.clientY - rect.top;
    }
};

const handleBarLeave = () => {
    hoveredIndex.value = null;
};

const handleBarClick = (dateKey: string) => {
    emit('barClick', dateKey);
};

// ResizeObserver for responsive width
onMounted(() => {
    if (!containerRef.value) return;
    containerWidth.value = containerRef.value.clientWidth;

    const observer = new ResizeObserver((entries) => {
        for (const entry of entries) {
            containerWidth.value = entry.contentRect.width;
        }
    });
    observer.observe(containerRef.value);

    onUnmounted(() => observer.disconnect());
});
</script>

<template>
    <div ref="containerRef" class="relative w-full">
        <!-- Loading overlay -->
        <div
            v-if="isLoading"
            class="flex items-center justify-center"
            :style="{ height: chartHeight + 'px' }"
        >
            <div class="bg-surface/80 border-border rounded-lg border px-3 py-1.5">
                <span class="text-dim text-xs">Loading activity...</span>
            </div>
        </div>

        <!-- Empty state -->
        <div
            v-else-if="filledCounts.length === 0"
            class="border-border flex items-center justify-center rounded-lg border border-dashed"
            :style="{ height: chartHeight + 'px' }"
        >
            <span class="text-dim text-xs">No activity data</span>
        </div>

        <!-- Chart -->
        <svg v-else :width="containerWidth" :height="chartHeight" class="block">
            <rect
                v-for="(bar, i) in barData"
                :key="bar.dateKey"
                :x="bar.x"
                :y="bar.y"
                :width="bar.width"
                :height="bar.height"
                :rx="Math.min(1.5, bar.width / 2)"
                class="cursor-pointer transition-opacity duration-75"
                :fill="
                    hoveredIndex === i
                        ? '#FF6B6B'
                        : bar.count > 0
                          ? '#FF4D4D'
                          : 'rgba(255,255,255,0.04)'
                "
                :opacity="hoveredIndex !== null && hoveredIndex !== i ? 0.5 : 1"
                @mouseenter="handleBarHover(i, $event)"
                @mouseleave="handleBarLeave"
                @click="handleBarClick(bar.dateKey)"
            />
        </svg>

        <!-- Tooltip -->
        <div
            v-if="hoveredBar"
            class="pointer-events-none absolute z-50"
            :style="{
                left: Math.min(tooltipX, containerWidth - 120) + 'px',
                top: Math.max(0, tooltipY - 40) + 'px',
            }"
        >
            <div class="bg-panel border-border rounded-md border px-2 py-1 shadow-lg">
                <p class="text-[10px] font-medium text-white">
                    {{ hoveredBar.count }} scene{{ hoveredBar.count !== 1 ? 's' : '' }}
                </p>
                <p class="text-dim text-[9px]">{{ formatTooltipDate(hoveredBar.dateKey) }}</p>
            </div>
        </div>
    </div>
</template>
