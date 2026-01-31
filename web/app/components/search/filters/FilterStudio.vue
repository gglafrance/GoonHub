<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);
const searchQuery = ref('');

const selectStudio = (studio: string) => {
    if (searchStore.studio === studio) {
        searchStore.studio = '';
    } else {
        searchStore.studio = studio;
    }
};

const filteredStudios = computed(() => {
    if (!searchQuery.value) return searchStore.filterOptions.studios;
    const q = searchQuery.value.toLowerCase();
    return searchStore.filterOptions.studios.filter((s) => s.toLowerCase().includes(q));
});

const badge = computed(() => (searchStore.studio ? searchStore.studio : undefined));
</script>

<template>
    <SearchFiltersFilterSection
        v-if="searchStore.filterOptions.studios.length > 0"
        title="Studio"
        icon="heroicons:building-office-2"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <input
            v-model="searchQuery"
            type="text"
            placeholder="Search studios..."
            class="border-border bg-surface text-dim mb-2 w-full rounded-md border px-2 py-1.5
                text-xs placeholder-white/30 focus:border-white/20 focus:outline-none"
        />
        <div v-if="filteredStudios.length === 0" class="text-dim px-2 py-1 text-xs">
            No matching studios
        </div>
        <div v-else class="max-h-40 space-y-1 overflow-y-auto">
            <button
                v-for="studio in filteredStudios"
                :key="studio"
                @click="selectStudio(studio)"
                class="w-full rounded px-2 py-1 text-left text-xs transition-colors"
                :class="
                    searchStore.studio === studio
                        ? 'bg-lava/10 text-white'
                        : 'text-dim hover:bg-white/5 hover:text-white'
                "
            >
                {{ studio }}
            </button>
        </div>
    </SearchFiltersFilterSection>
</template>
