<script setup lang="ts">
const { triggerReindex, getSearchConfig, updateSearchConfig } = useApiAdmin();

const isReindexing = ref(false);
const reindexMessage = ref('');
const reindexError = ref('');

const maxTotalHits = ref(100000);
const isSavingSearchConfig = ref(false);
const searchConfigMessage = ref('');
const searchConfigError = ref('');

const loadSearchConfig = async () => {
    try {
        const data = await getSearchConfig();
        maxTotalHits.value = data.max_total_hits;
    } catch {
        // Silently fail - default value is already set
    }
};

onMounted(() => {
    loadSearchConfig();
});

const handleReindex = async () => {
    reindexMessage.value = '';
    reindexError.value = '';
    isReindexing.value = true;
    try {
        await triggerReindex();
        reindexMessage.value = 'Search index rebuild started. This may take a few moments.';
    } catch (e: unknown) {
        reindexError.value = e instanceof Error ? e.message : 'Failed to trigger reindex';
    } finally {
        isReindexing.value = false;
    }
};

const handleSaveSearchConfig = async () => {
    searchConfigMessage.value = '';
    searchConfigError.value = '';
    isSavingSearchConfig.value = true;
    try {
        await updateSearchConfig({ max_total_hits: maxTotalHits.value });
        searchConfigMessage.value = 'Search configuration saved';
    } catch (e: unknown) {
        searchConfigError.value = e instanceof Error ? e.message : 'Failed to save search config';
    } finally {
        isSavingSearchConfig.value = false;
    }
};
</script>

<template>
    <div class="space-y-6">
        <!-- Search Configuration -->
        <div class="glass-panel p-5">
            <h3 class="mb-2 text-sm font-semibold text-white">Search</h3>
            <p class="text-dim mb-4 text-xs">Configure search engine settings.</p>

            <div
                v-if="searchConfigMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald mb-4 rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ searchConfigMessage }}
            </div>
            <div
                v-if="searchConfigError"
                class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ searchConfigError }}
            </div>

            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Max Total Hits
                </label>
                <p class="text-dim mb-2 text-xs">
                    Maximum number of search results that can be counted. Increase this if your
                    library has more scenes than the current limit.
                </p>
                <div class="flex items-center gap-3">
                    <input
                        v-model.number="maxTotalHits"
                        type="number"
                        min="1000"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full max-w-48 rounded-lg border px-3.5 py-2.5 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                    <button
                        :disabled="isSavingSearchConfig"
                        class="border-border hover:border-lava/40 hover:bg-lava/10 rounded-lg border
                            px-4 py-2 text-xs font-medium text-white transition-all
                            disabled:cursor-not-allowed disabled:opacity-40"
                        @click="handleSaveSearchConfig"
                    >
                        {{ isSavingSearchConfig ? 'Saving...' : 'Save' }}
                    </button>
                </div>
            </div>
        </div>

        <!-- Search Index -->
        <div class="glass-panel p-5">
            <h3 class="mb-2 text-sm font-semibold text-white">Search Index</h3>
            <p class="text-dim mb-4 text-xs">
                Rebuild the search index to sync all video data including actors, tags, and view
                counts.
            </p>

            <div
                v-if="reindexMessage"
                class="border-emerald/20 bg-emerald/5 text-emerald mb-4 rounded-lg border px-3 py-2
                    text-xs"
            >
                {{ reindexMessage }}
            </div>
            <div
                v-if="reindexError"
                class="border-lava/20 bg-lava/5 text-lava mb-4 rounded-lg border px-3 py-2 text-xs"
            >
                {{ reindexError }}
            </div>

            <button
                :disabled="isReindexing"
                class="border-border hover:border-lava/40 hover:bg-lava/10 flex items-center gap-2
                    rounded-lg border px-4 py-2 text-xs font-medium text-white transition-all
                    disabled:cursor-not-allowed disabled:opacity-40"
                @click="handleReindex"
            >
                <Icon
                    name="heroicons:arrow-path"
                    size="14"
                    :class="{ 'animate-spin': isReindexing }"
                />
                {{ isReindexing ? 'Rebuilding...' : 'Rebuild Search Index' }}
            </button>
        </div>
    </div>
</template>
