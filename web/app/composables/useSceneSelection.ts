import type { SceneListItem } from '~/types/scene';

export function useSceneSelection(pageScenes: Ref<SceneListItem[]> | ComputedRef<SceneListItem[]>) {
    const selectedSceneIDs = ref<Set<number>>(new Set());
    const isSelectingAll = ref(false);

    const hasSelection = computed(() => selectedSceneIDs.value.size > 0);
    const selectionCount = computed(() => selectedSceneIDs.value.size);

    const allPageScenesSelected = computed(() => {
        const scenes = toValue(pageScenes);
        if (scenes.length === 0) return false;
        return scenes.every((s) => selectedSceneIDs.value.has(s.id));
    });

    const toggleSceneSelection = (sceneId: number) => {
        const next = new Set(selectedSceneIDs.value);
        if (next.has(sceneId)) {
            next.delete(sceneId);
        } else {
            next.add(sceneId);
        }
        selectedSceneIDs.value = next;
    };

    const selectAllOnPage = () => {
        const next = new Set(selectedSceneIDs.value);
        for (const scene of toValue(pageScenes)) {
            next.add(scene.id);
        }
        selectedSceneIDs.value = next;
    };

    const selectAll = (ids: number[]) => {
        selectedSceneIDs.value = new Set(ids);
    };

    const clearSelection = () => {
        selectedSceneIDs.value = new Set();
    };

    const dragSelect = (ids: number[], additive: boolean) => {
        if (additive) {
            const next = new Set(selectedSceneIDs.value);
            for (const id of ids) next.add(id);
            selectedSceneIDs.value = next;
        } else {
            selectedSceneIDs.value = new Set(ids);
        }
    };

    const isSceneSelected = (id: number) => selectedSceneIDs.value.has(id);

    const getSelectedSceneIDs = () => [...selectedSceneIDs.value];

    return {
        selectedSceneIDs,
        isSelectingAll,
        hasSelection,
        selectionCount,
        allPageScenesSelected,
        toggleSceneSelection,
        selectAllOnPage,
        selectAll,
        dragSelect,
        clearSelection,
        isSceneSelected,
        getSelectedSceneIDs,
    };
}
