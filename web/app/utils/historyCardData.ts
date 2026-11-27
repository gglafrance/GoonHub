// The user's interactions with the scenes of the watch history, sent next to the entries when
// the history is requested with card_fields (like the scene lists): the card badges need them.
export interface HistoryCardData {
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizz_counts?: Record<string, number>;
}

// historyCardData keeps the card maps of a history response (missing maps stay undefined).
export function historyCardData(response: unknown): HistoryCardData {
    if (!response || typeof response !== 'object') return {};
    const r = response as Record<string, unknown>;
    const pick = <T>(v: unknown): Record<string, T> | undefined =>
        v && typeof v === 'object' ? (v as Record<string, T>) : undefined;
    return {
        ratings: pick<number>(r.ratings),
        likes: pick<boolean>(r.likes),
        jizz_counts: pick<number>(r.jizz_counts),
    };
}
