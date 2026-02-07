import { defineStore } from 'pinia';
import type { SceneListItem, SceneListResponse } from '~/types/scene';

export const useSceneStore = defineStore('scenes', () => {
    const scenes = ref<SceneListItem[]>([]);
    const total = ref(0);
    const currentPage = ref(1);
    const isLoading = ref(false);
    const error = ref<string | null>(null);

    // Interaction sidecar maps from card_fields
    const ratings = ref<Record<string, number>>({});
    const likes = ref<Record<string, boolean>>({});
    const jizzCounts = ref<Record<string, number>>({});

    const settingsStore = useSettingsStore();
    const { fetchScenes: apiFetchScenes, uploadScene: apiUploadScene } = useApi();

    const limit = computed(() => settingsStore.videosPerPage);

    const loadScenes = async (page = 1) => {
        isLoading.value = true;
        error.value = null;
        try {
            const response: SceneListResponse = await apiFetchScenes(page, limit.value);
            scenes.value = response.data;
            total.value = response.total;
            currentPage.value = response.page;
            ratings.value = response.ratings || {};
            likes.value = response.likes || {};
            jizzCounts.value = response.jizz_counts || {};
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
        } finally {
            isLoading.value = false;
        }
    };

    const uploadScene = async (file: File, title?: string) => {
        isLoading.value = true;
        error.value = null;
        try {
            await apiUploadScene(file, title);
            await loadScenes(1);
        } catch (e: unknown) {
            const message = e instanceof Error ? e.message : 'Unknown error';
            if (message !== 'Unauthorized') {
                error.value = message;
            }
            throw e;
        } finally {
            isLoading.value = false;
        }
    };

    const updateSceneFields = (sceneId: number, fields: Partial<SceneListItem>) => {
        const idx = scenes.value.findIndex((s) => s.id === sceneId);
        if (idx !== -1) {
            scenes.value[idx] = { ...scenes.value[idx], ...fields } as SceneListItem;
        }
    };

    const prependScene = (scene: SceneListItem) => {
        if (currentPage.value === 1) {
            const exists = scenes.value.some((s) => s.id === scene.id);
            if (!exists) {
                scenes.value.unshift(scene);
                total.value++;
                if (scenes.value.length > limit.value) {
                    scenes.value.pop();
                }
            }
        }
    };

    const removeScene = (sceneId: number) => {
        const idx = scenes.value.findIndex((s) => s.id === sceneId);
        if (idx !== -1) {
            scenes.value.splice(idx, 1);
            total.value = Math.max(0, total.value - 1);
        }
    };

    return {
        scenes,
        total,
        currentPage,
        limit,
        isLoading,
        error,
        ratings,
        likes,
        jizzCounts,
        loadScenes,
        uploadScene,
        updateSceneFields,
        prependScene,
        removeScene,
    };
});
