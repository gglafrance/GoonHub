<script setup lang="ts">
import type { SortOrder, KeyboardLayout } from '~/types/settings';

const settingsStore = useSettingsStore();
const authStore = useAuthStore();
const { message, error, clearMessages } = useSettingsMessage();
const { triggerReindex, getSearchConfig, updateSearchConfig } = useApiAdmin();
const { layout: keyboardLayout, setLayout: setKeyboardLayout } = useKeyboardLayout();

const appVideosPerPage = ref(20);
const appSortOrder = ref<SortOrder>('created_at_desc');

// Search index state
const isReindexing = ref(false);
const reindexMessage = ref('');
const reindexError = ref('');

// Search config state (admin only)
const isAdmin = computed(() => authStore.user?.role === 'admin');
const maxTotalHits = ref(100000);
const isSavingSearchConfig = ref(false);
const searchConfigMessage = ref('');
const searchConfigError = ref('');

const sortOptions: { value: SortOrder; label: string }[] = [
    { value: 'created_at_desc', label: 'Newest First' },
    { value: 'created_at_asc', label: 'Oldest First' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest First' },
    { value: 'duration_desc', label: 'Longest First' },
    { value: 'size_asc', label: 'Smallest First' },
    { value: 'size_desc', label: 'Largest First' },
];

const syncFromStore = () => {
    appVideosPerPage.value = settingsStore.videosPerPage;
    appSortOrder.value = settingsStore.defaultSortOrder;
};

const loadSearchConfig = async () => {
    if (!isAdmin.value) return;
    try {
        const data = await getSearchConfig();
        maxTotalHits.value = data.max_total_hits;
    } catch {
        // Silently fail - default value is already set
    }
};

onMounted(() => {
    syncFromStore();
    loadSearchConfig();
});

watch(() => settingsStore.settings, syncFromStore);

const handleSaveApp = async () => {
    clearMessages();
    try {
        await settingsStore.updateApp(appVideosPerPage.value, appSortOrder.value);
        message.value = 'App settings saved';
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to save settings';
    }
};

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
        <div
            v-if="message"
            class="border-emerald/20 bg-emerald/5 text-emerald rounded-lg border px-3 py-2 text-xs"
        >
            {{ message }}
        </div>
        <div
            v-if="error"
            class="border-lava/20 bg-lava/5 text-lava rounded-lg border px-3 py-2 text-xs"
        >
            {{ error }}
        </div>

        <div class="glass-panel p-5">
            <h3 class="mb-5 text-sm font-semibold text-white">App Preferences</h3>
            <div class="space-y-5">
                <!-- Videos Per Page -->
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Videos Per Page
                    </label>
                    <input
                        v-model.number="appVideosPerPage"
                        type="number"
                        min="1"
                        max="100"
                        class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20
                            w-full max-w-32 rounded-lg border px-3.5 py-2.5 text-sm text-white
                            transition-all focus:ring-1 focus:outline-none"
                    />
                </div>

                <!-- Sort Order -->
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Default Sort Order
                    </label>
                    <UiSelectMenu v-model="appSortOrder" :options="sortOptions" class="max-w-64" />
                </div>

                <!-- Keyboard Layout -->
                <div>
                    <label
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider
                            uppercase"
                    >
                        Keyboard Layout
                    </label>
                    <p class="text-dim mb-3 text-xs">
                        Adjusts keyboard shortcuts for your keyboard layout
                    </p>
                    <div class="flex gap-2">
                        <button
                            @click="setKeyboardLayout('qwerty')"
                            :class="[
                                'rounded-lg border px-4 py-2 text-xs font-medium transition-all',
                                keyboardLayout === 'qwerty'
                                    ? 'border-lava bg-lava/10 text-lava'
                                    : `border-border hover:border-border-hover text-muted
                                        hover:text-white`,
                            ]"
                        >
                            QWERTY
                        </button>
                        <button
                            @click="setKeyboardLayout('azerty')"
                            :class="[
                                'rounded-lg border px-4 py-2 text-xs font-medium transition-all',
                                keyboardLayout === 'azerty'
                                    ? 'border-lava bg-lava/10 text-lava'
                                    : `border-border hover:border-border-hover text-muted
                                        hover:text-white`,
                            ]"
                        >
                            AZERTY
                        </button>
                    </div>
                </div>

                <button
                    @click="handleSaveApp"
                    :disabled="settingsStore.isLoading"
                    class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-2 text-xs font-semibold
                        text-white transition-all disabled:cursor-not-allowed disabled:opacity-40"
                >
                    Save App Settings
                </button>
            </div>
        </div>

        <!-- Search Index -->
        <div class="glass-panel p-5">
            <h3 class="mb-2 text-sm font-semibold text-white">Search Index</h3>
            <p class="text-dim mb-4 text-xs">
                Rebuild the search index to sync all video data including actors, tags, and view
                counts.
            </p>

            <!-- Max Total Hits (admin only) -->
            <div v-if="isAdmin" class="mb-5">
                <div
                    v-if="searchConfigMessage"
                    class="border-emerald/20 bg-emerald/5 text-emerald mb-3 rounded-lg border px-3
                        py-2 text-xs"
                >
                    {{ searchConfigMessage }}
                </div>
                <div
                    v-if="searchConfigError"
                    class="border-lava/20 bg-lava/5 text-lava mb-3 rounded-lg border px-3 py-2
                        text-xs"
                >
                    {{ searchConfigError }}
                </div>

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
                        @click="handleSaveSearchConfig"
                        :disabled="isSavingSearchConfig"
                        class="border-border hover:border-lava/40 hover:bg-lava/10 rounded-lg
                            border px-4 py-2 text-xs font-medium text-white transition-all
                            disabled:cursor-not-allowed disabled:opacity-40"
                    >
                        {{ isSavingSearchConfig ? 'Saving...' : 'Save' }}
                    </button>
                </div>
            </div>

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
                @click="handleReindex"
                :disabled="isReindexing"
                class="border-border hover:border-lava/40 hover:bg-lava/10 flex items-center gap-2
                    rounded-lg border px-4 py-2 text-xs font-medium text-white transition-all
                    disabled:cursor-not-allowed disabled:opacity-40"
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
