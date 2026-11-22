import { describe, expect, test } from 'bun:test';
import type { SimilarScene } from '../app/types/duplicates';
import {
    describeSimilarSegment,
    duplicateGroupLink,
    formatConfidence,
    formatSegmentRange,
    isRelativeToCurrent,
    similarRelationLabel,
    similarRelationTitle,
    similarSceneLink,
} from '../app/utils/similarScenes';

const scene = (over: Partial<SimilarScene> = {}): SimilarScene => ({
    id: 2,
    title: 'Clip',
    duration: 90,
    width: 1920,
    height: 1080,
    size: 1000,
    thumbnail_path: 't.jpg',
    is_trashed: false,
    is_best: false,
    confidence_score: 0.87,
    match_type: 'audio',
    relation: 'contained',
    relation_ref_scene_id: 1,
    segment: { this_start: 70, this_end: 160, other_start: 0, other_end: 90 },
    ...over,
});

describe('formatSegmentRange', () => {
    test('minutes and hours', () => {
        expect(formatSegmentRange(70, 160)).toBe('01:10–02:40');
        expect(formatSegmentRange(3599.6, 3725)).toBe('59:59–1:02:05');
    });
});

describe('describeSimilarSegment', () => {
    test('maps this scene to the other one', () => {
        expect(describeSimilarSegment(scene().segment)).toBe(
            'matches 01:10–02:40 here, 00:00–01:30 there',
        );
    });
    test('no segment', () => {
        expect(describeSimilarSegment(null)).toBe('no matching segment recorded');
    });
});

describe('similarRelationLabel', () => {
    test('relative to the watched scene', () => {
        expect(isRelativeToCurrent(scene(), 1)).toBe(true);
        expect(similarRelationLabel(scene(), 1)).toBe('Clip of this');
        expect(similarRelationLabel(scene({ relation: 'contains' }), 1)).toBe('Contains this');
        expect(similarRelationLabel(scene({ relation: 'identical' }), 1)).toBe('Identical');
        expect(similarRelationLabel(scene({ relation: 'partial' }), 1)).toBe('Partial');
        expect(similarRelationLabel(scene({ relation: 'unknown' }), 1)).toBe('Unverified');
    });
    test('as stored, to another scene', () => {
        const other = scene({ relation_ref_scene_id: 9 });
        expect(isRelativeToCurrent(other, 1)).toBe(false);
        expect(similarRelationLabel(other, 1)).toBe('Clip of #9');
        expect(similarRelationLabel(scene({ relation_ref_scene_id: null }), 1)).toBe(
            'Clip of its match',
        );
    });
    test('titles name the reference', () => {
        expect(similarRelationTitle(scene(), 1)).toBe('A part of this scene, which is longer');
        expect(
            similarRelationTitle(scene({ relation: 'contains', relation_ref_scene_id: 9 }), 1),
        ).toBe('Contains scene #9, and more');
    });
});

describe('formatConfidence', () => {
    test('rounds and clamps', () => {
        expect(formatConfidence(0.874)).toBe('87%');
        expect(formatConfidence(1.5)).toBe('100%');
        expect(formatConfidence(-1)).toBe('0%');
        expect(formatConfidence(Number.NaN)).toBe('0%');
    });
});

describe('similarSceneLink', () => {
    test('opens at the matched start', () => {
        expect(
            similarSceneLink(
                scene({
                    segment: { this_start: 0, this_end: 1, other_start: 75.9, other_end: 80 },
                }),
            ),
        ).toEqual({ path: '/watch/2', query: { t: '75' } });
        expect(similarSceneLink(scene())).toEqual({ path: '/watch/2', query: { t: '0' } });
    });
    test('without segment', () => {
        expect(similarSceneLink(scene({ segment: null }))).toEqual({ path: '/watch/2' });
    });
});

describe('duplicateGroupLink', () => {
    test('with and without status', () => {
        expect(duplicateGroupLink(4, 'dismissed')).toEqual({
            path: '/duplicates',
            query: { group: '4', status: 'dismissed' },
        });
        expect(duplicateGroupLink(4)).toEqual({ path: '/duplicates', query: { group: '4' } });
    });
});
