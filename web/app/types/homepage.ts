import type { SceneListItem } from './scene';
import type { PlaylistListItem } from './playlist';

// Built-in scene list sections
export type SceneListSectionType =
    | 'latest'
    | 'actor'
    | 'studio'
    | 'tag'
    | 'saved_search'
    | 'continue_watching'
    | 'most_viewed'
    | 'liked'
    | 'playlist';

// Widget sections (backend: internal/core/homepage_widget_<type>.go, frontend:
// components/homepage/widgets/<PascalType>.vue, optional config form:
// components/settings/homepage/widget-config/<PascalType>.vue)
export type WidgetSectionType =
    // Scenes
    | 'discover'
    | 'shorts'
    | 'rediscover'
    | 'roulette'
    | 'on_this_day'
    | 'folder_latest'
    | 'because_you_watched'
    | 'showcase_4k'
    | 'long_form'
    | 'top_period'
    | 'hero'
    | 'preview_wall'
    | 'quick_rate'
    // Markers
    | 'moments'
    | 'marker_compilation'
    | 'recent_markers'
    | 'browse_tags'
    // Stats
    | 'stats'
    | 'activity_calendar'
    | 'watch_times'
    | 'records'
    | 'jizz_insights'
    | 'trends'
    | 'performer_spotlight'
    // Tools
    | 'curation_todo'
    | 'processing_status'
    | 'text_block'
    | 'saved_search_tiles';

export type SectionType = SceneListSectionType | WidgetSectionType;

export type SectionCategory = 'scenes' | 'markers' | 'stats' | 'tools';

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

export interface HomepageTab {
    id: string;
    name: string;
    icon: string;
    sections: HomepageSection[];
}

export type HomepageOpenOn = 'last' | 'default';

export interface HomepageConfig {
    show_upload: boolean;
    // Legacy single-page sections (always empty: migrated into tabs by the server)
    sections: HomepageSection[];
    tabs: HomepageTab[];
    default_tab_id: string;
    open_on: HomepageOpenOn;
}

export interface WatchProgress {
    last_position: number;
    duration: number;
}

export interface HomepageSectionData<TWidget = unknown> {
    section: HomepageSection;
    scenes: SceneListItem[];
    total: number;
    seed?: number;
    watch_progress?: Record<number, WatchProgress>;
    ratings?: Record<number, number>;
    playlists?: PlaylistListItem[];
    // Payload of widget sections
    widget?: TWidget;
}

export interface HomepageResponse {
    config: HomepageConfig;
    active_tab_id: string;
    sections: HomepageSectionData[];
}

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

export interface SectionTypeMeta {
    label: string;
    description: string;
    icon: string;
    // Tailwind color classes (text + background)
    color: string;
    category: SectionCategory;
    // Sort options (empty: no sort selector)
    sort: typeof SORT_OPTIONS;
    // Default item count, and whether the "Show N items" input applies
    defaultLimit: number;
    usesLimit: boolean;
    // Rendered by components/homepage/widgets/<PascalType>.vue
    widget: boolean;
    // Admin only (hidden for other users)
    adminOnly?: boolean;
}

const sceneSort = SORT_OPTIONS;
const noSort: typeof SORT_OPTIONS = [];

