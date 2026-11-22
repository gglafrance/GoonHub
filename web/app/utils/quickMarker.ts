// Pure helpers of the quick marker (M on the watch page: label, then sub-labels; Shift+M: end
// of the marker). They take plain values so that they can be unit tested without a DOM
// (web/tests/quickMarker.test.ts).
import { normalizeForMatch } from './fuzzy';

/** Suggestions shown under the label and sub-label inputs. */
export const QUICK_MARKER_SUGGESTION_LIMIT = 8;
/** Longest marker label (same limit as the Markers tab input). */
export const QUICK_MARKER_LABEL_MAX = 100;
/** Color of the tags created from the quick marker (same as the triage page). */
export const QUICK_MARKER_NEW_TAG_COLOR = '#6B7280';
/** sessionStorage key of the last label used, offered as the default of the next marker. */
export const QUICK_MARKER_LAST_LABEL_KEY = 'goonhub:quick-marker:last-label';

export interface QuickMarkerLabelOption {
    label: string;
    count: number;
}

export interface QuickMarkerTagOption {
    id: number;
    name: string;
    color?: string;
    scene_count?: number;
}

/** A sub-label of the marker being created: an existing tag, or a new one (id null). */
export interface QuickMarkerChip {
    id: number | null;
    name: string;
    color?: string;
}

/** Row of the autocomplete list under the quick marker inputs. */
export interface QuickMarkerSuggestionRow {
    key: string;
    text: string;
    hint?: string;
    color?: string;
    // "Create tag" row
    create?: boolean;
}

/**
 * How well `query` matches `text` (lower is better): 0 exact, 1 prefix, 2 word prefix,
 * 3 substring, -1 no match. Case and accents are ignored; an empty query matches everything (3).
 */
export function quickMarkerMatchRank(query: string, text: string): number {
    const q = normalizeForMatch(query);
    const t = normalizeForMatch(text);
    if (!q) return 3;
    if (!t) return -1;
    if (t === q) return 0;
    if (t.startsWith(q)) return 1;
    const idx = t.indexOf(q);
    if (idx < 0) return -1;
    return /[\s\-_/.,:]/.test(t.charAt(idx - 1)) ? 2 : 3;
}

const rankBy = <T>(
    items: T[],
    query: string,
    text: (item: T) => string,
    count: (item: T) => number,
    limit: number,
): T[] =>
    items
        .map((item) => ({ item, rank: quickMarkerMatchRank(query, text(item)) }))
        .filter((r) => r.rank >= 0 && text(r.item).trim() !== '')
        .sort(
            (a, b) =>
                a.rank - b.rank ||
                count(b.item) - count(a.item) ||
                text(a.item).localeCompare(text(b.item)),
        )
        .slice(0, limit)
        .map((r) => r.item);

/** Label suggestions for the typed text: best match first, then most used. */
export function rankQuickMarkerLabels(
    labels: QuickMarkerLabelOption[],
    query: string,
    limit = QUICK_MARKER_SUGGESTION_LIMIT,
): QuickMarkerLabelOption[] {
    return rankBy(
        labels,
        query,
        (l) => l.label,
        (l) => l.count,
        limit,
    );
}

/** Tag suggestions for the typed text, without the tags already chosen. */
export function rankQuickMarkerTags(
    tags: QuickMarkerTagOption[],
    query: string,
    chips: QuickMarkerChip[],
    limit = QUICK_MARKER_SUGGESTION_LIMIT,
): QuickMarkerTagOption[] {
    const available = tags.filter((t) => !chipsInclude(chips, t));
    return rankBy(
        available,
        query,
        (t) => t.name,
        (t) => t.scene_count ?? 0,
        limit,
    );
}

const sameName = (a: string, b: string) => a.trim().toLowerCase() === b.trim().toLowerCase();

/** The existing tag named exactly `name` (case-insensitive), if any. */
export function findQuickMarkerTag(
    tags: QuickMarkerTagOption[],
    name: string,
): QuickMarkerTagOption | undefined {
    if (!name.trim()) return undefined;
    return tags.find((t) => sameName(t.name, name));
}

/** Whether a chip for this tag (same id, or same name) is already chosen. */
export function chipsInclude(
    chips: QuickMarkerChip[],
    tag: { id?: number | null; name: string },
): boolean {
    return chips.some((c) => (tag.id != null && c.id === tag.id) || sameName(c.name, tag.name));
}

/** Adds a chip, ignoring empty names and duplicates. Returns a new array. */
export function addQuickMarkerChip(
    chips: QuickMarkerChip[],
    chip: QuickMarkerChip,
): QuickMarkerChip[] {
    const name = chip.name.trim();
    if (!name || chipsInclude(chips, { id: chip.id, name })) return chips;
    return [...chips, { ...chip, name }];
}

/** Removes the last chip (Backspace on an empty input). Returns a new array. */
export function removeLastQuickMarkerChip(chips: QuickMarkerChip[]): QuickMarkerChip[] {
    return chips.length ? chips.slice(0, -1) : chips;
}

/**
 * Chip for the typed sub-label: the existing tag of that name, else a new tag when the user
 * may create tags, else null.
 */
export function chipForTypedTag(
    tags: QuickMarkerTagOption[],
    typed: string,
    canCreate: boolean,
): QuickMarkerChip | null {
    const name = typed.trim();
    if (!name) return null;
    const existing = findQuickMarkerTag(tags, name);
    if (existing) return { id: existing.id, name: existing.name, color: existing.color };
    return canCreate ? { id: null, name } : null;
}

