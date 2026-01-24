<script setup lang="ts">
import type { SortOrder } from '~/types/settings';

const settingsStore = useSettingsStore();
const { message, error, clearMessages } = useSettingsMessage();

const appVideosPerPage = ref(20);
const appSortOrder = ref<SortOrder>('created_at_desc');

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

onMounted(syncFromStore);

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
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
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
                        class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                    >
                        Default Sort Order
                    </label>
                    <UiSelectMenu
                        v-model="appSortOrder"
                        :options="sortOptions"
                        class="max-w-64"
                    />
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
    </div>
</template>
