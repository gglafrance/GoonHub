import { describe, expect, test } from 'bun:test';
import {
    QUICK_MARKER_LABEL_MAX,
    addQuickMarkerChip,
    canOfferNewTag,
    chipForTypedTag,
    chipsInclude,
    findQuickMarkerTag,
    formatQuickMarkerTime,
    isQuickMarkerKey,
    quickMarkerMatchRank,
    quickMarkerTimestamp,
    quickMarkerToastMessage,
    rankQuickMarkerLabels,
    rankQuickMarkerTags,
    removeLastQuickMarkerChip,
    resolveQuickMarkerLabel,
    stepQuickMarkerHighlight,
    type QuickMarkerChip,
} from '../app/utils/quickMarker';

const labels = [
    { label: 'Intro', count: 3 },
    { label: 'Blowjob', count: 12 },
    { label: 'Cowgirl', count: 7 },
    { label: 'Reverse cowgirl', count: 9 },
    { label: 'Outro', count: 1 },
];

const tags = [
    { id: 1, name: 'Blonde', color: '#ff0', scene_count: 40 },
    { id: 2, name: 'Close-up', color: '#f00', scene_count: 5 },
    { id: 3, name: 'POV', color: '#0f0', scene_count: 22 },
    { id: 4, name: 'Outdoor', color: '#00f', scene_count: 3 },
];

describe('quickMarkerMatchRank', () => {
    test('orders exact, prefix, word prefix, substring', () => {
        expect(quickMarkerMatchRank('cowgirl', 'Cowgirl')).toBe(0);
        expect(quickMarkerMatchRank('cow', 'Cowgirl')).toBe(1);
        expect(quickMarkerMatchRank('cow', 'Reverse cowgirl')).toBe(2);
        expect(quickMarkerMatchRank('girl', 'Cowgirl')).toBe(3);
        expect(quickMarkerMatchRank('xyz', 'Cowgirl')).toBe(-1);
    });
    test('ignores case and accents', () => {
        expect(quickMarkerMatchRank('ECOLE', 'école')).toBe(0);
    });
    test('an empty query matches everything', () => {
        expect(quickMarkerMatchRank('  ', 'Intro')).toBe(3);
    });
});

describe('rankQuickMarkerLabels', () => {
    test('most used first without a query', () => {
        expect(rankQuickMarkerLabels(labels, '').map((l) => l.label)).toEqual([
            'Blowjob',
            'Reverse cowgirl',
            'Cowgirl',
            'Intro',
            'Outro',
        ]);
    });
    test('filters as you type, best match before usage', () => {
        expect(rankQuickMarkerLabels(labels, 'cow').map((l) => l.label)).toEqual([
            'Cowgirl',
            'Reverse cowgirl',
        ]);
        expect(rankQuickMarkerLabels(labels, 'tro').map((l) => l.label)).toEqual([
            'Intro',
            'Outro',
        ]);
    });
    test('respects the limit and skips empty labels', () => {
        const list = [...labels, { label: '', count: 100 }];
        expect(rankQuickMarkerLabels(list, '', 2).map((l) => l.label)).toEqual([
            'Blowjob',
            'Reverse cowgirl',
        ]);
    });
});

describe('rankQuickMarkerTags', () => {
    test('excludes the chosen tags and ranks by usage', () => {
        const chips: QuickMarkerChip[] = [{ id: 1, name: 'Blonde' }];
        expect(rankQuickMarkerTags(tags, '', chips).map((t) => t.id)).toEqual([3, 2, 4]);
    });
    test('excludes a new chip with the same name', () => {
        const chips: QuickMarkerChip[] = [{ id: null, name: 'pov' }];
        expect(rankQuickMarkerTags(tags, 'p', chips).map((t) => t.id)).toEqual([2]);
    });
    test('filters by name', () => {
        expect(rankQuickMarkerTags(tags, 'out', []).map((t) => t.name)).toEqual(['Outdoor']);
    });
});

describe('chips', () => {
    test('adds trimmed chips and ignores duplicates and empty names', () => {
        let chips: QuickMarkerChip[] = [];
        chips = addQuickMarkerChip(chips, { id: 3, name: ' POV ' });
        chips = addQuickMarkerChip(chips, { id: 3, name: 'POV' });
        chips = addQuickMarkerChip(chips, { id: null, name: 'pov' });
        chips = addQuickMarkerChip(chips, { id: null, name: '   ' });
        chips = addQuickMarkerChip(chips, { id: null, name: 'Sunset' });
        expect(chips).toEqual([
            { id: 3, name: 'POV' },
            { id: null, name: 'Sunset' },
        ]);
    });
    test('does not mutate the input', () => {
        const chips: QuickMarkerChip[] = [{ id: 1, name: 'Blonde' }];
        const next = addQuickMarkerChip(chips, { id: 2, name: 'Close-up' });
        expect(chips).toHaveLength(1);
        expect(next).toHaveLength(2);
    });
    test('removes the last chip', () => {
        const chips: QuickMarkerChip[] = [
            { id: 1, name: 'Blonde' },
            { id: null, name: 'Sunset' },
        ];
        expect(removeLastQuickMarkerChip(chips)).toEqual([{ id: 1, name: 'Blonde' }]);
        expect(removeLastQuickMarkerChip([])).toEqual([]);
    });
    test('chipsInclude matches by id or name', () => {
        const chips: QuickMarkerChip[] = [{ id: 1, name: 'Blonde' }];
        expect(chipsInclude(chips, { id: 1, name: 'Other' })).toBe(true);
        expect(chipsInclude(chips, { name: 'blonde' })).toBe(true);
        expect(chipsInclude(chips, { id: 2, name: 'POV' })).toBe(false);
    });
});

