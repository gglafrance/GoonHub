// Page loading plan of a paginated scene list, with or without infinite scroll.
//
// With infinite scroll the URL `page` is the last loaded page: the next page is appended, and
// any other page (first load, reload, back navigation) rebuilds the list from the pages before
// it, up to INFINITE_REBUILD_PAGES pages. Earlier pages load on demand (prepend). The context
// (filters, sort, folder...) of the loaded pages must match for a page to be appended.

export const INFINITE_REBUILD_PAGES = 10;

export interface LoadedPages {
    // First and last page in the list (0: nothing loaded)
    first: number;
    last: number;
    context?: string;
}

export interface PageLoadPlan {
    pages: number[];
    mode: 'replace' | 'append' | 'prepend';
    context?: string;
}

export function planPageLoad(
    loaded: LoadedPages,
    page: number,
    infinite: boolean,
    context = '',
): PageLoadPlan {
    if (!infinite) return { pages: [page], mode: 'replace', context };
    if (loaded.last > 0 && page === loaded.last + 1 && (loaded.context ?? '') === context) {
        return { pages: [page], mode: 'append', context };
    }
    const from = Math.max(1, page - INFINITE_REBUILD_PAGES + 1);
    const pages: number[] = [];
    for (let p = from; p <= page; p++) pages.push(p);
    return { pages, mode: 'replace', context };
}

export function planPreviousPage(loaded: LoadedPages): PageLoadPlan | null {
    if (loaded.first <= 1) return null;
    return { pages: [loaded.first - 1], mode: 'prepend', context: loaded.context };
}

export function commitPageLoad(loaded: LoadedPages, plan: PageLoadPlan): LoadedPages {
    const first = plan.pages[0] ?? 1;
    const last = plan.pages[plan.pages.length - 1] ?? first;
    const context = plan.context;
    if (plan.mode === 'append') return { first: loaded.first || first, last, context };
    if (plan.mode === 'prepend') return { first, last: loaded.last || last, context };
    return { first, last, context };
}

// Merges loaded items into the list (no duplicate ids if scenes moved between two loads)
export function mergePageItems<T extends { id: number }>(
    current: T[],
    items: T[],
    mode: PageLoadPlan['mode'],
): T[] {
    if (mode === 'replace') return items;
    const seen = new Set(current.map((i) => i.id));
    const fresh = items.filter((i) => !seen.has(i.id));
    return mode === 'append' ? [...current, ...fresh] : [...fresh, ...current];
}
