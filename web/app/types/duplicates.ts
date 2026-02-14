export interface DuplicateGroup {
    id: number
    status: 'unresolved' | 'resolved' | 'dismissed'
    scene_count: number
    best_scene_id: number | null
    members: DuplicateGroupMember[]
    created_at: string
    updated_at: string
    resolved_at?: string
}

export interface DuplicateGroupMember {
    scene_id: number
    title: string
    duration: number
    width: number
    height: number
    video_codec: string
    audio_codec: string
    bit_rate: number
    size: number
    thumbnail_path: string
    is_best: boolean
    confidence_score: number
    match_type: 'audio' | 'visual'
    is_trashed: boolean
    trashed_at?: string
}

export interface DuplicateStats {
    unresolved: number
    resolved: number
    dismissed: number
    total: number
}

export interface DuplicationConfig {
    audio_density_threshold: number
    audio_min_hashes: number
    audio_max_hash_occurrences: number
    audio_min_span: number
    visual_hamming_max: number
    visual_min_frames: number
    visual_min_span: number
    delta_tolerance: number
    fingerprint_mode: 'audio_only' | 'dual'
}
