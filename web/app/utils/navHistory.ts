/**
 * Mirror of the tab's history entries (position -> URL), so that the watch page can find the
 * list the user came from, even after hopping through related scenes. Positions are the
 * `history.state.position` that vue-router stores in each entry.
 */
export type NavEntries = Record<number, string>;

export interface NavOrigin {
    position: number;
    fullPath: string;
}

const MAX_ENTRIES = 200;

/** Records the URL of a history entry (replace keeps the position, push takes the next one). */
export function recordNavEntry(
    entries: NavEntries,
    position: number,
    fullPath: string,
): NavEntries {
    // Entries far behind are no longer useful (and keep the stored map small)
    const next: NavEntries = Object.fromEntries(
        Object.entries(entries).filter(([pos]) => Number(pos) > position - MAX_ENTRIES),
    );
    next[position] = fullPath;
    return next;
}

/** Path part of a full path (without query and hash). */
export function pathOf(fullPath: string): string {
    return fullPath.split(/[?#]/)[0] || '/';
}

/** Whether a page is a step of the scene viewing (skipped when looking for the origin). */
export function isTransientPath(path: string): boolean {
    return path.startsWith('/watch/') || path === '/login' || path.startsWith('/share/');
}

/**
 * Closest earlier history entry that is not a scene page: the list the user came from.
 * Null when the scene was opened directly (new tab, shared link).
 */
export function findNavOrigin(entries: NavEntries, currentPosition: number): NavOrigin | null {
    for (let pos = currentPosition - 1; pos >= 0; pos--) {
        const fullPath = entries[pos];
        if (fullPath === undefined) return null;
        if (!isTransientPath(pathOf(fullPath))) return { position: pos, fullPath };
    }
    return null;
}

/** Short label of a page, for "back to" buttons. */
export function navOriginLabel(fullPath: string): string {
    const path = pathOf(fullPath);
    const labels: [RegExp, string][] = [
        [/^\/$/, 'Library'],
        [/^\/search/, 'Search'],
        [/^\/actors\/.+/, 'Actor'],
        [/^\/actors/, 'Actors'],
        [/^\/studios\/.+/, 'Studio'],
        [/^\/studios/, 'Studios'],
        [/^\/playlists\/.+/, 'Playlist'],
        [/^\/playlists/, 'Playlists'],
        [/^\/explorer/, 'Explorer'],
        [/^\/markers/, 'Markers'],
        [/^\/history/, 'History'],
        [/^\/duplicates/, 'Duplicates'],
        [/^\/triage/, 'Triage'],
        [/^\/settings/, 'Settings'],
    ];
    return labels.find(([re]) => re.test(path))?.[1] ?? 'Previous page';
}

/**
 * Whether the previous history entry is the given list page (any query), so that a "back to
 * the list" button can return to it through the history, with its state and scroll position.
 */
export function previousEntryIs(
    entries: NavEntries,
    currentPosition: number,
    path: string,
): boolean {
    const previous = entries[currentPosition - 1];
    return previous !== undefined && pathOf(previous) === path;
}
