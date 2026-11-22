// Helpers of the partial settings saves (PATCH /api/v1/settings): only the settings that changed
// are sent, so that a save does not overwrite a setting changed meanwhile in another tab or
// device. Pure functions, unit tested in web/tests/settingsPatch.test.ts.
import type { UserSettings } from '~/types/settings';

/** Fields of the settings owned by the server: never sent in a PATCH. */
export const READ_ONLY_SETTINGS = [
    'id',
    'user_id',
    'created_at',
    'updated_at',
    'max_items_per_page',
] as const;

type ReadOnlySetting = (typeof READ_ONLY_SETTINGS)[number];

/** Body of a PATCH: the changed settings. Nested objects are sent whole. */
export type SettingsPatch = Partial<Omit<UserSettings, ReadOnlySetting>>;

const readOnly = new Set<string>(READ_ONLY_SETTINGS);

/** Deep equality of JSON values (object key order does not matter, array order does). */
export function settingsValueEqual(a: unknown, b: unknown): boolean {
    if (a === b) return true;
    if (a === null || b === null || typeof a !== 'object' || typeof b !== 'object') return false;
    if (Array.isArray(a) !== Array.isArray(b)) return false;
    if (Array.isArray(a) && Array.isArray(b)) {
        return a.length === b.length && a.every((v, i) => settingsValueEqual(v, b[i]));
    }
    const ao = a as Record<string, unknown>;
    const bo = b as Record<string, unknown>;
    // Keys holding undefined are absent once serialized
    const keys = new Set([...Object.keys(ao), ...Object.keys(bo)]);
    for (const key of keys) {
        if (!settingsValueEqual(ao[key], bo[key])) return false;
    }
    return true;
}

function cloneJSON<T>(value: T): T {
    return JSON.parse(JSON.stringify(value)) as T;
}

/**
 * The settings of the draft that differ from the saved ones (top-level fields; a changed nested
 * object, like the card template or the homepage layout, is sent whole). Read-only and undefined
 * fields are left out. The values are copies, detached from the draft.
 */
export function diffSettings(saved: UserSettings, draft: UserSettings): SettingsPatch {
    const patch: Record<string, unknown> = {};
    const savedRecord = saved as unknown as Record<string, unknown>;
    for (const [key, value] of Object.entries(draft as unknown as Record<string, unknown>)) {
        if (readOnly.has(key) || value === undefined) continue;
        if (!settingsValueEqual(value, savedRecord[key])) patch[key] = cloneJSON(value);
    }
    return patch as SettingsPatch;
}

/** Whether a patch has nothing to send. */
export function isEmptySettingsPatch(patch: SettingsPatch): boolean {
    return Object.keys(patch).length === 0;
}

/** A new draft: a copy of the saved settings with the pending (unsaved) changes on top. */
export function rebaseDraft(saved: UserSettings, pending: SettingsPatch): UserSettings {
    const draft = cloneJSON(saved);
    Object.assign(draft, cloneJSON(pending));
    return draft;
}
