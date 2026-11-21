<script setup lang="ts">
const route = useRoute();
const router = useRouter();
const explorerStore = useExplorerStore();

const pageTitle = computed(() => {
    const folder = explorerStore.currentPath.split('/').filter(Boolean).pop();
    return folder || 'Explorer';
});

useHead({ title: pageTitle });

useSeoMeta({
    title: pageTitle,
    ogTitle: computed(() => `${pageTitle.value} - GoonHub`),
    description: 'Browse your scene storage paths',
    ogDescription: 'Browse your scene storage paths',
});

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

// The view lives in the URL: folder in the path, `page`, and the folder search (`q`, `tpdb`),
// so that Back from a scene, a reload or a shared link open the same view.
const patchQuery = useUrlQueryPatch();
// URL syncs start once the store holds this page's view
let mounted = false;

/** Applies the URL to the store; returns whether the view changed. */
const applyUrl = (): boolean => {
    const { storagePathId, folderPath } = parseSlug(route.params.slug);
    const view = {
        page: queryPositiveInt(route.query.page, 1),
        q: queryString(route.query.q),
        tpdb: queryTriState(route.query.tpdb),
    };
    const folderChanged =
        storagePathId !== explorerStore.currentStoragePathID ||
        folderPath !== explorerStore.currentPath;
    const changed =
        folderChanged ||
        view.page !== explorerStore.page ||
        view.q !== explorerStore.searchQuery ||
        view.tpdb !== explorerStore.searchHasPornDBID;
    explorerStore.currentStoragePathID = storagePathId;
    explorerStore.currentPath = folderPath;
    explorerStore.page = view.page;
    explorerStore.searchQuery = view.q;
    explorerStore.searchHasPornDBID = view.tpdb;
    if (folderChanged) explorerStore.clearSelection();
    return changed;
};

// Store -> URL (page changes, folder search)
watch(
    () => [explorerStore.page, explorerStore.searchQuery, explorerStore.searchHasPornDBID],
    () => {
        if (!mounted) return;
        patchQuery({
            page: explorerStore.page > 1 ? String(explorerStore.page) : undefined,
            q: explorerStore.searchQuery || undefined,
            tpdb:
                explorerStore.searchHasPornDBID === null
                    ? undefined
                    : String(explorerStore.searchHasPornDBID),
        });
    },
);

onMounted(async () => {
    const { storagePathId } = parseSlug(route.params.slug);
    if (storagePathId === null) {
        router.replace('/explorer');
        return;
    }

    // Storage paths already known (Back from a scene): no need to wait for them again
    if (explorerStore.storagePaths.length === 0) await explorerStore.loadStoragePaths();

    // Check if storage path exists
    if (!explorerStore.storagePaths.some((sp) => sp.id === storagePathId)) {
        router.replace('/explorer');
        return;
    }

    applyUrl();
    mounted = true;
    await explorerStore.refresh();
});

// URL -> store (folder links, browser back/forward)
watch(
    () => route.fullPath,
    async () => {
        if (!mounted) return;
        const { storagePathId } = parseSlug(route.params.slug);
        if (storagePathId === null) {
            router.replace('/explorer');
            return;
        }
        if (applyUrl()) await explorerStore.refresh();
    },
);

const handleBulkComplete = () => {
    explorerStore.loadFolderContents();
    explorerStore.clearSelection();
};

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
                    </div>
                </div>
            </div>

            <!-- Error -->
            <ErrorAlert v-if="explorerStore.error" :message="explorerStore.error" class="mb-4" />

            <!-- Folder View -->
            <ExplorerFolderView
                :select-mode="selectMode"
                @update:select-mode="selectMode = $event"
            />

            <!-- Bulk Toolbar -->
            <BulkToolbar
                v-if="selectMode && explorerStore.hasSelection"
                ref="bulkToolbarRef"
                :scene-ids="explorerStore.getSelectedSceneIDs()"
                :selection-count="explorerStore.selectionCount"
                @clear-selection="explorerStore.clearSelection()"
                @complete="handleBulkComplete"
            />
        </div>
    </div>
</template>
