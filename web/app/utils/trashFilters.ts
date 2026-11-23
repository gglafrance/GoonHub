// Trash page (Settings > Trash): filter state -> API query, bulk helpers and labels.
// Tests in web/tests/trashFilters.test.ts.
import type { TrashBulkResult, TrashSort, TrashSource } from '~/types/trash';

export interface TrashFilterState {
    /** Text on the title and the file path */
    q: string;
    /** '' any, 'none' no storage path, or a storage path id */
    storage: string;
    /** File size bounds in megabytes (0 = no bound) */
    minSizeMb: number;
    maxSizeMb: number;
    /** Duration bounds in minutes (0 = no bound) */
    minDurationMin: number;
    maxDurationMin: number;
    /** Trashed date bounds, YYYY-MM-DD, inclusive ('' = no bound) */
    from: string;
    to: string;
    /** '' or a resolution key of the search filters (4k, 1080p, ...) */
    resolution: string;
    codec: string;
    source: '' | TrashSource;
}

export const TRASH_BULK_LIMIT = 500;

export const TRASH_SORTS: { value: TrashSort; label: string; defaultOrder: 'asc' | 'desc' }[] = [
    { value: 'trashed_at', label: 'Trashed date', defaultOrder: 'desc' },
    { value: 'size', label: 'Size', defaultOrder: 'desc' },
    { value: 'duration', label: 'Duration', defaultOrder: 'desc' },
    { value: 'title', label: 'Title', defaultOrder: 'asc' },
    { value: 'purge_at', label: 'Purge date', defaultOrder: 'asc' },
];

export const TRASH_RESOLUTIONS = [
    { value: '4k', label: '4K' },
    { value: '1440p', label: '1440p' },
    { value: '1080p', label: '1080p' },
    { value: '720p', label: '720p' },
    { value: '480p', label: '480p' },
    { value: '360p', label: '360p' },
];

export function emptyTrashFilters(): TrashFilterState {
    return {
        q: '',
        storage: '',
        minSizeMb: 0,
        maxSizeMb: 0,
        minDurationMin: 0,
        maxDurationMin: 0,
        from: '',
        to: '',
        resolution: '',
        codec: '',
        source: '',
    };
}

const MB = 1024 * 1024;

/** Query params of the trash list / ids endpoints for a filter and a sort (no page). */
export function trashFilterParams(
    f: TrashFilterState,
    sort: TrashSort,
    order: 'asc' | 'desc',
): Record<string, string> {
    const params: Record<string, string> = { sort, order };
    const q = f.q.trim();
    if (q) params.q = q;
    if (f.storage) params.storage_path_id = f.storage;
    if (f.minSizeMb > 0) params.min_size = String(Math.round(f.minSizeMb * MB));
    if (f.maxSizeMb > 0) params.max_size = String(Math.round(f.maxSizeMb * MB));
    if (f.minDurationMin > 0) params.min_duration = String(Math.round(f.minDurationMin * 60));
    if (f.maxDurationMin > 0) params.max_duration = String(Math.round(f.maxDurationMin * 60));
    if (f.from) params.trashed_from = f.from;
    if (f.to) params.trashed_to = f.to;
    if (f.resolution) params.resolution = f.resolution;
    if (f.codec) params.codec = f.codec;
    if (f.source) params.source = f.source;
    return params;
}

/** Number of active filters, the text search excluded. */
export function activeTrashFilterCount(f: TrashFilterState): number {
    let n = 0;
    if (f.storage) n++;
    if (f.minSizeMb > 0 || f.maxSizeMb > 0) n++;
    if (f.minDurationMin > 0 || f.maxDurationMin > 0) n++;
    if (f.from || f.to) n++;
    if (f.resolution) n++;
    if (f.codec) n++;
    if (f.source) n++;
    return n;
}

/** Splits ids in chunks the bulk endpoints accept. */
export function chunkIds(ids: readonly number[], size = TRASH_BULK_LIMIT): number[][] {
    const chunks: number[][] = [];
    for (let i = 0; i < ids.length; i += size) chunks.push(ids.slice(i, i + size));
    return chunks;
}

export function mergeBulkResults(results: readonly TrashBulkResult[]): TrashBulkResult {
    return {
        succeeded: results.flatMap((r) => r.succeeded),
        failed: results.flatMap((r) => r.failed),
        size: results.reduce((sum, r) => sum + r.size, 0),
    };
}

export interface TrashSelectionSummary {
    count: number;
    /** Sum of the known sizes, in bytes */
    size: number;
    /** Selected scenes whose size is not known on the client */
    unknownSize: number;
    /** Selected scenes known to be on a read-only storage path (file kept) */
    keptOnDisk: number;
}

export function summarizeTrashSelection(
    ids: Iterable<number>,
    sizes: ReadonlyMap<number, number>,
    keptOnDisk: ReadonlySet<number>,
): TrashSelectionSummary {
    const summary: TrashSelectionSummary = { count: 0, size: 0, unknownSize: 0, keptOnDisk: 0 };
    for (const id of new Set(ids)) {
        summary.count++;
        const size = sizes.get(id);
        if (size === undefined) summary.unknownSize++;
        else summary.size += size;
        if (keptOnDisk.has(id)) summary.keptOnDisk++;
    }
    return summary;
}

export function describeTrashBulk(
    action: 'restored' | 'deleted',
    result: TrashBulkResult,
    formatSize: (bytes: number) => string,
): string {
    const n = result.succeeded.length;
    let text = `${n} video${n === 1 ? '' : 's'} ${action === 'restored' ? 'restored' : 'permanently deleted'}`;
    if (n > 0 && result.size > 0) text += ` (${formatSize(result.size)})`;
    if (result.failed.length) {
        const first = result.failed[0]!;
        text += `, ${result.failed.length} failed (#${first.scene_id}: ${first.error})`;
    }
    return text;
}

/** Time left before the purge: "Purged in 3 days", "Purged in 5 h", "Purge pending". */
export function purgeCountdown(expiresAt: string, now: number): { label: string; soon: boolean } {
    const ms = new Date(expiresAt).getTime() - now;
    if (!Number.isFinite(ms)) return { label: '', soon: false };
    if (ms <= 0) return { label: 'Purge pending', soon: true };
    const hours = ms / 3_600_000;
    if (hours < 24) return { label: `Purged in ${Math.max(1, Math.ceil(hours))} h`, soon: true };
    const days = Math.ceil(hours / 24);
    return { label: `Purged in ${days} day${days === 1 ? '' : 's'}`, soon: days <= 1 };
}
