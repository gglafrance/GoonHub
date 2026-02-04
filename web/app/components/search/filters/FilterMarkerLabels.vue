<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);
const searchQuery = ref('');

const toggleLabel = (label: string) => {
    const idx = searchStore.selectedMarkerLabels.indexOf(label);
    if (idx >= 0) {
        searchStore.selectedMarkerLabels.splice(idx, 1);
    } else {
        searchStore.selectedMarkerLabels.push(label);
    }
};

const filteredLabels = computed(() => {
    if (!searchQuery.value) return searchStore.filterOptions.marker_labels;
    const q = searchQuery.value.toLowerCase();
    return searchStore.filterOptions.marker_labels.filter((l) => l.label.toLowerCase().includes(q));
});

const badge = computed(() =>
    searchStore.selectedMarkerLabels.length > 0
        ? searchStore.selectedMarkerLabels.length
        : undefined,
);
</script>

<template>
    <SearchFiltersFilterSection
        v-if="searchStore.filterOptions.marker_labels.length > 0"
        title="Marker Labels"
        icon="heroicons:bookmark"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <input
            v-model="searchQuery"
            type="text"
            placeholder="Search labels..."
            class="border-border bg-surface text-dim mb-2 w-full rounded-md border px-2 py-1.5
                text-xs placeholder-white/30 focus:border-white/20 focus:outline-none"
        />
        <div v-if="filteredLabels.length === 0" class="text-dim px-2 py-1 text-xs">
            No matching labels
        </div>
        <div v-else class="max-h-40 space-y-1 overflow-y-auto">
            <button
                v-for="item in filteredLabels"
                :key="item.label"
                class="flex w-full items-center gap-2 rounded px-2 py-1 text-left text-xs
                    transition-colors"
                :class="
                    searchStore.selectedMarkerLabels.includes(item.label)
                        ? 'bg-lava/10 text-white'
                        : 'text-dim hover:bg-white/5 hover:text-white'
                "
                @click="toggleLabel(item.label)"
            >
                <Icon name="heroicons:bookmark" size="12" class="shrink-0 opacity-50" />
                <span class="flex-1 truncate">{{ item.label }}</span>
                <span class="text-[10px] opacity-50">{{ item.count }}</span>
            </button>
        </div>
    </SearchFiltersFilterSection>
</template>
