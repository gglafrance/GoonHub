// Tabs and sub-tabs of the settings page, shared with the command palette. A tab or sub-tab
// with `permissions` is shown when the user has at least one of them; `admin` only places the
// tab in the sidebar's Admin section. Tests in web/tests/settingsTabs.test.ts.
import type { PermissionName } from '~/types/permissions';

export type SettingsTabId =
    | 'account'
    | 'privacy'
    | 'player'
    | 'app'
    | 'homepage'
    | 'tags'
    | 'parsing-rules'
    | 'users'
    | 'jobs'
    | 'storage'
    | 'workers'
    | 'duplicates'
    | 'health'
    | 'trash';

export interface SettingsSubTabConfig {
    id: string;
    label: string;
    permissions?: PermissionName[];
}

export interface SettingsTabConfig {
    id: SettingsTabId;
    label: string;
    icon: string;
    admin?: boolean;
    permissions?: PermissionName[];
    subTabs?: SettingsSubTabConfig[];
}

export const SETTINGS_TABS: SettingsTabConfig[] = [
    { id: 'account', label: 'Account', icon: 'heroicons:user-circle' },
    { id: 'privacy', label: 'Privacy', icon: 'heroicons:lock-closed' },
    { id: 'player', label: 'Player', icon: 'heroicons:play-circle' },
    {
        id: 'app',
        label: 'App',
        icon: 'heroicons:cog-6-tooth',
        subTabs: [
            { id: 'general', label: 'General' },
            { id: 'display', label: 'Display' },
            { id: 'sort-defaults', label: 'Sort Defaults' },
            { id: 'card-template', label: 'Card Template' },
            { id: 'search', label: 'Search', permissions: ['settings:manage'] },
            { id: 'advanced', label: 'Advanced', permissions: ['settings:manage'] },
        ],
    },
    { id: 'homepage', label: 'Homepage', icon: 'heroicons:home' },
    { id: 'tags', label: 'Tags', icon: 'heroicons:tag' },
    { id: 'parsing-rules', label: 'Parsing Rules', icon: 'heroicons:funnel' },
    {
        id: 'users',
        label: 'Users',
        icon: 'heroicons:users',
        admin: true,
        permissions: ['users:manage', 'roles:manage'],
    },
    {
        id: 'jobs',
        label: 'Jobs',
        icon: 'heroicons:queue-list',
        admin: true,
        permissions: ['jobs:manage'],
        subTabs: [
            { id: 'manual', label: 'Manual' },
            { id: 'history', label: 'History' },
            { id: 'workers', label: 'Workers' },
            { id: 'scheduling', label: 'Scheduling' },
            { id: 'processing', label: 'Processing' },
            { id: 'triggers', label: 'Triggers' },
            { id: 'retry', label: 'Retry' },
            { id: 'dlq', label: 'DLQ' },
            { id: 'failing', label: 'Failing' },
        ],
    },
    {
        id: 'storage',
        label: 'Storage',
        icon: 'heroicons:folder',
        admin: true,
        permissions: ['storage:manage'],
    },
    {
        id: 'workers',
        label: 'Workers',
        icon: 'heroicons:server-stack',
        admin: true,
        permissions: ['jobs:manage'],
    },
    {
        id: 'duplicates',
        label: 'Duplicates',
        icon: 'heroicons:document-duplicate',
        admin: true,
        permissions: ['duplicates:manage'],
    },
    {
        id: 'health',
        label: 'Health',
        icon: 'heroicons:heart',
        admin: true,
        permissions: ['library:health'],
    },
    {
        id: 'trash',
        label: 'Trash',
        icon: 'heroicons:trash',
        admin: true,
        permissions: ['trash:manage'],
    },
];

type PermissionCheck = (permission: PermissionName) => boolean;

/** True when the entry has no permission requirement or the user has one of them. */
export function settingsEntryAllowed(
    entry: { permissions?: readonly PermissionName[] },
    has: PermissionCheck,
): boolean {
    return !entry.permissions || entry.permissions.some((p) => has(p));
}

/** Sub-tabs of a tab the user can open. */
export function visibleSettingsSubTabs(
    tab: SettingsTabConfig,
    has: PermissionCheck,
): SettingsSubTabConfig[] {
    return (tab.subTabs ?? []).filter((st) => settingsEntryAllowed(st, has));
}

/** Tabs the user can open; a tab whose sub-tabs are all hidden is hidden too. */
export function visibleSettingsTabs(
    tabs: readonly SettingsTabConfig[],
    has: PermissionCheck,
): SettingsTabConfig[] {
    return tabs.filter(
        (tab) =>
            settingsEntryAllowed(tab, has) &&
            (!tab.subTabs || visibleSettingsSubTabs(tab, has).length > 0),
    );
}
