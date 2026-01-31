import { defineStore } from 'pinia';
import type { UserSettings, SortOrder, TagSort } from '~/types/settings';

export const useSettingsStore = defineStore(
    'settings',
    () => {
        const settings = ref<UserSettings | null>(null);
        const isLoading = ref(false);
        const error = ref<string | null>(null);
        const theaterMode = ref(false);

        const {
            fetchSettings: apiFetchSettings,
            updatePlayerSettings: apiUpdatePlayer,
            updateAppSettings: apiUpdateApp,
            updateTagSettings: apiUpdateTags,
        } = useApi();

        const autoplay = computed(() => settings.value?.autoplay ?? false);
        const defaultVolume = computed(() => settings.value?.default_volume ?? 100);
        const loop = computed(() => settings.value?.loop ?? false);
        const videosPerPage = computed(() => settings.value?.videos_per_page ?? 20);
        const defaultSortOrder = computed<SortOrder>(
            () => settings.value?.default_sort_order ?? 'created_at_desc',
        );
        const defaultTagSort = computed<TagSort>(() => settings.value?.default_tag_sort ?? 'az');

        const loadSettings = async () => {
            isLoading.value = true;
            error.value = null;
            try {
                const data: UserSettings = await apiFetchSettings();
                settings.value = data;
            } catch (e: unknown) {
                const message = e instanceof Error ? e.message : 'Unknown error';
                if (message !== 'Unauthorized') {
                    error.value = message;
                }
            } finally {
                isLoading.value = false;
            }
        };

        const updatePlayer = async (autoplay: boolean, defaultVolume: number, loop: boolean) => {
            isLoading.value = true;
            error.value = null;
            try {
                const data: UserSettings = await apiUpdatePlayer({
                    autoplay,
                    default_volume: defaultVolume,
                    loop,
                });
                settings.value = data;
            } catch (e: unknown) {
                const message = e instanceof Error ? e.message : 'Unknown error';
                error.value = message;
                throw e;
            } finally {
                isLoading.value = false;
            }
        };

        const updateApp = async (videosPerPage: number, sortOrder: SortOrder) => {
            isLoading.value = true;
            error.value = null;
            try {
                const data: UserSettings = await apiUpdateApp({
                    videos_per_page: videosPerPage,
                    default_sort_order: sortOrder,
                });
                settings.value = data;
            } catch (e: unknown) {
                const message = e instanceof Error ? e.message : 'Unknown error';
                error.value = message;
                throw e;
            } finally {
                isLoading.value = false;
            }
        };

        const updateTags = async (tagSort: TagSort) => {
            isLoading.value = true;
            error.value = null;
            try {
                const data: UserSettings = await apiUpdateTags({
                    default_tag_sort: tagSort,
                });
                settings.value = data;
            } catch (e: unknown) {
                const message = e instanceof Error ? e.message : 'Unknown error';
                error.value = message;
                throw e;
            } finally {
                isLoading.value = false;
            }
        };

        const toggleTheaterMode = () => {
            theaterMode.value = !theaterMode.value;
        };

        return {
            settings,
            isLoading,
            error,
            autoplay,
            defaultVolume,
            loop,
            videosPerPage,
            defaultSortOrder,
            defaultTagSort,
            theaterMode,
            loadSettings,
            updatePlayer,
            updateApp,
            updateTags,
            toggleTheaterMode,
        };
    },
    {
        persist: {
            key: 'settings-store',
            storage: {
                getItem: (key) => {
                    if (import.meta.client) {
                        return sessionStorage.getItem(key);
                    }
                    return null;
                },
                setItem: (key, value) => {
                    if (import.meta.client) {
                        sessionStorage.setItem(key, value);
                    }
                },
            },
            pick: ['settings', 'theaterMode'],
        },
    },
);
