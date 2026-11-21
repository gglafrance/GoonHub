// The user's interactions with listed scenes, as the scene list endpoints return them next to
// the scenes (only the maps asked for through card_fields): what the card badges show.

export interface CardSidecarResponse {
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizz_counts?: Record<string, number>;
    watched?: Record<string, boolean>;
}

export interface CardSidecarMaps {
    ratings: Record<string, number>;
    likes: Record<string, boolean>;
    jizzCounts: Record<string, number>;
    watched: Record<string, boolean>;
}

export const emptyCardSidecars = (): CardSidecarMaps => ({
    ratings: {},
    likes: {},
    jizzCounts: {},
    watched: {},
});

// Maps of the pages just loaded, added to the current ones when pages are appended or
// prepended (infinite scroll), or replacing them.
export function mergeCardSidecars(
    current: CardSidecarMaps,
    responses: CardSidecarResponse[],
    partial: boolean,
): CardSidecarMaps {
    const base = partial ? current : emptyCardSidecars();
    return {
        ratings: Object.assign({ ...base.ratings }, ...responses.map((r) => r.ratings ?? {})),
        likes: Object.assign({ ...base.likes }, ...responses.map((r) => r.likes ?? {})),
        jizzCounts: Object.assign(
            { ...base.jizzCounts },
            ...responses.map((r) => r.jizz_counts ?? {}),
        ),
        watched: Object.assign({ ...base.watched }, ...responses.map((r) => r.watched ?? {})),
    };
}
