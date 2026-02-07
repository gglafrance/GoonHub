<script setup lang="ts">
useHead({ title: 'Explorer' });

const route = useRoute();
const router = useRouter();
const explorerStore = useExplorerStore();

const selectMode = ref(false);

// Clear selection when select mode toggled off
watch(selectMode, (on) => {
    if (!on) explorerStore.clearSelection();
});

// Reference to bulk toolbar for triggering delete modal
const bulkToolbarRef = ref<{ showDeleteModal: Ref<boolean> } | null>(null);

// Keyboard shortcuts
useKeyboardShortcuts([
    {
        key: 'a',
        ctrl: true,
        action: () => {
            if (selectMode.value) explorerStore.selectAllOnPage();
        },
        description: 'Select all scenes on current page',
    },
    {
        key: 'a',
        ctrl: true,
        shift: true,
        action: () => {
            if (selectMode.value) explorerStore.selectAllInFolderRecursive();
        },
        description: 'Select all scenes including subfolders',
    },
    {
        key: 'Escape',
        action: () => {
            if (selectMode.value) {
                explorerStore.clearSelection();
                selectMode.value = false;
            }
        },
        description: 'Clear selection',
    },
    {
        key: 'Delete',
        action: () => {
            if (selectMode.value && explorerStore.hasSelection && bulkToolbarRef.value) {
                bulkToolbarRef.value.showDeleteModal = true;
            }
        },
        description: 'Delete selected scenes',
    },
]);

// Parse slug into storagePathId and folderPath
// slug = ['2'] -> storagePathId = 2, folderPath = ''
// slug = ['2', 'Movies', 'Action'] -> storagePathId = 2, folderPath = '/Movies/Action'
const parseSlug = (slug: string | string[] | undefined) => {
    if (!slug) {
        return { storagePathId: null, folderPath: '' };
    }

    const parts = Array.isArray(slug) ? slug : [slug];
    const firstPart = parts[0];
    if (!firstPart) {
        return { storagePathId: null, folderPath: '' };
    }

    const storagePathId = parseInt(firstPart, 10);
    if (isNaN(storagePathId)) {
        return { storagePathId: null, folderPath: '' };
    }

    const folderPath = parts.length > 1 ? '/' + parts.slice(1).join('/') : '';
    return { storagePathId, folderPath };
};

// Initialize from URL on mount
onMounted(async () => {
    const { storagePathId, folderPath } = parseSlug(route.params.slug);

    if (storagePathId === null) {
        router.replace('/explorer');
        return;
    }

    await explorerStore.loadStoragePaths();

    // Check if storage path exists
    const exists = explorerStore.storagePaths.some((sp) => sp.id === storagePathId);
    if (!exists) {
        router.replace('/explorer');
        return;
    }

    explorerStore.currentStoragePathID = storagePathId;
    explorerStore.currentPath = folderPath;
    // Read page from URL query, default to 1
    const urlPage = Number(route.query.page);
    explorerStore.page = urlPage > 0 ? urlPage : 1;
    explorerStore.clearSelection();

    await explorerStore.loadFolderContents();
});

// Watch URL changes (browser back/forward)
watch(
    () => route.params.slug,
    async (newSlug) => {
        const { storagePathId, folderPath } = parseSlug(newSlug);

        if (storagePathId === null) {
            router.replace('/explorer');
            return;
        }

        // Only update if changed
        if (
            storagePathId !== explorerStore.currentStoragePathID ||
            folderPath !== explorerStore.currentPath
        ) {
            explorerStore.currentStoragePathID = storagePathId;
            explorerStore.currentPath = folderPath;
            // Read page from URL query, default to 1
            const urlPage = Number(route.query.page);
            explorerStore.page = urlPage > 0 ? urlPage : 1;
            explorerStore.clearSelection();
            await explorerStore.loadFolderContents();
        }
    },
);

// Don't reset on unmount - let the destination page handle state

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Header -->
            <div class="mb-6">
                <div class="flex items-center justify-between gap-4">
                    <div class="flex min-w-0 flex-1 items-center gap-3">
                        <h1 class="shrink-0 text-lg font-semibold text-white">Explorer</h1>
                        <ExplorerBreadcrumbs class="min-w-0 flex-1" />
                    </div>
                    <div class="flex shrink-0 items-center gap-2">
                        <span
                            v-if="explorerStore.currentStoragePathID"
                            class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                                font-mono text-[11px]"
                        >
                            {{ explorerStore.totalScenes }} scenes
                        </span>
                        <button
                            v-if="explorerStore.scenes.length > 0"
                            class="flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs
                                font-medium transition-all"
                            :class="
                                selectMode
                                    ? 'border-lava/40 bg-lava/10 text-lava'
                                    : `border-border text-dim hover:border-border-hover
                                        hover:text-white`
                            "
                            @click="selectMode = !selectMode"
                        >
                            <Icon name="heroicons:check-circle" size="14" />
                            Select
                        </button>
                    </div>
                </div>
            </div>

            <!-- Error -->
            <ErrorAlert v-if="explorerStore.error" :message="explorerStore.error" class="mb-4" />

            <!-- Folder View -->
            <ExplorerFolderView :select-mode="selectMode" />

            <!-- Bulk Toolbar -->
            <ExplorerBulkToolbar
                v-if="selectMode && explorerStore.hasSelection"
                ref="bulkToolbarRef"
            />
        </div>
    </div>
</template>
