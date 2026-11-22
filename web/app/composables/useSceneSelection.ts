import type { SceneListItem } from '~/types/scene';

export function useSceneSelection(pageScenes: Ref<SceneListItem[]> | ComputedRef<SceneListItem[]>) {
    const selectedSceneIDs = ref<Set<number>>(new Set());
    const isSelectingAll = ref(false);
    // Last toggled scene: a shift-click selects from it to the clicked scene
    let anchorId: number | null = null;

    const hasSelection = computed(() => selectedSceneIDs.value.size > 0);
    const selectionCount = computed(() => selectedSceneIDs.value.size);

    const allPageScenesSelected = computed(() => {
        const scenes = toValue(pageScenes);
        if (scenes.length === 0) return false;
        return scenes.every((s) => selectedSceneIDs.value.has(s.id));
    });

    const toggleSceneSelection = (sceneId: number, range = false) => {
        const ids = toValue(pageScenes).map((s) => s.id);
        selectedSceneIDs.value = toggleSceneInSelection(
            selectedSceneIDs.value,
            ids,
            sceneId,
            anchorId,
            range,
        );
        anchorId = sceneId;
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
        anchorId = null;
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
