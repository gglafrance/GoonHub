import type { StudioListItem } from '~/types/studio';

interface StudioMatchResult {
    studioId: number | null;
    created: boolean;
    error?: string;
}

/**
 * Silent studio matching utility for bulk operations.
 * Automatically matches site names to existing studios or creates new ones.
 */
export function useSilentStudioMatcher() {
    const { fetchStudios, createStudio, setSceneStudio } = useApiStudios();
    const { searchPornDBSites, getPornDBSite } = useApiPornDB();

    /**
     * Matches a PornDB site name to an existing local studio or creates a new one.
     * Links the studio to the scene.
     *
     * @param sceneId - The scene to link the studio to
     * @param siteName - Name of the site from PornDB
     * @returns Object with studio ID, whether it was newly created, and any error
     */
    async function matchStudio(sceneId: number, siteName: string): Promise<StudioMatchResult> {
        if (!siteName) {
            return { studioId: null, created: false };
        }

        try {
            // Search for existing local studio by name
            const result = await fetchStudios(1, 10, siteName);
            const localStudios: StudioListItem[] = result.data || [];

            // Find exact name match (case-insensitive)
            const exactMatch = localStudios.find(
                (s) => s.name.toLowerCase() === siteName.toLowerCase(),
            );

            if (exactMatch) {
                // Link existing studio to scene
                await setSceneStudio(sceneId, exactMatch.id);
                return { studioId: exactMatch.id, created: false };
            }

            // No exact match - try to find on PornDB and create
            try {
                const porndbSites = await searchPornDBSites(siteName);
                const matchingSite = porndbSites.find(
                    (s: { name: string }) => s.name.toLowerCase() === siteName.toLowerCase(),
                );

                if (matchingSite) {
                    // Fetch full site details
                    const details = await getPornDBSite(matchingSite.id);
                    const newStudio = await createStudio({
                        name: details.name,
                        short_name: details.short_name,
                        url: details.url,
                        description: details.description,
                        rating: details.rating,
                        logo: details.logo,
                        favicon: details.favicon,
                        poster: details.poster,
                        porndb_id: details.id,
                    });
                    await setSceneStudio(sceneId, newStudio.id);
                    return { studioId: newStudio.id, created: true };
                }
            } catch {
                // PornDB lookup failed, continue to create with name only
            }

            // Create studio with just the name
            const newStudio = await createStudio({ name: siteName });
            await setSceneStudio(sceneId, newStudio.id);
            return { studioId: newStudio.id, created: true };
        } catch (e) {
            return {
                studioId: null,
                created: false,
                error: `Failed to match studio "${siteName}": ${e}`,
            };
        }
    }

    return { matchStudio };
}
