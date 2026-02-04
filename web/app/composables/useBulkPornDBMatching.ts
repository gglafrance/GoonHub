import type { PornDBScene } from '~/types/porndb';
import type { SceneMatchInfo } from '~/types/explorer';
import type { BulkMatchResult, ApplyPhase, ConfidenceBreakdown } from '~/types/bulk-match';
import type { ParsingRule } from '~/types/parsing-rules';
import type { BulkRequestCache } from './useBulkRequestCache';

// Number of scenes to process concurrently during apply phase
const APPLY_CONCURRENCY_LIMIT = 3;

/**
 * Main orchestrator for bulk PornDB scene matching.
 * Handles searching, matching, and applying metadata for multiple scenes.
 */
export function useBulkPornDBMatching() {
    const { searchPornDBScenes, applySceneMetadata } = useApiPornDB();
    const { calculateConfidence } = useConfidenceCalculator();
    const { matchActors } = useSilentActorMatcher();
    const { matchStudio } = useSilentStudioMatcher();
    const { applyRules } = useParsingRulesEngine();

    // Session-scoped request cache for bulk operations
    const requestCache = useBulkRequestCache();

    // State
    const results = ref<Map<number, BulkMatchResult>>(new Map());
    const isSearching = ref(false);
    const searchProgress = ref({ current: 0, total: 0 });
    const applyPhase = ref<ApplyPhase>('idle');
    const applyProgress = ref({ current: 0, total: 0, failed: 0 });
    const failedScenes = ref<BulkMatchResult[]>([]);

    // Computed
    const matchedCount = computed(() => {
        let count = 0;
        for (const result of results.value.values()) {
            if (result.status === 'matched') count++;
        }
        return count;
    });

    const resultsArray = computed(() => {
        return Array.from(results.value.values());
    });

    /**
     * Clean filename for use as search query.
     * Removes extension, resolution, codec info, and common patterns.
     */
    function cleanFilename(filename: string): string {
        return (
            filename
                // Remove extension
                .replace(/\.[^/.]+$/, '')
                // Remove resolution patterns
                .replace(/\b(2160p|1080p|720p|480p|360p|4K|UHD|FHD|HD)\b/gi, '')
                // Remove codec patterns
                .replace(/\b(x264|x265|h264|h265|hevc|avc|xvid|divx)\b/gi, '')
                // Remove common release group patterns
                .replace(/\b(rarbg|yts|yify|sparks|geckos|megusta|fgt)\b/gi, '')
                // Remove bitrate patterns
                .replace(/\b\d+kbps\b/gi, '')
                // Remove file size patterns
                .replace(/\b\d+(\.\d+)?(gb|mb)\b/gi, '')
                // Replace separators with spaces
                .replace(/[._-]+/g, ' ')
                // Clean up multiple spaces
                .replace(/\s+/g, ' ')
                .trim()
        );
    }

    /**
     * Build a search query from scene info.
     * Uses filename + actors + studio, falls back to title.
     * @param scene The scene to build a query for
     * @param rules Optional parsing rules to apply to filename
     */
    function buildSearchQuery(scene: SceneMatchInfo, rules?: ParsingRule[]): string {
        // Use parsing rules if provided, otherwise use cleanFilename
        const filename =
            rules && rules.length > 0
                ? applyRules(scene.original_filename, rules)
                : cleanFilename(scene.original_filename);

        // Start with cleaned filename
        let query = filename;

        // Add actors if not already in filename
        if (scene.actors && scene.actors.length > 0) {
            for (const actor of scene.actors) {
                if (!filename.toLowerCase().includes(actor.toLowerCase())) {
                    query += ` ${actor}`;
                }
            }
        }

        // Add studio if not already in filename
        if (scene.studio && !filename.toLowerCase().includes(scene.studio.toLowerCase())) {
            query += ` ${scene.studio}`;
        }

        // If query is too short, fall back to title
        if (query.trim().length < 5) {
            query = scene.title;
        }

        return query.trim();
    }

    /**
     * Select the best match from search results based on confidence scores.
     */
    function selectBestMatch(
        scene: SceneMatchInfo,
        searchResults: PornDBScene[],
    ): { match: PornDBScene; confidence: ConfidenceBreakdown } | null {
        if (searchResults.length === 0) return null;

        let bestMatch: PornDBScene | null = null;
        let bestConfidence: ConfidenceBreakdown | null = null;

        for (const result of searchResults) {
            const confidence = calculateConfidence(scene, result);

            // If only one result, take it
            if (searchResults.length === 1) {
                bestMatch = result;
                bestConfidence = confidence;
                break;
            }

            // Minimum threshold: at least 30 total score
            if (confidence.total < 30) continue;

            if (!bestConfidence || confidence.total > bestConfidence.total) {
                bestMatch = result;
                bestConfidence = confidence;
            }
        }

        if (bestMatch && bestConfidence) {
            return { match: bestMatch, confidence: bestConfidence };
        }

        return null;
    }

    /**
     * Search for matches for all provided scenes.
     * Results are streamed as they arrive.
     * @param scenes Scenes to search for
     * @param rules Optional parsing rules to apply to filenames
     */
    async function searchScenes(scenes: SceneMatchInfo[], rules?: ParsingRule[]): Promise<void> {
        isSearching.value = true;
        searchProgress.value = { current: 0, total: scenes.length };
        results.value = new Map();
        failedScenes.value = [];

        for (const scene of scenes) {
            // Skip scenes that already have a PornDB ID
            if (scene.porndb_scene_id) {
                results.value.set(scene.id, {
                    sceneId: scene.id,
                    localScene: scene,
                    match: null,
                    confidence: null,
                    status: 'skipped',
                });
                searchProgress.value.current++;
                continue;
            }

            // Set searching status
            results.value.set(scene.id, {
                sceneId: scene.id,
                localScene: scene,
                match: null,
                confidence: null,
                status: 'searching',
            });

            try {
                const query = buildSearchQuery(scene, rules);
                const searchResults = await searchPornDBScenes({ title: query });
                const best = selectBestMatch(scene, searchResults);

                if (best) {
                    results.value.set(scene.id, {
                        sceneId: scene.id,
                        localScene: scene,
                        match: best.match,
                        confidence: best.confidence,
                        status: 'matched',
                    });
                } else {
                    results.value.set(scene.id, {
                        sceneId: scene.id,
                        localScene: scene,
                        match: null,
                        confidence: null,
                        status: 'no-match',
                    });
                }
            } catch (e) {
                results.value.set(scene.id, {
                    sceneId: scene.id,
                    localScene: scene,
                    match: null,
                    confidence: null,
                    status: 'no-match',
                    error: e instanceof Error ? e.message : String(e),
                });
            }

            searchProgress.value.current++;
        }

        isSearching.value = false;
    }

    /**
     * Remove a match (user wants to search manually).
     */
    function removeMatch(sceneId: number): void {
        const existing = results.value.get(sceneId);
        if (existing) {
            results.value.set(sceneId, {
                ...existing,
                match: null,
                confidence: null,
                status: 'removed',
            });
        }
    }

    /**
     * Apply metadata from a single matched scene.
     * @param result - The bulk match result to apply
     * @param cache - Optional cache for bulk operations to avoid redundant requests
     */
    async function applySingleScene(
        result: BulkMatchResult,
        cache?: BulkRequestCache,
    ): Promise<void> {
        if (!result.match || result.status !== 'matched') return;

        const sceneId = result.sceneId;
        const match = result.match;

        // Update status to applying
        results.value.set(sceneId, { ...result, status: 'applying' });

        try {
            // Apply basic metadata first (must complete before actor/studio matching)
            await applySceneMetadata(sceneId, {
                title: match.title,
                description: match.description,
                studio: match.site?.name,
                thumbnail_url: match.image || match.poster,
                release_date: match.date,
                porndb_scene_id: match.id,
                tag_names: match.tags?.map((t) => t.name),
            });

            // Match actors and studio in parallel (they're independent operations)
            const parallelTasks: Promise<unknown>[] = [];

            if (match.performers && match.performers.length > 0) {
                parallelTasks.push(matchActors(sceneId, match.performers, cache));
            }

            if (match.site?.name) {
                parallelTasks.push(matchStudio(sceneId, match.site.name, cache));
            }

            if (parallelTasks.length > 0) {
                await Promise.all(parallelTasks);
            }

            results.value.set(sceneId, { ...result, status: 'applied' });
        } catch (e) {
            const errorMsg = e instanceof Error ? e.message : String(e);
            results.value.set(sceneId, { ...result, status: 'failed', error: errorMsg });
            throw e;
        }
    }

    /**
     * Process an array of items with a concurrency limit.
     * @param items - Items to process
     * @param concurrency - Maximum number of concurrent operations
     * @param processor - Async function to process each item
     */
    async function processWithConcurrency<T>(
        items: T[],
        concurrency: number,
        processor: (item: T) => Promise<void>,
    ): Promise<void> {
        const queue = [...items];
        const workers: Promise<void>[] = [];

        async function worker(): Promise<void> {
            while (queue.length > 0) {
                const item = queue.shift();
                if (item !== undefined) {
                    await processor(item);
                }
            }
        }

        // Start workers up to concurrency limit
        const workerCount = Math.min(concurrency, items.length);
        for (let i = 0; i < workerCount; i++) {
            workers.push(worker());
        }

        await Promise.all(workers);
    }

    /**
     * Apply all matched scenes with parallel processing.
     * Uses a session-scoped cache to avoid redundant API calls for shared actors/studios.
     * Processes multiple scenes concurrently for faster throughput.
     */
    async function applyAllMatched(): Promise<void> {
        const matchedResults = resultsArray.value.filter((r) => r.status === 'matched');

        if (matchedResults.length === 0) return;

        applyPhase.value = 'applying';
        applyProgress.value = { current: 0, total: matchedResults.length, failed: 0 };
        failedScenes.value = [];

        // Clear cache at start of bulk operation
        requestCache.clear();

        await processWithConcurrency(matchedResults, APPLY_CONCURRENCY_LIMIT, async (result) => {
            try {
                await applySingleScene(result, requestCache);
            } catch {
                applyProgress.value.failed++;
                const failedResult = results.value.get(result.sceneId);
                if (failedResult) {
                    failedScenes.value.push(failedResult);
                }
            }
            applyProgress.value.current++;
        });

        applyPhase.value = 'done';
    }

    /**
     * Retry all failed scenes with parallel processing.
     * Uses a session-scoped cache to avoid redundant API calls for shared actors/studios.
     */
    async function retryFailed(): Promise<void> {
        const toRetry = [...failedScenes.value];
        failedScenes.value = [];
        applyProgress.value = { current: 0, total: toRetry.length, failed: 0 };
        applyPhase.value = 'applying';

        // Clear cache at start of retry operation
        requestCache.clear();

        // Reset all statuses to matched before starting
        for (const result of toRetry) {
            results.value.set(result.sceneId, {
                ...result,
                status: 'matched',
                error: undefined,
            });
        }

        await processWithConcurrency(toRetry, APPLY_CONCURRENCY_LIMIT, async (result) => {
            try {
                await applySingleScene(results.value.get(result.sceneId)!, requestCache);
            } catch {
                applyProgress.value.failed++;
                const failedResult = results.value.get(result.sceneId);
                if (failedResult) {
                    failedScenes.value.push(failedResult);
                }
            }
            applyProgress.value.current++;
        });

        applyPhase.value = 'done';
    }

    /**
     * Clear failed scenes list.
     */
    function clearFailed(): void {
        failedScenes.value = [];
    }

    /**
     * Reset all state.
     */
    function reset(): void {
        results.value = new Map();
        isSearching.value = false;
        searchProgress.value = { current: 0, total: 0 };
        applyPhase.value = 'idle';
        applyProgress.value = { current: 0, total: 0, failed: 0 };
        failedScenes.value = [];
        requestCache.clear();
    }

    return {
        // State
        results,
        resultsArray,
        isSearching,
        searchProgress,
        applyPhase,
        applyProgress,
        failedScenes,
        matchedCount,

        // Actions
        searchScenes,
        removeMatch,
        applyAllMatched,
        applySingleScene,
        retryFailed,
        clearFailed,
        reset,
        buildSearchQuery,
    };
}
