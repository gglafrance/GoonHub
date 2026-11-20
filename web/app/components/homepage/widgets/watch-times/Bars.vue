<script setup lang="ts">
// Single-series bar chart of the watch_times widget (one hue, peak highlighted and labeled).
interface Bar {
    key: string;
    label: string;
    tooltip: string;
    count: number;
    peak: boolean;
}

const props = defineProps<{
    title: string;
    bars: Bar[];
}>();

const number = new Intl.NumberFormat();
const max = computed(() => Math.max(1, ...props.bars.map((b) => b.count)));
const hovered = ref<string | null>(null);
</script>

<template>
    <figure>
        <figcaption class="text-dim mb-2 text-[10px] font-medium uppercase">{{ title }}</figcaption>
        <div
            class="flex h-28 items-end gap-0.5 border-b border-white/10 pt-4"
            role="list"
            :aria-label="title"
            @mouseleave="hovered = null"
        >
            <div
                v-for="bar in bars"
                :key="bar.key"
                role="listitem"
                :aria-label="bar.tooltip"
                class="group relative flex h-full flex-1 items-end"
                @mouseenter="hovered = bar.key"
            >
                <div
                    class="w-full rounded-t transition-colors"
                    :class="
                        bar.peak ? 'bg-lava' : hovered === bar.key ? 'bg-lava/70' : 'bg-lava/40'
                    "
                    :style="{
                        height: `${Math.max(bar.count > 0 ? 2 : 0, (bar.count / max) * 100)}%`,
                    }"
                />
                <!-- Direct label on the peak only -->
                <span
                    v-if="bar.peak"
                    class="absolute -top-4 left-1/2 -translate-x-1/2 font-mono text-[9px]
                        whitespace-nowrap text-white"
                >
                    {{ number.format(bar.count) }}
                </span>
                <!-- Hover tooltip -->
                <span
                    v-if="hovered === bar.key"
                    class="border-border bg-panel/95 pointer-events-none absolute bottom-full
                        left-1/2 z-10 mb-5 -translate-x-1/2 rounded-md border px-2 py-1 text-[10px]
                        whitespace-nowrap text-white shadow-lg"
                >
                    {{ bar.tooltip }}
                </span>
            </div>
        </div>
        <div class="mt-1 flex gap-0.5" aria-hidden="true">
            <span
                v-for="bar in bars"
                :key="bar.key"
                class="text-dim flex-1 text-center text-[9px] whitespace-nowrap"
            >
                {{ bar.label }}
            </span>
        </div>
    </figure>
</template>
