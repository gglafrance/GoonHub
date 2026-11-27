// Ratings chart of a profile (Letterboxd style, docs/PROFILES.md): one bar per half star from
// 0.5 to 5. Pure helpers, tests in web/tests/ratingHistogram.test.ts.
import type { ProfileRatingBucket } from '~/types/profile';

export const RATING_MIN = 0.5;
export const RATING_MAX = 5;
export const RATING_STEP = 0.5;

/** The values of the bars, ascending (0.5, 1, ... 5). */
export const RATING_STEPS: number[] = Array.from(
    { length: Math.round((RATING_MAX - RATING_MIN) / RATING_STEP) + 1 },
    (_, i) => RATING_MIN + i * RATING_STEP,
);

/** Index of the bar of a rating (nearest half star, clamped to the scale). */
export function ratingBucketIndex(rating: number): number {
    const i = Math.round((rating - RATING_MIN) / RATING_STEP);
    return Math.min(RATING_STEPS.length - 1, Math.max(0, i));
}

/** Whether a value is a bar of the scale (a valid ?rating= filter). */
export function isRatingStep(value: number): boolean {
    return (
        Number.isFinite(value) &&
        value >= RATING_MIN &&
        value <= RATING_MAX &&
        Math.abs(value * 2 - Math.round(value * 2)) < 1e-9
    );
}

/** Every bar of the scale with its count (missing buckets are empty). */
export function fillRatingBuckets(buckets: ProfileRatingBucket[]): ProfileRatingBucket[] {
    const out = RATING_STEPS.map((rating) => ({ rating, count: 0 }));
    for (const b of buckets) {
        out[ratingBucketIndex(b.rating)]!.count += b.count;
    }
    return out;
}

/** Share of the ratings in a bar, as a whole percent (0 without ratings). */
export function bucketPercent(count: number, total: number): number {
    if (total <= 0 || count <= 0) return 0;
    return Math.round((count / total) * 100);
}

/**
 * Height of each bar in percent of the chart, scaled on the fullest bar (which gets 100). An
 * empty bar gets 0 (the chart draws a thin base line), a bar with ratings at least `minPct`
 * so that it stays visible next to a much fuller one.
 */
export function barHeights(buckets: ProfileRatingBucket[], minPct = 4): number[] {
    const max = Math.max(0, ...buckets.map((b) => b.count));
    return buckets.map((b) => {
        if (max === 0 || b.count <= 0) return 0;
        return Math.max(minPct, Math.round((b.count / max) * 1000) / 10);
    });
}

/** The rating as star glyphs: 3.5 is "★★★½", 0.5 is "½". */
export function ratingStars(rating: number): string {
    const halves = Math.round(rating * 2);
    return '★'.repeat(Math.floor(halves / 2)) + (halves % 2 ? '½' : '');
}

/** "3.5 stars", "1 star", "0.5 stars". */
export function ratingLabel(rating: number): string {
    const value = Number.isInteger(rating) ? String(rating) : rating.toFixed(1);
    return `${value} ${rating === 1 ? 'star' : 'stars'}`;
}

/** Tooltip of a bar: "12 ratings of 3.5 stars (8%)". */
export function bucketTooltip(bucket: ProfileRatingBucket, total: number): string {
    const n = bucket.count;
    return `${n} ${n === 1 ? 'rating' : 'ratings'} of ${ratingLabel(bucket.rating)} (${bucketPercent(n, total)}%)`;
}

/** The average shown next to the chart ("3.46"), or "" without ratings. */
export function formatAverage(average: number | null | undefined): string {
    return average == null ? '' : average.toFixed(2);
}

/** Reads a ?rating= query value: a bar of the scale, or null. */
export function parseRatingQuery(raw: unknown): number | null {
    const value = Array.isArray(raw) ? raw[0] : raw;
    if (typeof value !== 'string' || value === '') return null;
    const n = Number(value);
    return isRatingStep(n) ? n : null;
}
