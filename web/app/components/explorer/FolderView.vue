<script setup lang="ts">
import type { FolderInfo } from '~/types/explorer';
import type { SceneTableColumn } from '~/types/settings';

defineProps<{
    selectMode?: boolean;
}>();

defineEmits<{
    'update:selectMode': [value: boolean];
}>();

const route = useRoute();
const router = useRouter();
const explorerStore = useExplorerStore();
const { showSelector, maxLimit, updatePageSize } = usePageSize();
// Grid or table. Folder listings have no user data (rating) nor actors, and keep their order.
const { isTable } = useSceneListView('explorer');
const hiddenTableColumns: SceneTableColumn[] = ['rating', 'actors'];

let isUpdatingUrl = false;
let isSyncingFromUrl = false;

// Folder delete modal state
const showDeleteModal = ref(false);
const folderToDelete = ref<FolderInfo | null>(null);

const handleFolderDelete = (folder: FolderInfo) => {
    folderToDelete.value = folder;
    showDeleteModal.value = true;
};

const onFolderDeleted = async () => {
    showDeleteModal.value = false;
    folderToDelete.value = null;
    await explorerStore.loadFolderContents();
};

// Sync URL when store page changes
watch(
    () => explorerStore.page,
    (newPage) => {
        if (isSyncingFromUrl) return;

        const query = { ...route.query };
        if (newPage === 1) {
            delete query.page;
        } else {
            query.page = String(newPage);
        }

        isUpdatingUrl = true;
        router.replace({ query }).finally(() => {
            isUpdatingUrl = false;
        });
    },
);

// Handle browser back/forward navigation
watch(
    () => route.query.page,
    () => {
        if (isUpdatingUrl) return;

        const urlPage = Number(route.query.page);
        const targetPage = urlPage > 0 ? urlPage : 1;
        if (explorerStore.page !== targetPage) {
            isSyncingFromUrl = true;
            explorerStore.page = targetPage;
            nextTick(() => {
                isSyncingFromUrl = false;
            });

            if (explorerStore.isSearchActive) {
                explorerStore.performSearch();
            } else {
                explorerStore.loadFolderContents();
            }
        }
    },
);

const handlePageChange = async (page: number) => {
    explorerStore.page = page;
    if (explorerStore.isSearchActive) {
        await explorerStore.performSearch();
    } else {
        await explorerStore.loadFolderContents();
    }
};

// Infinite scroll instead of page controls (user setting)
const settingsStore = useSettingsStore();
const infiniteScroll = computed(() => settingsStore.infiniteScroll);
const { restore: restoreScroll, keepPositionWhile } = useInfiniteScrollPosition();
const hasMore = computed(
    () => explorerStore.loadedPages.last * explorerStore.limit < explorerStore.totalScenes,
);
const loadMore = () => {
    if (explorerStore.isLoading || explorerStore.isSearching || explorerStore.isLoadingMore) return;
    handlePageChange(explorerStore.loadedPages.last + 1);
};
const loadPrevious = () => keepPositionWhile(() => explorerStore.loadPrevious());
// Back on a scrolled folder: where the user was, once the list is rebuilt
watch(
    () => explorerStore.isLoading || explorerStore.isSearching,
    (loading, was) => {
        if (was && !loading) restoreScroll();
    },
);
watch(infiniteScroll, () => handlePageChange(explorerStore.page));

const hasContent = computed(
    () => explorerStore.subfolders.length > 0 || explorerStore.scenes.length > 0,
);

const showSearch = computed(() => hasContent.value || explorerStore.isSearchActive);
</script>

