import { defineStore } from 'pinia';
import type { UserSettings, SortOrder, TagSort, KeyboardLayout } from '~/types/settings';

export const useSettingsStore = defineStore(
    'settings',
    () => {
        const settings = ref<UserSettings | null>(null);
        const isLoading = ref(false);
        const error = ref<string | null>(null);
        const theaterMode = ref(false);
        const keyboardLayout = ref<KeyboardLayout>('qwerty');

        const {
            fetchSettings: apiFetchSettings,
            updatePlayerSettings: apiUpdatePlayer,
            updateAppSettings: apiUpdateApp,
            updateTagSettings: apiUpdateTags,
        } = useApi();

        const autoplay = computed(() => settings.value?.autoplay ?? false);
        const defaultVolume = computed(() => settings.value?.default_volume ?? 100);
        const loop = computed(() => settings.value?.loop ?? false);
        const scenesPerPage = computed(() => settings.value?.scenes_per_page ?? 20);
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

        const updateApp = async (scenesPerPage: number, sortOrder: SortOrder) => {
            isLoading.value = true;
            error.value = null;
            try {
                const data: UserSettings = await apiUpdateApp({
                    scenes_per_page: scenesPerPage,
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

        const setKeyboardLayout = (layout: KeyboardLayout) => {
            keyboardLayout.value = layout;
        };

        return {
            settings,
            isLoading,
            error,
            autoplay,
            defaultVolume,
            loop,
            scenesPerPage,
            defaultSortOrder,
            defaultTagSort,
            theaterMode,
            keyboardLayout,
            loadSettings,
            updatePlayer,
            updateApp,
            updateTags,
            toggleTheaterMode,
            setKeyboardLayout,
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
            pick: ['settings', 'theaterMode', 'keyboardLayout'],
        },
    },
);
