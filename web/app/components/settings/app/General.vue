<script setup lang="ts">
const settingsStore = useSettingsStore();
const { layout: keyboardLayout, setLayout: setKeyboardLayout } = useKeyboardLayout();

const appVideosPerPage = computed({
    get: () => settingsStore.draft?.videos_per_page ?? 20,
    set: (v) => {
        if (settingsStore.draft) {
            settingsStore.draft.videos_per_page = Math.min(
                Math.max(1, v),
                settingsStore.maxItemsPerPage,
            );
        }
    },
});

const appMarkerThumbnailCycling = computed({
    get: () => settingsStore.draft?.marker_thumbnail_cycling ?? true,
    set: (v) => {
        if (settingsStore.draft) settingsStore.draft.marker_thumbnail_cycling = v;
    },
});

const appShowPageSizeSelector = computed({
    get: () => settingsStore.draft?.show_page_size_selector ?? false,
    set: (v) => {
        if (settingsStore.draft) settingsStore.draft.show_page_size_selector = v;
    },
});
</script>

<template>
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
                    :max="settingsStore.maxItemsPerPage"
                    class="border-border bg-void/80 focus:border-lava/40 focus:ring-lava/20 w-full
                        max-w-32 rounded-lg border px-3.5 py-2.5 text-sm text-white transition-all
                        focus:ring-1 focus:outline-none"
                />
                <p class="text-dim mt-1.5 text-[11px]">
                    Value between 1 and {{ settingsStore.maxItemsPerPage }}
                </p>
            </div>

            <!-- Page Size Selector -->
            <div class="flex items-center justify-between">
                <div>
                    <label class="text-sm font-medium text-white"> Page Size Selector </label>
                    <p class="text-dim mt-0.5 text-xs">
                        Show a page size dropdown on paginated pages
                    </p>
                </div>
                <UiToggle v-model="appShowPageSizeSelector" />
            </div>

            <!-- Keyboard Layout -->
            <div>
                <label
                    class="text-dim mb-1.5 block text-[11px] font-medium tracking-wider uppercase"
                >
                    Keyboard Layout
                </label>
                <p class="text-dim mb-3 text-xs">
                    Adjusts keyboard shortcuts for your keyboard layout
                </p>
                <div class="flex gap-2">
                    <button
                        :class="[
                            'rounded-lg border px-4 py-2 text-xs font-medium transition-all',
                            keyboardLayout === 'qwerty'
                                ? 'border-lava bg-lava/10 text-lava'
                                : `border-border hover:border-border-hover text-muted
                                    hover:text-white`,
                        ]"
                        @click="setKeyboardLayout('qwerty')"
                    >
                        QWERTY
                    </button>
                    <button
                        :class="[
                            'rounded-lg border px-4 py-2 text-xs font-medium transition-all',
                            keyboardLayout === 'azerty'
                                ? 'border-lava bg-lava/10 text-lava'
                                : `border-border hover:border-border-hover text-muted
                                    hover:text-white`,
                        ]"
                        @click="setKeyboardLayout('azerty')"
                    >
                        AZERTY
                    </button>
                </div>
            </div>

            <!-- Marker Thumbnail Cycling -->
            <div class="flex items-center justify-between">
                <div>
                    <label class="text-sm font-medium text-white"> Marker Thumbnail Cycling </label>
                    <p class="text-dim mt-0.5 text-xs">
                        Automatically cycle through thumbnails on marker label cards
                    </p>
                </div>
                <UiToggle v-model="appMarkerThumbnailCycling" />
            </div>
        </div>
    </div>
</template>
