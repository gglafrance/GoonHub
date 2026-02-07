export const usePageSize = () => {
    const settingsStore = useSettingsStore();
    const limit = computed(() => settingsStore.videosPerPage);
    const showSelector = computed(() => settingsStore.showPageSizeSelector);
    const maxLimit = computed(() => settingsStore.maxItemsPerPage);

    const updatePageSize = async (newLimit: number) => {
        if (!settingsStore.settings) return;
        settingsStore.settings.videos_per_page = newLimit;
        if (settingsStore.draft) settingsStore.draft.videos_per_page = newLimit;
        await settingsStore.saveAllSettings();
    };

    return { limit, showSelector, maxLimit, updatePageSize };
};
