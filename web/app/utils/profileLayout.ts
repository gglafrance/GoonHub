// Tabs, sections and custom lists of the profile pages (docs/PROFILES.md). Pure helpers, tests
// in web/tests/profileLayout.test.ts.
import type {
    ProfileLayoutBlock,
    ProfileSectionConfig,
    ProfileTabConfig,
    ProfileTabInfo,
    ProfileTabKind,
} from '~/types/profile';
import type { SavedSearchFilters } from '~/types/saved_search';

/** Column count of each section template (the server's core.ProfileSectionTemplates). */
export const SECTION_TEMPLATE_WIDTHS: Record<string, number[]> = {
    '1': [12],
    '1-1': [6, 6],
    '1-1-1': [4, 4, 4],
    '2-1': [8, 4],
    '1-2': [4, 8],
};

// Literal classes so that Tailwind keeps them (columns stack below md)
const SPAN_CLASSES: Record<number, string> = {
    4: 'md:col-span-4',
    6: 'md:col-span-6',
    8: 'md:col-span-8',
    12: 'md:col-span-12',
};

/** Width classes of the columns of a template (12-column grid from md, stacked below). */
export function sectionColumnClasses(template: string): string[] {
    const widths = SECTION_TEMPLATE_WIDTHS[template] ?? [12];
    return widths.map((w) => SPAN_CLASSES[w] ?? 'md:col-span-12');
}

/** Number of columns of a template. */
export function templateColumnCount(template: string): number {
    return (SECTION_TEMPLATE_WIDTHS[template] ?? [12]).length;
}

/** A new id, unique among `taken` (lowercase letters and digits, the server's pattern). */
export function newLayoutId(prefix: string, taken: Iterable<string>, random = Math.random): string {
    const used = new Set(taken);
    for (;;) {
        const id = `${prefix}${Math.floor(random() * 36 ** 5)
            .toString(36)
            .padStart(5, '0')}`;
        if (!used.has(id)) return id;
    }
}

/** Every id used in tabs (tabs, sections and blocks), for newLayoutId. */
export function layoutIds(tabs: ProfileTabConfig[]): string[] {
    const out: string[] = [];
    for (const t of tabs) {
        out.push(t.id);
        for (const s of t.sections ?? []) {
            out.push(s.id);
            for (const col of s.columns) for (const b of col) if (b.id) out.push(b.id);
        }
    }
    return out;
}

/** Whether a tab kind is made of sections. */
export function tabHasSections(kind: ProfileTabKind): boolean {
    return kind === 'profile' || kind === 'custom';
}

/** Number of blocks of a tab. */
export function countTabBlocks(tab: ProfileTabConfig): number {
    return (tab.sections ?? []).reduce(
        (n, s) => n + s.columns.reduce((m, c) => m + c.length, 0),
        0,
    );
}

/** Path of a tab of a profile. */
export function tabPath(
    username: string,
    tab: Pick<ProfileTabInfo, 'kind' | 'slug' | 'id'>,
): string {
    const base = `/u/${encodeURIComponent(username)}`;
    if (tab.kind === 'profile') return base;
    if (tab.kind === 'custom') return `${base}/t/${encodeURIComponent(tab.slug || tab.id)}`;
    return `${base}/${tab.kind}`;
}

/**
 * The tab a profile route shows, as the API `?tab=` expects: '' (the profile tab), a list kind,
 * a custom tab slug, or '-' for the pages that need no tab content (a custom list).
 */
export function tabKeyOfPath(path: string): string {
    const parts = path.split('/').filter(Boolean).slice(2).map(decodeURIComponent);
    if (parts.length === 0) return '';
    if (parts[0] === 't') return parts[1] ?? '';
    if (parts[0] === 'likes' || parts[0] === 'ratings' || parts[0] === 'jizz') return parts[0];
    return '-';
}

