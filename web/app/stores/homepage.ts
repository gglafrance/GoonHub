import { defineStore } from 'pinia';
import type {
    HomepageConfig,
    HomepageSectionData,
    HomepageResponse,
    HomepageTab,
} from '~/types/homepage';

const LAST_TAB_STORAGE_KEY = 'homepage-last-tab';

export const useHomepageStore = defineStore('homepage', () => {
    const config = ref<HomepageConfig | null>(null);
    const activeTabId = ref<string | null>(null);
    const sectionsData = ref<Map<string, HomepageSectionData>>(new Map());
    // Tabs whose sections were loaded during this visit
    const loadedTabs = ref<Set<string>>(new Set());
    const isLoading = ref(false);
    const error = ref<string | null>(null);

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
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isLoading.value = false;
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
        selectTab,
        refreshSection,
        getSectionData,
        reset,
    };
});
