import { defineStore } from 'pinia';
import type { SceneListItem } from '~/types/scene';
import type { StoragePathWithCount, FolderInfo, FolderContentsResponse } from '~/types/explorer';
import type { LoadedPages, PageLoadPlan } from '~/utils/infinitePages';

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

    // Infinite scroll (user setting): pages in the list and what they were loaded for
    const loadedPages = ref<LoadedPages>({ first: 0, last: 0 });
    const isLoadingMore = ref(false);
    let contentsRequest = 0;

    // Data version the list was (re)built at: contents loaded before an edit anywhere are not
    // shown again on Back (see useDataVersion and refresh())
    let loadedVersion = -1;

    /** Runs a page plan; silent: refreshes the list in place, without the loading states. */
    const runFolderPlan = async (plan: PageLoadPlan, silent = false) => {
        if (!currentStoragePathID.value) return;
        const storagePathID = currentStoragePathID.value;
        const folderPath = currentPath.value;
        const request = ++contentsRequest;
        const version = currentDataVersion();
        const partial = plan.mode !== 'replace';

        if (!silent) {
            if (partial) isLoadingMore.value = true;
            else isLoading.value = true;
        }
        error.value = null;
        try {
            const responses: FolderContentsResponse[] = await Promise.all(
                plan.pages.map((p) => getFolderContents(storagePathID, folderPath, p, limit.value)),
            );
            // A newer load replaced this one
            if (request !== contentsRequest) return;
            const last = responses[responses.length - 1]!;
            subfolders.value = last.subfolders;
            scenes.value = mergePageItems(
                scenes.value,
                responses.flatMap((r) => r.scenes),
                plan.mode,
            );
            totalScenes.value = last.total_scenes;
            loadedPages.value = commitPageLoad(loadedPages.value, plan);
            if (!partial) loadedVersion = version;
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized' && request === contentsRequest) {
                error.value = message;
            }
        } finally {
            if (request === contentsRequest) {
                isLoading.value = false;
                isLoadingMore.value = false;
            }
        }
    };

    const folderContext = () =>
        JSON.stringify(['folder', currentStoragePathID.value, currentPath.value, limit.value]);

    // Loads the current page (appended to the list when it is the next one with infinite scroll)
    const loadFolderContents = () =>
        runFolderPlan(
            planPageLoad(
                loadedPages.value,
                page.value,
                settingsStore.infiniteScroll,
                folderContext(),
            ),
        );

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
    const runSearchPlan = async (plan: PageLoadPlan, silent = false) => {
        if (!currentStoragePathID.value) return;
        const request = ++contentsRequest;
        const version = currentDataVersion();
        const partial = plan.mode !== 'replace';
        const criteria = {
            storage_path_id: currentStoragePathID.value,
            folder_path: currentPath.value,
            recursive: false,
            query: searchQuery.value,
            tag_ids: searchTags.value,
            actors: searchActors.value,
            has_porndb_id: searchHasPornDBID.value ?? undefined,
        };

        if (!silent) {
            if (partial) isLoadingMore.value = true;
            else isSearching.value = true;
        }
        error.value = null;
        try {
            const responses = await Promise.all(
                plan.pages.map((p) => searchInFolder({ ...criteria, page: p, limit: limit.value })),
            );
            if (request !== contentsRequest) return;
            scenes.value = mergePageItems(
                scenes.value,
                responses.flatMap((r) => r.scenes),
                plan.mode,
            );
            totalScenes.value = responses[responses.length - 1]?.total ?? 0;
            // Hide subfolders during search
            subfolders.value = [];
            loadedPages.value = commitPageLoad(loadedPages.value, plan);
            if (!partial) loadedVersion = version;
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized' && request === contentsRequest) {
                error.value = message;
            }
        } finally {
            if (request === contentsRequest) {
                isSearching.value = false;
                isLoadingMore.value = false;
            }
        }
    };

    const searchContext = () =>
        JSON.stringify([
            'search',
            currentStoragePathID.value,
            currentPath.value,
            searchQuery.value,
            searchTags.value,
            searchActors.value,
            searchHasPornDBID.value,
            limit.value,
        ]);

    const performSearch = async () => {
        if (!currentStoragePathID.value) return;
        if (!isSearchActive.value) {
            // If no search criteria, load normal folder contents
            await loadFolderContents();
            return;
        }
        await runSearchPlan(
            planPageLoad(
                loadedPages.value,
                page.value,
                settingsStore.infiniteScroll,
                searchContext(),
            ),
        );
    };

    /**
     * Loads the current view (folder contents or search results), as the explorer page does
     * after reading the URL. The list already held for this exact view (Back from a scene: same
     * folder, search and URL page, no edit since) stays on screen, with all its infinite scroll
     * pages, and is refreshed in place; the contents of another view are cleared first.
     */
    const refresh = async () => {
        const searching = isSearchActive.value;
        const context = searching ? searchContext() : folderContext();
        const held = loadedPages.value;
        const sameView =
            held.last > 0 &&
            (held.context ?? '') === context &&
            loadedVersion === currentDataVersion();
        const infinite = settingsStore.infiniteScroll;
        if (sameView && held.last === page.value && (infinite || held.first === held.last)) {
            const pages: number[] = [];
            for (let p = held.first; p <= held.last; p++) pages.push(p);
            // A long scrolled list is kept as is (no edit since): refreshing it would be heavy
            if (pages.length > INFINITE_REBUILD_PAGES) return;
            const plan: PageLoadPlan = { pages, mode: 'replace', context };
            await (searching ? runSearchPlan(plan, true) : runFolderPlan(plan, true));
            return;
        }
        if (!sameView) {
            subfolders.value = [];
            scenes.value = [];
            totalScenes.value = 0;
            loadedPages.value = { first: 0, last: 0 };
        }
        if (searching) await performSearch();
        else await loadFolderContents();
    };

    // Infinite scroll opened on a later page: loads the page before the list
    const loadPrevious = async () => {
        const plan = planPreviousPage(loadedPages.value);
        if (!plan) return;
        await (isSearchActive.value ? runSearchPlan(plan) : runFolderPlan(plan));
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
        loadedPages,
        isLoadingMore,
        loadPrevious,
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
        refresh,
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
