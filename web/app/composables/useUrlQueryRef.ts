/**
 * A ref kept in sync with one URL query param, so that the state survives Back, a reload and a
 * shared link. The param is left out of the URL while the value equals the fallback.
 *
 * @param key - query param name
 * @param fallback - value when the param is absent or invalid
 * @param parse - raw param -> value (undefined when invalid)
 * @param serialize - value -> raw param ('' removes the param)
 */
export function useUrlQueryRef<T>(
    key: string,
    fallback: T,
    parse: (raw: string) => T | undefined,
    serialize: (value: T) => string = (v) => String(v),
): Ref<T> {
    const route = useRoute();
    const patch = useUrlQueryPatch();

    const read = (): T => {
        const raw = queryString(route.query[key]);
        if (!raw) return fallback;
        const value = parse(raw);
        return value === undefined ? fallback : value;
    };

    const state = ref(read()) as Ref<T>;
    const fallbackRaw = serialize(fallback);

    // State -> URL (serialized, so that lists are compared by value)
    watch(
        () => serialize(state.value),
        (raw) => patch({ [key]: raw === fallbackRaw ? undefined : raw }),
    );

    // URL -> state (browser back/forward on the same page, links to the same page)
    watch(
        () => route.query[key],
        () => {
            const value = read();
            if (serialize(value) !== serialize(state.value)) state.value = value;
        },
    );

    return state;
}

/** String query param ('' = absent). */
export function useUrlString(key: string, fallback = ''): Ref<string> {
    return useUrlQueryRef(key, fallback, (raw) => raw);
}

/** Comma separated list query param. */
export function useUrlList(key: string, fallback: string[] = []): Ref<string[]> {
    return useUrlQueryRef(
        key,
        fallback,
        (raw) => raw.split(',').filter(Boolean),
        (v) => v.join(','),
    );
}

/**
 * Seed of a random sort, kept in the URL (`seed`) while the random sort is active, so that Back
 * or a reload bring the same order back. A new seed is drawn when the random sort gets selected
 * without one; reshuffle() draws another.
 */
export function useUrlSeed(sort: Ref<string>, randomValue = 'random') {
    const route = useRoute();
    const patch = useUrlQueryPatch();

    const isRandom = () => sort.value === randomValue;
    const urlSeed = querySeed(route.query.seed);
    const seed = ref(urlSeed || (isRandom() ? newRandomSeed() : 0));

    const sync = () => patch({ seed: isRandom() && seed.value ? String(seed.value) : undefined });

    watch(sort, () => {
        if (isRandom() && !seed.value) seed.value = newRandomSeed();
        else if (!isRandom()) seed.value = 0;
    });
    watch([sort, seed], sync);
    // A random sort reached without a seed (old link, default sort): pin it in the URL now
    if (isRandom() && !urlSeed) sync();

    watch(
        () => route.query.seed,
        (raw) => {
            const value = querySeed(raw);
            if (value && value !== seed.value) seed.value = value;
        },
    );

    const reshuffle = () => {
        seed.value = newRandomSeed();
    };

    return { seed, reshuffle };
}
