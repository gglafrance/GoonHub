import type { SceneListView, SceneTableColumn } from '~/types/settings';

export interface SceneTableColumnDef {
    id: SceneTableColumn;
    label: string;
    // card_fields entry the list endpoints need for this column (none: always included)
    field?: string;
    // Sort orders of the scene list API ("" when the backend cannot sort on it)
    sortAsc?: string;
    sortDesc?: string;
    // Direction of the first click on the header
    firstDir?: 'asc' | 'desc';
    align?: 'left' | 'right';
    // Header/cell width classes
    width?: string;
}

export const SCENE_TABLE_COLUMNS: SceneTableColumnDef[] = [
    { id: 'thumbnail', label: 'Thumbnail', width: 'w-[76px]' },
    {
        id: 'title',
        label: 'Title',
        sortAsc: 'title_asc',
        sortDesc: 'title_desc',
        firstDir: 'asc',
    },
    {
        id: 'duration',
        label: 'Duration',
        sortAsc: 'duration_asc',
        sortDesc: 'duration_desc',
        firstDir: 'desc',
        align: 'right',
        width: 'w-20',
    },
    { id: 'resolution', label: 'Resolution', field: 'resolution', width: 'w-20' },
    {
        id: 'file_size',
        label: 'Size',
        sortAsc: 'size_asc',
        sortDesc: 'size_desc',
        firstDir: 'desc',
        align: 'right',
        width: 'w-20',
    },
    { id: 'rating', label: 'Rating', field: 'rating', width: 'w-16' },
    {
        id: 'views',
        label: 'Views',
        field: 'views',
        sortAsc: 'view_count_asc',
        sortDesc: 'view_count_desc',
        firstDir: 'desc',
        align: 'right',
        width: 'w-16',
    },
    { id: 'studio', label: 'Studio', field: 'studio', width: 'w-32' },
    { id: 'actors', label: 'Actors', field: 'actors', width: 'w-44' },
    { id: 'release_date', label: 'Released', field: 'release_date', width: 'w-24' },
    {
        id: 'added_at',
        label: 'Added',
        sortAsc: 'created_at_asc',
        sortDesc: 'created_at_desc',
        firstDir: 'desc',
        width: 'w-24',
    },
];

export const DEFAULT_SCENE_TABLE_COLUMNS: SceneTableColumn[] = [
    'thumbnail',
    'title',
    'duration',
    'resolution',
    'file_size',
    'rating',
    'views',
    'studio',
    'actors',
    'added_at',
];

export function defaultSceneListView(): SceneListView {
    return { pages: {}, columns: [...DEFAULT_SCENE_TABLE_COLUMNS] };
}

export function sceneTableColumnDef(id: SceneTableColumn): SceneTableColumnDef | undefined {
    return SCENE_TABLE_COLUMNS.find((c) => c.id === id);
}

// Resolution label from the frame height (as the card badge)
export function resolutionLabel(height?: number): string {
    if (!height) return '';
    if (height >= 2160) return '4K';
    if (height >= 1440) return '1440p';
    if (height >= 1080) return '1080p';
    if (height >= 720) return '720p';
    if (height >= 480) return '480p';
    return `${height}p`;
}
