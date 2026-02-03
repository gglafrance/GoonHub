import type { Actor } from '~/types/actor';
import type { PornDBScenePerformer } from '~/types/porndb';

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
    const { fetchActors, createActor, fetchSceneActors, setSceneActors } = useApiActors();
    const { searchPornDBPerformers, getPornDBPerformer } = useApiPornDB();

    /**
     * Matches performers to existing local actors or creates new ones.
     * Links all matched/created actors to the scene.
     *
     * Pattern (like ActorMatchFlow.vue):
     * 1. Search local actors by name
     * 2. If exact match found, use existing actor
     * 3. If no match, search PornDB performers by name
     * 4. Take the first search result, fetch full details by its ID
     * 5. Create actor with full details
     * 6. If no PornDB results, create with basic name/image from scene
     *
     * @param sceneId - The scene to link actors to
     * @param performers - PornDB performers from the matched scene
     * @returns Object with actor IDs, count of newly created actors, and any errors
     */
    async function matchActors(
        sceneId: number,
        performers: PornDBScenePerformer[],
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

            try {
                // Step 1: Search for existing local actor by exact name
                const result = await fetchActors(1, 10, performer.name);
                const localActors: Actor[] = result.data || [];

                // Find exact name match (case-insensitive) - also check aliases
                const exactMatch = localActors.find((a) => {
                    // Check name match
                    if (a.name.toLowerCase() === performer.name.toLowerCase()) {
                        return true;
                    }
                    // Check alias match
                    if (a.aliases && a.aliases.length > 0) {
                        return a.aliases.some(
                            (alias) => alias.toLowerCase() === performer.name.toLowerCase(),
                        );
                    }
                    return false;
                });

                if (exactMatch && !actorIds.includes(exactMatch.id)) {
                    actorIds.push(exactMatch.id);
                    continue;
                }

                // Step 2: No local match - search PornDB performers by name
                let porndbSearchResults: { id: string; name: string; image?: string }[] = [];
                try {
                    porndbSearchResults = await searchPornDBPerformers(performer.name);
                } catch {
                    // PornDB search failed, will fall back to basic creation
                }

                if (porndbSearchResults.length > 0) {
                    // Step 3: Take the first result and fetch full details
                    const firstResult = porndbSearchResults[0];
                    if (firstResult) {
                        try {
                            const details = await getPornDBPerformer(firstResult.id);
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
