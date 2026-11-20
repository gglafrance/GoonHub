<script setup lang="ts">
// Shared config fields of the marker list widgets: labels, ranges only and (optionally) order.
const props = defineProps<{
    withOrder?: boolean;
}>();

const config = defineModel<Record<string, unknown>>({ required: true });

const labels = computed({
    get: () => (Array.isArray(config.value.labels) ? (config.value.labels as string[]) : []),
    set: (v: string[]) => (config.value = { ...config.value, labels: v }),
});

const rangesOnly = computed({
    get: () => config.value.ranges_only === true,
    set: (v: boolean) => (config.value = { ...config.value, ranges_only: v }),
});

const order = computed({
    get: () => (config.value.order === 'recent' ? 'recent' : 'random'),
    set: (v: string) => (config.value = { ...config.value, order: v }),
});

const orderOptions = [
    { value: 'random', label: 'Random' },
    { value: 'recent', label: 'Most recent' },
];
</script>

<template>
    <div class="space-y-3">
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Labels
            </label>
            <SettingsHomepageWidgetConfigMarkersLabelPicker v-model="labels" source="labels" />
        </div>
        <div class="flex items-center justify-between">
            <div>
                <span class="block text-xs font-medium text-white">With end only</span>
                <span class="text-dim text-[11px]">Only markers that have an end time</span>
            </div>
            <UiToggle v-model="rangesOnly" />
        </div>
        <div v-if="props.withOrder">
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Order
            </label>
            <div class="flex gap-1.5">
                <button
                    v-for="option in orderOptions"
                    :key="option.value"
                    class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                    :class="
                        order === option.value
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    @click="order = option.value"
                >
                    {{ option.label }}
                </button>
            </div>
        </div>
    </div>
</template>
