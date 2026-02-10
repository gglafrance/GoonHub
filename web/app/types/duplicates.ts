export interface DuplicateConfig {
    enabled: boolean;
    check_on_upload: boolean;
    match_threshold: number;
    hamming_distance: number;
    sample_interval: number;
    duplicate_action: 'flag' | 'mark' | 'trash';
    keep_best_rules: string[];
    keep_best_enabled: Record<string, boolean>;
    codec_preference: string[];
}

export interface DuplicateGroup {
    id: number;
    status: 'pending' | 'resolved' | 'dismissed';
    winner_scene_id: number | null;
    members: DuplicateGroupMember[];
    created_at: string;
    updated_at: string;
}

export interface DuplicateGroupMember {
    id: number;
    scene_id: number;
    match_percentage: number;
    frame_offset: number;
    is_winner: boolean;
    scene?: DuplicateSceneSummary;
}

export interface DuplicateSceneSummary {
    id: number;
    title: string;
    duration: number;
    width: number;
    height: number;
    video_codec: string;
    bit_rate: number;
    file_size: number;
    thumbnail_path: string;
}

export interface RescanStatus {
    running: boolean;
    total: number;
    completed: number;
    matched: number;
}

export interface DuplicateGroupListResponse {
    data: DuplicateGroup[];
    pagination: {
        page: number;
        limit: number;
        total_items: number;
        total_pages: number;
    };
}
