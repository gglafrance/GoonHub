import type { GridDensity } from '~/types/settings';

// Actor and studio grids
const ENTITY_GRID: Record<GridDensity, string> = {
    compact:
        'grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 2xl:grid-cols-9',
    comfortable: 'grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6',
    large: 'grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5',
};

/**
 * Grid density user setting (compact, comfortable, large) as grid classes and styles.
 */
export function useGridDensity() {
    const settingsStore = useSettingsStore();
    const density = computed(() => settingsStore.gridDensity);

    const sceneGridClass = computed(() => SCENE_GRID[density.value].class);
    // Card width of the `scene-grid` utility (grid-template-columns from 640px)
    const sceneGridStyle = computed(() => ({
        '--scene-card-width': `${SCENE_GRID[density.value].cardWidthPx}px`,
    }));
    const entityGridClass = computed(() => ENTITY_GRID[density.value]);

    return { density, sceneGridClass, sceneGridStyle, entityGridClass };
}