<template>
    <div>
        <!-- Loading State -->
        <div
            v-if="explorerStore.isLoading && !hasContent"
            class="flex h-64 items-center justify-center"
        >
            <LoadingSpinner label="Loading folder..." />
        </div>

        <!-- Empty State (only when no search is active) -->
        <div
            v-else-if="!hasContent && !explorerStore.isSearchActive"
            class="border-border flex h-64 flex-col items-center justify-center rounded-xl border
                border-dashed text-center"
        >
            <div
                class="bg-panel border-border flex h-10 w-10 items-center justify-center rounded-lg
                    border"
            >
                <Icon name="heroicons:folder-open" size="20" class="text-dim" />
            </div>
            <p class="text-muted mt-3 text-sm">This folder is empty</p>
        </div>

        <div v-else>
            <!-- Search -->
            <div v-if="showSearch" class="mb-4">
                <ExplorerFolderSearch />
            </div>

            <!-- Subfolders -->
            <div v-if="explorerStore.subfolders.length > 0" class="mb-6">
                <h3 class="text-dim mb-3 text-xs font-medium tracking-wider uppercase">Folders</h3>
                <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
                    <ExplorerFolderCard
                        v-for="folder in explorerStore.subfolders"
                        :key="folder.path"
                        :folder="folder"
                        @delete="handleFolderDelete"
                    />
                </div>
            </div>

            <!-- No Search Results -->
            <div
                v-if="explorerStore.isSearchActive && !hasContent"
                class="border-border flex h-48 flex-col items-center justify-center rounded-xl
                    border border-dashed text-center"
            >
                <div
                    class="bg-panel border-border flex h-10 w-10 items-center justify-center
                        rounded-lg border"
                >
                    <Icon name="heroicons:magnifying-glass" size="20" class="text-dim" />
                </div>
                <p class="text-muted mt-3 text-sm">No results found</p>
            </div>

            <!-- Scenes -->
            <div v-if="explorerStore.scenes.length > 0">
                <Pagination
                    v-if="!infiniteScroll"
                    position="top"
                    :model-value="explorerStore.page"
                    :total="explorerStore.totalScenes"
                    :limit="explorerStore.limit"
                    :show-page-size-selector="showSelector"
                    :max-limit="maxLimit"
                    @update:model-value="handlePageChange"
                    @update:limit="
                        (v: number) => {
                            updatePageSize(v);
                            handlePageChange(1);
                        }
                    "
                />

                <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                    <h3 class="text-dim text-xs font-medium tracking-wider uppercase">Scenes</h3>
                    <div class="flex items-center gap-3">
                        <SceneSelectionControls
                            :select-mode="selectMode"
                            :has-selection="explorerStore.hasSelection"
                            :is-selecting-all="explorerStore.isSelectingAll"
                            :all-page-scenes-selected="explorerStore.allPageScenesSelected"
                            :all-scenes-selected="explorerStore.allFolderScenesSelected"
                            :total-scenes="explorerStore.totalScenes"
                            :show-recursive="explorerStore.subfolders.length > 0"
                            :is-recursive-disabled="explorerStore.isSelectingAll"
                            @update:select-mode="$emit('update:selectMode', $event)"
                            @deselect-all="explorerStore.clearSelection()"
                            @select-page="explorerStore.selectAllOnPage()"
                            @select-all="explorerStore.selectAllInFolder()"
                            @select-recursive="explorerStore.selectAllInFolderRecursive()"
                        />
                        <SceneTableViewControls
                            page="explorer"
                            :hidden-columns="hiddenTableColumns"
                        />
                    </div>
                </div>

                <SceneListLoadPrevious
                    v-if="infiniteScroll && explorerStore.loadedPages.first > 1"
                    :first-page="explorerStore.loadedPages.first"
                    :loading="explorerStore.isLoadingMore"
                    @load-previous="loadPrevious"
                />

                <SceneCollection
                    :scenes="explorerStore.scenes"
                    :table="isTable"
                    :select-mode="selectMode"
                    :is-scene-selected="explorerStore.isSceneSelected"
                    :hidden-columns="hiddenTableColumns"
                    @toggle-selection="explorerStore.toggleSceneSelection"
                    @drag-select="(ids, additive) => explorerStore.dragSelect(ids, additive)"
                />

                <SceneListInfiniteLoader
                    v-if="infiniteScroll"
                    :has-more="hasMore"
                    :loading="explorerStore.isLoadingMore"
                    :total="explorerStore.totalScenes"
                    @load-more="loadMore"
                />
                <Pagination
                    v-else
                    :model-value="explorerStore.page"
                    :total="explorerStore.totalScenes"
                    :limit="explorerStore.limit"
                    :show-page-size-selector="showSelector"
                    :max-limit="maxLimit"
                    @update:model-value="handlePageChange"
                    @update:limit="
                        (v: number) => {
                            updatePageSize(v);
                            handlePageChange(1);
                        }
                    "
                />
            </div>
        </div>

        <!-- Folder Delete Modal -->
        <ExplorerFolderDeleteModal
            v-if="explorerStore.currentStoragePathID"
            :visible="showDeleteModal"
            :folder-name="folderToDelete?.name ?? ''"
            :storage-path-id="explorerStore.currentStoragePathID"
            :folder-path="folderToDelete?.path ?? ''"
            @close="showDeleteModal = false"
            @deleted="onFolderDeleted"
        />
    </div>
</template>
