<script setup lang="ts">
const explorerStore = useExplorerStore();

const inputRef = ref<HTMLInputElement>();
const localQuery = ref('');

// Debounce search
let debounceTimer: ReturnType<typeof setTimeout> | null = null;

const handleInput = () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
        explorerStore.searchQuery = localQuery.value;
        explorerStore.page = 1;
        explorerStore.performSearch();
    }, 300);
};

const handleClear = () => {
    localQuery.value = '';
    explorerStore.clearSearch();
};

// Sync with store on mount
onMounted(() => {
    localQuery.value = explorerStore.searchQuery;
});

// Watch for external clears
watch(
    () => explorerStore.searchQuery,
    (newVal) => {
        if (newVal !== localQuery.value) {
            localQuery.value = newVal;
        }
    },
);
</script>

<template>
    <div class="flex items-center gap-2">
        <div class="relative flex-1">
            <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                <Icon
                    v-if="explorerStore.isSearching"
                    name="heroicons:arrow-path"
                    size="14"
                    class="text-dim animate-spin"
                />
                <Icon v-else name="heroicons:magnifying-glass" size="14" class="text-dim" />
            </div>
            <input
                ref="inputRef"
                v-model="localQuery"
                @input="handleInput"
                type="text"
                placeholder="Search in this folder..."
                class="border-border bg-panel focus:border-lava/50 focus:ring-lava/20 w-full rounded-lg
                    border py-1.5 pr-8 pl-9 text-xs text-white placeholder-gray-500
                    transition-colors focus:outline-none focus:ring-2"
            />
            <button
                v-if="localQuery"
                @click="handleClear"
                class="absolute inset-y-0 right-0 flex items-center pr-2"
            >
                <Icon name="heroicons:x-mark" size="14" class="text-dim hover:text-white" />
            </button>
        </div>

        <div
            v-if="explorerStore.isSearchActive"
            class="text-dim shrink-0 text-[11px]"
        >
            {{ explorerStore.totalVideos }} result{{ explorerStore.totalVideos === 1 ? '' : 's' }}
        </div>
    </div>
</template>
