/**
 * Loads (and prefetches) what the triage page shows for a scene: the scene, its tags, actors,
 * studio and the user's rating and like. Results are cached per scene ID for the session of
 * the page; the triage actions update the cached entry in place.
 */
import type { Scene } from '~/types/scene';
import type { Tag } from '~/types/tag';
import type { Actor } from '~/types/actor';
import type { Studio } from '~/types/studio';

export interface TriageSceneDetails {
    id: number;
    scene: Scene | null;
    // The scene could not be loaded (deleted, in the trash...)
    missing: boolean;
    tags: Tag[];
    actors: Actor[];
    studio: Studio | null;
    rating: number;
    liked: boolean;
}

const CACHE_LIMIT = 30;

export function useTriageSceneDetails() {
    const { fetchScene, fetchSceneInteractions } = useApiScenes();
    const { fetchSceneTags } = useApiTags();
    const { fetchSceneActors } = useApiActors();
    const { fetchSceneStudio } = useApiStudios();

    const cache = new Map<number, Promise<TriageSceneDetails>>();

    const fetchDetails = async (id: number): Promise<TriageSceneDetails> => {
        const details: TriageSceneDetails = {
            id,
            scene: null,
            missing: false,
            tags: [],
            actors: [],
            studio: null,
            rating: 0,
            liked: false,
        };
        try {
            details.scene = await fetchScene(id);
        } catch {
            details.missing = true;
            return details;
        }
        await Promise.all([
            fetchSceneTags(id)
                .then((res) => (details.tags = res.data || []))
                .catch(() => {}),
            fetchSceneActors(id)
                .then((res) => (details.actors = res.data || []))
                .catch(() => {}),
            fetchSceneStudio(id)
                .then((res) => (details.studio = res.data || null))
                .catch(() => {}),
            fetchSceneInteractions(id)
                .then((res) => {
                    details.rating = res.rating || 0;
                    details.liked = res.liked || false;
                })
                .catch(() => {}),
        ]);
        return details;
    };

    const load = (id: number): Promise<TriageSceneDetails> => {
        const cached = cache.get(id);
        if (cached) return cached;
        const promise = fetchDetails(id);
        cache.set(id, promise);
        // Failed loads are not kept, so the next visit retries
        promise.then((d) => {
            if (d.missing) cache.delete(id);
        });
        if (cache.size > CACHE_LIMIT) {
            const oldest = cache.keys().next().value;
            if (oldest !== undefined && oldest !== id) cache.delete(oldest);
        }
        return promise;
    };

    const prefetch = (ids: number[]) => {
        for (const id of ids) void load(id);
    };

    /** Stores the updated details of a scene after an action. */
    const update = (details: TriageSceneDetails) => {
        cache.set(details.id, Promise.resolve(details));
    };

    const invalidate = (id: number) => {
        cache.delete(id);
    };

    return { load, prefetch, update, invalidate };
}
