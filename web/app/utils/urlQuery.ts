import type { LocationQuery, LocationQueryValue } from 'vue-router';

/**
 * Pure helpers mapping list state to and from the URL query, so that Back, a reload or a shared
 * link restore the same view. Values left out of the URL take the page defaults.
 */

type RawQueryValue = LocationQueryValue | LocationQueryValue[] | undefined;

/** Value of a query patch: a string sets the key, undefined/null/'' removes it. */
export type QueryPatch = Record<string, string | null | undefined>;

/** First string value of a query param ('' when absent). */
export function queryString(raw: RawQueryValue): string {
    const v = Array.isArray(raw) ? raw[0] : raw;
    return typeof v === 'string' ? v : '';
}

/** Positive integer query param, or the fallback. */
export function queryPositiveInt(raw: RawQueryValue, fallback: number): number {
    const n = Number(queryString(raw));
    return Number.isInteger(n) && n > 0 ? n : fallback;
}

/** Comma separated list query param. */
export function queryList(raw: RawQueryValue): string[] {
    const v = queryString(raw);
    return v ? v.split(',').filter(Boolean) : [];
}

/** Tri-state boolean query param: 'true' / 'false', null when absent or invalid. */
export function queryTriState(raw: RawQueryValue): boolean | null {
    const v = queryString(raw);
    if (v === 'true') return true;
    if (v === 'false') return false;
    return null;
}

/** Query param restricted to a set of values, or the fallback. */
export function queryEnum<T extends string>(
    raw: RawQueryValue,
    allowed: readonly T[],
    fallback: T,
): T {
    const v = queryString(raw);
    return (allowed as readonly string[]).includes(v) ? (v as T) : fallback;
}

/** Random sort seed from the URL (0 when absent or invalid). */
export function querySeed(raw: RawQueryValue): number {
    return queryPositiveInt(raw, 0);
}

/** New random sort seed (a positive integer that the backend accepts). */
export function newRandomSeed(): number {
    return Math.floor(Math.random() * 2_000_000_000) + 1;
}

/** Applies a patch on a query: string values set the key, empty values remove it. */
export function mergeQuery(base: LocationQuery, patch: QueryPatch): LocationQuery {
    const out: LocationQuery = Object.fromEntries(
        Object.entries(base).filter(([key]) => !(key in patch)),
    );
    for (const [key, value] of Object.entries(patch)) {
        if (value !== undefined && value !== null && value !== '') out[key] = value;
    }
    return out;
}

/** Whether two queries hold the same params (order independent). */
export function sameQuery(a: LocationQuery, b: LocationQuery): boolean {
    const norm = (q: LocationQuery) =>
        JSON.stringify(
            Object.keys(q)
                .filter((k) => q[k] !== undefined)
                .sort()
                .map((k) => [k, q[k]]),
        );
    return norm(a) === norm(b);
}

/** Query patch value of a setting: removed from the URL when it equals the default. */
export function unlessDefault<T>(
    value: T,
    fallback: T,
    serialize: (v: T) => string = (v) => String(v),
): string | undefined {
    return value === fallback ? undefined : serialize(value);
}
