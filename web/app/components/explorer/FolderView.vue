<script setup lang="ts">
import type { FolderInfo } from '~/types/explorer';

const route = useRoute();
const router = useRouter();
const explorerStore = useExplorerStore();

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

        <!-- Empty State -->
        <div
            v-else-if="!hasContent"
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

            <!-- Scenes -->
            <div v-if="explorerStore.scenes.length > 0">
                <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                    <h3 class="text-dim text-xs font-medium tracking-wider uppercase">Scenes</h3>
                    <div class="flex items-center gap-3">
                        <!-- Deselect all -->
                        <button
                            v-if="explorerStore.hasSelection"
                            @click="explorerStore.clearSelection()"
                            class="text-dim hover:text-lava text-xs transition-colors"
                        >
                            Deselect all
                        </button>

                        <!-- Select all on page -->
                        <button
                            v-if="!explorerStore.allPageScenesSelected"
                            @click="explorerStore.selectAllOnPage()"
                            class="text-dim hover:text-lava text-xs transition-colors"
                        >
                            Select page
                        </button>

                        <!-- Select all in folder -->
                        <button
                            v-if="!explorerStore.allFolderScenesSelected"
                            @click="explorerStore.selectAllInFolder()"
                            :disabled="explorerStore.isSelectingAll"
                            class="text-lava hover:text-lava/80 text-xs font-medium
                                transition-colors disabled:opacity-50"
                        >
                            <template v-if="explorerStore.isSelectingAll">Selecting...</template>
                            <template v-else>
                                Select all {{ explorerStore.totalScenes }} scenes
                            </template>
                        </button>

                        <!-- Select all recursive (when subfolders exist) -->
                        <button
                            v-if="explorerStore.subfolders.length > 0"
                            @click="explorerStore.selectAllInFolderRecursive()"
                            :disabled="explorerStore.isSelectingAll"
                            class="text-dim hover:text-lava text-xs transition-colors
                                disabled:opacity-50"
                        >
                            <template v-if="explorerStore.isSelectingAll">Selecting...</template>
                            <template v-else>+ subfolders</template>
                        </button>
                    </div>
                </div>

                <ExplorerSelectableSceneGrid :scenes="explorerStore.scenes" />

                <Pagination
                    :model-value="explorerStore.page"
                    :total="explorerStore.totalScenes"
                    :limit="explorerStore.limit"
                    @update:model-value="handlePageChange"
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
