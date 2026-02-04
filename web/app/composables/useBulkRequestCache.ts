/**
 * Simple Map-based cache for bulk operations.
 * Lives only during the bulk operation session to prevent redundant API calls.
 */
export interface BulkRequestCache {
    get<T>(key: string): T | undefined;
    set<T>(key: string, value: T): void;
    has(key: string): boolean;
    delete(key: string): void;
    clear(): void;
}

export function useBulkRequestCache(): BulkRequestCache {
    const cache = new Map<string, unknown>();

    return {
        get: <T>(key: string) => cache.get(key) as T | undefined,
        set: <T>(key: string, value: T) => {
            cache.set(key, value);
        },
        has: (key: string) => cache.has(key),
        delete: (key: string) => cache.delete(key),
        clear: () => cache.clear(),
    };
}
