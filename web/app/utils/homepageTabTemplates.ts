import type { HomepageSection, HomepageTab, SectionType } from '~/types/homepage';
import { SECTION_TYPE_META } from '~/types/homepage';

export interface HomepageTabTemplate {
    id: string;
    name: string;
    icon: string;
    description: string;
    sections: { type: SectionType; title?: string; config?: Record<string, unknown> }[];
}

// Ready-made tabs offered in the homepage editor
export const HOMEPAGE_TAB_TEMPLATES: HomepageTabTemplate[] = [
    {
        id: 'blank',
        name: 'New tab',
        icon: 'heroicons:squares-2x2',
        description: 'An empty tab',
        sections: [],
    },
    {
        id: 'discover',
        name: 'Discover',
        icon: 'heroicons:sparkles',
        description: 'Unwatched picks, roulette, rediscoveries and similar scenes',
        sections: [
            { type: 'roulette' },
            { type: 'discover' },
            { type: 'because_you_watched' },
            { type: 'rediscover' },
            { type: 'long_form' },
        ],
    },
    {
        id: 'shorts',
        name: 'Shorts',
        icon: 'heroicons:bolt',
        description: 'Short clips and a live preview wall',
        sections: [{ type: 'shorts' }, { type: 'preview_wall' }],
    },
    {
        id: 'moments',
        name: 'Moments',
        icon: 'heroicons:bookmark',
        description: 'Marker previews, auto compilations and tag tiles',
        sections: [
            { type: 'moments' },
            { type: 'marker_compilation' },
            { type: 'recent_markers' },
            { type: 'browse_tags' },
        ],
    },
    {
        id: 'stats',
        name: 'Stats',
        icon: 'heroicons:chart-bar',
        description: 'Library stats, activity, records, trends and top performers',
        sections: [
            { type: 'stats' },
            { type: 'activity_calendar' },
            { type: 'watch_times' },
            { type: 'records' },
            { type: 'top_period' },
            { type: 'trends' },
            { type: 'performer_spotlight' },
        ],
    },
    {
        id: 'organize',
        name: 'Organize',
        icon: 'heroicons:clipboard-document-check',
        description: 'Quick rating, curation to-do and processing status',
        sections: [
            { type: 'quick_rate' },
            { type: 'curation_todo' },
            { type: 'processing_status' },
        ],
    },
];

const uniqueId = (prefix: string) =>
    `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`;

/** Builds a new tab (with fresh IDs) from a template. */
export function createTabFromTemplate(template: HomepageTabTemplate): HomepageTab {
    const sections: HomepageSection[] = template.sections.map((s, order) => {
        const meta = SECTION_TYPE_META[s.type];
        return {
            id: uniqueId('section'),
            type: s.type,
            title: s.title ?? meta.label,
            enabled: true,
            limit: meta.defaultLimit,
            order,
            sort: meta.sort[0]?.value ?? '',
            config: { ...(s.config ?? {}) },
        };
    });
    return { id: uniqueId('tab'), name: template.name, icon: template.icon, sections };
}

// Icons offered for tabs
export const HOMEPAGE_TAB_ICONS = [
    'heroicons:home',
    'heroicons:sparkles',
    'heroicons:bolt',
    'heroicons:bookmark',
    'heroicons:chart-bar',
    'heroicons:clipboard-document-check',
    'heroicons:heart',
    'heroicons:fire',
    'heroicons:star',
    'heroicons:film',
    'heroicons:user-group',
    'heroicons:tag',
    'heroicons:folder',
    'heroicons:clock',
    'heroicons:squares-2x2',
    'heroicons:moon',
];
