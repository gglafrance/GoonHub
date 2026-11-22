import type {
    DuplicateGroupKind,
    DuplicateGroupMember,
    DuplicateRelation,
} from '~/types/duplicates';

// Labels, colors and explanations of duplicate relations (member to matched scene) and group
// kinds, shared by the duplicates page, group cards and the compare modal.
export const useDuplicateRelation = () => {
    const relationLabel = (relation: DuplicateRelation, refSceneId?: number) => {
        const ref = refSceneId ? `#${refSceneId}` : 'match';
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
    };

    const relationTitle = (relation: DuplicateRelation, refSceneId?: number) => {
        const ref = refSceneId ? `scene #${refSceneId}` : 'the matched scene';
        switch (relation) {
            case 'identical':
                return `Same video as ${ref}: the match covers nearly all of both scenes`;
            case 'contained':
                return `This scene is a part of ${ref}, which is longer: not a duplicate of it`;
            case 'contains':
                return `${ref[0]!.toUpperCase()}${ref.slice(1)} is a part of this longer scene: not a duplicate of it`;
            case 'partial':
                return `Only a minor part of this scene and ${ref} match`;
            default:
                return 'No matching segment recorded yet: re-run the fingerprint of these scenes to verify';
        }
    };

    const relationClass = (relation: DuplicateRelation) => {
        switch (relation) {
            case 'identical':
                return 'bg-emerald/15 text-emerald';
            case 'contained':
            case 'contains':
                return 'bg-lava/15 text-lava';
            case 'partial':
                return 'bg-amber-500/15 text-amber-400';
            default:
                return 'text-dim bg-white/5';
        }
    };

    const memberRelationLabel = (m: DuplicateGroupMember) =>
        relationLabel(m.relation, m.ref_scene_id);
    const memberRelationTitle = (m: DuplicateGroupMember) =>
        relationTitle(m.relation, m.ref_scene_id);

    const kindLabel = (kind: DuplicateGroupKind) => {
        switch (kind) {
            case 'identical':
                return 'Identical';
            case 'containment':
                return 'Containment';
            case 'partial':
                return 'Partial match';
            default:
                return 'Unverified';
        }
    };

    const kindTitle = (kind: DuplicateGroupKind) =>
        kind === 'identical'
            ? 'Every scene is the same video: safe for auto-select and auto-resolve'
            : 'Not all scenes are identical: skipped by auto-select and auto-resolve, resolve it by hand';

    const kindClass = (kind: DuplicateGroupKind) => {
        switch (kind) {
            case 'identical':
                return 'bg-emerald/15 text-emerald';
            case 'containment':
                return 'bg-lava/15 text-lava';
            case 'partial':
                return 'bg-amber-500/15 text-amber-400';
            default:
                return 'text-dim bg-white/5';
        }
    };

    return {
        relationLabel,
        relationTitle,
        relationClass,
        memberRelationLabel,
        memberRelationTitle,
        kindLabel,
        kindTitle,
        kindClass,
    };
};
