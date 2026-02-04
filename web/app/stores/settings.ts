import { defineStore } from 'pinia';
import type { UserSettings, SortOrder, TagSort, KeyboardLayout } from '~/types/settings';
import type { ParsingRulesSettings, ParsingPreset } from '~/types/parsing-rules';

export const useSettingsStore = defineStore(
    'settings',
    () => {
        const settings = ref<UserSettings | null>(null);
        const isLoading = ref(false);
        const error = ref<string | null>(null);
        const theaterMode = ref(false);
        const keyboardLayout = ref<KeyboardLayout>('qwerty');
        const parsingRules = ref<ParsingRulesSettings | null>(null);
        const parsingRulesLoading = ref(false);

        const {
            fetchSettings: apiFetchSettings,
            updatePlayerSettings: apiUpdatePlayer,
            updateAppSettings: apiUpdateApp,
            updateTagSettings: apiUpdateTags,
            getParsingRules: apiGetParsingRules,
            updateParsingRules: apiUpdateParsingRules,
        } = useApi();

        const autoplay = computed(() => settings.value?.autoplay ?? false);
        const defaultVolume = computed(() => settings.value?.default_volume ?? 100);
        const loop = computed(() => settings.value?.loop ?? false);
        const videosPerPage = computed(() => settings.value?.videos_per_page ?? 20);
        const defaultSortOrder = computed<SortOrder>(
            () => settings.value?.default_sort_order ?? 'created_at_desc',
        );
        const defaultTagSort = computed<TagSort>(() => settings.value?.default_tag_sort ?? 'az');
        const markerThumbnailCycling = computed(
            () => settings.value?.marker_thumbnail_cycling ?? true,
        );

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

        const updateApp = async (
            videosPerPage: number,
            sortOrder: SortOrder,
            markerThumbnailCyclingVal: boolean,
        ) => {
            isLoading.value = true;
            error.value = null;
            try {
                const data: UserSettings = await apiUpdateApp({
                    videos_per_page: videosPerPage,
                    default_sort_order: sortOrder,
                    marker_thumbnail_cycling: markerThumbnailCyclingVal,
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

        // Parsing rules
        const activePreset = computed<ParsingPreset | null>(() => {
            if (!parsingRules.value || !parsingRules.value.activePresetId) return null;
            return (
                parsingRules.value.presets.find(
                    (p) => p.id === parsingRules.value!.activePresetId,
                ) || null
            );
        });

        const loadParsingRules = async () => {
            parsingRulesLoading.value = true;
            try {
                const data = await apiGetParsingRules();
                parsingRules.value = data;
            } catch (e: unknown) {
                const message = e instanceof Error ? e.message : 'Unknown error';
                if (message !== 'Unauthorized') {
                    console.error('Failed to load parsing rules:', message);
                }
            } finally {
                parsingRulesLoading.value = false;
            }
        };

        const saveParsingRules = async (rules: ParsingRulesSettings) => {
            parsingRulesLoading.value = true;
            try {
                const data = await apiUpdateParsingRules(rules);
                parsingRules.value = data;
            } catch (e: unknown) {
                const message = e instanceof Error ? e.message : 'Unknown error';
                error.value = message;
                throw e;
            } finally {
                parsingRulesLoading.value = false;
            }
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
            markerThumbnailCycling,
            theaterMode,
            keyboardLayout,
            parsingRules,
            parsingRulesLoading,
            activePreset,
            loadSettings,
            updatePlayer,
            updateApp,
            updateTags,
            toggleTheaterMode,
            setKeyboardLayout,
            loadParsingRules,
            saveParsingRules,
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
