import { defineStore } from 'pinia';
import type { Video } from '~/types/video';
import type {
    StoragePathWithCount,
    FolderInfo,
    FolderContentsResponse,
} from '~/types/explorer';

export interface Breadcrumb {
    name: string;
    path: string;
}

export const useExplorerStore = defineStore('explorer', () => {
    // State
    const storagePaths = ref<StoragePathWithCount[]>([]);
    const currentStoragePathID = ref<number | null>(null);
    const currentPath = ref('');
    const subfolders = ref<FolderInfo[]>([]);
    const videos = ref<Video[]>([]);
    const totalVideos = ref(0);
    const page = ref(1);
    const limit = ref(24);
    const isLoading = ref(false);
    const error = ref<string | null>(null);

    // Selection state
    const selectedVideoIDs = ref<Set<number>>(new Set());

    // API
    const { getStoragePaths, getFolderContents, getFolderVideoIDs, searchInFolder } = useApiExplorer();

    // Track if all folder videos are selected (across all pages)
    const allFolderVideoIDs = ref<number[]>([]);
    const isSelectingAll = ref(false);

    // Search state
    const searchQuery = ref('');
    const searchTags = ref<number[]>([]);
    const searchActors = ref<string[]>([]);
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

    const totalPages = computed(() => Math.ceil(totalVideos.value / limit.value));

    const hasSelection = computed(() => selectedVideoIDs.value.size > 0);
    const selectionCount = computed(() => selectedVideoIDs.value.size);
    const allPageVideosSelected = computed(
        () => videos.value.length > 0 && videos.value.every((v) => selectedVideoIDs.value.has(v.id)),
    );
    const allFolderVideosSelected = computed(
        () => totalVideos.value > 0 && selectedVideoIDs.value.size === totalVideos.value,
    );

    const isSearchActive = computed(
        () => searchQuery.value.length > 0 || searchTags.value.length > 0 || searchActors.value.length > 0,
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
            videos.value = [];
            totalVideos.value = 0;
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
            videos.value = response.videos;
            totalVideos.value = response.total_videos;
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
    const toggleVideoSelection = (videoID: number) => {
        if (selectedVideoIDs.value.has(videoID)) {
            selectedVideoIDs.value.delete(videoID);
        } else {
            selectedVideoIDs.value.add(videoID);
        }
        // Trigger reactivity
        selectedVideoIDs.value = new Set(selectedVideoIDs.value);
    };

    const selectAllOnPage = () => {
        for (const video of videos.value) {
            selectedVideoIDs.value.add(video.id);
        }
        selectedVideoIDs.value = new Set(selectedVideoIDs.value);
    };

    const selectAllInFolder = async () => {
        if (!currentStoragePathID.value) return;

        isSelectingAll.value = true;
        try {
            const response = await getFolderVideoIDs({
                storage_path_id: currentStoragePathID.value,
                folder_path: currentPath.value,
                recursive: false,
            });
            allFolderVideoIDs.value = response.video_ids;
            selectedVideoIDs.value = new Set(response.video_ids);
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
            const response = await getFolderVideoIDs({
                storage_path_id: currentStoragePathID.value,
                folder_path: currentPath.value,
                recursive: true,
            });
            allFolderVideoIDs.value = response.video_ids;
            selectedVideoIDs.value = new Set(response.video_ids);
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isSelectingAll.value = false;
        }
    };

    const clearSelection = () => {
        selectedVideoIDs.value = new Set();
        allFolderVideoIDs.value = [];
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
                page: page.value,
                limit: limit.value,
            });
            videos.value = response.videos;
            totalVideos.value = response.total;
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
        page.value = 1;
        await loadFolderContents();
    };

    const isVideoSelected = (videoID: number) => {
        return selectedVideoIDs.value.has(videoID);
    };

    const getSelectedVideoIDs = () => {
        return Array.from(selectedVideoIDs.value);
    };

    // Reset state when leaving explorer
    const reset = () => {
        currentStoragePathID.value = null;
        currentPath.value = '';
        subfolders.value = [];
        videos.value = [];
        totalVideos.value = 0;
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
        videos,
        totalVideos,
        page,
        limit,
        isLoading,
        error,
        selectedVideoIDs,
        searchQuery,
        searchTags,
        searchActors,
        isSearching,

        // Computed
        currentStoragePath,
        breadcrumbs,
        totalPages,
        hasSelection,
        selectionCount,
        allPageVideosSelected,
        allFolderVideosSelected,
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
        toggleVideoSelection,
        selectAllOnPage,
        selectAllInFolder,
        selectAllInFolderRecursive,
        clearSelection,
        isVideoSelected,
        getSelectedVideoIDs,
        performSearch,
        clearSearch,
        reset,
    };
});
