import type { GridDensity } from '~/types/settings';

// Scene grids per density: column classes and gaps on phones (with sm: gaps from 640px), and
// the card width from 640px, where the grid fits as many cards as the container allows
// (auto-fill, the `scene-grid` utility). A wider page (content width) gets more columns.
export const SCENE_GRID: Record<GridDensity, { class: string; cardWidthPx: number }> = {
    compact: { class: 'grid-cols-3 gap-1.5 sm:gap-2', cardWidthPx: 240 },
    comfortable: { class: 'grid-cols-2 gap-2 sm:gap-3', cardWidthPx: 320 },
    large: { class: 'grid-cols-1 gap-3 sm:gap-4', cardWidthPx: 440 },
};
