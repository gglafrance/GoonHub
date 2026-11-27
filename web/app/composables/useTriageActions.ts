/**
 * Triage actions on the current scene. Each action saves right away through the existing APIs,
 * updates the shown details and confirms with a toast. Actions the user lacks the permission
 * for are refused here too (the UI hides or disables them).
 */
import type { Tag } from '~/types/tag';
import type { Actor, ActorListItem } from '~/types/actor';
import type { Studio, StudioListItem } from '~/types/studio';
import type { TriageSceneDetails } from '~/composables/useTriageSceneDetails';

// The trash is applied after this delay when it cannot be undone server side (restoring
// from the trash needs trash:manage)
const DEFERRED_TRASH_MS = 5000;
const NEW_TAG_COLOR = '#6B7280';

interface TriageActionsDeps {
    current: Ref<TriageSceneDetails | null>;
    details: ReturnType<typeof useTriageSceneDetails>;
    queue: ReturnType<typeof useTriageQueue>;
    toasts: ReturnType<typeof useTriageToasts>;
    advance: () => Promise<void>;
}

export function useTriageActions({ current, details, queue, toasts, advance }: TriageActionsDeps) {
    const { setSceneRating, deleteSceneRating, toggleSceneLike, deleteScene } = useApiScenes();
    const { setSceneTags, createTag: apiCreateTag } = useApiTags();
    const { setSceneActors, createActor: apiCreateActor } = useApiActors();
    const { setSceneStudio, createStudio: apiCreateStudio } = useApiStudios();
    const { restoreScene } = useApiAdmin();
    const perms = useMyPermissions();

    const canEdit = computed(() => perms.has('scenes:edit'));
    const canTrash = computed(() => perms.has('scenes:trash'));
    const canRestore = computed(() => perms.has('trash:manage'));
    const canCreateTags = computed(() => perms.has('tags:manage'));
    const canCreateActors = computed(() => perms.has('actors:manage'));
    const canCreateStudios = computed(() => perms.has('studios:manage'));

    const errorMessage = (e: unknown, fallback: string) =>
        e instanceof Error && e.message ? e.message : fallback;

    /** Applies a change to a scene's details, in the cache and on screen if still shown. */
    const patch = (id: number, change: Partial<TriageSceneDetails>) => {
        const base = current.value?.id === id ? current.value : null;
        if (!base) {
            details.invalidate(id);
            return;
        }
        const updated = { ...base, ...change };
        details.update(updated);
        current.value = updated;
    };

    // The scene shown and ready (not while the next one is loading)
    const withScene = (): TriageSceneDetails | null => {
        const d = current.value;
        return d && d.id === queue.currentId.value && d.scene && !d.missing ? d : null;
    };

    // Rating: 1-5, 0 clears
    const rate = async (rating: number) => {
        const d = withScene();
        if (!d) return;
        const previous = d.rating;
        patch(d.id, { rating });
        queue.touch(d.id);
        try {
            if (rating === 0) {
                await deleteSceneRating(d.id);
                toasts.show('Rating cleared', { icon: 'heroicons:star' });
            } else {
                await setSceneRating(d.id, rating);
                toasts.show(`Rated ${rating}/5`, { icon: 'heroicons:star-solid' });
            }
        } catch (e: unknown) {
            patch(d.id, { rating: previous });
            toasts.error(errorMessage(e, 'Failed to save the rating'));
        }
    };

    const toggleLike = async () => {
        const d = withScene();
        if (!d) return;
        queue.touch(d.id);
        try {
            const res = await toggleSceneLike(d.id);
            const liked = !!res.liked;
            patch(d.id, { liked });
            toasts.show(liked ? 'Liked' : 'Like removed', {
                icon: liked ? 'heroicons:heart-solid' : 'heroicons:heart',
            });
        } catch (e: unknown) {
            toasts.error(errorMessage(e, 'Failed to update the like'));
        }
    };

    // Tags
    const saveTags = async (d: TriageSceneDetails, tags: Tag[], message: string) => {
        queue.touch(d.id);
        try {
            const res = await setSceneTags(
                d.id,
                tags.map((t) => t.id),
            );
            patch(d.id, { tags: res.data || tags });
            toasts.show(message, { icon: 'heroicons:tag' });
        } catch (e: unknown) {
            toasts.error(errorMessage(e, 'Failed to update the tags'));
        }
    };

    const addTag = async (tag: Tag) => {
        const d = withScene();
        if (!d || !canEdit.value) return;
        if (d.tags.some((t) => t.id === tag.id)) return;
        await saveTags(d, [...d.tags, tag], `Tag "${tag.name}" added`);
    };

    const createTag = async (name: string): Promise<Tag | null> => {
        const d = withScene();
        if (!d || !canEdit.value || !canCreateTags.value) return null;
        try {
            const tag: Tag = await apiCreateTag(name, NEW_TAG_COLOR);
            await saveTags(d, [...d.tags, tag], `Tag "${tag.name}" created and added`);
            return tag;
        } catch (e: unknown) {
            toasts.error(errorMessage(e, 'Failed to create the tag'));
            return null;
        }
    };

    const removeTag = async (tagId: number) => {
        const d = withScene();
        if (!d || !canEdit.value) return;
        const tag = d.tags.find((t) => t.id === tagId);
        if (!tag) return;
        await saveTags(
            d,
            d.tags.filter((t) => t.id !== tagId),
            `Tag "${tag.name}" removed`,
        );
    };

    // Actors
    const saveActors = async (d: TriageSceneDetails, actorIds: number[], message: string) => {
        queue.touch(d.id);
        try {
            const res = await setSceneActors(d.id, actorIds);
            patch(d.id, { actors: res.data || [] });
            toasts.show(message, { icon: 'heroicons:user' });
        } catch (e: unknown) {
            toasts.error(errorMessage(e, 'Failed to update the actors'));
        }
    };

    const addActor = async (actor: Pick<ActorListItem, 'id' | 'name'>) => {
        const d = withScene();
        if (!d || !canEdit.value) return;
        if (d.actors.some((a) => a.id === actor.id)) return;
        await saveActors(d, [...d.actors.map((a) => a.id), actor.id], `${actor.name} added`);
    };

    const createActor = async (name: string) => {
        const d = withScene();
        if (!d || !canEdit.value || !canCreateActors.value) return;
        try {
            const actor: Actor = await apiCreateActor({ name });
            await saveActors(
                d,
                [...d.actors.map((a) => a.id), actor.id],
                `${actor.name} created and added`,
            );
        } catch (e: unknown) {
            toasts.error(errorMessage(e, 'Failed to create the actor'));
        }
    };

    const removeActor = async (actorId: number) => {
        const d = withScene();
        if (!d || !canEdit.value) return;
        const actor = d.actors.find((a) => a.id === actorId);
        if (!actor) return;
        await saveActors(
            d,
            d.actors.filter((a) => a.id !== actorId).map((a) => a.id),
            `${actor.name} removed`,
        );
    };

    // Studio
    const saveStudio = async (d: TriageSceneDetails, studioId: number | null, message: string) => {
        queue.touch(d.id);
        try {
            const res = await setSceneStudio(d.id, studioId);
            patch(d.id, { studio: (res.data as Studio | null) || null });
            toasts.show(message, { icon: 'heroicons:building-office-2' });
        } catch (e: unknown) {
            toasts.error(errorMessage(e, 'Failed to update the studio'));
        }
    };

    const setStudio = async (studio: Pick<StudioListItem, 'id' | 'name'>) => {
        const d = withScene();
        if (!d || !canEdit.value || d.studio?.id === studio.id) return;
        await saveStudio(d, studio.id, `Studio set to ${studio.name}`);
    };

    const createStudio = async (name: string) => {
        const d = withScene();
        if (!d || !canEdit.value || !canCreateStudios.value) return;
        try {
            const studio: Studio = await apiCreateStudio({ name });
            await saveStudio(d, studio.id, `Studio ${studio.name} created and set`);
        } catch (e: unknown) {
            toasts.error(errorMessage(e, 'Failed to create the studio'));
        }
    };

    const clearStudio = async () => {
        const d = withScene();
        if (!d || !canEdit.value || !d.studio) return;
        await saveStudio(d, null, 'Studio removed');
    };

    // Trash (never a permanent delete)
    const pendingTrash = new Map<number, ReturnType<typeof setTimeout>>();

    const commitTrash = async (id: number) => {
        pendingTrash.delete(id);
        try {
            await deleteScene(id, false);
            details.invalidate(id);
        } catch (e: unknown) {
            queue.unskip(id);
            toasts.error(errorMessage(e, 'Failed to move the scene to the trash'));
        }
    };

    const trash = async () => {
        const d = withScene();
        if (!d || !canTrash.value) return;
        const id = d.id;
        const title = d.scene?.title || `Scene ${id}`;

        if (canRestore.value) {
            try {
                await deleteScene(id, false);
            } catch (e: unknown) {
                toasts.error(errorMessage(e, 'Failed to move the scene to the trash'));
                return;
            }
            details.invalidate(id);
            queue.skip(id);
            toasts.show(`"${title}" moved to trash`, {
                icon: 'heroicons:trash',
                tone: 'info',
                duration: 6000,
                actionLabel: 'Undo',
                action: async () => {
                    try {
                        await restoreScene(id);
                        queue.unskip(id);
                        details.invalidate(id);
                        queue.goTo(id);
                        toasts.show('Scene restored', { icon: 'heroicons:arrow-uturn-left' });
                    } catch (e: unknown) {
                        toasts.error(errorMessage(e, 'Failed to restore the scene'));
                    }
                },
            });
            await advance();
            return;
        }

        // Restoring needs trash:manage: give a short window to undo before trashing
        queue.skip(id);
        pendingTrash.set(
            id,
            setTimeout(() => void commitTrash(id), DEFERRED_TRASH_MS),
        );
        toasts.show(`"${title}" will be moved to trash`, {
            icon: 'heroicons:trash',
            tone: 'info',
            duration: DEFERRED_TRASH_MS,
            actionLabel: 'Undo',
            action: () => {
                const timer = pendingTrash.get(id);
                if (!timer) return;
                clearTimeout(timer);
                pendingTrash.delete(id);
                queue.unskip(id);
                queue.goTo(id);
                toasts.show('Kept', { icon: 'heroicons:arrow-uturn-left' });
            },
        });
        await advance();
    };

    /** Applies the deferred trash actions right away (leaving the page). */
    const flushPendingTrash = () => {
        for (const [id, timer] of pendingTrash) {
            clearTimeout(timer);
            pendingTrash.delete(id);
            // keepalive lets the request finish when the page is being unloaded
            void fetch(`/api/v1/scenes/${id}`, {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ permanent: false }),
                credentials: 'include',
                keepalive: true,
            }).catch(() => {});
        }
    };

    return {
        canEdit,
        canTrash,
        canCreateTags,
        canCreateActors,
        canCreateStudios,
        rate,
        toggleLike,
        addTag,
        createTag,
        removeTag,
        addActor,
        createActor,
        removeActor,
        setStudio,
        createStudio,
        clearStudio,
        trash,
        flushPendingTrash,
    };
}