describe('typed sub-labels', () => {
    test('findQuickMarkerTag is case-insensitive', () => {
        expect(findQuickMarkerTag(tags, ' pov ')?.id).toBe(3);
        expect(findQuickMarkerTag(tags, '')).toBeUndefined();
    });
    test('an existing name gives the existing tag', () => {
        expect(chipForTypedTag(tags, 'close-up', false)).toEqual({
            id: 2,
            name: 'Close-up',
            color: '#f00',
        });
    });
    test('a new name gives a new tag only when allowed', () => {
        expect(chipForTypedTag(tags, ' Sunset ', true)).toEqual({ id: null, name: 'Sunset' });
        expect(chipForTypedTag(tags, 'Sunset', false)).toBeNull();
        expect(chipForTypedTag(tags, '  ', true)).toBeNull();
    });
    test('the create row is offered for unknown names only', () => {
        expect(canOfferNewTag(tags, [], 'Sunset', true)).toBe(true);
        expect(canOfferNewTag(tags, [], 'Sunset', false)).toBe(false);
        expect(canOfferNewTag(tags, [], 'pov', true)).toBe(false);
        expect(canOfferNewTag(tags, [{ id: null, name: 'sunset' }], 'Sunset', true)).toBe(false);
        expect(canOfferNewTag(tags, [], ' ', true)).toBe(false);
    });
});

describe('resolveQuickMarkerLabel', () => {
    test('uses the typed label, else the last one', () => {
        expect(resolveQuickMarkerLabel('  Intro ', 'Outro')).toBe('Intro');
        expect(resolveQuickMarkerLabel('   ', 'Outro')).toBe('Outro');
        expect(resolveQuickMarkerLabel('', '')).toBeNull();
        expect(resolveQuickMarkerLabel('', null)).toBeNull();
    });
    test('caps the length', () => {
        expect(resolveQuickMarkerLabel('x'.repeat(150))).toHaveLength(QUICK_MARKER_LABEL_MAX);
    });
});

describe('time', () => {
    test('timestamps are whole non-negative seconds', () => {
        expect(quickMarkerTimestamp(754.9)).toBe(754);
        expect(quickMarkerTimestamp(-3)).toBe(0);
        expect(quickMarkerTimestamp(Number.NaN)).toBe(0);
        expect(quickMarkerTimestamp(undefined)).toBe(0);
    });
    test('formats like the marker badges', () => {
        expect(formatQuickMarkerTime(0)).toBe('00:00');
        expect(formatQuickMarkerTime(754.6)).toBe('12:34');
        expect(formatQuickMarkerTime(3723)).toBe('1:02:03');
    });
    test('toast message', () => {
        expect(quickMarkerToastMessage(754, 'Cowgirl')).toBe('Marker added at 12:34 - Cowgirl');
    });
});

describe('stepQuickMarkerHighlight', () => {
    test('wraps through the rows', () => {
        expect(stepQuickMarkerHighlight(0, 3, 1)).toBe(1);
        expect(stepQuickMarkerHighlight(2, 3, 1)).toBe(0);
        expect(stepQuickMarkerHighlight(0, 3, -1)).toBe(2);
    });
    test('includes "none" when allowed', () => {
        expect(stepQuickMarkerHighlight(-1, 3, 1, true)).toBe(0);
        expect(stepQuickMarkerHighlight(2, 3, 1, true)).toBe(-1);
        expect(stepQuickMarkerHighlight(-1, 3, -1, true)).toBe(2);
        expect(stepQuickMarkerHighlight(0, 3, -1, true)).toBe(-1);
    });
    test('no rows, no highlight', () => {
        expect(stepQuickMarkerHighlight(0, 0, 1, true)).toBe(-1);
    });
});

describe('isQuickMarkerKey', () => {
    const key = (over: Partial<Parameters<typeof isQuickMarkerKey>[0]> = {}) => ({
        key: 'm',
        ctrlKey: false,
        metaKey: false,
        altKey: false,
        shiftKey: false,
        target: { tagName: 'BODY' },
        ...over,
    });
    test('M alone opens it (caps lock too)', () => {
        expect(isQuickMarkerKey(key())).toBe(true);
        expect(isQuickMarkerKey(key({ key: 'M' }))).toBe(true);
    });
    test('modifiers, repeats and handled events do not', () => {
        expect(isQuickMarkerKey(key({ ctrlKey: true }))).toBe(false);
        expect(isQuickMarkerKey(key({ metaKey: true }))).toBe(false);
        expect(isQuickMarkerKey(key({ altKey: true }))).toBe(false);
        expect(isQuickMarkerKey(key({ shiftKey: true, key: 'M' }))).toBe(false);
        expect(isQuickMarkerKey(key({ repeat: true }))).toBe(false);
        expect(isQuickMarkerKey(key({ defaultPrevented: true }))).toBe(false);
        expect(isQuickMarkerKey(key({ key: 'n' }))).toBe(false);
    });
    test('typing in a field does not', () => {
        expect(isQuickMarkerKey(key({ target: { tagName: 'INPUT' } }))).toBe(false);
        expect(isQuickMarkerKey(key({ target: { tagName: 'textarea' } }))).toBe(false);
        expect(isQuickMarkerKey(key({ target: { tagName: 'DIV', isContentEditable: true } }))).toBe(
            false,
        );
    });
});
