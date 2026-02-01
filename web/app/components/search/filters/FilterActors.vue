<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);
const searchQuery = ref('');

const toggleActor = (actor: string) => {
    const idx = searchStore.selectedActors.indexOf(actor);
    if (idx >= 0) {
        searchStore.selectedActors.splice(idx, 1);
    } else {
        searchStore.selectedActors.push(actor);
    }
};

const filteredActors = computed(() => {
    if (!searchQuery.value) return searchStore.filterOptions.actors;
    const q = searchQuery.value.toLowerCase();
    return searchStore.filterOptions.actors.filter((a) => a.toLowerCase().includes(q));
});

const badge = computed(() =>
    searchStore.selectedActors.length > 0 ? searchStore.selectedActors.length : undefined,
);
</script>

<template>
    <SearchFiltersFilterSection
        v-if="searchStore.filterOptions.actors.length > 0"
        title="Actors"
        icon="heroicons:user-group"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <input
            v-model="searchQuery"
            type="text"
            placeholder="Search actors..."
            class="border-border bg-surface text-dim mb-2 w-full rounded-md border px-2 py-1.5
                text-xs placeholder-white/30 focus:border-white/20 focus:outline-none"
        />
        <div v-if="filteredActors.length === 0" class="text-dim px-2 py-1 text-xs">
            No matching actors
        </div>
        <div v-else class="max-h-40 space-y-1 overflow-y-auto">
            <button
                v-for="actor in filteredActors"
                :key="actor"
                @click="toggleActor(actor)"
                class="w-full rounded px-2 py-1 text-left text-xs transition-colors"
                :class="
                    searchStore.selectedActors.includes(actor)
                        ? 'bg-lava/10 text-white'
                        : 'text-dim hover:bg-white/5 hover:text-white'
                "
            >
                {{ actor }}
            </button>
        </div>
    </SearchFiltersFilterSection>
</template>
