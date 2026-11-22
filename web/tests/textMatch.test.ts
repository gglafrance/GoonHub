import { describe, expect, test } from 'bun:test';
import { matchRank, rankByMatch } from '../app/utils/textMatch';

describe('matchRank', () => {
    test('prefix, word start, contains, none', () => {
        expect(matchRank('Shower', 'sho')).toBe(0);
        expect(matchRank('Hot Shower', 'sho')).toBe(1);
        expect(matchRank('big-shot', 'sho')).toBe(1);
        expect(matchRank('Cumshot', 'sho')).toBe(2);
        expect(matchRank('Blowjob', 'sho')).toBe(3);
    });
    test('case and spaces of the query', () => {
        expect(matchRank('Shower', '  SHO ')).toBe(0);
        expect(matchRank('anything', '')).toBe(0);
    });
});

describe('rankByMatch', () => {
    const tags = ['Cumshot', 'Anal', 'Hot Shower', 'Shower', 'Shoes', 'Blowjob'];
    test('"Sho": starting with it first, then a word starting with it, then containing it', () => {
        expect(rankByMatch(tags, 'Sho', (t) => t)).toEqual([
            'Shower',
            'Shoes',
            'Hot Shower',
            'Cumshot',
        ]);
    });
    test('keeps the given order inside a rank (the picker sort)', () => {
        const mostUsedFirst = ['Shoes', 'Cumshot', 'Shower'];
        expect(rankByMatch(mostUsedFirst, 'sho', (t) => t)).toEqual(['Shoes', 'Shower', 'Cumshot']);
    });
    test('no query: everything, same order', () => {
        expect(rankByMatch(tags, '  ', (t) => t)).toEqual(tags);
    });
});
