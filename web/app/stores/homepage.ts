import { defineStore } from 'pinia';
import type {
    HomepageConfig,
    HomepageSectionData,
    HomepageResponse,
    HomepageTab,
} from '~/types/homepage';

const LAST_TAB_STORAGE_KEY = 'homepage-last-tab';
// Sections loaded during the visit are shown again as they were when coming back to the
// homepage (Back from a scene keeps the same random picks and the scroll position), unless
// they are older than this or the library data changed since.
const KEEP_SECTIONS_MS = 5 * 60 * 1000;

export const useHomepageStore = defineStore('homepage', () => {
    const config = ref<HomepageConfig | null>(null);
    const activeTabId = ref<string | null>(null);
    const sectionsData = ref<Map<string, HomepageSectionData>>(new Map());
    // Tabs whose sections were loaded during this visit
    const loadedTabs = ref<Set<string>>(new Set());
    const isLoading = ref(false);
    const error = ref<string | null>(null);
    let loadedAt = 0;
    let loadedVersion = -1;

    const { fetchHomepageData, fetchSectionData } = useApiHomepage();

    const tabs = computed<HomepageTab[]>(() => config.value?.tabs ?? []);

    const activeTab = computed<HomepageTab | null>(
        () => tabs.value.find((t) => t.id === activeTabId.value) ?? tabs.value[0] ?? null,
    );

    const enabledSections = computed(() => {
        if (!activeTab.value) return [];
        return [...activeTab.value.sections]
            .filter((s) => s.enabled)
            .sort((a, b) => a.order - b.order);
    });

    /** Tab to open when none is requested: last visited or the configured default. */
    const initialTabId = (cfg: HomepageConfig): string => {
        if (cfg.open_on === 'last' && import.meta.client) {
            const last = localStorage.getItem(LAST_TAB_STORAGE_KEY);
            if (last && cfg.tabs.some((t) => t.id === last)) return last;
        }
        return cfg.default_tab_id || cfg.tabs[0]?.id || '';
    };

    const applyResponse = (data: HomepageResponse) => {
        config.value = data.config;
        activeTabId.value = data.active_tab_id;
        for (const section of data.sections) {
            sectionsData.value.set(section.section.id, section);
        }
        loadedTabs.value.add(data.active_tab_id);
        if (import.meta.client) {
            localStorage.setItem(LAST_TAB_STORAGE_KEY, data.active_tab_id);
        }
    };

    /**
     * Loads the homepage. Without tabId, the tab follows the "open on" setting (which needs the
     * config: the first request then returns the default tab and is followed by a second one
     * when the last visited tab differs).
     */
    const loadHomepage = async (tabId?: string) => {
        isLoading.value = true;
        error.value = null;
        const version = currentDataVersion();
        try {
            sectionsData.value = new Map();
            loadedTabs.value = new Set();
            let data: HomepageResponse = await fetchHomepageData(tabId);
            if (!tabId) {
                const wanted = initialTabId(data.config);
                if (wanted && wanted !== data.active_tab_id) {
                    data = await fetchHomepageData(wanted);
                }
            }
            applyResponse(data);
            loadedAt = Date.now();
            loadedVersion = version;
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isLoading.value = false;
        }
    };

    /**
     * Opens the homepage: reuses the sections loaded earlier in the visit when they are still
     * fresh (no refetch flash, same random order), else loads it. The tab is the requested one,
     * else follows the "open on" setting, as on a first load.
     */
    const openHomepage = async (tabId?: string) => {
        const cfg = config.value;
        const fresh =
            !!cfg &&
            !error.value &&
            loadedVersion === currentDataVersion() &&
            Date.now() - loadedAt < KEEP_SECTIONS_MS;
        if (!fresh || (tabId && !cfg.tabs.some((t) => t.id === tabId))) {
            await loadHomepage(tabId);
            return;
        }
        await selectTab(tabId || initialTabId(cfg));
        // The scene just watched changes this one: refresh it in place
        for (const section of enabledSections.value) {
            if (section.type === 'continue_watching') refreshSection(section.id);
        }
    };

    /** Switches tab, loading its sections on first visit. */
    const selectTab = async (tabId: string) => {
        if (!config.value || tabId === activeTabId.value) return;
        activeTabId.value = tabId;
        if (import.meta.client) localStorage.setItem(LAST_TAB_STORAGE_KEY, tabId);
        if (loadedTabs.value.has(tabId)) return;
        isLoading.value = true;
        try {
            applyResponse(await fetchHomepageData(tabId));
        } catch (e: unknown) {
            error.value = e instanceof Error ? e.message : 'Unknown error';
        } finally {
            isLoading.value = false;
        }
    };

    const refreshSection = async (sectionId: string) => {
        try {
            const data = await fetchSectionData(sectionId);
            sectionsData.value.set(sectionId, data);
        } catch (e: unknown) {
            console.error('Failed to refresh section:', e);
        }
    };

    const getSectionData = (sectionId: string): HomepageSectionData | undefined => {
        return sectionsData.value.get(sectionId);
    };

    const reset = () => {
        loadedAt = 0;
        loadedVersion = -1;
        config.value = null;
        activeTabId.value = null;
        sectionsData.value = new Map();
        loadedTabs.value = new Set();
        isLoading.value = false;
        error.value = null;
    };

    return {
        config,
        tabs,
        activeTabId,
        activeTab,
        sectionsData,
        isLoading,
        error,
        enabledSections,
        loadHomepage,
        openHomepage,
        selectTab,
        refreshSection,
        getSectionData,
        reset,
    };
});
