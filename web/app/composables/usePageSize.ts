export const usePageSize = () => {
    const settingsStore = useSettingsStore();
    const limit = computed(() => settingsStore.videosPerPage);
    const showSelector = computed(() => settingsStore.showPageSizeSelector);
    const maxLimit = computed(() => settingsStore.maxItemsPerPage);

    const updatePageSize = async (newLimit: number) => {
        await settingsStore.saveSettingsNow({ videos_per_page: newLimit });
    };

    return { limit, showSelector, maxLimit, updatePageSize };
};
