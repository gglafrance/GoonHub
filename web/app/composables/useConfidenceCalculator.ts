import type { PornDBScene, PornDBScenePerformer, PornDBSite } from '~/types/porndb';
import type { SceneMatchInfo } from '~/types/explorer';
import type { ConfidenceBreakdown } from '~/types/bulk-match';

/**
 * Cleans a string for comparison: lowercase, remove special characters.
 */
function cleanForComparison(str: string): string {
    return str
        .toLowerCase()
        .replace(/[^\w\s]/g, ' ')
        .replace(/\s+/g, ' ')
        .trim();
}

/**
 * Extracts meaningful words from a string (length >= 3).
 */
function extractWords(str: string): string[] {
    return cleanForComparison(str)
        .split(' ')
        .filter((word) => word.length >= 3);
}

/**
 * Normalizes a name for actor matching (handles variations like "Jane Doe" vs "JaneDoe").
 */
function normalizeActorName(name: string): string {
    return name.toLowerCase().replace(/[^a-z0-9]/g, '');
}

/**
 * Calculates title similarity score (0-30).
 *
 * Strategy: Check how many words from the remote title appear in the local title.
 * Local titles are often noisy (contain studio, date, codec, etc.) but the real
 * title should be contained within.
 *
 * Example: Local "FantasyMassage.Worst Day EVER.2019-06-21.mp4"
 *          Remote "Worst Day Ever!"
 *          → "worst", "day", "ever" all found in local → high score
 */
function calculateTitleScore(local: string, remote: string): number {
    const localClean = cleanForComparison(local);
    const remoteWords = extractWords(remote);

    // No meaningful words in remote title
    if (remoteWords.length === 0) {
        return 15; // Neutral score
    }

    // Count how many remote words appear in local title
    let foundWords = 0;
    for (const word of remoteWords) {
        if (localClean.includes(word)) {
            foundWords++;
        }
    }

    // Calculate ratio of found words
    const ratio = foundWords / remoteWords.length;

    // Scale to 0-30
    return Math.round(30 * ratio);
}

/**
 * Calculates actor matching score (0-30).
 *
 * Strategy: Check if LOCAL actors are found in PornDB results.
 * We don't care if PornDB has extra actors - only that our local actors appear.
 *
 * Example: Local has [A], PornDB has [A, B] → 100% of local found → high score
 *          Local has [A], PornDB has [B, C] → 0% of local found → low score
 */
function calculateActorScore(
    localActors: string[],
    remotePerformers: PornDBScenePerformer[] | undefined,
): number {
    // No local actors set - can't verify match, give neutral score
    if (localActors.length === 0) {
        return 15;
    }

    // Local actors but no PornDB performers - mismatch
    if (!remotePerformers || remotePerformers.length === 0) {
        return 0;
    }

    // Normalize all names for comparison
    const normalizedLocal = localActors.map(normalizeActorName);
    const normalizedRemote = remotePerformers.map((p) => normalizeActorName(p.name));

    // Count how many local actors are found in remote list
    let foundActors = 0;
    for (const localName of normalizedLocal) {
        // Check if this local actor appears in any remote performer
        const found = normalizedRemote.some((remoteName) => {
            // Exact match after normalization
            if (localName === remoteName) return true;
            // One contains the other (handles "Jane" matching "Jane Doe")
            if (localName.includes(remoteName) || remoteName.includes(localName)) return true;
            return false;
        });

        if (found) {
            foundActors++;
        }
    }

    // Calculate ratio: what percentage of our local actors were found?
    const ratio = foundActors / localActors.length;

    // Scale to 0-30
    return Math.round(30 * ratio);
}

/**
 * Calculates studio matching score (0-20).
 *
 * Strategy: Check if studio names match or one contains the other.
 * Handles cases like "Fantasy Massage" vs "FantasyMassage".
 */
function calculateStudioScore(
    localStudio: string | null,
    remoteSite: PornDBSite | undefined,
): number {
    // Both missing - neutral
    if (!localStudio && !remoteSite) return 10;

    // One missing - can't compare
    if (!localStudio || !remoteSite) return 10;

    const normalizedLocal = localStudio.toLowerCase().replace(/[^a-z0-9]/g, '');
    const normalizedRemote = remoteSite.name.toLowerCase().replace(/[^a-z0-9]/g, '');

    // Exact match after normalization
    if (normalizedLocal === normalizedRemote) {
        return 20;
    }

    // One contains the other
    if (normalizedLocal.includes(normalizedRemote) || normalizedRemote.includes(normalizedLocal)) {
        return 15;
    }

    // Check word overlap
    const localWords = extractWords(localStudio);
    const remoteWords = extractWords(remoteSite.name);

    if (localWords.length > 0 && remoteWords.length > 0) {
        const commonWords = localWords.filter((w) => remoteWords.includes(w));
        if (commonWords.length > 0) {
            return 10;
        }
    }

    return 0;
}

/**
 * Calculates duration matching score (0-20).
 *
 * Strategy: Compare video durations with tolerance.
 * Duration is always computed locally, so it's a reliable signal.
 *
 * Scoring:
 * - Within 10 seconds: 20 points (near perfect match)
 * - Within 30 seconds: 15 points (very close)
 * - Within 60 seconds: 10 points (close enough)
 * - Within 2 minutes: 5 points (roughly similar)
 * - Beyond 2 minutes: 0 points (likely different content)
 */
function calculateDurationScore(localDuration: number, remoteDuration: number | undefined): number {
    // No remote duration available - give neutral score
    if (remoteDuration === undefined || remoteDuration === null || remoteDuration === 0) {
        return 10;
    }

    // No local duration - shouldn't happen but handle it
    if (!localDuration || localDuration === 0) {
        return 10;
    }

    const diff = Math.abs(localDuration - remoteDuration);

    if (diff <= 10) return 20; // Within 10 seconds
    if (diff <= 30) return 15; // Within 30 seconds
    if (diff <= 60) return 10; // Within 1 minute
    if (diff <= 120) return 5; // Within 2 minutes

    return 0; // Too different
}

/**
 * Composable for calculating confidence scores between local scenes and PornDB matches.
 */
export function useConfidenceCalculator() {
    /**
     * Calculates a full confidence breakdown for a local scene vs PornDB match.
     *
     * Weights:
     * - Title: 30 points (word containment)
     * - Actors: 30 points (local actors found in remote)
     * - Studio: 20 points (name matching)
     * - Duration: 20 points (time similarity)
     * Total: 100 points
     */
    function calculateConfidence(local: SceneMatchInfo, remote: PornDBScene): ConfidenceBreakdown {
        const titleScore = calculateTitleScore(local.title, remote.title);
        const actorScore = calculateActorScore(local.actors || [], remote.performers);
        const studioScore = calculateStudioScore(local.studio, remote.site);
        const durationScore = calculateDurationScore(local.duration, remote.duration);
        const total = titleScore + actorScore + studioScore + durationScore;

        return {
            titleScore,
            actorScore,
            studioScore,
            durationScore,
            total,
        };
    }

    return {
        calculateConfidence,
        cleanForComparison,
    };
}
