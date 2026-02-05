<script setup lang="ts">
const searchStore = useSearchStore();

const sortOptions = [
    { value: '', label: 'Default' },
    { value: 'relevance', label: 'Relevance' },
    { value: 'random', label: 'Random' },
    { value: 'created_at_desc', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest' },
    { value: 'duration_desc', label: 'Longest' },
    { value: 'view_count_desc', label: 'Most Viewed' },
    { value: 'view_count_asc', label: 'Least Viewed' },
];
</script>

<template>
    <div class="flex gap-2 sm:gap-3">
        <div class="relative flex-1">
            <Icon
                name="heroicons:magnifying-glass"
                size="16"
                class="text-dim absolute top-1/2 left-3 -translate-y-1/2"
            />
            <input
                v-model="searchStore.query"
                type="text"
                placeholder="Search videos..."
                class="border-border bg-surface placeholder:text-dim h-10 w-full rounded-lg border
                    py-2 pr-3 pl-9 text-sm text-white transition-colors focus:border-white/20
                    focus:outline-none"
                enterkeyhint="search"
            />
        </div>

        <UiSortSelect v-model="searchStore.sort" :options="sortOptions" class="hidden sm:block" />

        <button
            v-if="searchStore.sort === 'random'"
            class="border-border bg-surface hover:border-lava/40 hover:bg-lava/10 hidden h-10 w-10
                shrink-0 items-center justify-center rounded-lg border transition-all sm:flex"
            title="Reshuffle"
            @click="searchStore.reshuffle()"
        >
            <Icon name="heroicons:arrow-path" size="16" class="text-white" />
        </button>
    </div>
</template>
