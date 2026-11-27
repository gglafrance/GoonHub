<script setup lang="ts">
// Segmented toggle between the views and O series of a stats widget.
type StatsMetric = 'views' | 'jizz';

defineProps<{
    options?: StatsMetric[];
}>();

const model = defineModel<StatsMetric>({ required: true });

const LABELS: Record<StatsMetric, string> = { views: 'Views', jizz: 'O' };
</script>

<template>
    <div class="border-border inline-flex rounded-md border p-0.5" role="group" aria-label="Metric">
        <button
            v-for="option in options ?? ['views', 'jizz']"
            :key="option"
            type="button"
            class="rounded px-2 py-0.5 text-[10px] font-medium transition-colors"
            :class="model === option ? 'bg-lava/15 text-lava' : 'text-dim hover:text-white'"
            :aria-pressed="model === option"
            @click="model = option"
        >
            {{ LABELS[option] }}
        </button>
    </div>
</template>
