import { describe, expect, test } from 'bun:test';
import {
    SETTINGS_TABS,
    settingsEntryAllowed,
    visibleSettingsSubTabs,
    visibleSettingsTabs,
    type SettingsTabConfig,
} from '../app/utils/settingsTabs';
import type { PermissionName } from '../app/types/permissions';

const withPermissions =
    (...granted: PermissionName[]) =>
    (permission: PermissionName) =>
        granted.includes(permission);

const ids = (tabs: { id: string }[]) => tabs.map((t) => t.id);

describe('settingsEntryAllowed', () => {
    test('an entry without permissions is always allowed', () => {
        expect(settingsEntryAllowed({}, withPermissions())).toBe(true);
    });

    test('one of the permissions is enough', () => {
        const entry = { permissions: ['users:manage', 'roles:manage'] as PermissionName[] };
        expect(settingsEntryAllowed(entry, withPermissions('roles:manage'))).toBe(true);
        expect(settingsEntryAllowed(entry, withPermissions('jobs:manage'))).toBe(false);
    });
});

describe('visibleSettingsTabs', () => {
    test('a regular user only gets the personal tabs', () => {
        expect(ids(visibleSettingsTabs(SETTINGS_TABS, withPermissions('scenes:view')))).toEqual([
            'account',
            'player',
            'app',
            'homepage',
            'tags',
            'parsing-rules',
        ]);
    });

    test('marker labels need marker_labels:manage (not an admin tab)', () => {
        const tabs = visibleSettingsTabs(
            SETTINGS_TABS,
            withPermissions('scenes:view', 'marker_labels:manage'),
        );
        expect(ids(tabs)).toContain('marker-labels');
        expect(tabs.find((t) => t.id === 'marker-labels')?.admin).toBeUndefined();
    });

    test('each admin tab follows its own permission', () => {
        const tabs = visibleSettingsTabs(
            SETTINGS_TABS,
            withPermissions('jobs:manage', 'trash:manage', 'roles:manage'),
        );
        expect(ids(tabs.filter((t) => t.admin))).toEqual(['users', 'jobs', 'workers', 'trash']);
    });

    test('every permission shows every tab', () => {
        expect(visibleSettingsTabs(SETTINGS_TABS, () => true)).toHaveLength(SETTINGS_TABS.length);
    });

    test('a tab whose sub-tabs are all hidden is hidden', () => {
        const tabs: SettingsTabConfig[] = [
            {
                id: 'app',
                label: 'App',
                icon: 'x',
                subTabs: [{ id: 'search', label: 'Search', permissions: ['settings:manage'] }],
            },
        ];
        expect(visibleSettingsTabs(tabs, withPermissions())).toEqual([]);
        expect(ids(visibleSettingsTabs(tabs, withPermissions('settings:manage')))).toEqual(['app']);
    });
});

describe('visibleSettingsSubTabs', () => {
    const app = SETTINGS_TABS.find((t) => t.id === 'app')!;

    test('search and advanced need settings:manage', () => {
        expect(ids(visibleSettingsSubTabs(app, withPermissions()))).toEqual([
            'general',
            'display',
            'sort-defaults',
            'card-template',
        ]);
        expect(ids(visibleSettingsSubTabs(app, withPermissions('settings:manage')))).toContain(
            'advanced',
        );
    });

    test('a tab without sub-tabs gives none', () => {
        const account = SETTINGS_TABS.find((t) => t.id === 'account')!;
        expect(visibleSettingsSubTabs(account, () => true)).toEqual([]);
    });
});
