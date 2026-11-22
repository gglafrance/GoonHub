import { describe, expect, test } from 'bun:test';
import {
    defaultMergeTarget,
    filterTagsByName,
    mergeSourceIds,
    normalizeTagName,
    parseHexColor,
    plural,
} from '../app/utils/tagManagement';

describe('normalizeTagName', () => {
    test('folds case and spaces like the backend', () => {
        expect(normalizeTagName('Cumshot')).toBe('cumshot');
        expect(normalizeTagName('  cumshot ')).toBe('cumshot');
        expect(normalizeTagName('Big   Tits')).toBe('big tits');
        expect(normalizeTagName('big\ttits')).toBe('big tits');
        expect(normalizeTagName('ÉCOLE')).toBe('école');
        expect(normalizeTagName('   ')).toBe('');
    });
});

describe('parseHexColor', () => {
    test('accepts 6-digit hex with or without #', () => {
        expect(parseHexColor('#ff4d4d')).toBe('#FF4D4D');
        expect(parseHexColor('22c55e')).toBe('#22C55E');
        expect(parseHexColor('  #6B7280 ')).toBe('#6B7280');
    });
    test('rejects anything else', () => {
        for (const bad of ['', '#fff', 'red', '#GGGGGG', '#FF4D4D0', '##FF4D4D']) {
            expect(parseHexColor(bad)).toBeNull();
        }
    });
});

const tags = [
    { id: 1, name: 'Cumshot', scene_count: 3 },
    { id: 2, name: 'cumshot ', scene_count: 10 },
    { id: 3, name: 'Big  Tits', scene_count: 10 },
    { id: 4, name: 'Anal', scene_count: 0 },
];

describe('filterTagsByName', () => {
    test('matches normalized substrings', () => {
        expect(filterTagsByName(tags, 'CUM').map((t) => t.id)).toEqual([1, 2]);
        expect(filterTagsByName(tags, 'big tits').map((t) => t.id)).toEqual([3]);
        expect(filterTagsByName(tags, '  ').map((t) => t.id)).toEqual([1, 2, 3, 4]);
        expect(filterTagsByName(tags, 'zzz')).toEqual([]);
    });
});

describe('defaultMergeTarget', () => {
    test('keeps the tag with most scenes, oldest on a tie', () => {
        expect(defaultMergeTarget(tags)?.id).toBe(2);
        expect(defaultMergeTarget([tags[0]!, tags[3]!])?.id).toBe(1);
        expect(defaultMergeTarget([])).toBeUndefined();
    });
});

describe('mergeSourceIds', () => {
    test('drops the target and duplicates', () => {
        expect(mergeSourceIds([1, 2, 3, 2], 2)).toEqual([1, 3]);
        expect(mergeSourceIds([1, 2], 9)).toEqual([1, 2]);
        expect(mergeSourceIds([1], 1)).toEqual([]);
        expect(mergeSourceIds([1, 2], null)).toEqual([1, 2]);
    });
});

describe('plural', () => {
    test('pluralizes the noun', () => {
        expect(plural(1, 'scene')).toBe('1 scene');
        expect(plural(0, 'scene')).toBe('0 scenes');
        expect(plural(3, 'tag')).toBe('3 tags');
    });
});
