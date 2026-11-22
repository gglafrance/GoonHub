/**
 * Content width user setting (default, wide, full) as the classes of the page container of the
 * scene list pages.
 */
export function useContentWidth() {
    const settingsStore = useSettingsStore();
    const contentWidth = computed(() => settingsStore.contentWidth);
    const containerClass = computed(() => contentWidthClass(contentWidth.value));

    return { contentWidth, containerClass };
}