/** Whether the "create tag" row is offered for the typed text. */
export function canOfferNewTag(
    tags: QuickMarkerTagOption[],
    chips: QuickMarkerChip[],
    typed: string,
    canCreate: boolean,
): boolean {
    const name = typed.trim();
    if (!canCreate || !name) return false;
    return !findQuickMarkerTag(tags, name) && !chipsInclude(chips, { name });
}

/** Label to save: the typed one, else the last label used, else null (nothing to save). */
export function resolveQuickMarkerLabel(typed: string, lastLabel?: string | null): string | null {
    const label = typed.trim() || lastLabel?.trim() || '';
    return label ? label.slice(0, QUICK_MARKER_LABEL_MAX) : null;
}

/** Marker timestamp (whole seconds, as the API stores them) of a player time. */
export function quickMarkerTimestamp(time: number | null | undefined): number {
    return typeof time === 'number' && Number.isFinite(time) && time > 0 ? Math.floor(time) : 0;
}

/** Player time as shown on the markers: "mm:ss", or "h:mm:ss" past an hour. */
export function formatQuickMarkerTime(seconds: number): string {
    const total = quickMarkerTimestamp(seconds);
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = total % 60;
    const pad = (n: number) => n.toString().padStart(2, '0');
    return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

/** Confirmation shown once the marker is saved. */
export function quickMarkerToastMessage(seconds: number, label: string): string {
    return `Marker added at ${formatQuickMarkerTime(seconds)} - ${label}`;
}

/**
 * Moves a highlighted suggestion with the arrows, wrapping around. With `allowNone`, -1 (no
 * highlight, the typed text is used) is part of the cycle.
 */
export function stepQuickMarkerHighlight(
    index: number,
    count: number,
    delta: 1 | -1,
    allowNone = false,
): number {
    if (count <= 0) return -1;
    const min = allowNone ? -1 : 0;
    const size = count - min;
    return ((((index - min + delta) % size) + size) % size) + min;
}

interface KeyLike {
    key: string;
    ctrlKey: boolean;
    metaKey: boolean;
    altKey: boolean;
    shiftKey: boolean;
    repeat?: boolean;
    defaultPrevented?: boolean;
    target: unknown;
}

const isTypingTarget = (target: unknown): boolean => {
    if (!target || typeof target !== 'object') return false;
    const el = target as { tagName?: string; isContentEditable?: boolean };
    const tag = (el.tagName ?? '').toUpperCase();
    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || !!el.isContentEditable;
};

/** Whether a keydown opens the quick marker: M alone, outside of a text field. */
export function isQuickMarkerKey(e: KeyLike): boolean {
    if (e.key.toLowerCase() !== 'm') return false;
    if (e.ctrlKey || e.metaKey || e.altKey || e.shiftKey || e.repeat || e.defaultPrevented) {
        return false;
    }
    return !isTypingTarget(e.target);
}

/** Whether a keydown sets the end of a marker: Shift+M, outside of a text field. */
export function isQuickMarkerEndKey(e: KeyLike): boolean {
    if (e.key.toLowerCase() !== 'm' || !e.shiftKey) return false;
    if (e.ctrlKey || e.metaKey || e.altKey || e.repeat || e.defaultPrevented) return false;
    return !isTypingTarget(e.target);
}

/** What the quick marker end (Shift+M) needs of a marker. */
export interface QuickMarkerEndCandidate {
    id: number;
    timestamp: number;
    end_timestamp?: number | null;
    label: string;
}

/**
 * Marker whose end Shift+M sets: the last one made with the quick marker during this visit of
 * the scene (while it still exists), else the one with the latest start before `end` (the
 * newest one when several start at the same time). Null when there is none.
 */
export function pickQuickMarkerEndTarget<T extends QuickMarkerEndCandidate>(
    markers: T[],
    end: number,
    lastQuickMarkerId?: number | null,
): T | null {
    if (lastQuickMarkerId != null) {
        const last = markers.find((m) => m.id === lastQuickMarkerId);
        if (last) return last;
    }
    let best: T | null = null;
    for (const m of markers) {
        if (m.timestamp >= end) continue;
        if (
            !best ||
            m.timestamp > best.timestamp ||
            (m.timestamp === best.timestamp && m.id > best.id)
        ) {
            best = m;
        }
    }
    return best;
}

/**
 * Marker end (whole seconds) of a player time, kept within the scene duration when it is
 * known (the API refuses an end past it).
 */
export function quickMarkerEndTimestamp(
    time: number | null | undefined,
    duration?: number | null,
): number {
    const end = quickMarkerTimestamp(time);
    return duration != null && duration > 0 ? Math.min(end, Math.floor(duration)) : end;
}

/** Why the end can't be set (null when it can): it must come after the marker's start. */
export function validateQuickMarkerEnd(
    marker: QuickMarkerEndCandidate | null,
    end: number,
): string | null {
    if (!marker) return 'No marker before the current time';
    if (end <= marker.timestamp) {
        return `Seek past the marker start (${formatQuickMarkerTime(marker.timestamp)}) to set its end`;
    }
    return null;
}

/** Confirmation shown once the end is set, e.g. "End set: 12:34 - 13:10 Kiss". */
export function quickMarkerEndToastMessage(start: number, end: number, label: string): string {
    const range = `${formatQuickMarkerTime(start)} - ${formatQuickMarkerTime(end)}`;
    return label.trim() ? `End set: ${range} ${label.trim()}` : `End set: ${range}`;
}
