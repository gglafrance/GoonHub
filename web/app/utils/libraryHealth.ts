// Categories of the library health page (Settings > Health), grouped as displayed.

import type { HealthAction, HealthCategory, HealthPhase } from '~/types/libraryHealth';

export interface HealthCategoryInfo {
    key: HealthCategory;
    label: string;
    description: string;
    /** Action offered for the category: queue the missing jobs of a phase, or fix the sizes */
    action?: HealthAction;
    /** Comes from the background missing-file check rather than live database counts */
    fromCheck?: boolean;
}

export interface HealthCategoryGroup {
    id: string;
    label: string;
    icon: string;
    categories: HealthCategoryInfo[];
}

export const HEALTH_GROUPS: HealthCategoryGroup[] = [
    {
        id: 'files',
        label: 'Files',
        icon: 'heroicons:document',
        categories: [
            {
                key: 'missing_file',
                label: 'Missing files',
                description: 'The stored path does not exist on disk',
                fromCheck: true,
            },
            {
                key: 'unreadable_file',
                label: 'Unreadable files',
                description: 'The file could not be read (permissions, I/O error, not a file)',
                fromCheck: true,
            },
            {
                key: 'corrupted',
                label: 'Corrupted',
                description: 'Flagged as corrupted during processing',
            },
            {
                key: 'size_zero',
                label: 'Size 0',
                description: 'Stored with a file size of 0 bytes',
                action: 'fix_sizes',
            },
            {
                key: 'duration_zero',
                label: 'Duration 0',
                description: 'No metadata yet (duration, resolution, codecs)',
                action: 'metadata',
            },
        ],
    },
    {
        id: 'playback',
        label: 'Playback',
        icon: 'heroicons:play-circle',
        categories: [
            {
                key: 'unplayable',
                label: 'Not playable in browsers',
                description:
                    'Container or codec browsers cannot play (AVI, MKV, MPEG-2, HEVC, AC-3...)',
            },
        ],
    },
    {
        id: 'assets',
        label: 'Generated assets',
        icon: 'heroicons:photo',
        categories: [
            {
                key: 'no_thumbnail',
                label: 'No thumbnail',
                description: 'No preview image',
                action: 'thumbnail',
            },
            {
                key: 'no_sprites',
                label: 'No sprites',
                description: 'No sprite sheet for the timeline preview',
                action: 'sprites',
            },
            {
                key: 'no_preview',
                label: 'No animated preview',
                description: 'No hover preview video',
                action: 'animated_thumbnails',
            },
            {
                key: 'no_fingerprint',
                label: 'No fingerprint',
                description: 'Not fingerprinted for duplicate detection',
                action: 'fingerprint',
            },
        ],
    },
    {
        id: 'storage',
        label: 'Storage',
        icon: 'heroicons:folder',
        categories: [
            {
                key: 'no_storage_path',
                label: 'In no storage path',
                description: 'Not attached to a configured storage path',
            },
            {
                key: 'outside_storage_path',
                label: 'Outside every storage path',
                description: 'The stored path is not under any configured storage path',
            },
        ],
    },
];

export const healthCategoryInfo = (key: HealthCategory): HealthCategoryInfo | undefined =>
    HEALTH_GROUPS.flatMap((g) => g.categories).find((c) => c.key === key);

const PHASE_LABELS: Record<HealthPhase, string> = {
    metadata: 'metadata',
    thumbnail: 'thumbnail',
    sprites: 'sprites',
    animated_thumbnails: 'animated thumbnail',
    fingerprint: 'fingerprint',
};

export const isHealthPhase = (action: HealthAction): action is HealthPhase =>
    action in PHASE_LABELS;

/** Short label of the button of an action (the confirmation names the phase) */
export const healthActionLabel = (action: HealthAction): string => {
    if (isHealthPhase(action)) return 'Queue jobs';
    return action === 'fix_sizes' ? 'Fix sizes' : 'Check files';
};

export const healthPhaseLabel = (phase: HealthPhase): string => PHASE_LABELS[phase];
