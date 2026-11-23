// Relation of a member to the scene it was matched against (ref_scene_id): identical,
// contained (this scene is a clip of the reference), contains (the reference is a part of this
// scene), partial, or unknown (no matching segment yet)
export type DuplicateRelation = 'identical' | 'contained' | 'contains' | 'partial' | 'unknown';

// Summary of the members' relations. Only identical groups are resolved automatically.
export type DuplicateGroupKind = 'identical' | 'containment' | 'partial' | 'unverified';

export interface DuplicateGroup {
    id: number;
    status: 'unresolved' | 'resolved' | 'dismissed';
    scene_count: number;
    best_scene_id: number | null;
    kind: DuplicateGroupKind;
    members: DuplicateGroupMember[];
    created_at: string;
    updated_at: string;
    resolved_at?: string;
}

export interface DuplicateGroupMember {
    scene_id: number;
    title: string;
    duration: number;
    width: number;
    height: number;
    video_codec: string;
    audio_codec: string;
    bit_rate: number;
    size: number;
    thumbnail_path: string;
    is_best: boolean;
    confidence_score: number;
    match_type: 'audio' | 'visual';
    is_trashed: boolean;
    trashed_at?: string;
    file_name: string;
    // Matching segment in this scene and the corresponding one in ref_scene_id (seconds)
    match_start?: number;
    match_end?: number;
    ref_scene_id?: number;
    ref_start?: number;
    ref_end?: number;
    relation: DuplicateRelation;
    // When this scene (not kept at resolution) will be permanently deleted
    delete_after?: string;
}

// A group left untouched by an automatic action because not all its scenes are identical
export interface DuplicateSkippedGroup {
    group_id: number;
    kind: DuplicateGroupKind;
    reason: string;
    members: { scene_id: number; relation: DuplicateRelation; ref_scene_id?: number }[];
}

// What happened to the markers of the trashed scenes when a group was resolved with metadata
// merge: moved to the kept scene (shifted: with their times changed by the offset between the
// scenes), deleted there as duplicates of another marker, or left on the trashed scene
// (outside the part matching the kept scene)
export interface MarkerMergeSummary {
    moved: number;
    shifted: number;
    deduplicated: number;
    left_behind: number;
}

export interface ResolveGroupResult {
    message: string;
    markers: MarkerMergeSummary;
}

export interface BulkResolveResult {
    resolved: number[];
    failed: { group_id: number; error: string }[];
    skipped: DuplicateSkippedGroup[];
    markers: MarkerMergeSummary;
}

// POST /admin/duplicates/:id/trash-all: scenes moved to the trash, and scenes left alone
// (already in the trash or missing)
export interface TrashAllResult {
    trashed: number[];
    skipped: number[];
}

// POST /admin/duplicates/trash-all-bulk
// Unresolved group IDs for "select all" (at most limit, the bulk actions' maximum)
export interface SelectableDuplicateIds {
    ids: number[];
    total: number;
    limit: number;
}

export interface BulkTrashAllResult {
    resolved: number[];
    failed: { group_id: number; error: string }[];
    trashed_scenes: number;
    skipped_scenes: number;
}

export interface AutoSelectResult {
    selected: Record<string, number>;
    skipped: DuplicateSkippedGroup[];
}

export interface DuplicateStats {
    unresolved: number;
    resolved: number;
    dismissed: number;
    total: number;
}

export interface DuplicationConfig {
    audio_density_threshold: number;
    audio_min_hashes: number;
    audio_max_hash_occurrences: number;
    audio_min_span: number;
    visual_hamming_max: number;
    visual_min_frames: number;
    visual_min_span: number;
    delta_tolerance: number;
    fingerprint_mode: 'audio_only' | 'dual';
    delete_resolved_files: boolean;
    deletion_delay_hours: number;
    best_selection_criteria: string[];
    available_best_criteria?: string[];
}

// Matching part of the current scene and of a similar one (seconds): this scene from
// this_start to this_end matches the other one from other_start to other_end
export interface SimilarSegment {
    this_start: number;
    this_end: number;
    other_start: number;
    other_end: number;
}

// A scene sharing the duplicate group (fingerprint match) of the watched scene
export interface SimilarScene {
    id: number;
    title: string;
    duration: number;
    width: number;
    height: number;
    size: number;
    thumbnail_path: string;
    // Only admins get trashed scenes
    is_trashed: boolean;
    is_best: boolean;
    confidence_score: number;
    match_type: 'audio' | 'visual';
    // Relation of this scene to relation_ref_scene_id: when that is the watched scene,
    // contained = a clip of it, contains = contains it; else as stored, to another scene
    // (null when that scene is not visible)
    relation: DuplicateRelation;
    relation_ref_scene_id: number | null;
    segment: SimilarSegment | null;
}

// GET /scenes/:id/similar
export interface SimilarScenesResponse {
    scene_id: number;
    group_id: number | null;
    group_status?: DuplicateGroup['status'];
    scenes: SimilarScene[];
}
