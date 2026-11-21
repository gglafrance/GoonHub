import { defineStore } from 'pinia';
import type {
    UserSettings,
    SortOrder,
    TagSort,
    KeyboardLayout,
    SortPreferences,
    SceneCardConfig,
    MarkerView,
    PaginationPosition,
    DateFormat,
    SceneListView,
    SceneListPage,
    SceneListViewMode,
    SceneTableColumn,
} from '~/types/settings';
import type { ParsingRulesSettings, ParsingPreset } from '~/types/parsing-rules';

export const useSettingsStore = defineStore(
    'settings',
    () => {
        const settings = ref<UserSettings | null>(null);
        const draft = ref<UserSettings | null>(null);
        const isLoading = ref(false);
        const error = ref<string | null>(null);
        const theaterMode = ref(false);
        const keyboardLayout = ref<KeyboardLayout>('qwerty');
        const parsingRules = ref<ParsingRulesSettings | null>(null);
        const parsingRulesLoading = ref(false);

        const {
            fetchSettings: apiFetchSettings,
            updateAllSettings: apiUpdateAllSettings,
            getParsingRules: apiGetParsingRules,
            updateParsingRules: apiUpdateParsingRules,
        } = useApi();

        const autoplay = computed(() => settings.value?.autoplay ?? false);
        const defaultVolume = computed(() => settings.value?.default_volume ?? 100);
        const loop = computed(() => settings.value?.loop ?? false);
        const abLoopControls = computed(() => settings.value?.ab_loop_controls ?? false);
        const markerLoop = computed(() => settings.value?.marker_loop ?? false);
        // Touch gestures on the player (on by default, also for share pages without settings)
        const playerGestures = computed(() => settings.value?.player_gestures ?? true);
        const gestureSeekStep = computed(() =>
            normalizeGestureSeekStep(settings.value?.gesture_seek_step),
        );
        const advancedMarkerFilters = computed(
            () => settings.value?.advanced_marker_filters ?? false,
        );
        const defaultMarkerView = computed<MarkerView>(
            () => settings.value?.default_marker_view ?? 'grouped',
        );
        const paginationPosition = computed<PaginationPosition>(
            () => settings.value?.pagination_position ?? 'bottom',
        );
        const dateFormat = computed<DateFormat>(() => {
            const format = settings.value?.date_format;
            return isDateFormat(format) ? format : DEFAULT_DATE_FORMAT;
        });
        const showReplayHeatmap = computed(() => settings.value?.show_replay_heatmap ?? true);
        const recordReplayHeatmap = computed(() => settings.value?.record_replay_heatmap ?? true);
        const videosPerPage = computed(() => settings.value?.videos_per_page ?? 20);
        const defaultSortOrder = computed<SortOrder>(
            () => settings.value?.default_sort_order ?? 'created_at_desc',
        );
        const defaultTagSort = computed<TagSort>(() => settings.value?.default_tag_sort ?? 'az');
        const markerThumbnailCycling = computed(
            () => settings.value?.marker_thumbnail_cycling ?? true,
        );
        const sortPreferences = computed<SortPreferences | null>(
            () => settings.value?.sort_preferences ?? null,
        );
        const showPageSizeSelector = computed(
            () => settings.value?.show_page_size_selector ?? false,
        );
        const maxItemsPerPage = computed(() => settings.value?.max_items_per_page ?? 100);

        const defaultSceneCardConfig: SceneCardConfig = {
            badges: {
                top_left: { items: ['rating'], direction: 'vertical' },
                top_right: { items: ['watched'], direction: 'vertical' },
                bottom_left: { items: [], direction: 'vertical' },
                bottom_right: { items: ['duration'], direction: 'horizontal' },
            },
            content_rows: [{ type: 'split', left: 'file_size', right: 'added_at' }],
        };

        const sceneCardConfig = computed<SceneCardConfig>(
            () => settings.value?.scene_card_config ?? defaultSceneCardConfig,
        );

        // Build comma-separated card_fields param from the card config
        const cardFieldsParam = computed(() => {
            const config = sceneCardConfig.value;
            const fieldsSet = new Set<string>();

            // Badge field name -> API card_fields name mapping
            const badgeFieldMap: Record<string, string> = {
                views: 'views',
                resolution: 'resolution',
                rating: 'rating',
                liked: 'liked',
                jizz_count: 'jizz_count',
                frame_rate: 'frame_rate',
                tags: 'tags',
                actors: 'actors',
            };

            // Content field name -> API card_fields name mapping
            const contentFieldMap: Record<string, string> = {
                views: 'views',
                resolution: 'resolution',
                frame_rate: 'frame_rate',
                description: 'description',
                studio: 'studio',
                tags: 'tags',
                actors: 'actors',
                rating: 'rating',
                jizz_count: 'jizz_count',
            };

            // Scan badge zones
            const zones = [
                config.badges.top_left,
                config.badges.top_right,
                config.badges.bottom_left,
                config.badges.bottom_right,
            ];
            for (const zone of zones) {
                for (const item of zone.items) {
                    if (badgeFieldMap[item]) fieldsSet.add(badgeFieldMap[item]);
                }
            }

            // Scan content rows
            for (const row of config.content_rows) {
                if (row.type === 'full' && row.field) {
                    if (contentFieldMap[row.field]) fieldsSet.add(contentFieldMap[row.field]);
                } else if (row.type === 'split') {
                    if (row.left && contentFieldMap[row.left])
                        fieldsSet.add(contentFieldMap[row.left]);
                    if (row.right && contentFieldMap[row.right])
                        fieldsSet.add(contentFieldMap[row.right]);
                }
            }

            return fieldsSet.size > 0 ? [...fieldsSet].join(',') : '';
        });

        const sceneListView = computed<SceneListView>(() => {
            const view = settings.value?.scene_list_view;
            return {
                pages: view?.pages ?? {},
                columns: view?.columns?.length ? view.columns : [...DEFAULT_SCENE_TABLE_COLUMNS],
            };
        });

        const sceneListMode = (page: SceneListPage): SceneListViewMode =>
            sceneListView.value.pages[page] ?? 'grid';

        // card_fields of the scene list pages: the card fields, plus the fields of the visible
        // table columns when a page shows a table
        const listFieldsParam = computed(() => {
            const fields = new Set(cardFieldsParam.value ? cardFieldsParam.value.split(',') : []);
            const view = sceneListView.value;
            if (Object.values(view.pages).includes('table')) {
                for (const col of view.columns) {
                    const field = sceneTableColumnDef(col)?.field;
                    if (field) fields.add(field);
                }
            }
            return [...fields].join(',');
        });

        // Saves a new scene list view right away (view toggles and column pickers)
        const saveSceneListView = async (view: SceneListView) => {
            if (!settings.value) return;
            settings.value.scene_list_view = view;
            if (draft.value) draft.value.scene_list_view = JSON.parse(JSON.stringify(view));
            try {
                await saveAllSettings();
            } catch (e: unknown) {
                console.error('Failed to save the scene list view:', e);
            }
        };

        const setSceneListMode = (page: SceneListPage, mode: SceneListViewMode) => {
            const view = sceneListView.value;
            return saveSceneListView({ ...view, pages: { ...view.pages, [page]: mode } });
        };

        const setSceneTableColumns = (columns: SceneTableColumn[]) =>
            saveSceneListView({ ...sceneListView.value, columns: [...columns] });

        const hasUnsavedChanges = computed(() => {
            if (!draft.value || !settings.value) return false;
            return JSON.stringify(draft.value) !== JSON.stringify(settings.value);
        });

        function initDraft() {
            if (settings.value) {
                draft.value = JSON.parse(JSON.stringify(settings.value));
            }
        }

        function discardDraft() {
            initDraft();
        }

        const loadSettings = async () => {
            isLoading.value = true;
            error.value = null;
            try {
                const data: UserSettings = await apiFetchSettings();
                settings.value = data;
                initDraft();
            } catch (e: unknown) {
                const message = e instanceof Error ? e.message : 'Unknown error';
                if (message !== 'Unauthorized') {
                    error.value = message;
                }
            } finally {
                isLoading.value = false;
            }
        };

        const saveAllSettings = async () => {
            if (!draft.value) return;
            isLoading.value = true;
            error.value = null;
            try {
                const data: UserSettings = await apiUpdateAllSettings(draft.value);
                settings.value = data;
                initDraft();
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
            draft,
            isLoading,
            error,
            autoplay,
            defaultVolume,
            loop,
            abLoopControls,
            markerLoop,
            playerGestures,
            gestureSeekStep,
            advancedMarkerFilters,
            defaultMarkerView,
            paginationPosition,
            dateFormat,
            showReplayHeatmap,
            recordReplayHeatmap,
            videosPerPage,
            defaultSortOrder,
            defaultTagSort,
            markerThumbnailCycling,
            sortPreferences,
            showPageSizeSelector,
            maxItemsPerPage,
            sceneCardConfig,
            cardFieldsParam,
            sceneListView,
            sceneListMode,
            listFieldsParam,
            setSceneListMode,
            setSceneTableColumns,
            hasUnsavedChanges,
            theaterMode,
            keyboardLayout,
            parsingRules,
            parsingRulesLoading,
            activePreset,
            loadSettings,
            saveAllSettings,
            initDraft,
            discardDraft,
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
