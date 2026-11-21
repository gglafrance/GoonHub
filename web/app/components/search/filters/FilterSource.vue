<script setup lang="ts">
// Source filter: the sites scenes come from (multi-select, OR) and source presence.
const searchStore = useSearchStore();

const collapsed = ref(true);
const searchQuery = ref('');

const presenceOptions: { value: boolean | null; label: string }[] = [
    { value: null, label: 'Any' },
    { value: true, label: 'Has a source' },
    { value: false, label: 'No source' },
];

const setPresence = (value: boolean | null) => {
    searchStore.status.has_source = value;
    // Scenes without a source have no site: drop the selected sites
    if (value === false) searchStore.selectedSourceSites = [];
};

const toggleSite = (site: string) => {
    const idx = searchStore.selectedSourceSites.indexOf(site);
    if (idx >= 0) {
        searchStore.selectedSourceSites.splice(idx, 1);
    } else {
        searchStore.selectedSourceSites.push(site);
        if (searchStore.status.has_source === false) searchStore.status.has_source = null;
    }
};

// Known sites, plus selected ones the list does not have (e.g. from a shared URL)
const sites = computed(() => {
    const known = searchStore.sourceSites;
    const extra = searchStore.selectedSourceSites
        .filter((s) => !known.some((k) => k.site === s))
        .map((site) => ({ site, count: 0 }));
    return [...extra, ...known];
});

const filteredSites = computed(() => {
    if (!searchQuery.value) return sites.value;
    const q = searchQuery.value.toLowerCase();
    return sites.value.filter((s) => s.site.includes(q));
});

const badge = computed(() => {
    const count =
        searchStore.selectedSourceSites.length + (searchStore.status.has_source !== null ? 1 : 0);
    return count > 0 ? count : undefined;
});
</script>

<template>
    <SearchFiltersFilterSection
        title="Source"
        icon="heroicons:globe-alt"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="border-border mb-2 flex rounded-md border p-0.5">
            <button
                v-for="opt in presenceOptions"
                :key="String(opt.value)"
                class="flex-1 rounded px-1.5 py-0.5 text-[10px] font-medium transition-colors"
                :class="
                    searchStore.status.has_source === opt.value
                        ? 'bg-lava/15 text-lava'
                        : 'text-dim hover:text-white'
                "
                @click="setPresence(opt.value)"
            >
                {{ opt.label }}
            </button>
        </div>

        <template v-if="sites.length > 0">
            <input
                v-if="sites.length > 8"
                v-model="searchQuery"
                type="text"
                placeholder="Search sites..."
                class="border-border bg-surface text-dim mb-2 w-full rounded-md border px-2 py-1.5
                    text-xs placeholder-white/30 focus:border-white/20 focus:outline-none"
            />
            <div v-if="filteredSites.length === 0" class="text-dim px-2 py-1 text-xs">
                No matching sites
            </div>
            <div v-else class="max-h-48 space-y-1 overflow-y-auto">
                <button
                    v-for="item in filteredSites"
                    :key="item.site"
                    class="flex w-full items-center justify-between gap-2 rounded px-2 py-1
                        text-left text-xs transition-colors"
                    :class="
                        searchStore.selectedSourceSites.includes(item.site)
                            ? 'bg-lava/10 text-white'
                            : 'text-dim hover:bg-white/5 hover:text-white'
                    "
                    :title="item.site"
                    @click="toggleSite(item.site)"
                >
                    <span class="truncate">{{ item.site }}</span>
                    <span v-if="item.count > 0" class="shrink-0 font-mono text-[10px] opacity-60">
                        {{ item.count }}
                    </span>
                </button>
            </div>
        </template>
        <div v-else class="text-dim px-2 py-1 text-xs">No source sites indexed yet</div>
    </SearchFiltersFilterSection>
</template>
