<script setup lang="ts">
const searchStore = useSearchStore();

const collapsed = ref(false);

const toggleTag = (tagName: string) => {
    const idx = searchStore.selectedTags.indexOf(tagName);
    if (idx >= 0) {
        searchStore.selectedTags.splice(idx, 1);
    } else {
        searchStore.selectedTags.push(tagName);
    }
};

const badge = computed(() =>
    searchStore.selectedTags.length > 0 ? searchStore.selectedTags.length : undefined,
);
</script>

<template>
    <SearchFiltersFilterSection
        v-if="searchStore.filterOptions.tags.length > 0"
        title="Tags"
        :collapsed="collapsed"
        :badge="badge"
        @toggle="collapsed = !collapsed"
    >
        <div class="max-h-40 space-y-1 overflow-y-auto">
            <button
                v-for="tag in searchStore.filterOptions.tags"
                :key="tag.id"
                @click="toggleTag(tag.name)"
                class="flex w-full items-center gap-2 rounded px-2 py-1 text-left text-xs
                    transition-colors"
                :class="
                    searchStore.selectedTags.includes(tag.name)
                        ? 'bg-lava/10 text-white'
                        : 'text-dim hover:bg-white/5 hover:text-white'
                "
            >
                <span
                    class="h-2 w-2 shrink-0 rounded-full"
                    :style="{ backgroundColor: tag.color }"
                ></span>
                <span class="flex-1 truncate">{{ tag.name }}</span>
                <span class="text-[10px] opacity-50">{{ tag.video_count }}</span>
            </button>
        </div>
    </SearchFiltersFilterSection>
</template>
