<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);

const durationPresets = [
    { label: '5+ min', min: 300 },
    { label: '10+ min', min: 600 },
    { label: '20+ min', min: 1200 },
    { label: '30+ min', min: 1800 },
    { label: '60+ min', min: 3600 },
];

const setDuration = (min: number) => {
    if (searchStore.minDuration === min) {
        searchStore.minDuration = 0;
    } else {
        searchStore.minDuration = min;
    }
    searchStore.maxDuration = 0;
};

const badge = computed(() => {
    if (!searchStore.minDuration && !searchStore.maxDuration) return undefined;
    if (searchStore.minDuration && searchStore.maxDuration) {
        return `${Math.floor(searchStore.minDuration / 60)}-${Math.floor(searchStore.maxDuration / 60)}m`;
    }
    if (searchStore.minDuration) {
        return `${Math.floor(searchStore.minDuration / 60)}+ min`;
    }
    return `<${Math.floor(searchStore.maxDuration / 60)}m`;
});
</script>

<template>
    <SearchFiltersFilterSection
        title="Duration"
        icon="heroicons:clock"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="grid grid-cols-2 gap-1.5">
            <button
                v-for="preset in durationPresets"
                :key="preset.label"
                @click="setDuration(preset.min)"
                class="rounded-md px-2 py-1.5 text-[11px] font-medium transition-colors"
                :class="
                    searchStore.minDuration === preset.min && !searchStore.maxDuration
                        ? 'bg-lava/10 border-lava/30 border text-white'
                        : `border-border bg-surface text-dim border hover:border-white/20
                            hover:text-white`
                "
            >
                {{ preset.label }}
            </button>
        </div>
        <div class="mt-2 flex items-center gap-1.5">
            <input
                :value="searchStore.minDuration ? Math.floor(searchStore.minDuration / 60) : ''"
                @input="
                    searchStore.minDuration = ($event.target as HTMLInputElement).value
                        ? Number(($event.target as HTMLInputElement).value) * 60
                        : 0
                "
                type="number"
                min="0"
                placeholder="Min"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                    text-xs focus:border-white/20 focus:outline-none"
            />
            <span class="text-dim text-[10px]">-</span>
            <input
                :value="searchStore.maxDuration ? Math.floor(searchStore.maxDuration / 60) : ''"
                @input="
                    searchStore.maxDuration = ($event.target as HTMLInputElement).value
                        ? Number(($event.target as HTMLInputElement).value) * 60
                        : 0
                "
                type="number"
                min="0"
                placeholder="Max"
                class="border-border bg-surface text-dim w-full rounded-md border px-2 py-1.5
                    text-xs focus:border-white/20 focus:outline-none"
            />
            <span class="text-dim shrink-0 text-[10px]">min</span>
        </div>
    </SearchFiltersFilterSection>
</template>
