import { describe, expect, test } from 'bun:test';
import {
    RATING_STEPS,
    barHeights,
    bucketPercent,
    bucketTooltip,
    fillRatingBuckets,
    formatAverage,
    isRatingStep,
    parseRatingQuery,
    ratingBucketIndex,
    ratingLabel,
    ratingStars,
} from '../app/utils/ratingHistogram';
import { profileTabPath } from '../app/utils/profile';

describe('rating scale', () => {
    test('ten half-star bars from 0.5 to 5', () => {
        expect(RATING_STEPS).toEqual([0.5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 4.5, 5]);
    });

    test('bucket of a rating: nearest half star, clamped', () => {
        expect(ratingBucketIndex(0.5)).toBe(0);
        expect(ratingBucketIndex(3.5)).toBe(6);
        expect(ratingBucketIndex(3.6)).toBe(6);
        expect(ratingBucketIndex(3.8)).toBe(7);
        expect(ratingBucketIndex(5)).toBe(9);
        expect(ratingBucketIndex(0)).toBe(0);
        expect(ratingBucketIndex(9)).toBe(9);
    });

    test('valid filter values', () => {
        expect(isRatingStep(4.5)).toBe(true);
        expect(isRatingStep(1)).toBe(true);
        expect(isRatingStep(3.3)).toBe(false);
        expect(isRatingStep(0)).toBe(false);
        expect(isRatingStep(5.5)).toBe(false);
        expect(isRatingStep(Number.NaN)).toBe(false);
        expect(parseRatingQuery('4.5')).toBe(4.5);
        expect(parseRatingQuery(['2', '3'])).toBe(2);
        expect(parseRatingQuery('abc')).toBeNull();
        expect(parseRatingQuery('3.3')).toBeNull();
        expect(parseRatingQuery('')).toBeNull();
        expect(parseRatingQuery(undefined)).toBeNull();
    });
});

describe('fillRatingBuckets', () => {
    test('every bar, empty ones included, off-scale values merged', () => {
        const filled = fillRatingBuckets([
            { rating: 3.5, count: 4 },
            { rating: 3.6, count: 1 },
            { rating: 5, count: 2 },
        ]);
        expect(filled).toHaveLength(10);
        expect(filled.map((b) => b.count)).toEqual([0, 0, 0, 0, 0, 0, 5, 0, 0, 2]);
        expect(filled[0]).toEqual({ rating: 0.5, count: 0 });
    });

    test('no ratings: ten empty bars', () => {
        expect(fillRatingBuckets([]).every((b) => b.count === 0)).toBe(true);
    });
});

describe('percentages and heights', () => {
    test('whole percents of the total', () => {
        expect(bucketPercent(1, 3)).toBe(33);
        expect(bucketPercent(2, 3)).toBe(67);
        expect(bucketPercent(5, 5)).toBe(100);
        expect(bucketPercent(0, 5)).toBe(0);
        expect(bucketPercent(3, 0)).toBe(0);
    });

    test('heights scaled on the fullest bar, small bars stay visible', () => {
        const filled = fillRatingBuckets([
            { rating: 1, count: 1 },
            { rating: 4, count: 50 },
            { rating: 4.5, count: 25 },
        ]);
        const heights = barHeights(filled);
        expect(heights[ratingBucketIndex(4)]).toBe(100);
        expect(heights[ratingBucketIndex(4.5)]).toBe(50);
        // 1/50 = 2%, raised to the minimum
        expect(heights[ratingBucketIndex(1)]).toBe(4);
        expect(heights[ratingBucketIndex(0.5)]).toBe(0);
        expect(barHeights(fillRatingBuckets([])).every((h) => h === 0)).toBe(true);
    });
});

describe('labels', () => {
    test('stars and labels', () => {
        expect(ratingStars(3.5)).toBe('★★★½');
        expect(ratingStars(5)).toBe('★★★★★');
        expect(ratingStars(0.5)).toBe('½');
        expect(ratingLabel(1)).toBe('1 star');
        expect(ratingLabel(3.5)).toBe('3.5 stars');
        expect(ratingLabel(4)).toBe('4 stars');
    });

    test('tooltip of a bar', () => {
        expect(bucketTooltip({ rating: 3.5, count: 12 }, 150)).toBe('12 ratings of 3.5 stars (8%)');
        expect(bucketTooltip({ rating: 1, count: 1 }, 4)).toBe('1 rating of 1 star (25%)');
        expect(bucketTooltip({ rating: 2, count: 0 }, 4)).toBe('0 ratings of 2 stars (0%)');
    });

    test('average', () => {
        expect(formatAverage(3.456)).toBe('3.46');
        expect(formatAverage(null)).toBe('');
        expect(formatAverage(undefined)).toBe('');
    });
});

describe('profileTabPath', () => {
    test('profile, lists and query', () => {
        expect(profileTabPath('nade')).toBe('/u/nade');
        expect(profileTabPath('nade', 'likes')).toBe('/u/nade/likes');
        expect(profileTabPath('a b', 'ratings', { rating: '4.5' })).toBe(
            '/u/a%20b/ratings?rating=4.5',
        );
    });
});
