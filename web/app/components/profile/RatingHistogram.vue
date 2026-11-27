<script setup lang="ts">
import type { ProfileRatingBucket } from '~/types/profile';

// Ratings chart of a profile (Letterboxd style): one bar per half star from 0.5 (one star on
// the left) to 5 (five stars on the right), heights scaled on the fullest bar, no axis. A bar
// with ratings emits `select` (the ratings list of that value); `selected` highlights one.
const props = defineProps<{
    buckets: ProfileRatingBucket[];
    total: number;
    average?: number | null;
    selected?: number | null;
}>();

const emit = defineEmits<{ select: [rating: number] }>();

const hovered = ref<number | null>(null);

const bars = computed(() => {
    const filled = fillRatingBuckets(props.buckets);
    const heights = barHeights(filled);
    const last = filled.length - 1;
    return filled.map((b, i) => ({
        ...b,
        height: heights[i] ?? 0,
        tooltip: bucketTooltip(b, props.total),
        // Tooltips of the end bars open inwards so that they stay inside the panel
        align: i < 2 ? 'left-0' : i > last - 2 ? 'right-0' : 'left-1/2 -translate-x-1/2',
    }));
});

const average = computed(() => formatAverage(props.average));

function onClick(bar: { rating: number; count: number }) {
    if (bar.count > 0) emit('select', bar.rating);
}
</script>

<template>
    <div>
        <div class="flex items-end gap-2">
            <span class="text-lava pb-px text-[10px] leading-none" aria-hidden="true">★</span>
            <div
                class="relative flex h-16 flex-1 items-end gap-px"
                role="group"
                aria-label="Ratings chart"
                @mouseleave="hovered = null"
            >
                <button
                    v-for="bar in bars"
                    :key="bar.rating"
                    type="button"
                    class="relative flex h-full flex-1 items-end focus:outline-none"
                    :class="bar.count > 0 ? 'cursor-pointer' : 'cursor-default'"
                    :aria-label="bar.tooltip"
                    :aria-pressed="selected === bar.rating"
                    :aria-disabled="bar.count === 0"
                    @mouseenter="hovered = bar.rating"
                    @focus="hovered = bar.rating"
                    @blur="hovered = null"
                    @click="onClick(bar)"
                >
                    <span
                        class="w-full rounded-t-[2px] transition-colors duration-100"
                        :class="
                            selected === bar.rating
                                ? 'bg-lava shadow-[0_0_10px_rgba(255,77,77,0.45)]'
                                : hovered === bar.rating && bar.count > 0
                                  ? 'bg-lava/75'
                                  : selected != null
                                    ? 'bg-white/10'
                                    : 'bg-white/[0.16]'
                        "
                        :style="{ height: bar.count > 0 ? `${bar.height}%` : '1px' }"
                    />
                    <span
                        v-if="hovered === bar.rating"
                        class="pointer-events-none absolute bottom-full z-10 mb-1.5 rounded-md
                            border border-white/10 bg-[#0b0b0b]/95 px-2 py-1 text-[10px]
                            whitespace-nowrap text-white backdrop-blur"
                        :class="bar.align"
                    >
                        {{ bar.tooltip }}
                    </span>
                </button>
            </div>
            <span
                class="text-lava pb-px text-[10px] leading-none tracking-[-0.05em]"
                aria-hidden="true"
                >★★★★★</span
            >
        </div>
        <div v-if="average" class="text-dim mt-1.5 text-right text-[10px]">
            Average <span class="font-mono text-white">{{ average }}</span>
            <span class="text-lava">★</span>
        </div>
    </div>
</template>
