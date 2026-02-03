import { defineStore } from 'pinia';
import type { HomepageConfig, HomepageSectionData, HomepageResponse } from '~/types/homepage';

export const useHomepageStore = defineStore('homepage', () => {
    const config = ref<HomepageConfig | null>(null);
    const sectionsData = ref<Map<string, HomepageSectionData>>(new Map());
    const isLoading = ref(false);
    const error = ref<string | null>(null);

    const { fetchHomepageData, fetchSectionData } = useApiHomepage();

    const enabledSections = computed(() => {
        if (!config.value?.sections) return [];
        return [...config.value.sections]
            .filter((s) => s.enabled)
            .sort((a, b) => a.order - b.order);
    });

    const loadHomepage = async () => {
        isLoading.value = true;
        error.value = null;
        try {
            const data: HomepageResponse = await fetchHomepageData();
            config.value = data.config;

            // Store section data in map for quick access
            sectionsData.value = new Map();
            for (const section of data.sections) {
                sectionsData.value.set(section.section.id, section);
            }
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
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
        sectionsData.value = new Map();
        isLoading.value = false;
        error.value = null;
    };

    return {
        config,
        sectionsData,
        isLoading,
        error,
        enabledSections,
        loadHomepage,
        refreshSection,
        getSectionData,
        reset,
    };
});
