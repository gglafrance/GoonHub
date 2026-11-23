// Copies recorded by the library scan, triaged on the Duplicates page > Copies: query string
// of the filters, size inputs, selection totals and bulk result messages.
// Tests in web/tests/duplicateCopies.test.ts.

import type {
    SceneCopyBulkResult,
    SceneCopyFilters,
    SceneCopyItem,
    SceneCopySort,
} from '~/types/duplicates';

const MB = 1024 * 1024;

export const COPY_SORTS: { value: SceneCopySort; label: string }[] = [
    { value: 'newest', label: 'Newest' },
    { value: 'size', label: 'Size' },
    { value: 'copies', label: 'Copies' },
];

export function defaultCopyFilters(): SceneCopyFilters {
    return { storagePathId: null, search: '', minSizeMB: null, maxSizeMB: null, sort: 'newest' };
}

// parseSizeMB reads a size input in MB: a positive number, or null when empty or invalid.
export function parseSizeMB(raw: string | number | null | undefined): number | null {
    if (raw === null || raw === undefined) return null;
    const text = String(raw).trim().replace(',', '.');
    if (text === '') return null;
    const n = Number(text);
    return Number.isFinite(n) && n > 0 ? n : null;
}

// buildCopyQuery returns the query of GET /admin/duplicates/copies (and /ids without page).
export function buildCopyQuery(
    filters: SceneCopyFilters,
    page?: number,
    limit?: number,
): URLSearchParams {
    const params = new URLSearchParams();
    if (page !== undefined) params.set('page', String(page));
    if (limit !== undefined) params.set('limit', String(limit));
    if (filters.storagePathId) params.set('storage_path_id', String(filters.storagePathId));
    const search = filters.search.trim();
    if (search) params.set('q', search);
    if (filters.minSizeMB) params.set('min_size', String(Math.floor(filters.minSizeMB * MB)));
    if (filters.maxSizeMB) params.set('max_size', String(Math.ceil(filters.maxSizeMB * MB)));
    if (filters.sort !== 'newest') params.set('sort_by', filters.sort);
    return params;
}

// sizeRangeError tells why a size range is invalid, or null.
export function sizeRangeError(filters: SceneCopyFilters): string | null {
    if (filters.minSizeMB && filters.maxSizeMB && filters.minSizeMB > filters.maxSizeMB) {
        return 'The minimum size is larger than the maximum size';
    }
    return null;
}

// selectionSummary counts the selected copies and sums the sizes of those loaded on the
// client; copies selected but not loaded (select all across pages) are counted as unknown.
export function selectionSummary(
    selected: ReadonlySet<number>,
    known: ReadonlyMap<number, Pick<SceneCopyItem, 'size'>>,
): { count: number; size: number; unknown: number } {
    let size = 0;
    let unknown = 0;
    for (const id of selected) {
        const c = known.get(id);
        if (c) size += c.size;
        else unknown++;
    }
    return { count: selected.size, size, unknown };
}

// describeCopyBulkResult is the message shown after a bulk action.
export function describeCopyBulkResult(
    action: 'forget' | 'delete',
    result: SceneCopyBulkResult,
    formatSize: (bytes: number) => string,
): string {
    const done = result.done.length;
    let msg =
        action === 'forget'
            ? `${done} cop${done === 1 ? 'y' : 'ies'} forgotten (files kept on disk)`
            : `${done} file(s) deleted, ${formatSize(result.freed_bytes)} freed`;
    if (result.failed.length) {
        const first = result.failed[0]?.error ?? '';
        msg += `; ${result.failed.length} refused or failed`;
        if (first) msg += ` (e.g. ${first})`;
    }
    return msg;
}

// fileName returns the last element of a path.
export function fileName(path: string): string {
    const parts = path.split(/[\\/]/).filter(Boolean);
    return parts[parts.length - 1] ?? path;
}
