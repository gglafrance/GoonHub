import type { Actor } from '~/types/actor';
import type { PornDBScenePerformer, PornDBPerformerDetails } from '~/types/porndb';
import type { BulkRequestCache } from './useBulkRequestCache';

interface ActorMatchResult {
    actorIds: number[];
    created: number;
    errors: string[];
}

/**
 * Silent actor matching utility for bulk operations.
 * Automatically matches performers to existing actors or creates new ones.
 */
export function useSilentActorMatcher() {
    const { resolveActor, createActor, fetchSceneActors, setSceneActors } = useApiActors();
    const { searchPornDBPerformers, getPornDBPerformer } = useApiPornDB();

    /**
     * Matches performers to existing local actors or creates new ones.
     * Links all matched/created actors to the scene.
     *
     * Pattern (like ActorMatchFlow.vue):
     * 1. Resolve the name to a local actor (exact name or alias, case-insensitive)
     * 2. If found, use the existing actor
     * 3. If no match, search PornDB performers by name
     * 4. Take the first search result, fetch full details by its ID
     * 5. Create actor with full details
     * 6. If no PornDB results, create with basic name/image from scene
     *
     * @param sceneId - The scene to link actors to
     * @param performers - PornDB performers from the matched scene
     * @param cache - Optional cache for bulk operations to avoid redundant requests
     * @returns Object with actor IDs, count of newly created actors, and any errors
     */
    async function matchActors(
        sceneId: number,
        performers: PornDBScenePerformer[],
        cache?: BulkRequestCache,
    ): Promise<ActorMatchResult> {
        const actorIds: number[] = [];
        let created = 0;
        const errors: string[] = [];

        // Get existing actors linked to this scene
        try {
            const existing = await fetchSceneActors(sceneId);
            const existingActors = existing.data || [];
            for (const actor of existingActors) {
                actorIds.push(actor.id);
            }
        } catch {
            // Continue without existing actors
        }

        // Process each performer
        for (const performer of performers) {
            if (!performer || !performer.name) continue;

            const nameLower = performer.name.toLowerCase();
            const localActorsCacheKey = `local-actors:${nameLower}`;

            try {
                // Step 1: Resolve the name to an existing local actor, by exact name or alias (cached)
                let localActors: Actor[];
                if (cache?.has(localActorsCacheKey)) {
                    localActors = cache.get<Actor[]>(localActorsCacheKey) || [];
                } else {
                    const resolved = await resolveActor(performer.name);
                    localActors = resolved ? [resolved.data] : [];
                    cache?.set(localActorsCacheKey, localActors);
                }

                const exactMatch = localActors[0];

                if (exactMatch) {
                    if (!actorIds.includes(exactMatch.id)) {
                        actorIds.push(exactMatch.id);
                    }
                    // Skip to next performer - actor already exists locally
                    continue;
                }

                // Step 2: No local match - search PornDB performers by name (cached)
                const porndbSearchCacheKey = `porndb-performers:${nameLower}`;
                let porndbSearchResults: { id: string; name: string; image?: string }[] = [];
                try {
                    if (cache?.has(porndbSearchCacheKey)) {
                        porndbSearchResults =
                            cache.get<{ id: string; name: string; image?: string }[]>(
                                porndbSearchCacheKey,
                            ) || [];
                    } else {
                        porndbSearchResults = await searchPornDBPerformers(performer.name);
                        cache?.set(porndbSearchCacheKey, porndbSearchResults);
                    }
                } catch {
                    // PornDB search failed, will fall back to basic creation
                }

                if (porndbSearchResults.length > 0) {
                    // Step 3: Take the first result and fetch full details (cached)
                    const firstResult = porndbSearchResults[0];
                    if (firstResult) {
                        const porndbDetailsCacheKey = `porndb-performer:${firstResult.id}`;
                        try {
                            let details: PornDBPerformerDetails;
                            if (cache?.has(porndbDetailsCacheKey)) {
                                details = cache.get<PornDBPerformerDetails>(porndbDetailsCacheKey)!;
                            } else {
                                details = await getPornDBPerformer(firstResult.id);
                                cache?.set(porndbDetailsCacheKey, details);
                            }

                            // PornDB's name may differ from the scene's: it may be a local actor
                            if (details.name.toLowerCase() !== nameLower) {
                                const known = await resolveActor(details.name);
                                if (known) {
                                    cache?.set(localActorsCacheKey, [known.data]);
                                    if (!actorIds.includes(known.data.id)) {
                                        actorIds.push(known.data.id);
                                    }
                                    continue;
                                }
                            }

                            const newActor = await createActor({
                                name: details.name,
                                aliases: details.aliases,
                                image_url: details.image,
                                gender: details.gender,
                                birthday: details.birthday,
                                birthplace: details.birthplace,
                                ethnicity: details.ethnicity,
                                nationality: details.nationality,
                                height_cm: details.height,
                                weight_kg: details.weight,
                                measurements: details.measurements,
                                cupsize: details.cupsize,
                                hair_color: details.hair_colour,
                                eye_color: details.eye_colour,
                                tattoos: details.tattoos,
                                piercings: details.piercings,
                                career_start_year: details.career_start_year,
                                career_end_year: details.career_end_year,
                                fake_boobs: details.fake_boobs,
                                same_sex_only: details.same_sex_only,
                            });

                            // Update cache with newly created actor
                            const updatedLocalActors = [...localActors, newActor];
                            cache?.set(localActorsCacheKey, updatedLocalActors);

                            actorIds.push(newActor.id);
                            created++;
                            continue;
                        } catch {
                            // Failed to fetch details, try basic creation with search result info
                            try {
                                const newActor = await createActor({
                                    name: firstResult.name,
                                    image_url: firstResult.image,
                                });

                                // Update cache with newly created actor
                                const updatedLocalActors = [...localActors, newActor];
                                cache?.set(localActorsCacheKey, updatedLocalActors);

                                actorIds.push(newActor.id);
                                created++;
                                continue;
                            } catch {
                                // Will fall through to scene performer fallback
                            }
                        }
                    }
                }

                // Step 4: No PornDB results or all attempts failed - create with scene performer data
                try {
                    const newActor = await createActor({
                        name: performer.name,
                        image_url: performer.image,
                    });

                    // Update cache with newly created actor
                    const updatedLocalActors = [...localActors, newActor];
                    cache?.set(localActorsCacheKey, updatedLocalActors);

                    actorIds.push(newActor.id);
                    created++;
                } catch (e) {
                    errors.push(`Failed to create actor "${performer.name}": ${e}`);
                }
            } catch (e) {
                errors.push(`Failed to process performer "${performer.name}": ${e}`);
            }
        }

        // Link all actors to the scene
        if (actorIds.length > 0) {
            try {
                await setSceneActors(sceneId, [...new Set(actorIds)]);
            } catch (e) {
                errors.push(`Failed to link actors to scene: ${e}`);
            }
        }

        return { actorIds: [...new Set(actorIds)], created, errors };
    }

    return { matchActors };
}
