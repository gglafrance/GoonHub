// Tabs and sub-tabs of the settings page, shared with the command palette.

export type SettingsTabId =
    | 'account'
    | 'player'
    | 'app'
    | 'homepage'
    | 'tags'
    | 'parsing-rules'
    | 'users'
    | 'jobs'
    | 'storage'
    | 'duplicates'
    | 'health'
    | 'trash';

export interface SettingsSubTabConfig {
    id: string;
    label: string;
    admin?: boolean;
}

export interface SettingsTabConfig {
    id: SettingsTabId;
    label: string;
    icon: string;
    admin?: boolean;
    subTabs?: SettingsSubTabConfig[];
}

export const SETTINGS_TABS: SettingsTabConfig[] = [
    { id: 'account', label: 'Account', icon: 'heroicons:user-circle' },
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
            { id: 'search', label: 'Search', admin: true },
            { id: 'advanced', label: 'Advanced', admin: true },
        ],
    },
    { id: 'homepage', label: 'Homepage', icon: 'heroicons:home' },
    { id: 'tags', label: 'Tags', icon: 'heroicons:tag' },
    { id: 'parsing-rules', label: 'Parsing Rules', icon: 'heroicons:funnel' },
    { id: 'users', label: 'Users', icon: 'heroicons:users', admin: true },
    {
        id: 'jobs',
        label: 'Jobs',
        icon: 'heroicons:queue-list',
        admin: true,
        subTabs: [
            { id: 'manual', label: 'Manual' },
            { id: 'history', label: 'History' },
            { id: 'workers', label: 'Workers' },
            { id: 'scheduling', label: 'Scheduling' },
            { id: 'processing', label: 'Processing' },
            { id: 'triggers', label: 'Triggers' },
            { id: 'retry', label: 'Retry' },
            { id: 'dlq', label: 'DLQ' },
        ],
    },
    { id: 'storage', label: 'Storage', icon: 'heroicons:folder', admin: true },
    { id: 'duplicates', label: 'Duplicates', icon: 'heroicons:document-duplicate', admin: true },
    { id: 'health', label: 'Health', icon: 'heroicons:heart', admin: true },
    { id: 'trash', label: 'Trash', icon: 'heroicons:trash', admin: true },
];
