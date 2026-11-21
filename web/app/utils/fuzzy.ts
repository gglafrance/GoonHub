/**
 * Small fuzzy matcher for the command palette's static entries (pages, settings tabs, actions).
 * Ranks, best first: exact match, prefix, word prefix, substring, then letters in order
 * (subsequence) with a penalty for gaps. Case and accents are ignored.
 */

export const normalizeForMatch = (text: string): string =>
    text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().trim();

/** Score of `query` against one text, 0 when it doesn't match (higher is better). */
export const fuzzyScore = (query: string, text: string): number => {
    const q = normalizeForMatch(query);
    const t = normalizeForMatch(text);
    if (!q) return 1;
    if (!t) return 0;
    if (t === q) return 1000;
    if (t.startsWith(q)) return 800 - Math.min(t.length - q.length, 100);

    const idx = t.indexOf(q);
    if (idx > 0) {
        const prev = t.charAt(idx - 1);
        const wordStart = prev === ' ' || prev === '-' || prev === '/' || prev === '_';
        return (wordStart ? 600 : 400) - Math.min(idx, 100);
    }

    // Subsequence: every query letter (spaces ignored) appears in order
    const letters = q.replace(/\s+/g, '');
    let ti = 0;
    let gaps = 0;
    let last = -1;
    for (const ch of letters) {
        const found = t.indexOf(ch, ti);
        if (found === -1) return 0;
        if (last !== -1 && found > last + 1) gaps += found - last - 1;
        last = found;
        ti = found + 1;
    }
    return Math.max(1, 200 - gaps * 5);
};

/** Best score of `query` over a label and its extra keywords. */
export const fuzzyScoreAll = (query: string, texts: string[]): number => {
    let best = 0;
    for (const [i, text] of texts.entries()) {
        // Keywords count a little less than the label itself
        const score = fuzzyScore(query, text) * (i === 0 ? 1 : 0.9);
        if (score > best) best = score;
    }
    return best;
};
