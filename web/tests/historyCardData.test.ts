import { describe, expect, test } from 'bun:test';
import { historyCardData } from '../app/utils/historyCardData';

describe('historyCardData', () => {
    test('keeps the card maps of the history response', () => {
        const data = historyCardData({
            entries: [],
            ratings: { 7: 4.5 },
            likes: { 7: true },
            jizz_counts: { 7: 2 },
        });
        expect(data.ratings?.['7']).toBe(4.5);
        expect(data.likes?.['7']).toBe(true);
        expect(data.jizz_counts?.['7']).toBe(2);
    });

    test('a response without card maps (no card_fields) gives empty data', () => {
        expect(historyCardData({ entries: [] })).toEqual({
            ratings: undefined,
            likes: undefined,
            jizz_counts: undefined,
        });
        expect(historyCardData(null)).toEqual({});
    });
});