/** The id of the tab a route shows among the tabs (undefined when none matches). */
export function activeTabId(path: string, tabs: ProfileTabInfo[]): string | undefined {
    const key = tabKeyOfPath(path);
    if (key === '') return tabs.find((t) => t.kind === 'profile')?.id;
    return tabs.find((t) => t.id === key || (t.kind === 'custom' && t.slug === key))?.id;
}

/** A position in a tab: section, column, index in the column. */
export interface BlockPosition {
    section: number;
    column: number;
    index: number;
}

const cloneSections = (sections: ProfileSectionConfig[]): ProfileSectionConfig[] =>
    sections.map((s) => ({ ...s, columns: s.columns.map((c) => c.slice()) }));

/**
 * Moves a block to another place (another column, another section, or the same column). `to`
 * is the index in the target column before the move; past the end appends.
 */
export function moveBlock(
    sections: ProfileSectionConfig[],
    from: BlockPosition,
    to: BlockPosition,
): ProfileSectionConfig[] {
    const out = cloneSections(sections);
    const source = out[from.section]?.columns[from.column];
    const target = out[to.section]?.columns[to.column];
    if (!source || !target || from.index < 0 || from.index >= source.length) return out;
    const [block] = source.splice(from.index, 1);
    let index = to.index;
    // Same column, moving down: the removal shifted the target
    if (source === target && from.index < index) index -= 1;
    index = Math.max(0, Math.min(index, target.length));
    target.splice(index, 0, block!);
    return out;
}

/** Where a block goes with the move buttons (up, down, left, right), or null at the edge. */
export function neighbourPosition(
    sections: ProfileSectionConfig[],
    at: BlockPosition,
    direction: 'up' | 'down' | 'left' | 'right',
): BlockPosition | null {
    const section = sections[at.section];
    if (!section) return null;
    const column = section.columns[at.column] ?? [];
    switch (direction) {
        case 'up':
            if (at.index > 0) return { ...at, index: at.index - 1 };
            if (at.section > 0) {
                const prev = sections[at.section - 1]!;
                const col = Math.min(at.column, prev.columns.length - 1);
                return { section: at.section - 1, column: col, index: prev.columns[col]!.length };
            }
            return null;
        case 'down':
            if (at.index < column.length - 1) return { ...at, index: at.index + 2 };
            if (at.section < sections.length - 1) {
                const next = sections[at.section + 1]!;
                return {
                    section: at.section + 1,
                    column: Math.min(at.column, next.columns.length - 1),
                    index: 0,
                };
            }
            return null;
        case 'left':
            if (at.column === 0) return null;
            return {
                section: at.section,
                column: at.column - 1,
                index: section.columns[at.column - 1]!.length,
            };
        case 'right':
            if (at.column >= section.columns.length - 1) return null;
            return {
                section: at.section,
                column: at.column + 1,
                index: section.columns[at.column + 1]!.length,
            };
    }
}

/** Changes the template of a section; the blocks of the columns that go join the last one. */
export function setSectionTemplate(
    section: ProfileSectionConfig,
    template: string,
): ProfileSectionConfig {
    const count = templateColumnCount(template);
    const columns: ProfileLayoutBlock[][] = Array.from({ length: count }, () => []);
    section.columns.forEach((col, i) => columns[Math.min(i, count - 1)]!.push(...col));
    return { ...section, template, columns };
}

/** A new empty section. */
export function newSection(id: string, template = '1'): ProfileSectionConfig {
    return {
        id,
        template,
        columns: Array.from({ length: templateColumnCount(template) }, () => []),
    };
}

/** A tab slug from its title (the server's core.ProfileSlug). */
export function profileSlug(title: string): string {
    let out = '';
    let dash = false;
    for (const ch of title.toLowerCase()) {
        if (/[a-z0-9]/.test(ch)) {
            out += ch;
            dash = false;
        } else if (out.length > 0 && !dash) {
            out += '-';
            dash = true;
        }
    }
    out = out.replace(/-+$/, '');
    return out.length > 40 ? out.slice(0, 40).replace(/-+$/, '') : out;
}

