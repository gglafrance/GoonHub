<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(false);

const toggleActor = (actor: string) => {
    const idx = searchStore.selectedActors.indexOf(actor);
    if (idx >= 0) {
        searchStore.selectedActors.splice(idx, 1);
    } else {
        searchStore.selectedActors.push(actor);
    }
};

const badge = computed(() =>
    searchStore.selectedActors.length > 0 ? searchStore.selectedActors.length : undefined,
);
</script>

<template>
    <SearchFiltersFilterSection
        v-if="searchStore.filterOptions.actors.length > 0"
        title="Actors"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="max-h-40 space-y-1 overflow-y-auto">
            <button
                v-for="actor in searchStore.filterOptions.actors"
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
