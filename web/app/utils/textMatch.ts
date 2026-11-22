// Ranking of names for a typed search (tag pickers): the names that start with the text come
// first ("Sho" -> "Shower"), then those where a word starts with it ("Hot Shower"), then those
// that only contain it ("Cumshot"). Within a rank the incoming order is kept (the picker's own
// sort: A-Z, most used...).

/** 0: starts with the text, 1: a word starts with it, 2: contains it, 3: no match. */
export type MatchRank = 0 | 1 | 2 | 3;

const WORD_BREAK = /[\s\-_/.,:;()[\]&+'"]+/;

export function matchRank(name: string, query: string): MatchRank {
    const q = query.trim().toLowerCase();
    if (!q) return 0;
    const n = name.toLowerCase();
    if (n.startsWith(q)) return 0;
    if (n.split(WORD_BREAK).some((w) => w.startsWith(q))) return 1;
    return n.includes(q) ? 2 : 3;
}

/** The items whose name matches the query, best matches first, keeping the given order inside a rank. */
export function rankByMatch<T>(
    items: readonly T[],
    query: string,
    nameOf: (item: T) => string,
): T[] {
    if (!query.trim()) return [...items];
    return items
        .map((item, index) => ({ item, index, rank: matchRank(nameOf(item), query) }))
        .filter((x) => x.rank !== 3)
        .sort((a, b) => a.rank - b.rank || a.index - b.index)
        .map((x) => x.item);
}