/** Sorts on the user's own interactions (besides the O sorts), as the search offers them. */
export const USER_SORT_OPTIONS = [
    { value: 'my_rating_desc', label: 'My highest rated' },
    { value: 'my_rating_asc', label: 'My lowest rated' },
    { value: 'liked_at_desc', label: 'Recently liked' },
    { value: 'my_views_desc', label: 'Most watched by me' },
    { value: 'last_watched_desc', label: 'Last watched' },
];

const SORT_ITEMS: Record<string, string> = {
    jizz_count_desc: 'jizz',
    jizz_count_asc: 'jizz',
    last_jizz_desc: 'jizz',
    last_jizz_asc: 'jizz',
    my_rating_desc: 'ratings',
    my_rating_asc: 'ratings',
    liked_at_desc: 'liked',
    my_views_desc: 'activity',
    last_watched_desc: 'activity',
};

/** Catalog order of the items a list can depend on (core.PrivacyCatalog). */
const DEPENDENCY_ORDER = ['activity', 'recent_markers', 'liked', 'ratings', 'jizz'];

/**
 * The privacy items custom list criteria depend on (the server's core.ProfileListDependencies)
 * and whether they use file sizes.
 */
export function criteriaDependencies(f: SavedSearchFilters): { items: string[]; size: boolean } {
    const set = new Set<string>();
    if (f.liked) set.add('liked');
    if ((f.min_rating ?? 0) > 0 || (f.max_rating ?? 0) > 0 || f.rated !== undefined)
        set.add('ratings');
    if ((f.min_jizz_count ?? 0) > 0 || (f.max_jizz_count ?? 0) > 0) set.add('jizz');
    if (f.watched !== undefined) set.add('activity');
    if (f.has_markers !== undefined || (f.selected_marker_labels?.length ?? 0) > 0)
        set.add('recent_markers');
    const sortItem = f.sort ? SORT_ITEMS[f.sort] : undefined;
    if (sortItem) set.add(sortItem);
    const size =
        (f.min_size ?? 0) > 0 ||
        (f.max_size ?? 0) > 0 ||
        f.sort === 'size_asc' ||
        f.sort === 'size_desc';
    return { items: DEPENDENCY_ORDER.filter((k) => set.has(k)), size };
}

/** Ready-made custom lists offered by the editor. */
export const CUSTOM_LIST_PRESETS: { title: string; filters: SavedSearchFilters }[] = [
    { title: 'Top O videos', filters: { min_jizz_count: 1, sort: 'jizz_count_desc' } },
    { title: 'Top rated', filters: { rated: true, sort: 'my_rating_desc' } },
    { title: 'Recently liked', filters: { liked: true, sort: 'liked_at_desc' } },
    { title: 'Most watched', filters: { watched: true, sort: 'my_views_desc' } },
    { title: 'Newest in the library', filters: { sort: 'created_at_desc' } },
];

/** A short human summary of criteria ("tags: a, b · sort: Most O"). */
export function criteriaSummary(f: SavedSearchFilters, sortLabels: Record<string, string>): string {
    const parts: string[] = [];
    if (f.query) parts.push(`"${f.query}"`);
    if (f.selected_tags?.length) parts.push(`tags: ${f.selected_tags.join(', ')}`);
    if (f.selected_actors?.length) parts.push(`actors: ${f.selected_actors.join(', ')}`);
    if (f.studio) parts.push(`studio: ${f.studio}`);
    if (f.liked) parts.push('liked');
    if (f.rated === true) parts.push('rated');
    if (f.rated === false) parts.push('unrated');
    if (f.min_rating) parts.push(`rating >= ${f.min_rating}`);
    if (f.min_jizz_count) parts.push(`O >= ${f.min_jizz_count}`);
    if (f.watched === true) parts.push('watched');
    if (f.watched === false) parts.push('unwatched');
    if (f.selected_marker_labels?.length)
        parts.push(`markers: ${f.selected_marker_labels.join(', ')}`);
    if (f.sort) parts.push(`sort: ${sortLabels[f.sort] ?? f.sort}`);
    return parts.join(' · ') || 'Every video';
}
