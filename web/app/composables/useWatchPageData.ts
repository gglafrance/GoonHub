/**
 * Centralized data loading composable for the watch page.
 *
 * Orchestrates all API requests with explicit priority tiers:
 * - P0: Scene (critical - blocks rendering)
 * - P1: Markers + Resume position (player experience)
 * - P2: Interactions + Studio + Tags + Actors (details tab)
 * - P3: Related scenes + PornDB status (below fold / admin only)
 *
 * Child components inject data instead of fetching independently,
 * eliminating network congestion and ensuring predictable load order.
 */
import type { Scene, SceneListItem } from '~/types/scene';
import type { Marker } from '~/types/marker';
import type { Studio } from '~/types/studio';
import type { Tag } from '~/types/tag';
import type { Actor } from '~/types/actor';

export interface SceneInteractions {
    rating: number;
    liked: boolean;
    jizzed_count: number;
}

export interface LoadingState {
    scene: boolean;
    player: boolean;
    details: boolean;
    related: boolean;
}

export interface WatchPageData {
    // Core data
    scene: Ref<Scene | null>;
    markers: Ref<Marker[]>;
    resumePosition: Ref<number>;

    // Details data
    interactions: Ref<SceneInteractions | null>;
    studio: Ref<Studio | null>;
    tags: Ref<Tag[]>;
    actors: Ref<Actor[]>;

    // Below fold
    relatedScenes: Ref<SceneListItem[]>;
    pornDBConfigured: Ref<boolean>;

    // Loading states per tier (reactive object for proper template unwrapping)
    loading: LoadingState;

    // Error state
    error: Ref<string | null>;

    // Actions for child components
    refreshMarkers: () => Promise<void>;
    refreshStudio: () => Promise<void>;
    refreshTags: () => Promise<void>;
    refreshActors: () => Promise<void>;
    refreshInteractions: () => Promise<void>;
    refreshAll: () => Promise<void>;

    // Setters for optimistic updates from child components
    setScene: (scene: Scene) => void;
    setStudio: (studio: Studio | null) => void;
    setTags: (tags: Tag[]) => void;
    setActors: (actors: Actor[]) => void;
    setInteractions: (interactions: SceneInteractions) => void;
}

export const WATCH_PAGE_DATA_KEY = 'watchPageData';

