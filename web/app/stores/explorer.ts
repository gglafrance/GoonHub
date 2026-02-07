import { defineStore } from 'pinia';
import type { SceneListItem } from '~/types/scene';
import type { StoragePathWithCount, FolderInfo, FolderContentsResponse } from '~/types/explorer';

export interface Breadcrumb {
    name: string;
    path: string;
}

export const useExplorerStore = defineStore('explorer', () => {
    // Settings
    const settingsStore = useSettingsStore();

    // State
    const storagePaths = ref<StoragePathWithCount[]>([]);
    const currentStoragePathID = ref<number | null>(null);
    const currentPath = ref('');
    const subfolders = ref<FolderInfo[]>([]);
    const scenes = ref<SceneListItem[]>([]);
    const totalScenes = ref(0);
    const page = ref(1);
    const isLoading = ref(false);
    const error = ref<string | null>(null);

    // Use videosPerPage from settings
    const limit = computed(() => settingsStore.videosPerPage);

    // Selection state
    const selectedSceneIDs = ref<Set<number>>(new Set());

    // API
    const { getStoragePaths, getFolderContents, getFolderSceneIDs, searchInFolder } =
        useApiExplorer();

    // Track if all folder scenes are selected (across all pages)
    const allFolderSceneIDs = ref<number[]>([]);
    const isSelectingAll = ref(false);

    // Search state
    const searchQuery = ref('');
    const searchTags = ref<number[]>([]);
    const searchActors = ref<string[]>([]);
    // null = no filter, true = has PornDB ID, false = missing PornDB ID
    const searchHasPornDBID = ref<boolean | null>(null);
    const isSearching = ref(false);

    // Computed
    const currentStoragePath = computed(() => {
        if (!currentStoragePathID.value) return null;
        return storagePaths.value.find((sp) => sp.id === currentStoragePathID.value) || null;
    });

    const breadcrumbs = computed<Breadcrumb[]>(() => {
        if (!currentPath.value) return [];

        const parts = currentPath.value.split('/').filter(Boolean);
        const crumbs: Breadcrumb[] = [];
        let accumulated = '';

        for (const part of parts) {
            accumulated += '/' + part;
            crumbs.push({
                name: part,
                path: accumulated,
            });
        }

        return crumbs;
    });

    const totalPages = computed(() => Math.ceil(totalScenes.value / limit.value));

    const hasSelection = computed(() => selectedSceneIDs.value.size > 0);
    const selectionCount = computed(() => selectedSceneIDs.value.size);
    const allPageScenesSelected = computed(
        () =>
            scenes.value.length > 0 && scenes.value.every((s) => selectedSceneIDs.value.has(s.id)),
    );
    const allFolderScenesSelected = computed(
        () => totalScenes.value > 0 && selectedSceneIDs.value.size === totalScenes.value,
    );

    const isSearchActive = computed(
        () =>
            searchQuery.value.length > 0 ||
            searchTags.value.length > 0 ||
            searchActors.value.length > 0 ||
            searchHasPornDBID.value !== null,
    );

    // Actions
    const loadStoragePaths = async () => {
        isLoading.value = true;
        error.value = null;
        try {
            const response = await getStoragePaths();
            storagePaths.value = response.storage_paths;
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isLoading.value = false;
        }
    };

    const navigateToStoragePath = async (storagePathID: number) => {
        currentStoragePathID.value = storagePathID;
        currentPath.value = '';
        page.value = 1;
        clearSelection();
        await loadFolderContents();
    };

    const navigateToFolder = async (folderPath: string) => {
        currentPath.value = folderPath;
        page.value = 1;
        clearSelection();
        await loadFolderContents();
    };

    const navigateUp = async () => {
        if (!currentPath.value) {
            // At root of storage path, go back to storage path list
            currentStoragePathID.value = null;
            subfolders.value = [];
            scenes.value = [];
            totalScenes.value = 0;
            clearSelection();
            return;
        }

        // Go to parent folder
        const parts = currentPath.value.split('/').filter(Boolean);
        parts.pop();
        currentPath.value = parts.length > 0 ? '/' + parts.join('/') : '';
        page.value = 1;
        clearSelection();
        await loadFolderContents();
    };

    const navigateToBreadcrumb = async (path: string) => {
        currentPath.value = path;
        page.value = 1;
        clearSelection();
        await loadFolderContents();
    };

    const loadFolderContents = async () => {
        if (!currentStoragePathID.value) return;

        isLoading.value = true;
        error.value = null;
        try {
            const response: FolderContentsResponse = await getFolderContents(
                currentStoragePathID.value,
                currentPath.value,
                page.value,
                limit.value,
            );

            subfolders.value = response.subfolders;
            scenes.value = response.scenes;
            totalScenes.value = response.total_scenes;
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isLoading.value = false;
        }
    };

    const setPage = async (newPage: number) => {
        page.value = newPage;
        await loadFolderContents();
    };

    // Selection actions
    const toggleSceneSelection = (sceneID: number) => {
        if (selectedSceneIDs.value.has(sceneID)) {
            selectedSceneIDs.value.delete(sceneID);
        } else {
            selectedSceneIDs.value.add(sceneID);
        }
        // Trigger reactivity
        selectedSceneIDs.value = new Set(selectedSceneIDs.value);
    };

    const selectAllOnPage = () => {
        for (const scene of scenes.value) {
            selectedSceneIDs.value.add(scene.id);
        }
        selectedSceneIDs.value = new Set(selectedSceneIDs.value);
    };

    const selectAllInFolder = async () => {
        if (!currentStoragePathID.value) return;

        isSelectingAll.value = true;
        try {
            const response = await getFolderSceneIDs({
                storage_path_id: currentStoragePathID.value,
                folder_path: currentPath.value,
                recursive: false,
                // Pass active search filters so selection respects current view
                query: searchQuery.value || undefined,
                tag_ids: searchTags.value.length > 0 ? searchTags.value : undefined,
                actors: searchActors.value.length > 0 ? searchActors.value : undefined,
                has_porndb_id: searchHasPornDBID.value ?? undefined,
            });
            allFolderSceneIDs.value = response.scene_ids;
            selectedSceneIDs.value = new Set(response.scene_ids);
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isSelectingAll.value = false;
        }
    };

    const selectAllInFolderRecursive = async () => {
        if (!currentStoragePathID.value) return;

        isSelectingAll.value = true;
        try {
            const response = await getFolderSceneIDs({
                storage_path_id: currentStoragePathID.value,
                folder_path: currentPath.value,
                recursive: true,
                // Pass active search filters so selection respects current view
                query: searchQuery.value || undefined,
                tag_ids: searchTags.value.length > 0 ? searchTags.value : undefined,
                actors: searchActors.value.length > 0 ? searchActors.value : undefined,
                has_porndb_id: searchHasPornDBID.value ?? undefined,
            });
            allFolderSceneIDs.value = response.scene_ids;
            selectedSceneIDs.value = new Set(response.scene_ids);
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isSelectingAll.value = false;
        }
    };

    const dragSelect = (ids: number[], additive: boolean) => {
        if (additive) {
            for (const id of ids) selectedSceneIDs.value.add(id);
            selectedSceneIDs.value = new Set(selectedSceneIDs.value);
        } else {
            selectedSceneIDs.value = new Set(ids);
        }
    };

    const clearSelection = () => {
        selectedSceneIDs.value = new Set();
        allFolderSceneIDs.value = [];
    };

    // Search actions
    const performSearch = async () => {
        if (!currentStoragePathID.value) return;
        if (!isSearchActive.value) {
            // If no search criteria, load normal folder contents
            await loadFolderContents();
            return;
        }

        isSearching.value = true;
        error.value = null;
        try {
            const response = await searchInFolder({
                storage_path_id: currentStoragePathID.value,
                folder_path: currentPath.value,
                recursive: false,
                query: searchQuery.value,
                tag_ids: searchTags.value,
                actors: searchActors.value,
                has_porndb_id: searchHasPornDBID.value ?? undefined,
                page: page.value,
                limit: limit.value,
            });
            scenes.value = response.scenes;
            totalScenes.value = response.total;
            // Hide subfolders during search
            subfolders.value = [];
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isSearching.value = false;
        }
    };

    const clearSearch = async () => {
        searchQuery.value = '';
        searchTags.value = [];
        searchActors.value = [];
        searchHasPornDBID.value = null;
        page.value = 1;
        await loadFolderContents();
    };

    const isSceneSelected = (sceneID: number) => {
        return selectedSceneIDs.value.has(sceneID);
    };

    const getSelectedSceneIDs = () => {
        return Array.from(selectedSceneIDs.value);
    };

    // Reset state when leaving explorer
    const reset = () => {
        currentStoragePathID.value = null;
        currentPath.value = '';
        subfolders.value = [];
        scenes.value = [];
        totalScenes.value = 0;
        page.value = 1;
        clearSelection();
        error.value = null;
    };

    return {
        // State
        storagePaths,
        currentStoragePathID,
        currentPath,
        subfolders,
        scenes,
        totalScenes,
        page,
        limit,
        isLoading,
        error,
        selectedSceneIDs,
        searchQuery,
        searchTags,
        searchActors,
        searchHasPornDBID,
        isSearching,

        // Computed
        currentStoragePath,
        breadcrumbs,
        totalPages,
        hasSelection,
        selectionCount,
        allPageScenesSelected,
        allFolderScenesSelected,
        isSelectingAll,
        isSearchActive,

        // Actions
        loadStoragePaths,
        navigateToStoragePath,
        navigateToFolder,
        navigateUp,
        navigateToBreadcrumb,
        loadFolderContents,
        setPage,
        toggleSceneSelection,
        selectAllOnPage,
        selectAllInFolder,
        selectAllInFolderRecursive,
        dragSelect,
        clearSelection,
        isSceneSelected,
        getSelectedSceneIDs,
        performSearch,
        clearSearch,
        reset,
    };
});
