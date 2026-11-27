import { describe, expect, test } from 'bun:test';
import type { UserSettings } from '../app/types/settings';
import {
    diffSettings,
    isEmptySettingsPatch,
    rebaseDraft,
    settingsValueEqual,
} from '../app/utils/settingsPatch';

const saved = (): UserSettings =>
    ({
        id: 3,
        user_id: 1,
        autoplay: false,
        default_volume: 80,
        videos_per_page: 20,
        default_sort_order: 'created_at_desc',
        grid_density: 'comfortable',
        infinite_scroll: false,
        show_scrubber: false,
        scrubber_mode: 'free',
        content_width: 'default',
        date_format: 'auto',
        sort_preferences: {
            actors: 'name_asc',
            studios: 'name_asc',
            markers: 'label_asc',
            actor_scenes: '',
            studio_scenes: '',
        },
        scene_list_view: { pages: { search: 'grid' }, columns: ['title', 'duration'] },
        scene_card_config: {
            badges: {
                top_left: { items: ['rating'], direction: 'vertical' },
                top_right: { items: [], direction: 'vertical' },
                bottom_left: { items: [], direction: 'vertical' },
                bottom_right: { items: ['duration'], direction: 'horizontal' },
            },
            content_rows: [{ type: 'split', left: 'file_size', right: 'added_at' }],
        },
        parsing_rules: { presets: [], activePresetId: null },
        max_items_per_page: 100,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-02T00:00:00Z',
    }) as unknown as UserSettings;

const clone = <T>(v: T): T => JSON.parse(JSON.stringify(v));

describe('settingsValueEqual', () => {
    test('compares scalars', () => {
        expect(settingsValueEqual(1, 1)).toBe(true);
        expect(settingsValueEqual(false, 0)).toBe(false);
        expect(settingsValueEqual('', null)).toBe(false);
        expect(settingsValueEqual(null, null)).toBe(true);
    });
    test('ignores the key order of objects', () => {
        expect(settingsValueEqual({ a: 1, b: { c: 2 } }, { b: { c: 2 }, a: 1 })).toBe(true);
    });
    test('keeps the order of arrays', () => {
        expect(settingsValueEqual(['a', 'b'], ['a', 'b'])).toBe(true);
        expect(settingsValueEqual(['a', 'b'], ['b', 'a'])).toBe(false);
        expect(settingsValueEqual(['a'], ['a', 'b'])).toBe(false);
        expect(settingsValueEqual([], {})).toBe(false);
    });
    test('treats undefined keys as absent', () => {
        expect(settingsValueEqual({ a: 1, b: undefined }, { a: 1 })).toBe(true);
    });
});

describe('diffSettings', () => {
    test('is empty for an untouched draft', () => {
        const patch = diffSettings(saved(), clone(saved()));
        expect(patch).toEqual({});
        expect(isEmptySettingsPatch(patch)).toBe(true);
    });

    test('sends only the changed scalar settings', () => {
        const draft = clone(saved());
        draft.show_scrubber = true;
        draft.default_volume = 0;
        draft.grid_density = 'large';
        draft.scrubber_mode = 'centered';
        draft.content_width = 'wide';
        expect(diffSettings(saved(), draft)).toEqual({
            show_scrubber: true,
            default_volume: 0,
            grid_density: 'large',
            scrubber_mode: 'centered',
            content_width: 'wide',
        });
    });

    test('sends a changed nested object whole', () => {
        const draft = clone(saved());
        draft.sort_preferences.actors = 'name_desc';
        expect(diffSettings(saved(), draft)).toEqual({
            sort_preferences: { ...saved().sort_preferences, actors: 'name_desc' },
        });
    });

    test('detects deep changes in arrays of objects', () => {
        const draft = clone(saved());
        draft.scene_card_config.content_rows[0]!.right = 'views';
        const patch = diffSettings(saved(), draft);
        expect(Object.keys(patch)).toEqual(['scene_card_config']);
        expect(patch.scene_card_config?.badges.top_left.items).toEqual(['rating']);
    });

    test('ignores a nested object rebuilt with another key order', () => {
        const draft = clone(saved());
        draft.scene_list_view = { columns: ['title', 'duration'], pages: { search: 'grid' } };
        expect(diffSettings(saved(), draft)).toEqual({});
    });

    test('never sends the read-only fields', () => {
        const draft = clone(saved());
        draft.id = 9;
        draft.user_id = 2;
        draft.max_items_per_page = 500;
        draft.updated_at = 'later';
        draft.created_at = 'earlier';
        draft.autoplay = true;
        expect(diffSettings(saved(), draft)).toEqual({ autoplay: true });
    });

    test('leaves out undefined values (they cannot be sent)', () => {
        const draft = clone(saved()) as unknown as Record<string, unknown>;
        draft.date_format = undefined;
        expect(diffSettings(saved(), draft as unknown as UserSettings)).toEqual({});
    });

    test('sends a setting missing from the saved settings', () => {
        const base = saved() as unknown as Record<string, unknown>;
        delete base.show_scrubber;
        const draft = clone(saved());
        expect(diffSettings(base as unknown as UserSettings, draft)).toEqual({
            show_scrubber: false,
        });
    });

    test('returns copies detached from the draft', () => {
        const draft = clone(saved());
        draft.sort_preferences.markers = 'recent';
        const patch = diffSettings(saved(), draft);
        draft.sort_preferences.markers = 'oldest';
        expect(patch.sort_preferences?.markers).toBe('recent');
    });
});

describe('rebaseDraft', () => {
    test('keeps the pending changes on top of the new saved settings', () => {
        // Saved meanwhile elsewhere: infinite scroll; pending in the draft: autoplay
        const fresh = { ...saved(), infinite_scroll: true };
        const draft = rebaseDraft(fresh, { autoplay: true });
        expect(draft.infinite_scroll).toBe(true);
        expect(draft.autoplay).toBe(true);
        expect(diffSettings(fresh, draft)).toEqual({ autoplay: true });
    });

    test('is a copy of the saved settings without pending changes', () => {
        const fresh = saved();
        const draft = rebaseDraft(fresh, {});
        expect(draft).toEqual(fresh);
        draft.sort_preferences.actors = 'name_desc';
        expect(fresh.sort_preferences.actors).toBe('name_asc');
    });
});
