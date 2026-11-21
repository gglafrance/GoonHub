import type { SceneListPage, SceneListViewMode, SceneTableColumn } from '~/types/settings';

/**
 * Grid or table display of a scene list page, saved per page type in the user settings.
 *
 * @param page - The page type
 * @param onFieldsChange - Called when the card_fields of the lists change (a table with new
 *   columns was turned on), so the page reloads its scenes with the missing fields
 */
export function useSceneListView(page: SceneListPage, onFieldsChange?: () => void) {
    const settingsStore = useSettingsStore();

    const mode = computed<SceneListViewMode>({
        get: () => settingsStore.sceneListMode(page),
        set: (value) => {
            if (value !== settingsStore.sceneListMode(page)) {
                settingsStore.setSceneListMode(page, value);
            }
        },
    });

    const isTable = computed(() => mode.value === 'table');
    const columns = computed(() => settingsStore.sceneListView.columns);

    const setColumns = (value: SceneTableColumn[]) => settingsStore.setSceneTableColumns(value);

    if (onFieldsChange) {
        watch(
            () => settingsStore.listFieldsParam,
            (next, prev) => {
                const before = new Set(prev ? prev.split(',') : []);
                // Only reload when a field is missing (fewer fields: the data is still fine)
                if (next.split(',').some((f) => f && !before.has(f))) onFieldsChange();
            },
        );
    }

    return { mode, isTable, columns, setColumns };
}
