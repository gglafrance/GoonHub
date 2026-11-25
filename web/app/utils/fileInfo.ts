// File information gated by the files:* permissions (docs/RBAC.md): the API omits the fields
// and refuses the size filters and sorts without them, so the UI does not offer them either.
// Tests in web/tests/fileInfo.test.ts.
import type { PermissionName } from '~/types/permissions';

type PermissionCheck = (permission: PermissionName) => boolean;

/** Card, badge and table fields that need a permission. */
const FIELD_PERMISSIONS: Readonly<Record<string, PermissionName>> = {
    file_size: 'files:view_size',
    frame_rate: 'files:view_technical',
};

/** Sort orders of the scene lists that need a permission. */
const SORT_PERMISSIONS: Readonly<Record<string, PermissionName>> = {
    size_asc: 'files:view_size',
    size_desc: 'files:view_size',
};

/** True when the user may see the card, badge or table field. */
export function fileFieldAllowed(field: string, has: PermissionCheck): boolean {
    const permission = FIELD_PERMISSIONS[field];
    return !permission || has(permission);
}

/** True when the user may sort scenes this way. */
export function sceneSortAllowed(sort: string, has: PermissionCheck): boolean {
    const permission = SORT_PERMISSIONS[sort];
    return !permission || has(permission);
}

/** The sort options the user may pick. */
export function allowedSortOptions<T extends { value: string }>(
    options: readonly T[],
    has: PermissionCheck,
): T[] {
    return options.filter((o) => sceneSortAllowed(o.value, has));
}

/** The fields (by `value`) the user may pick in the card and table editors. */
export function allowedFileFields<T extends { value: string }>(
    fields: readonly T[],
    has: PermissionCheck,
): T[] {
    return fields.filter((f) => fileFieldAllowed(f.value, has));
}
