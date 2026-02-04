import type { StudioListItem } from '~/types/studio';
import type { PornDBSiteDetails } from '~/types/porndb';
import type { BulkRequestCache } from './useBulkRequestCache';

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
     * @param cache - Optional cache for bulk operations to avoid redundant requests
     * @returns Object with studio ID, whether it was newly created, and any error
     */
    async function matchStudio(
        sceneId: number,
        siteName: string,
        cache?: BulkRequestCache,
    ): Promise<StudioMatchResult> {
        if (!siteName) {
            return { studioId: null, created: false };
        }

        const nameLower = siteName.toLowerCase();
        const localStudiosCacheKey = `local-studios:${nameLower}`;

        try {
            // Search for existing local studio by name (cached)
            let localStudios: StudioListItem[];
            if (cache?.has(localStudiosCacheKey)) {
                localStudios = cache.get<StudioListItem[]>(localStudiosCacheKey) || [];
            } else {
                const result = await fetchStudios(1, 10, siteName);
                localStudios = result.data || [];
                cache?.set(localStudiosCacheKey, localStudios);
            }

            // Find exact name match (case-insensitive)
            const exactMatch = localStudios.find(
                (s) => s.name.toLowerCase() === siteName.toLowerCase(),
            );

            if (exactMatch) {
                // Link existing studio to scene
                await setSceneStudio(sceneId, exactMatch.id);
                return { studioId: exactMatch.id, created: false };
            }

            // No exact match - try to find on PornDB and create (cached)
            const porndbSitesCacheKey = `porndb-sites:${nameLower}`;
            try {
                let porndbSites: { id: string; name: string }[];
                if (cache?.has(porndbSitesCacheKey)) {
                    porndbSites =
                        cache.get<{ id: string; name: string }[]>(porndbSitesCacheKey) || [];
                } else {
                    porndbSites = await searchPornDBSites(siteName);
                    cache?.set(porndbSitesCacheKey, porndbSites);
                }

                const matchingSite = porndbSites.find(
                    (s: { name: string }) => s.name.toLowerCase() === siteName.toLowerCase(),
                );

                if (matchingSite) {
                    // Fetch full site details (cached)
                    const porndbSiteDetailsCacheKey = `porndb-site:${matchingSite.id}`;
                    let details: PornDBSiteDetails;
                    if (cache?.has(porndbSiteDetailsCacheKey)) {
                        details = cache.get<PornDBSiteDetails>(porndbSiteDetailsCacheKey)!;
                    } else {
                        details = await getPornDBSite(matchingSite.id);
                        cache?.set(porndbSiteDetailsCacheKey, details);
                    }

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

                    // Update cache with newly created studio
                    const updatedLocalStudios = [...localStudios, newStudio];
                    cache?.set(localStudiosCacheKey, updatedLocalStudios);

                    await setSceneStudio(sceneId, newStudio.id);
                    return { studioId: newStudio.id, created: true };
                }
            } catch {
                // PornDB lookup failed, continue to create with name only
            }

            // Create studio with just the name
            const newStudio = await createStudio({ name: siteName });

            // Update cache with newly created studio
            const updatedLocalStudios = [...localStudios, newStudio];
            cache?.set(localStudiosCacheKey, updatedLocalStudios);

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
