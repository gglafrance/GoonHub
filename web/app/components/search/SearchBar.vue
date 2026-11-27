<script setup lang="ts">
const searchStore = useSearchContext();

// sortOnMobile: the sort select also shows on small screens (the search page has it in its
// mobile filters drawer instead)
defineProps<{ sortOnMobile?: boolean }>();

// Enter: search at once (the search page searches as the user types, debounced)
const emit = defineEmits<{ submit: [] }>();
const input = ref<HTMLInputElement | null>(null);
defineExpose({ focus: () => input.value?.focus() });

const allSortOptions = [
    { value: '', label: 'Default' },
    { value: 'relevance', label: 'Relevance' },
    { value: 'random', label: 'Random' },
    { value: 'created_at_desc', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest' },
    { value: 'duration_desc', label: 'Longest' },
    { value: 'size_desc', label: 'Largest' },
    { value: 'size_asc', label: 'Smallest' },
    { value: 'jizz_count_desc', label: 'Most O' },
    { value: 'jizz_count_asc', label: 'Least O' },
    { value: 'last_jizz_desc', label: 'Last O' },
    { value: 'last_jizz_asc', label: 'Oldest O' },
    { value: 'view_count_desc', label: 'Most Viewed' },
    { value: 'view_count_asc', label: 'Least Viewed' },
];
// Size sorts need files:view_size
const perms = useMyPermissions();
const sortOptions = computed(() => allowedSortOptions(allSortOptions, perms.has));
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
                ref="input"
                v-model="searchStore.query"
                type="text"
                placeholder="Search videos..."
                class="border-border bg-surface placeholder:text-dim h-10 w-full rounded-lg border
                    py-2 pr-3 pl-9 text-sm text-white transition-colors focus:border-white/20
                    focus:outline-none"
                enterkeyhint="search"
                @keydown.enter="emit('submit')"
            />
        </div>

        <UiSortSelect
            v-model="searchStore.sort"
            :options="sortOptions"
            :class="sortOnMobile ? '' : 'hidden sm:block'"
        />

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
