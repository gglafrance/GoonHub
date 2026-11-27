<script setup lang="ts">
// File size filter (bounds stored in megabytes; inputs in MB or GB).
const searchStore = useSearchContext();

const collapsed = ref(true);
const unit = ref<'MB' | 'GB'>('GB');

const presets = [
    { label: '< 100 MB', min: 0, max: 100 },
    { label: '100 MB - 1 GB', min: 100, max: 1024 },
    { label: '1 - 5 GB', min: 1024, max: 5120 },
    { label: '5 GB +', min: 5120, max: 0 },
];

const isPreset = (p: { min: number; max: number }) =>
    searchStore.minSize === p.min && searchStore.maxSize === p.max;

const applyPreset = (p: { min: number; max: number }) => {
    if (isPreset(p)) {
        searchStore.minSize = 0;
        searchStore.maxSize = 0;
        return;
    }
    searchStore.minSize = p.min;
    searchStore.maxSize = p.max;
};

const factor = computed(() => (unit.value === 'GB' ? 1024 : 1));

const toInput = (mb: number) => (mb ? +(mb / factor.value).toFixed(2) : '');

const fromInput = (event: Event) => {
    const value = (event.target as HTMLInputElement).value;
    return value ? Math.max(0, Math.round(Number(value) * factor.value)) : 0;
};

const formatMB = (mb: number) => (mb >= 1024 ? `${+(mb / 1024).toFixed(1)} GB` : `${mb} MB`);

const badge = computed(() => {
    const { minSize, maxSize } = searchStore;
    if (!minSize && !maxSize) return undefined;
    if (minSize && maxSize) return `${formatMB(minSize)}-${formatMB(maxSize)}`;
    if (minSize) return `${formatMB(minSize)}+`;
    return `<${formatMB(maxSize)}`;
});
</script>

<template>
    <SearchFiltersFilterSection
        title="File Size"
        icon="heroicons:circle-stack"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="grid grid-cols-2 gap-1.5">
            <button
                v-for="preset in presets"
                :key="preset.label"
                class="rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors"
                :class="
                    isPreset(preset)
                        ? 'bg-lava/10 border-lava/30 border text-white'
                        : `border-border bg-surface text-dim border hover:border-white/20
                            hover:text-white`
                "
                @click="applyPreset(preset)"
            >
                {{ preset.label }}
            </button>
        </div>
        <div class="mt-2 flex items-center gap-1.5">
            <input
                :value="toInput(searchStore.minSize)"
                type="number"
                min="0"
                step="any"
                placeholder="Min"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                    text-xs focus:border-white/20 focus:outline-none"
                @change="searchStore.minSize = fromInput($event)"
            />
            <span class="text-dim text-[10px]">-</span>
            <input
                :value="toInput(searchStore.maxSize)"
                type="number"
                min="0"
                step="any"
                placeholder="Max"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                    text-xs focus:border-white/20 focus:outline-none"
                @change="searchStore.maxSize = fromInput($event)"
            />
            <button
                class="border-border text-dim shrink-0 rounded-md border px-1.5 py-1 text-[10px]
                    transition-colors hover:text-white"
                title="Switch unit"
                @click="unit = unit === 'GB' ? 'MB' : 'GB'"
            >
                {{ unit }}
            </button>
        </div>
    </SearchFiltersFilterSection>
</template>
