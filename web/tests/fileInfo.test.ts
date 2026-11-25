import { describe, expect, test } from 'bun:test';
import {
    allowedFileFields,
    allowedSortOptions,
    fileFieldAllowed,
    sanitizeSceneSort,
    sceneSortAllowed,
    stripRestrictedSceneParams,
} from '../app/utils/fileInfo';
import type { PermissionName } from '../app/types/permissions';

const withPermissions =
    (...granted: PermissionName[]) =>
    (permission: PermissionName) =>
        granted.includes(permission);

describe('fileFieldAllowed', () => {
    test('file size needs files:view_size', () => {
        expect(fileFieldAllowed('file_size', withPermissions())).toBe(false);
        expect(fileFieldAllowed('file_size', withPermissions('files:view_size'))).toBe(true);
    });

    test('frame rate needs files:view_technical', () => {
        expect(fileFieldAllowed('frame_rate', withPermissions('files:view_size'))).toBe(false);
        expect(fileFieldAllowed('frame_rate', withPermissions('files:view_technical'))).toBe(true);
    });

    test('resolution, duration and the other fields stay', () => {
        for (const field of ['resolution', 'duration', 'added_at', 'views', 'tags']) {
            expect(fileFieldAllowed(field, withPermissions())).toBe(true);
        }
    });
});

describe('sceneSortAllowed', () => {
    test('size sorts need files:view_size', () => {
        expect(sceneSortAllowed('size_asc', withPermissions())).toBe(false);
        expect(sceneSortAllowed('size_desc', withPermissions())).toBe(false);
        expect(sceneSortAllowed('size_desc', withPermissions('files:view_size'))).toBe(true);
    });

    test('other sorts are always allowed', () => {
        expect(sceneSortAllowed('', withPermissions())).toBe(true);
        expect(sceneSortAllowed('duration_desc', withPermissions())).toBe(true);
    });
});

describe('option filters', () => {
    const sorts = [
        { value: 'created_at_desc', label: 'Newest' },
        { value: 'size_desc', label: 'Largest' },
        { value: 'size_asc', label: 'Smallest' },
    ];
    const fields = [
        { value: 'file_size', label: 'File Size' },
        { value: 'resolution', label: 'Resolution' },
        { value: 'frame_rate', label: 'Frame Rate' },
    ];

    test('drop the options the user cannot use, keeping the order', () => {
        expect(allowedSortOptions(sorts, withPermissions()).map((o) => o.value)).toEqual([
            'created_at_desc',
        ]);
        expect(
            allowedFileFields(fields, withPermissions('files:view_technical')).map((f) => f.value),
        ).toEqual(['resolution', 'frame_rate']);
    });

    test('keep everything with the permissions', () => {
        expect(allowedSortOptions(sorts, () => true)).toEqual(sorts);
        expect(allowedFileFields(fields, () => true)).toEqual(fields);
    });
});

describe('sanitizeSceneSort', () => {
    test('keeps an allowed sort', () => {
        expect(sanitizeSceneSort('duration_desc', withPermissions())).toBe('duration_desc');
        expect(sanitizeSceneSort('size_desc', withPermissions('files:view_size'))).toBe(
            'size_desc',
        );
    });

    test('a size sort without files:view_size falls back to the default', () => {
        expect(sanitizeSceneSort('size_desc', withPermissions(), 'title_asc')).toBe('title_asc');
        expect(sanitizeSceneSort('size_asc', withPermissions())).toBe('');
    });

    test('a refused fallback gives the list default', () => {
        expect(sanitizeSceneSort('size_asc', withPermissions(), 'size_desc')).toBe('');
    });

    test('empty or missing sorts give the fallback or the list default', () => {
        expect(sanitizeSceneSort(undefined, withPermissions())).toBe('');
        expect(sanitizeSceneSort(null, withPermissions(), 'random')).toBe('random');
        expect(sanitizeSceneSort('', withPermissions(), 'size_desc')).toBe('');
    });
});

describe('stripRestrictedSceneParams', () => {
    const query = () =>
        new URLSearchParams({
            q: 'beach',
            min_size: '100',
            max_size: '2000',
            sort: 'size_desc',
            page: '2',
        });

    test('removes the size bounds and the size sort without files:view_size', () => {
        const params = stripRestrictedSceneParams(query(), withPermissions());
        expect(params.toString()).toBe('q=beach&page=2');
    });

    test('keeps everything with files:view_size', () => {
        const params = stripRestrictedSceneParams(query(), withPermissions('files:view_size'));
        expect(params.toString()).toBe(query().toString());
    });

    test('keeps other sorts and their seed', () => {
        const params = new URLSearchParams({ sort: 'random', seed: '42', min_size: '5' });
        stripRestrictedSceneParams(params, withPermissions());
        expect(params.toString()).toBe('sort=random&seed=42');
    });

    test('drops the seed with a refused sort', () => {
        const params = new URLSearchParams({ sort: 'size_asc', seed: '42' });
        expect(stripRestrictedSceneParams(params, withPermissions()).toString()).toBe('');
    });
});
