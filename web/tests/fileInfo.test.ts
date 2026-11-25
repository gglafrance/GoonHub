import { describe, expect, test } from 'bun:test';
import {
    allowedFileFields,
    allowedSortOptions,
    fileFieldAllowed,
    sceneSortAllowed,
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
