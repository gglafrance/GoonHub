import type { ContentWidth } from '~/types/settings';

// Maximum width of the scene list pages. The scene grids fill the rows with fixed-size cards
// (auto-fill), so a wider page gets more columns at every grid density: with "wide" a 1920 px
// screen fits 4 large cards instead of 3.
// default: 1660px (the usual width), wide: 2240px, full: edge to edge (the page keeps its padding)
const CONTENT_WIDTH_CLASS: Record<ContentWidth, string> = {
    default: 'max-w-415',
    wide: 'max-w-560',
    full: 'max-w-none',
};

export const CONTENT_WIDTH_OPTIONS: { value: ContentWidth; label: string; hint: string }[] = [
    { value: 'default', label: 'Default', hint: 'Usual page width' },
    { value: 'wide', label: 'Wide', hint: 'Wider pages, more columns on large screens' },
    { value: 'full', label: 'Full', hint: 'Edge to edge, as many columns as fit' },
];

export function normalizeContentWidth(width: unknown): ContentWidth {
    return width === 'wide' || width === 'full' ? width : 'default';
}

// Classes of the centred page container (mx-auto and the maximum width)
export function contentWidthClass(width: unknown): string {
    return `mx-auto ${CONTENT_WIDTH_CLASS[normalizeContentWidth(width)]}`;
}
