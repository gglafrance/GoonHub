import type { PermissionName } from '~/types/permissions';

declare module '#app' {
    interface PageMeta {
        // The page needs at least one of these permissions (checked by the auth middleware)
        permissions?: PermissionName[];
    }
}

export {};
