export interface Tag {
    id: number;
    name: string;
    color: string;
    created_at: string;
    scene_count: number;
}

/** Tags whose names are equal ignoring case and extra spaces (GET /tags/duplicates). */
export interface TagDuplicateGroup {
    key: string;
    suggested_target_id: number;
    tags: Tag[];
}

/** Answer of POST /tags/merge (counts of what was, or would be for a dry run, changed). */
export interface TagMergeResult {
    target: Omit<Tag, 'scene_count'>;
    sources: Omit<Tag, 'scene_count'>[];
    dry_run: boolean;
    scenes_retagged: number;
    markers_updated: number;
    label_tags_updated: number;
    playlists_updated: number;
    references_updated: number;
    sources_deleted: number;
}