export const SECTION_TYPE_META: Record<SectionType, SectionTypeMeta> = {
    // --- Built-in scene lists ---
    latest: {
        label: 'Latest Uploads',
        description: 'Recently uploaded scenes',
        icon: 'heroicons:clock',
        color: 'text-blue-400 bg-blue-400/10',
        category: 'scenes',
        sort: sceneSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },
    actor: {
        label: 'Actor',
        description: 'Scenes featuring a specific actor',
        icon: 'heroicons:user',
        color: 'text-purple-400 bg-purple-400/10',
        category: 'scenes',
        sort: sceneSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },
    studio: {
        label: 'Studio',
        description: 'Scenes from a specific studio',
        icon: 'heroicons:building-office-2',
        color: 'text-amber-400 bg-amber-400/10',
        category: 'scenes',
        sort: sceneSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },
    tag: {
        label: 'Tag',
        description: 'Scenes with a specific tag',
        icon: 'heroicons:tag',
        color: 'text-emerald bg-emerald/10',
        category: 'scenes',
        sort: sceneSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },
    saved_search: {
        label: 'Saved Search',
        description: 'Scenes matching a saved search',
        icon: 'heroicons:bookmark',
        color: 'text-cyan-400 bg-cyan-400/10',
        category: 'scenes',
        sort: [{ value: '', label: 'From Template' }, ...SORT_OPTIONS],
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },
    continue_watching: {
        label: 'Continue Watching',
        description: 'Resume where you left off',
        icon: 'heroicons:play',
        color: 'text-lava bg-lava/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },
    most_viewed: {
        label: 'Most Viewed',
        description: 'Popular scenes by view count',
        icon: 'heroicons:fire',
        color: 'text-orange-400 bg-orange-400/10',
        category: 'scenes',
        sort: [
            { value: 'view_count_desc', label: 'Most Viewed' },
            { value: 'view_count_asc', label: 'Least Viewed' },
        ],
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },
    liked: {
        label: 'Liked Scenes',
        description: 'Your liked scenes',
        icon: 'heroicons:heart',
        color: 'text-pink-400 bg-pink-400/10',
        category: 'scenes',
        sort: sceneSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },
    playlist: {
        label: 'Playlists',
        description: 'Your playlists',
        icon: 'heroicons:queue-list',
        color: 'text-indigo-400 bg-indigo-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: false,
    },

    // --- Scene widgets ---
    discover: {
        label: 'Discover',
        description: 'Random scenes you have never watched, reshuffled on demand',
        icon: 'heroicons:sparkles',
        color: 'text-lava bg-lava/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    shorts: {
        label: 'Shorts',
        description: 'Short clips playing on their own, zapping style',
        icon: 'heroicons:bolt',
        color: 'text-yellow-400 bg-yellow-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    rediscover: {
        label: 'Rediscover',
        description: 'Favorites you have not watched for a while',
        icon: 'heroicons:arrow-uturn-left',
        color: 'text-pink-400 bg-pink-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    roulette: {
        label: 'Roulette',
        description: 'One click to play a random scene matching your criteria',
        icon: 'heroicons:cube',
        color: 'text-emerald bg-emerald/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
    },
    on_this_day: {
        label: 'On This Day',
        description: 'What you watched or added on this date in past years',
        icon: 'heroicons:calendar-days',
        color: 'text-amber-400 bg-amber-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    folder_latest: {
        label: 'New in Folder',
        description: 'Latest additions from a storage path or folder',
        icon: 'heroicons:folder-open',
        color: 'text-blue-400 bg-blue-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    because_you_watched: {
        label: 'Because You Watched',
        description: 'Scenes similar to your last watch',
        icon: 'heroicons:link',
        color: 'text-purple-400 bg-purple-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    showcase_4k: {
        label: '4K Showcase',
        description: 'Your highest resolution scenes',
        icon: 'heroicons:tv',
        color: 'text-cyan-400 bg-cyan-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    long_form: {
        label: 'Long Form',
        description: 'Long scenes, unwatched or in progress',
        icon: 'heroicons:film',
        color: 'text-indigo-400 bg-indigo-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    top_period: {
        label: 'Top of the Month',
        description: 'Most watched scenes over a period, compared to the previous one',
        icon: 'heroicons:trophy',
        color: 'text-orange-400 bg-orange-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    hero: {
        label: 'Spotlight',
        description: 'A large banner featuring one scene with its preview',
        icon: 'heroicons:star',
        color: 'text-lava bg-lava/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 5,
        usesLimit: true,
        widget: true,
    },
    preview_wall: {
        label: 'Preview Wall',
        description: 'A mosaic of previews playing together',
        icon: 'heroicons:squares-2x2',
        color: 'text-yellow-400 bg-yellow-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    quick_rate: {
        label: 'Quick Rate',
        description: 'Rate or like recently watched scenes, one card at a time',
        icon: 'heroicons:star',
        color: 'text-amber-400 bg-amber-400/10',
        category: 'scenes',
        sort: noSort,
        defaultLimit: 20,
        usesLimit: true,
        widget: true,
    },

    // --- Markers ---
    moments: {
        label: 'Moments',
        description: 'Carousel of marker previews by label',
        icon: 'heroicons:bookmark',
        color: 'text-lava bg-lava/10',
        category: 'markers',
        sort: noSort,
        defaultLimit: 16,
        usesLimit: true,
        widget: true,
    },
    marker_compilation: {
        label: 'Auto Compilation',
        description: 'A player chaining the markers of a label',
        icon: 'heroicons:queue-list',
        color: 'text-pink-400 bg-pink-400/10',
        category: 'markers',
        sort: noSort,
        defaultLimit: 30,
        usesLimit: true,
        widget: true,
    },
    recent_markers: {
        label: 'Recent Markers',
        description: 'The markers you added recently',
        icon: 'heroicons:bookmark-square',
        color: 'text-purple-400 bg-purple-400/10',
        category: 'markers',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
    browse_tags: {
        label: 'Browse by Tag',
        description: 'Tiles of tags or marker labels with a cover',
        icon: 'heroicons:rectangle-group',
        color: 'text-emerald bg-emerald/10',
        category: 'markers',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },

    // --- Stats ---
    stats: {
        label: 'Library Stats',
        description: 'Scenes, hours, size, watched share, views of the month',
        icon: 'heroicons:chart-bar',
        color: 'text-cyan-400 bg-cyan-400/10',
        category: 'stats',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
    },
    activity_calendar: {
        label: 'Activity Calendar',
        description: 'Daily views or O heatmap with streaks',
        icon: 'heroicons:calendar',
        color: 'text-emerald bg-emerald/10',
        category: 'stats',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
    },
    watch_times: {
        label: 'When You Watch',
        description: 'Views and O by hour of the day and day of the week',
        icon: 'heroicons:clock',
        color: 'text-blue-400 bg-blue-400/10',
        category: 'stats',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
    },
    records: {
        label: 'Records',
        description: 'Most watched scene, longest streak, biggest file...',
        icon: 'heroicons:trophy',
        color: 'text-amber-400 bg-amber-400/10',
        category: 'stats',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
    },
    jizz_insights: {
        label: 'O Insights',
        description: 'Dated O per month, favorite hour and day, top scenes',
        icon: 'heroicons:chart-bar-square',
        color: 'text-lava bg-lava/10',
        category: 'stats',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
    },
    trends: {
        label: 'Trends',
        description: 'Tags and actors rising in your recent views',
        icon: 'heroicons:arrow-trending-up',
        color: 'text-lava bg-lava/10',
        category: 'stats',
        sort: noSort,
        defaultLimit: 8,
        usesLimit: true,
        widget: true,
    },
    performer_spotlight: {
        label: 'Top Performers',
        description: 'Your most watched performers with their stats',
        icon: 'heroicons:user-group',
        color: 'text-purple-400 bg-purple-400/10',
        category: 'stats',
        sort: noSort,
        defaultLimit: 8,
        usesLimit: true,
        widget: true,
    },

    // --- Tools ---
    curation_todo: {
        label: 'To Organize',
        description: 'Scenes missing tags, actors or studio, and unresolved duplicates',
        icon: 'heroicons:clipboard-document-check',
        color: 'text-orange-400 bg-orange-400/10',
        category: 'tools',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
    },
    processing_status: {
        label: 'Processing Status',
        description: 'Progress of the thumbnail, sprite and fingerprint queues',
        icon: 'heroicons:cpu-chip',
        color: 'text-cyan-400 bg-cyan-400/10',
        category: 'tools',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
        adminOnly: true,
    },
    text_block: {
        label: 'Text & Links',
        description: 'Free text and shortcut links',
        icon: 'heroicons:document-text',
        color: 'text-dim bg-white/5',
        category: 'tools',
        sort: noSort,
        defaultLimit: 1,
        usesLimit: false,
        widget: true,
    },
    saved_search_tiles: {
        label: 'Search Shortcuts',
        description: 'Your saved searches as tiles with counts',
        icon: 'heroicons:magnifying-glass',
        color: 'text-cyan-400 bg-cyan-400/10',
        category: 'tools',
        sort: noSort,
        defaultLimit: 12,
        usesLimit: true,
        widget: true,
    },
};

export const SECTION_CATEGORY_LABELS: Record<SectionCategory, string> = {
    scenes: 'Scenes',
    markers: 'Markers',
    stats: 'Stats',
    tools: 'Tools',
};

const mapMeta = <T>(pick: (m: SectionTypeMeta) => T) =>
    Object.fromEntries(
        Object.entries(SECTION_TYPE_META).map(([type, meta]) => [type, pick(meta)]),
    ) as Record<SectionType, T>;

export const SECTION_TYPE_LABELS = mapMeta((m) => m.label);
export const SECTION_SORT_OPTIONS = mapMeta((m) => m.sort);
export const SECTION_ICONS = mapMeta((m) => m.icon);
export const SECTION_COLORS = mapMeta((m) => m.color);
export const SECTION_COLORS_WITH_BORDER = mapMeta((m) => {
    const text = m.color.split(' ')[0] ?? 'text-dim';
    const border = text.startsWith('text-') ? `border-${text.slice(5)}/20` : 'border-white/10';
    return `${m.color} ${border}`;
});
export const SECTION_DESCRIPTIONS = mapMeta((m) => m.description);

/** Converts a section type to the PascalCase file name of its widget components. */
export function sectionTypeComponentName(type: string): string {
    return type
        .split('_')
        .map((part) => (part === '4k' ? '4K' : part.charAt(0).toUpperCase() + part.slice(1)))
        .join('');
}
