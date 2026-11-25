import type { PaletteGroup, PaletteItem } from '~/types/command-palette';

const PAGE_LIMIT = 8;
const ACTION_LIMIT = 6;
// Lowest score of a word-prefix match: static entries this good are listed before entities
const STRONG_MATCH = 500;

const rank = (q: string, items: PaletteItem[], limit: number) =>
    items
        .map((item) => ({ item, score: fuzzyScoreAll(q, [item.label, ...(item.keywords ?? [])]) }))
        .filter((m) => m.score > 0)
        .sort((a, b) => b.score - a.score)
        .slice(0, limit);

/**
 * Groups shown by the command palette. Empty input: recent entries, the current scene's
 * shortcuts, actions and the main pages. Otherwise: fuzzy-matched static entries and the
 * server results, static groups first when they match strongly (e.g. "act" for Actors).
 */
export function useCommandPaletteResults(query: Ref<string>) {
    const { recent } = useCommandPalette();
    const commands = useCommandPaletteCommands();
    const search = useCommandPaletteSearch(query);

    const recentItems = computed<PaletteItem[]>(() => {
        const runnable = new Map(
            [...commands.actions.value, ...commands.context.value].map((a) => [a.id, a]),
        );
        const items: PaletteItem[] = [];
        for (const entry of recent.value) {
            if (!commands.allowed(entry)) continue;
            if (entry.admin && !entry.permissions && !commands.isSuperuser.value) continue;
            if (entry.to) {
                items.push({ ...entry, icon: entry.icon || 'heroicons:clock' });
                continue;
            }
            // Actions are re-resolved so their label and state are current
            const action = runnable.get(entry.id);
            if (action) items.push(action);
        }
        return items;
    });

    const groups = computed<PaletteGroup[]>(() => {
        const q = query.value.trim();

        if (!q) {
            const initial: PaletteGroup[] = [
                { id: 'recent', label: 'Recent', items: recentItems.value },
                { id: 'context', label: 'Current scene', items: commands.context.value },
                { id: 'actions', label: 'Actions', items: commands.actions.value },
                { id: 'pages', label: 'Pages', items: commands.corePages.value },
            ];
            return initial.filter((g) => g.items.length > 0);
        }

        const context = rank(q, commands.context.value, ACTION_LIMIT);
        const actions = rank(q, commands.actions.value, ACTION_LIMIT);
        const pages = rank(q, commands.pages.value, PAGE_LIMIT);

        const staticGroups: (PaletteGroup & { best: number })[] = [
            {
                id: 'context',
                label: 'Current scene',
                items: context.map((m) => m.item),
                best: context[0]?.score ?? 0,
            },
            {
                id: 'pages',
                label: 'Pages',
                items: pages.map((m) => m.item),
                best: pages[0]?.score ?? 0,
            },
            {
                id: 'actions',
                label: 'Actions',
                items: actions.map((m) => m.item),
                best: actions[0]?.score ?? 0,
            },
        ];
        staticGroups.sort((a, b) => b.best - a.best);

        // Always offer the full search page for the typed text
        const searchAll: PaletteItem = {
            id: `search:${q.toLowerCase()}`,
            group: 'scenes',
            label: `Search scenes for "${q}"`,
            sublabel: 'Open the search page',
            icon: 'heroicons:magnifying-glass',
            to: `/search?q=${encodeURIComponent(q)}`,
        };

        // Under the scene results when there are some, else at the very end
        const hasScenes = search.scenes.value.length > 0;
        const entityGroups: PaletteGroup[] = [
            {
                id: 'scenes',
                label: 'Scenes',
                items: hasScenes ? [...search.scenes.value, searchAll] : [],
            },
            { id: 'actors', label: 'Actors', items: search.actors.value },
            { id: 'studios', label: 'Studios', items: search.studios.value },
            { id: 'tags', label: 'Tags', items: search.tags.value },
            { id: 'playlists', label: 'Playlists', items: search.playlists.value },
            { id: 'saved', label: 'Saved searches', items: search.savedSearches.value },
        ];

        const strong = staticGroups.filter((g) => g.best >= STRONG_MATCH);
        const weak = staticGroups.filter((g) => g.best < STRONG_MATCH);
        const trailing: PaletteGroup[] = hasScenes
            ? []
            : [{ id: 'scenes', label: 'Scenes', items: [searchAll] }];
        return [...strong, ...entityGroups, ...weak, ...trailing]
            .map(({ id, label, items }): PaletteGroup => ({ id, label, items }))
            .filter((g) => g.items.length > 0);
    });

    /** Refreshes what depends on the page or on server lists; called on each opening. */
    const prepare = () => {
        commands.loadContext();
        search.loadLists();
    };

    return { groups, loading: search.loading, prepare };
}
