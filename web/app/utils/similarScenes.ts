// Pure helpers of the watch page's Similar tab: scenes sharing the duplicate group
// (fingerprint match) of the watched scene.
import type { DuplicateRelation, SimilarScene, SimilarSegment } from '~/types/duplicates';
import { formatMarkerTime } from './markerTime';

/** A time range as "01:10–02:40". */
export function formatSegmentRange(start: number, end: number): string {
    return `${formatMarkerTime(start)}–${formatMarkerTime(end)}`;
}

/** Where the scenes match: "matches 01:10–02:40 here, 00:00–01:30 there". */
export function describeSimilarSegment(segment: SimilarSegment | null): string {
    if (!segment) return 'no matching segment recorded';
    return (
        `matches ${formatSegmentRange(segment.this_start, segment.this_end)} here, ` +
        `${formatSegmentRange(segment.other_start, segment.other_end)} there`
    );
}

/** Whether the relation is relative to the watched scene (else to another scene). */
export function isRelativeToCurrent(scene: SimilarScene, currentSceneId: number): boolean {
    return scene.relation_ref_scene_id === currentSceneId;
}

/** Short label of how a similar scene relates to the watched one (or to its own match). */
export function similarRelationLabel(scene: SimilarScene, currentSceneId: number): string {
    const ref = isRelativeToCurrent(scene, currentSceneId)
        ? 'this'
        : scene.relation_ref_scene_id
          ? `#${scene.relation_ref_scene_id}`
          : 'its match';
    return relationLabelTo(scene.relation, ref);
}

function relationLabelTo(relation: DuplicateRelation, ref: string): string {
    switch (relation) {
        case 'identical':
            return 'Identical';
        case 'contained':
            return `Clip of ${ref}`;
        case 'contains':
            return `Contains ${ref}`;
        case 'partial':
            return 'Partial';
        default:
            return 'Unverified';
    }
}

/** Explanation of the relation label, for its tooltip. */
export function similarRelationTitle(scene: SimilarScene, currentSceneId: number): string {
    const relative = isRelativeToCurrent(scene, currentSceneId);
    const ref = relative
        ? 'this scene'
        : scene.relation_ref_scene_id
          ? `scene #${scene.relation_ref_scene_id}`
          : 'the scene it was matched against';
    switch (scene.relation) {
        case 'identical':
            return `Same video as ${ref}: the match covers nearly all of both`;
        case 'contained':
            return `A part of ${ref}, which is longer`;
        case 'contains':
            return `Contains ${ref}, and more`;
        case 'partial':
            return `Only a minor part of it and ${ref} match`;
        default:
            return 'No matching segment recorded yet: re-run the fingerprint to verify';
    }
}

/** Match confidence as a percentage, e.g. "87%". */
export function formatConfidence(score: number): string {
    const pct = Number.isFinite(score) ? Math.round(Math.min(Math.max(score, 0), 1) * 100) : 0;
    return `${pct}%`;
}

/** Link opening a similar scene, at its matched start when known. */
export function similarSceneLink(scene: Pick<SimilarScene, 'id' | 'segment'>): {
    path: string;
    query?: { t: string };
} {
    const path = `/watch/${scene.id}`;
    const start = scene.segment?.other_start;
    if (start === undefined || !Number.isFinite(start)) return { path };
    return { path, query: { t: String(Math.max(0, Math.floor(start))) } };
}

/** Link to the duplicate group on the (admin) Duplicates page. */
export function duplicateGroupLink(
    groupId: number,
    status?: string,
): { path: string; query: Record<string, string> } {
    const query: Record<string, string> = { group: String(groupId) };
    if (status) query.status = status;
    return { path: '/duplicates', query };
}
