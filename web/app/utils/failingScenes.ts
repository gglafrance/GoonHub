// Logic of the Failing view (Settings > Jobs > Failing): query string, phases, error preview,
// selection and bulk request body.

import type { FailingAction, FailingBulkRequest } from '~/types/jobs';

export const FAILING_PHASES = [
    'metadata',
    'thumbnail',
    'sprites',
    'animated_thumbnails',
    'fingerprint',
] as const;

const PHASE_LABELS: Record<string, string> = {
    metadata: 'Metadata',
    thumbnail: 'Thumbnail',
    sprites: 'Sprites',
    animated_thumbnails: 'Animated thumbnails',
    fingerprint: 'Fingerprint',
};

export function failingPhaseLabel(phase: string): string {
    return PHASE_LABELS[phase] ?? phase;
}

export interface FailingQuery {
    phase?: string;
    q?: string;
    page?: number;
    limit?: number;
}

// Query string of GET /admin/jobs/failing (empty filters left out)
export function failingQueryString(query: FailingQuery): string {
    const params = new URLSearchParams();
    if (query.phase) params.set('phase', query.phase);
    const q = query.q?.trim();
    if (q) params.set('q', q);
    params.set('page', String(Math.max(1, Math.floor(query.page ?? 1))));
    params.set('limit', String(Math.max(1, Math.floor(query.limit ?? 25))));
    return params.toString();
}

// First line of an error, cut to max characters (the full text is shown on demand)
export function errorPreview(error: string, max = 90): string {
    const firstLine = (error ?? '').trim().split('\n')[0] ?? '';
    if (firstLine.length <= max) return firstLine;
    return `${firstLine.slice(0, max - 1).trimEnd()}…`;
}

export function isErrorTruncated(error: string, max = 90): boolean {
    return errorPreview(error, max) !== (error ?? '').trim();
}

// Sum of the per-phase totals
export function failingTotal(totals: Record<string, number>): number {
    return Object.values(totals).reduce((sum, n) => sum + n, 0);
}

// Phases to show as filters: the known ones that have failing scenes, then any other phase
export function failingPhasesWithTotals(totals: Record<string, number>): string[] {
    const known = FAILING_PHASES.filter((p) => (totals[p] ?? 0) > 0);
    const others = Object.keys(totals)
        .filter((p) => !(FAILING_PHASES as readonly string[]).includes(p) && totals[p]! > 0)
        .sort();
    return [...known, ...others];
}

export function toggleId(selected: number[], id: number): number[] {
    return selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id];
}

// Select every row of the page, or clear the page when it is all selected already
export function togglePage(selected: number[], pageIds: number[]): number[] {
    const allSelected = pageIds.length > 0 && pageIds.every((id) => selected.includes(id));
    if (allSelected) return selected.filter((id) => !pageIds.includes(id));
    return [...selected, ...pageIds.filter((id) => !selected.includes(id))];
}

// Body of a bulk action: the selected IDs, or every failing match of the filters when none is
// selected
export function failingBulkBody(
    action: FailingAction,
    selected: number[],
    filters: { phase?: string; q?: string },
): FailingBulkRequest {
    if (selected.length > 0) return { action, ids: [...selected] };
    const body: FailingBulkRequest = { action, all: true };
    if (filters.phase) body.phase = filters.phase;
    const q = filters.q?.trim();
    if (q) body.q = q;
    return body;
}

export function failingActionMessage(
    action: FailingAction,
    result: { retried: number; cleared: number; errors: number },
): string {
    if (action === 'clear') {
        return `Forgot the failures of ${result.cleared} scene${result.cleared === 1 ? '' : 's'}`;
    }
    const base = `Queued ${result.retried} job${result.retried === 1 ? '' : 's'}`;
    return result.errors > 0 ? `${base} (${result.errors} could not be queued)` : base;
}
