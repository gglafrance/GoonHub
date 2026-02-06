import type { SceneListItem } from './scene';
import type { PlaylistListItem } from './playlist';

export type SectionType =
    | 'latest'
    | 'actor'
    | 'studio'
    | 'tag'
    | 'saved_search'
    | 'continue_watching'
    | 'most_viewed'
    | 'liked'
    | 'playlist';

export interface HomepageSection {
    id: string;
    type: SectionType;
    title: string;
    enabled: boolean;
    limit: number;
    order: number;
    sort: string;
    config: Record<string, unknown>;
}

export interface HomepageConfig {
    show_upload: boolean;
    sections: HomepageSection[];
}

export interface WatchProgress {
    last_position: number;
    duration: number;
}

export interface HomepageSectionData {
    section: HomepageSection;
    scenes: SceneListItem[];
    total: number;
    seed?: number;
    watch_progress?: Record<number, WatchProgress>;
    ratings?: Record<number, number>;
    playlists?: PlaylistListItem[];
}

export interface HomepageResponse {
    config: HomepageConfig;
    sections: HomepageSectionData[];
}

export const SECTION_TYPE_LABELS: Record<SectionType, string> = {
    latest: 'Latest Uploads',
    actor: 'Actor',
    studio: 'Studio',
    tag: 'Tag',
    saved_search: 'Saved Search',
    continue_watching: 'Continue Watching',
    most_viewed: 'Most Viewed',
    liked: 'Liked Scenes',
    playlist: 'Playlists',
};

export const SORT_OPTIONS = [
    { value: 'created_at_desc', label: 'Newest First' },
    { value: 'created_at_asc', label: 'Oldest First' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest First' },
    { value: 'duration_desc', label: 'Longest First' },
    { value: 'view_count_desc', label: 'Most Viewed' },
    { value: 'view_count_asc', label: 'Least Viewed' },
    { value: 'random', label: 'Random' },
];

// Sort options available for each section type
// Some section types have restricted sort options
export const SECTION_SORT_OPTIONS: Record<SectionType, typeof SORT_OPTIONS> = {
    latest: SORT_OPTIONS,
    actor: SORT_OPTIONS,
    studio: SORT_OPTIONS,
    tag: SORT_OPTIONS,
    saved_search: [{ value: '', label: 'From Template' }, ...SORT_OPTIONS],
    continue_watching: [], // No sorting - ordered by watch position
    most_viewed: [
        { value: 'view_count_desc', label: 'Most Viewed' },
        { value: 'view_count_asc', label: 'Least Viewed' },
    ],
    liked: SORT_OPTIONS,
    playlist: [], // No scene sorting - displays playlists, not scenes
};

// Icon names for each section type (heroicons)
export const SECTION_ICONS: Record<SectionType, string> = {
    latest: 'heroicons:clock',
    actor: 'heroicons:user',
    studio: 'heroicons:building-office-2',
    tag: 'heroicons:tag',
    saved_search: 'heroicons:bookmark',
    continue_watching: 'heroicons:play',
    most_viewed: 'heroicons:fire',
    liked: 'heroicons:heart',
    playlist: 'heroicons:queue-list',
};

// Color classes for each section type (icon + background styling)
export const SECTION_COLORS: Record<SectionType, string> = {
    latest: 'text-blue-400 bg-blue-400/10',
    actor: 'text-purple-400 bg-purple-400/10',
    studio: 'text-amber-400 bg-amber-400/10',
    tag: 'text-emerald bg-emerald/10',
    saved_search: 'text-cyan-400 bg-cyan-400/10',
    continue_watching: 'text-lava bg-lava/10',
    most_viewed: 'text-orange-400 bg-orange-400/10',
    liked: 'text-pink-400 bg-pink-400/10',
    playlist: 'text-indigo-400 bg-indigo-400/10',
};

// Extended color classes including border (for modal type selection)
export const SECTION_COLORS_WITH_BORDER: Record<SectionType, string> = {
    latest: 'text-blue-400 bg-blue-400/10 border-blue-400/20',
    actor: 'text-purple-400 bg-purple-400/10 border-purple-400/20',
    studio: 'text-amber-400 bg-amber-400/10 border-amber-400/20',
    tag: 'text-emerald bg-emerald/10 border-emerald/20',
    saved_search: 'text-cyan-400 bg-cyan-400/10 border-cyan-400/20',
    continue_watching: 'text-lava bg-lava/10 border-lava/20',
    most_viewed: 'text-orange-400 bg-orange-400/10 border-orange-400/20',
    liked: 'text-pink-400 bg-pink-400/10 border-pink-400/20',
    playlist: 'text-indigo-400 bg-indigo-400/10 border-indigo-400/20',
};

// Section type descriptions for UI
export const SECTION_DESCRIPTIONS: Record<SectionType, string> = {
    latest: 'Recently uploaded scenes',
    actor: 'Scenes featuring a specific actor',
    studio: 'Scenes from a specific studio',
    tag: 'Scenes with a specific tag',
    saved_search: 'Scenes matching a saved search',
    continue_watching: 'Resume where you left off',
    most_viewed: 'Popular scenes by view count',
    liked: 'Your liked scenes',
    playlist: 'Your playlists',
};
