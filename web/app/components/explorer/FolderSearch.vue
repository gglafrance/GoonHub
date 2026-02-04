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

// Cycle through: null (off) -> true (has) -> false (missing) -> null (off)
const togglePornDBFilter = () => {
    if (explorerStore.searchHasPornDBID === null) {
        explorerStore.searchHasPornDBID = true;
    } else if (explorerStore.searchHasPornDBID === true) {
        explorerStore.searchHasPornDBID = false;
    } else {
        explorerStore.searchHasPornDBID = null;
    }
    explorerStore.page = 1;
    explorerStore.performSearch();
};

const pornDBFilterLabel = computed(() => {
    if (explorerStore.searchHasPornDBID === true) return 'TPDB';
    if (explorerStore.searchHasPornDBID === false) return 'NO TPDB';
    return 'TPDB';
});

const pornDBFilterTitle = computed(() => {
    if (explorerStore.searchHasPornDBID === true)
        return 'Showing videos with ThePornDB ID (click for missing)';
    if (explorerStore.searchHasPornDBID === false)
        return 'Showing videos without ThePornDB ID (click to clear)';
    return 'Filter by ThePornDB ID';
});

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
                class="border-border bg-panel focus:border-lava/50 focus:ring-lava/20 w-full
                    rounded-lg border py-1.5 pr-8 pl-9 text-xs text-white placeholder-gray-500
                    transition-colors focus:ring-2 focus:outline-none"
            />
            <button
                v-if="localQuery"
                @click="handleClear"
                class="absolute inset-y-0 right-0 flex items-center pr-2"
            >
                <Icon name="heroicons:x-mark" size="14" class="text-dim hover:text-white" />
            </button>
        </div>

        <button
            @click="togglePornDBFilter"
            :class="[
                'shrink-0 rounded px-2 py-1.5 text-[10px] font-medium tracking-wide uppercase',
                'border transition-all duration-150',
                explorerStore.searchHasPornDBID === true
                    ? 'border-lava/50 bg-lava/20 text-lava'
                    : explorerStore.searchHasPornDBID === false
                      ? 'border-amber-500/50 bg-amber-500/20 text-amber-400'
                      : 'border-border bg-panel text-dim hover:border-white/20 hover:text-white/70',
            ]"
            :title="pornDBFilterTitle"
        >
            {{ pornDBFilterLabel }}
        </button>

        <div v-if="explorerStore.isSearchActive" class="text-dim shrink-0 text-[11px]">
            {{ explorerStore.totalScenes }} result{{ explorerStore.totalScenes === 1 ? '' : 's' }}
        </div>
    </div>
</template>
