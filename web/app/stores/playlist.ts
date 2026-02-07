import type {
    PlaylistListItem,
    PlaylistDetail,
    CreatePlaylistInput,
    UpdatePlaylistInput,
    PlaylistSortOption,
    PlaylistOwnerFilter,
    PlaylistVisibilityFilter,
} from '~/types/playlist';

export const usePlaylistStore = defineStore('playlist', () => {
    const api = useApiPlaylists();
    const settingsStore = useSettingsStore();

    // List state
    const playlists = ref<PlaylistListItem[]>([]);
    const total = ref(0);
    const currentPage = ref(1);
    const limit = computed(() => settingsStore.videosPerPage);
    const isLoading = ref(false);
    const error = ref('');

    // Detail state
    const currentPlaylist = ref<PlaylistDetail | null>(null);
    const isLoadingDetail = ref(false);
    const detailError = ref('');

    // Filters
    const ownerFilter = ref<PlaylistOwnerFilter>('me');
    const visibilityFilter = ref<PlaylistVisibilityFilter>('');
    const sortOrder = ref<PlaylistSortOption>('created_at_desc');
    const tagFilter = ref<number[]>([]);
    const searchQuery = ref('');

    const totalPages = computed(() => Math.ceil(total.value / limit.value));

    const loadPlaylists = async () => {
        isLoading.value = true;
        error.value = '';

        try {
            const params: Record<string, string | number | undefined> = {
                page: currentPage.value,
                limit: limit.value,
                owner: ownerFilter.value,
                sort: sortOrder.value,
            };

            if (visibilityFilter.value) {
                params.visibility = visibilityFilter.value;
            }
            if (tagFilter.value.length > 0) {
                params.tag_ids = tagFilter.value.join(',');
            }
            if (searchQuery.value) {
                params.search = searchQuery.value;
            }

            const result = await api.fetchPlaylists(params);
            playlists.value = result.data;
            total.value = result.pagination.total_items;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to load playlists';
        } finally {
            isLoading.value = false;
        }
    };

    const loadPlaylist = async (uuid: string) => {
        isLoadingDetail.value = true;
        detailError.value = '';

        try {
            currentPlaylist.value = await api.fetchPlaylist(uuid);
        } catch (e: unknown) {
            detailError.value = e instanceof Error ? e.message : 'Failed to load playlist';
        } finally {
            isLoadingDetail.value = false;
        }
    };

    const createPlaylist = async (input: CreatePlaylistInput): Promise<PlaylistListItem | null> => {
        try {
            const result = await api.createPlaylist(input);
            await loadPlaylists();
            return result;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to create playlist';
            return null;
        }
    };

    const updatePlaylist = async (uuid: string, input: UpdatePlaylistInput): Promise<boolean> => {
        try {
            const result = await api.updatePlaylist(uuid, input);
            // Update in list if present
            const idx = playlists.value.findIndex((p) => p.uuid === uuid);
            if (idx !== -1) {
                playlists.value[idx] = { ...playlists.value[idx], ...result };
            }
            // Update current detail if viewing
            if (currentPlaylist.value?.uuid === uuid) {
                currentPlaylist.value = { ...currentPlaylist.value, ...result };
            }
            return true;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to update playlist';
            return false;
        }
    };

    const deletePlaylist = async (uuid: string): Promise<boolean> => {
        try {
            await api.deletePlaylist(uuid);
            playlists.value = playlists.value.filter((p) => p.uuid !== uuid);
            total.value--;
            if (currentPlaylist.value?.uuid === uuid) {
                currentPlaylist.value = null;
            }
            return true;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to delete playlist';
            return false;
        }
    };

    const addScenes = async (uuid: string, sceneIDs: number[]): Promise<boolean> => {
        try {
            await api.addScenes(uuid, sceneIDs);
            // Reload detail if viewing this playlist
            if (currentPlaylist.value?.uuid === uuid) {
                await loadPlaylist(uuid);
            }
            return true;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to add scenes';
            return false;
        }
    };

    const removeScene = async (uuid: string, sceneID: number): Promise<boolean> => {
        try {
            await api.removeScene(uuid, sceneID);
            if (currentPlaylist.value?.uuid === uuid) {
                currentPlaylist.value.scenes = currentPlaylist.value.scenes.filter(
                    (s) => s.scene.id !== sceneID,
                );
                currentPlaylist.value.scene_count--;
            }
            return true;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to remove scene';
            return false;
        }
    };

    const removeScenes = async (uuid: string, sceneIDs: number[]): Promise<boolean> => {
        try {
            await api.removeScenes(uuid, sceneIDs);
            if (currentPlaylist.value?.uuid === uuid) {
                const removedSet = new Set(sceneIDs);
                currentPlaylist.value.scenes = currentPlaylist.value.scenes.filter(
                    (s) => !removedSet.has(s.scene.id),
                );
                currentPlaylist.value.scene_count -= sceneIDs.length;
            }
            return true;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to remove scenes';
            return false;
        }
    };

    const reorderScenes = async (uuid: string, sceneIDs: number[]): Promise<boolean> => {
        try {
            await api.reorderScenes(uuid, sceneIDs);
            if (currentPlaylist.value?.uuid === uuid) {
                await loadPlaylist(uuid);
            }
            return true;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to reorder scenes';
            return false;
        }
    };

    const toggleLike = async (uuid: string): Promise<boolean | null> => {
        try {
            const result = await api.toggleLike(uuid);
            // Update in list
            const idx = playlists.value.findIndex((p) => p.uuid === uuid);
            if (idx !== -1 && playlists.value[idx]) {
                playlists.value[idx].is_liked = result.liked;
                playlists.value[idx].like_count += result.liked ? 1 : -1;
            }
            // Update detail
            if (currentPlaylist.value?.uuid === uuid) {
                currentPlaylist.value.is_liked = result.liked;
                currentPlaylist.value.like_count += result.liked ? 1 : -1;
            }
            return result.liked;
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Failed to toggle like';
            return null;
        }
    };

    const updateProgress = async (
        uuid: string,
        sceneID: number,
        positionS: number,
    ): Promise<boolean> => {
        try {
            await api.updateProgress(uuid, sceneID, positionS);
            return true;
        } catch {
            return false;
        }
    };

    const resetFilters = () => {
        ownerFilter.value = 'me';
        visibilityFilter.value = '';
        sortOrder.value = 'created_at_desc';
        tagFilter.value = [];
        searchQuery.value = '';
        currentPage.value = 1;
    };

    return {
        // List state
        playlists,
        total,
        currentPage,
        limit,
        isLoading,
        error,
        totalPages,

        // Detail state
        currentPlaylist,
        isLoadingDetail,
        detailError,

        // Filters
        ownerFilter,
        visibilityFilter,
        sortOrder,
        tagFilter,
        searchQuery,

        // Actions
        loadPlaylists,
        loadPlaylist,
        createPlaylist,
        updatePlaylist,
        deletePlaylist,
        addScenes,
        removeScene,
        removeScenes,
        reorderScenes,
        toggleLike,
        updateProgress,
        resetFilters,
    };
});
