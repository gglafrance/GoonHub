<script setup lang="ts">
const searchStore = useSearchStore();

const sortOptions = [
    { value: '', label: 'Default' },
    { value: 'relevance', label: 'Relevance' },
    { value: 'created_at_desc', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest' },
    { value: 'duration_desc', label: 'Longest' },
];
</script>

<template>
    <div class="flex gap-3">
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
                class="border-border bg-surface text-sm text-white placeholder:text-dim w-full
                    rounded-lg border py-2 pr-3 pl-9 transition-colors
                    focus:border-white/20 focus:outline-none"
            />
        </div>

        <select
            v-model="searchStore.sort"
            class="border-border bg-surface text-dim rounded-lg border px-3 py-2 text-xs
                transition-colors focus:border-white/20 focus:outline-none"
        >
            <option v-for="opt in sortOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
            </option>
        </select>
    </div>
</template>
