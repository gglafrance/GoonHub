// Server-wide marker labels (docs/MARKER_LABELS.md). Managed with marker_labels:manage.
import type { Tag } from '~/types/tag';

export interface MarkerLabel {
    id: number;
    name: string;
    color: string;
    description: string;
    created_at: string;
    updated_at: string;
}

/** GET /marker-labels item: usage over every user's markers, and the default tags. */
export interface MarkerLabelWithUsage extends MarkerLabel {
    marker_count: number;
    user_count: number;
    default_tags: Tag[];
}

export interface MarkerLabelInput {
    name?: string;
    color?: string;
    description?: string;
}

export interface MarkerLabelMergeResult {
    target: MarkerLabel;
    sources: MarkerLabel[];
    dry_run: boolean;
    markers_moved: number;
    default_tags_added: number;
    references_updated: number;
    sources_deleted: number;
}

export interface MarkerLabelDeleteResult {
    deleted: number;
    markers_cleared: number;
}
