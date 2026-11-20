<script setup lang="ts">
// Config form of the browse_tags widget.
const config = defineModel<Record<string, unknown>>({ required: true });

const source = computed({
    get: () => (config.value.source === 'tags' ? 'tags' : 'labels'),
    set: (v: 'labels' | 'tags') => (config.value = { ...config.value, source: v, items: [] }),
});

const items = computed({
    get: () => (Array.isArray(config.value.items) ? (config.value.items as string[]) : []),
    set: (v: string[]) => (config.value = { ...config.value, items: v }),
});

const sourceOptions = [
    { value: 'labels', label: 'Marker labels' },
    { value: 'tags', label: 'Scene tags' },
] as const;
</script>

<template>
    <div class="space-y-3">
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Source
            </label>
            <div class="flex gap-1.5">
                <button
                    v-for="option in sourceOptions"
                    :key="option.value"
                    class="rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all"
                    :class="
                        source === option.value
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:text-white'
                    "
                    @click="source = option.value"
                >
                    {{ option.label }}
                </button>
            </div>
        </div>
        <div>
            <label class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase">
                Tiles
            </label>
            <SettingsHomepageWidgetConfigMarkersLabelPicker
                v-model="items"
                :source="source"
                placeholder="Pick tiles in order, or leave empty for the most used"
            />
        </div>
    </div>
</template>
