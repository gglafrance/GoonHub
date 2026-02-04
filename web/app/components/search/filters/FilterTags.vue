<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(true);
const searchQuery = ref('');

const toggleTag = (tagName: string) => {
    const idx = searchStore.selectedTags.indexOf(tagName);
    if (idx >= 0) {
        searchStore.selectedTags.splice(idx, 1);
    } else {
        searchStore.selectedTags.push(tagName);
    }
};

const filteredTags = computed(() => {
    if (!searchQuery.value) return searchStore.filterOptions.tags;
    const q = searchQuery.value.toLowerCase();
    return searchStore.filterOptions.tags.filter((t) => t.name.toLowerCase().includes(q));
});

const badge = computed(() =>
    searchStore.selectedTags.length > 0 ? searchStore.selectedTags.length : undefined,
);
</script>

<template>
    <SearchFiltersFilterSection
        v-if="searchStore.filterOptions.tags.length > 0"
        title="Tags"
        icon="heroicons:tag"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <input
            v-model="searchQuery"
            type="text"
            placeholder="Search tags..."
            class="border-border bg-surface text-dim mb-2 w-full rounded-md border px-2 py-1.5
                text-xs placeholder-white/30 focus:border-white/20 focus:outline-none"
        />
        <div v-if="filteredTags.length === 0" class="text-dim px-2 py-1 text-xs">
            No matching tags
        </div>
        <div v-else class="max-h-40 space-y-1 overflow-y-auto">
            <button
                v-for="tag in filteredTags"
                :key="tag.id"
                class="flex w-full items-center gap-2 rounded px-2 py-1 text-left text-xs
                    transition-colors"
                :class="
                    searchStore.selectedTags.includes(tag.name)
                        ? 'bg-lava/10 text-white'
                        : 'text-dim hover:bg-white/5 hover:text-white'
                "
                @click="toggleTag(tag.name)"
            >
                <span
                    class="h-2 w-2 shrink-0 rounded-full"
                    :style="{ backgroundColor: tag.color }"
                ></span>
                <span class="flex-1 truncate">{{ tag.name }}</span>
                <span class="text-[10px] opacity-50">{{ tag.scene_count }}</span>
            </button>
        </div>
    </SearchFiltersFilterSection>
</template>