export function useWatchPageData(sceneId: Ref<number>): WatchPageData {
    const authStore = useAuthStore();
    const { fetchScene, fetchSceneInteractions, getResumePosition, fetchRelatedScenes } =
        useApiScenes();
    const { fetchMarkers } = useApiMarkers();
    const { fetchSceneStudio } = useApiStudios();
    const { fetchSceneTags } = useApiTags();
    const { fetchSceneActors } = useApiActors();
    const { getPornDBStatus } = useApiPornDB();

    // Core data
    const scene = ref<Scene | null>(null);
    const markers = ref<Marker[]>([]);
    const resumePosition = ref(0);

    // Details data
    const interactions = ref<SceneInteractions | null>(null);
    const studio = ref<Studio | null>(null);
    const tags = ref<Tag[]>([]);
    const actors = ref<Actor[]>([]);

    // Below fold
    const relatedScenes = ref<SceneListItem[]>([]);
    const pornDBConfigured = ref(false);

    // Loading states (reactive for proper template unwrapping)
    const loading = reactive<LoadingState>({
        scene: true,
        player: true,
        details: true,
        related: true,
    });

    // Error state
    const error = ref<string | null>(null);

    const isAdmin = computed(() => authStore.user?.role === 'admin');

    // P0: Critical - fetch scene
    async function loadP0(): Promise<boolean> {
        loading.scene = true;
        error.value = null;

        try {
            scene.value = await fetchScene(sceneId.value);
            return true;
        } catch (err: unknown) {
            error.value = err instanceof Error ? err.message : 'Failed to load scene';
            return false;
        } finally {
            loading.scene = false;
        }
    }

    // P1: Player experience - markers + resume position
    async function loadP1(): Promise<void> {
        loading.player = true;

        const markersPromise = fetchMarkers(sceneId.value)
            .then((data) => {
                markers.value = data.markers || [];
            })
            .catch(() => {
                // Silent fail - markers are optional
            });

        const resumePromise = getResumePosition(sceneId.value)
            .then((res) => {
                resumePosition.value = res.position > 0 ? res.position : 0;
            })
            .catch(() => {
                // Silent fail - resume is optional
            });

        await Promise.all([markersPromise, resumePromise]);
        loading.player = false;
    }

    // P2: Details tab - interactions, studio, tags, actors
    async function loadP2(): Promise<void> {
        loading.details = true;

        const interactionsPromise = fetchSceneInteractions(sceneId.value)
            .then((res) => {
                interactions.value = {
                    rating: res.rating || 0,
                    liked: res.liked || false,
                    jizzed_count: res.jizzed_count || 0,
                };
            })
            .catch(() => {
                // Silent fail
            });

        const studioPromise = fetchSceneStudio(sceneId.value)
            .then((res) => {
                studio.value = res.data || null;
            })
            .catch((err: unknown) => {
                // 404 means no studio assigned, that's fine
                if (err instanceof Error && err.message.includes('not found')) {
                    studio.value = null;
                }
                // Other errors silently fail
            });

        const tagsPromise = fetchSceneTags(sceneId.value)
            .then((res) => {
                tags.value = res.data || [];
            })
            .catch(() => {
                // Silent fail
            });

        const actorsPromise = fetchSceneActors(sceneId.value)
            .then((res) => {
                actors.value = res.data || [];
            })
            .catch(() => {
                // Silent fail
            });

        await Promise.all([interactionsPromise, studioPromise, tagsPromise, actorsPromise]);
        loading.details = false;
    }

    // P3: Below fold - related scenes + PornDB status (deferred)
    async function loadP3(): Promise<void> {
        loading.related = true;

        const RELATED_LIMIT = 15;

        const relatedPromise = fetchRelatedScenes(sceneId.value, RELATED_LIMIT)
            .then((res) => {
                relatedScenes.value = res.data || [];
            })
            .catch(() => {
                // Silent fail
            });

        // Only check PornDB status for admins
        const pornDBPromise = isAdmin.value
            ? getPornDBStatus()
                  .then((status) => {
                      pornDBConfigured.value = status.configured;
                  })
                  .catch(() => {
                      pornDBConfigured.value = false;
                  })
            : Promise.resolve();

        await Promise.all([relatedPromise, pornDBPromise]);
        loading.related = false;
    }

    // Main load orchestration
    async function loadAll(): Promise<void> {
        // Reset state
        markers.value = [];
        resumePosition.value = 0;
        interactions.value = null;
        studio.value = null;
        tags.value = [];
        actors.value = [];
        relatedScenes.value = [];
        pornDBConfigured.value = false;

        // P0: Scene (critical)
        const sceneLoaded = await loadP0();
        if (!sceneLoaded) return;

        // P1: Player data (after scene loads)
        await loadP1();

        // P2: Details data (after player data)
        await loadP2();

        // P3: Below fold (deferred via queueMicrotask for better UX)
        queueMicrotask(() => {
            loadP3();
        });
    }

    // Individual refresh functions for child components
    async function refreshMarkers(): Promise<void> {
        try {
            const data = await fetchMarkers(sceneId.value);
            markers.value = data.markers || [];
        } catch {
            // Silent fail
        }
    }

    async function refreshStudio(): Promise<void> {
        try {
            const res = await fetchSceneStudio(sceneId.value);
            studio.value = res.data || null;
        } catch (err: unknown) {
            if (err instanceof Error && err.message.includes('not found')) {
                studio.value = null;
            }
        }
    }

    async function refreshTags(): Promise<void> {
        try {
            const res = await fetchSceneTags(sceneId.value);
            tags.value = res.data || [];
        } catch {
            // Silent fail
        }
    }

    async function refreshActors(): Promise<void> {
        try {
            const res = await fetchSceneActors(sceneId.value);
            actors.value = res.data || [];
        } catch {
            // Silent fail
        }
    }

    async function refreshInteractions(): Promise<void> {
        try {
            const res = await fetchSceneInteractions(sceneId.value);
            interactions.value = {
                rating: res.rating || 0,
                liked: res.liked || false,
                jizzed_count: res.jizzed_count || 0,
            };
        } catch {
            // Silent fail
        }
    }

    // Setters for optimistic updates
    function setScene(newScene: Scene): void {
        scene.value = newScene;
    }

    function setStudio(newStudio: Studio | null): void {
        studio.value = newStudio;
    }

    function setTags(newTags: Tag[]): void {
        tags.value = newTags;
    }

    function setActors(newActors: Actor[]): void {
        actors.value = newActors;
    }

    function setInteractions(newInteractions: SceneInteractions): void {
        interactions.value = newInteractions;
    }

    return {
        // Data
        scene,
        markers,
        resumePosition,
        interactions,
        studio,
        tags,
        actors,
        relatedScenes,
        pornDBConfigured,

        // Loading states
        loading,

        // Error
        error,

        // Actions
        refreshMarkers,
        refreshStudio,
        refreshTags,
        refreshActors,
        refreshInteractions,
        refreshAll: loadAll,

        // Setters
        setScene,
        setStudio,
        setTags,
        setActors,
        setInteractions,
    };
}
