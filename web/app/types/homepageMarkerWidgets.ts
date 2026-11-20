// Payloads of the marker homepage widgets (internal/core/homepage_widget_*.go)

export interface WidgetMarker {
    id: number;
    scene_id: number;
    scene_title: string;
    label: string;
    color: string;
    timestamp: number;
    end_timestamp?: number | null;
    has_thumbnail: boolean;
    has_animated: boolean;
    // Marker tags (label tag included)
    tags?: { id: number; name: string; color: string; is_from_label: boolean }[];
}

export interface MarkerWidgetPayload {
    items: WidgetMarker[];
    order?: 'random' | 'recent';
    clip_seconds?: number;
}

export interface WidgetTile {
    key: string;
    label: string;
    color?: string;
    count: number;
    cover_marker_id?: number;
    cover_scene_id?: number;
}

export interface BrowseTagsPayload {
    source: 'labels' | 'tags';
    tiles: WidgetTile[];
}
