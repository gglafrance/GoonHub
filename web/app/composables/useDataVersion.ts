// Bumped after every request that changes library data (see plugins/data-version.client.ts)
const dataVersion = ref(0);

/**
 * Version of the library data. List data kept in memory for Back (search results, homepage,
 * explorer folder...) records the version it was loaded at and is not shown again once it
 * changed, so that an edit made on a scene page never shows stale cards.
 */
export function useDataVersion() {
    return {
        version: readonly(dataVersion),
        bump: () => {
            dataVersion.value++;
        },
    };
}

/** Current data version (non reactive read, for stores). */
export function currentDataVersion(): number {
    return dataVersion.value;
}
