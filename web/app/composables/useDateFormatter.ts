import type { DateInput, FormatDateOptions } from '~/utils/dateFormat';

/**
 * Formats dates with the user's date format setting (Settings > App).
 * Falls back to the browser locale when settings are not loaded (e.g. public share page).
 */
export function useDateFormatter() {
    const settingsStore = useSettingsStore();
    const dateFormat = computed(() => settingsStore.dateFormat);

    const formatDate = (value: DateInput, options?: FormatDateOptions) =>
        formatDateWith(value, dateFormat.value, options);

    const formatDateTime = (value: DateInput, options?: FormatDateOptions) =>
        formatDateWith(value, dateFormat.value, { ...options, time: true });

    return { dateFormat, formatDate, formatDateTime };
}
