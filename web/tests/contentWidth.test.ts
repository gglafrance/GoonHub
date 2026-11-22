import { describe, expect, test } from 'bun:test';
import type { ContentWidth, GridDensity } from '../app/types/settings';
import {
    CONTENT_WIDTH_OPTIONS,
    contentWidthClass,
    normalizeContentWidth,
} from '../app/utils/contentWidth';
import { SCENE_GRID } from '../app/utils/sceneGrid';

// Tailwind spacing unit (0.25rem)
const UNIT_PX = 4;

// Maximum width of the container, from its max-w-<n> class (Infinity for max-w-none)
const maxWidthPx = (width: ContentWidth): number => {
    const match = contentWidthClass(width).match(/max-w-(\d+)/);
    return match ? Number(match[1]) * UNIT_PX : Infinity;
};

// Columns the `scene-grid` utility gives from 640px: repeat(auto-fill, <card width>) with the
// sm: gap of the density
const columns = (innerWidth: number, density: GridDensity): number => {
    const grid = SCENE_GRID[density];
    const gap = Number(grid.class.match(/sm:gap-([\d.]+)/)![1]) * UNIT_PX;
    return Math.max(1, Math.floor((innerWidth + gap) / (grid.cardWidthPx + gap)));
};

// Width the grid gets in a viewport: page padding (px-4 / sm:px-5) and the max width
const gridWidth = (viewport: number, width: ContentWidth, padding = 20): number =>
    Math.min(viewport - 2 * padding, maxWidthPx(width));

describe('normalizeContentWidth', () => {
    test('keeps the known widths', () => {
        expect(normalizeContentWidth('default')).toBe('default');
        expect(normalizeContentWidth('wide')).toBe('wide');
        expect(normalizeContentWidth('full')).toBe('full');
    });

    test('falls back to default', () => {
        for (const value of [undefined, null, '', 'Full', 'huge', 3]) {
            expect(normalizeContentWidth(value)).toBe('default');
        }
    });
});

describe('contentWidthClass', () => {
    test('centres the container with a growing max width', () => {
        expect(contentWidthClass('default')).toBe('mx-auto max-w-415');
        expect(contentWidthClass('wide')).toBe('mx-auto max-w-560');
        expect(contentWidthClass('full')).toBe('mx-auto max-w-none');
        expect(contentWidthClass('bogus')).toBe('mx-auto max-w-415');
    });

    test('default keeps the usual 1660px width, wide is wider, full has no limit', () => {
        expect(maxWidthPx('default')).toBe(1660);
        expect(maxWidthPx('wide')).toBeGreaterThan(maxWidthPx('default'));
        expect(maxWidthPx('full')).toBe(Infinity);
    });

    test('lists one option per width', () => {
        expect(CONTENT_WIDTH_OPTIONS.map((o) => o.value)).toEqual(['default', 'wide', 'full']);
    });
});

describe('scene grid columns by content width', () => {
    test('large cards: 3 columns at the default width, 4 on a 1920px screen when wider', () => {
        expect(columns(gridWidth(1920, 'default'), 'large')).toBe(3);
        expect(columns(gridWidth(1920, 'wide'), 'large')).toBe(4);
        expect(columns(gridWidth(1920, 'full'), 'large')).toBe(4);
    });

    test('full width keeps adding columns on bigger screens, wide stops at its max width', () => {
        expect(columns(gridWidth(2560, 'wide'), 'large')).toBe(4);
        expect(columns(gridWidth(2560, 'full'), 'large')).toBe(5);
        expect(columns(gridWidth(3840, 'full'), 'large')).toBe(8);
    });

    test('every density gets more columns when the page is wider', () => {
        for (const density of Object.keys(SCENE_GRID) as GridDensity[]) {
            const atDefault = columns(gridWidth(2560, 'default'), density);
            const atWide = columns(gridWidth(2560, 'wide'), density);
            const atFull = columns(gridWidth(2560, 'full'), density);
            expect(atWide).toBeGreaterThan(atDefault);
            expect(atFull).toBeGreaterThanOrEqual(atWide);
        }
    });

    test('a screen narrower than the default width is not affected', () => {
        for (const density of Object.keys(SCENE_GRID) as GridDensity[]) {
            const atDefault = columns(gridWidth(1440, 'default'), density);
            expect(columns(gridWidth(1440, 'wide'), density)).toBe(atDefault);
            expect(columns(gridWidth(1440, 'full'), density)).toBe(atDefault);
        }
    });

    test('card widths stay those of each density', () => {
        expect(SCENE_GRID.compact.cardWidthPx).toBe(240);
        expect(SCENE_GRID.comfortable.cardWidthPx).toBe(320);
        expect(SCENE_GRID.large.cardWidthPx).toBe(440);
    });
});
