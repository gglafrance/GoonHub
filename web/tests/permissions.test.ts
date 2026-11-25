import { describe, expect, test } from 'bun:test';
import {
    choiceOf,
    defaultRoleName,
    describeSource,
    descendantRoleIds,
    effectiveByName,
    grantsPermission,
    groupPermissions,
    isValidRoleName,
    parentCandidates,
    roleBadgeClass,
    sameSettings,
    sortSettings,
    withChoice,
} from '../app/utils/permissions';
import type { CatalogPermission, PermissionSetting } from '../app/types/permissions';

const perm = (id: number, name: string, group: string): CatalogPermission => ({
    id,
    name,
    group,
    description: '',
});

describe('grantsPermission', () => {
    test('a listed permission is granted, others are not', () => {
        const state = { superuser: false, permissions: ['scenes:view', 'files:view_size'] };
        expect(grantsPermission(state, 'scenes:view')).toBe(true);
        expect(grantsPermission(state, 'jobs:manage')).toBe(false);
    });

    test('a superuser has every permission', () => {
        expect(grantsPermission({ superuser: true, permissions: [] }, 'roles:manage')).toBe(true);
    });
});

describe('groupPermissions', () => {
    test('keeps the catalog order of groups and permissions', () => {
        const groups = groupPermissions([
            perm(1, 'scenes:view', 'Scenes'),
            perm(2, 'tags:manage', 'Library'),
            perm(3, 'scenes:edit', 'Scenes'),
            perm(4, 'actors:merge', 'Library'),
        ]);
        expect(groups.map((g) => g.group)).toEqual(['Scenes', 'Library']);
        expect(groups[0]!.permissions.map((p) => p.id)).toEqual([1, 3]);
        expect(groups[1]!.permissions.map((p) => p.id)).toEqual([2, 4]);
    });

    test('a permission without group goes to Other', () => {
        const groups = groupPermissions([perm(1, 'x:y', '')]);
        expect(groups).toEqual([{ group: 'Other', permissions: [perm(1, 'x:y', '')] }]);
    });

    test('empty catalog', () => {
        expect(groupPermissions([])).toEqual([]);
    });
});

describe('choiceOf and withChoice', () => {
    const settings: PermissionSetting[] = [
        { permission_id: 5, effect: 'deny' },
        { permission_id: 2, effect: 'allow' },
    ];

    test('reads the setting, inherit when absent', () => {
        expect(choiceOf(settings, 2)).toBe('allow');
        expect(choiceOf(settings, 5)).toBe('deny');
        expect(choiceOf(settings, 9)).toBe('inherit');
    });

    test('adds a setting, sorted by permission id', () => {
        expect(withChoice(settings, 3, 'allow')).toEqual([
            { permission_id: 2, effect: 'allow' },
            { permission_id: 3, effect: 'allow' },
            { permission_id: 5, effect: 'deny' },
        ]);
    });

    test('replaces a setting', () => {
        expect(withChoice(settings, 5, 'allow')).toEqual([
            { permission_id: 2, effect: 'allow' },
            { permission_id: 5, effect: 'allow' },
        ]);
    });

    test('inherit removes the setting', () => {
        expect(withChoice(settings, 2, 'inherit')).toEqual([{ permission_id: 5, effect: 'deny' }]);
        expect(withChoice(settings, 9, 'inherit')).toEqual(sortSettings(settings));
    });

    test('does not mutate the input', () => {
        withChoice(settings, 2, 'inherit');
        expect(settings).toHaveLength(2);
    });
});

describe('sameSettings', () => {
    test('equal whatever the order', () => {
        expect(
            sameSettings(
                [
                    { permission_id: 1, effect: 'allow' },
                    { permission_id: 2, effect: 'deny' },
                ],
                [
                    { permission_id: 2, effect: 'deny' },
                    { permission_id: 1, effect: 'allow' },
                ],
            ),
        ).toBe(true);
        expect(sameSettings([], [])).toBe(true);
    });

    test('different effect or length', () => {
        expect(
            sameSettings(
                [{ permission_id: 1, effect: 'allow' }],
                [{ permission_id: 1, effect: 'deny' }],
            ),
        ).toBe(false);
        expect(sameSettings([{ permission_id: 1, effect: 'allow' }], [])).toBe(false);
    });
});

describe('effectiveByName', () => {
    test('indexes by name', () => {
        const map = effectiveByName([
            { name: 'scenes:view', granted: true, source: 'role' },
            { name: 'jobs:manage', granted: false, source: 'none' },
        ]);
        expect(map.get('scenes:view')?.granted).toBe(true);
        expect(map.get('jobs:manage')?.source).toBe('none');
        expect(map.has('roles:manage')).toBe(false);
    });
});

describe('describeSource', () => {
    test('every source', () => {
        expect(describeSource('superuser')).toBe('superuser role');
        expect(describeSource('role')).toBe('role setting');
        expect(describeSource('override')).toBe('user override');
        expect(describeSource('parent:moderator')).toBe('from moderator');
        expect(describeSource('none')).toBe('not set');
    });
});

describe('descendantRoleIds', () => {
    // 1 <- 2 <- 3, 1 <- 4, 3 and 4 <- 5 (diamond), 6 unrelated
    const roles = [
        { id: 1, parent_ids: [] },
        { id: 2, parent_ids: [1] },
        { id: 3, parent_ids: [2] },
        { id: 4, parent_ids: [1] },
        { id: 5, parent_ids: [3, 4] },
        { id: 6, parent_ids: [] },
    ];

    test('direct and indirect children, diamond counted once', () => {
        expect([...descendantRoleIds(roles, 1)].sort()).toEqual([2, 3, 4, 5]);
        expect([...descendantRoleIds(roles, 2)].sort()).toEqual([3, 5]);
    });

    test('a leaf has none, never itself', () => {
        expect(descendantRoleIds(roles, 5).size).toBe(0);
        expect(descendantRoleIds(roles, 6).size).toBe(0);
    });

    test('stops on an existing cycle', () => {
        const cyclic = [
            { id: 1, parent_ids: [2] },
            { id: 2, parent_ids: [1] },
        ];
        expect([...descendantRoleIds(cyclic, 1)]).toEqual([2]);
    });
});

describe('parentCandidates', () => {
    const roles = [
        { id: 1, parent_ids: [] },
        { id: 2, parent_ids: [1] },
        { id: 3, parent_ids: [2] },
        { id: 4, parent_ids: [] },
    ];

    test('every role when creating', () => {
        expect(parentCandidates(roles, null).map((r) => r.id)).toEqual([1, 2, 3, 4]);
    });

    test('excludes the role itself and its descendants', () => {
        expect(parentCandidates(roles, 1).map((r) => r.id)).toEqual([4]);
        expect(parentCandidates(roles, 3).map((r) => r.id)).toEqual([1, 2, 4]);
    });
});

describe('isValidRoleName', () => {
    test('accepts the API pattern', () => {
        expect(isValidRoleName('viewer')).toBe(true);
        expect(isValidRoleName('no-file_info2')).toBe(true);
        expect(isValidRoleName('0x')).toBe(true);
    });

    test('refuses the rest', () => {
        expect(isValidRoleName('a')).toBe(false);
        expect(isValidRoleName('Viewer')).toBe(false);
        expect(isValidRoleName('-viewer')).toBe(false);
        expect(isValidRoleName('my role')).toBe(false);
        expect(isValidRoleName('a'.repeat(51))).toBe(false);
        expect(isValidRoleName('a'.repeat(50))).toBe(true);
    });
});

describe('defaultRoleName', () => {
    test('user when it exists', () => {
        expect(
            defaultRoleName([
                { name: 'admin', is_superuser: true },
                { name: 'viewer', is_superuser: false },
                { name: 'user', is_superuser: false },
            ]),
        ).toBe('user');
    });

    test('else the first regular role, else the first role', () => {
        expect(
            defaultRoleName([
                { name: 'admin', is_superuser: true },
                { name: 'viewer', is_superuser: false },
            ]),
        ).toBe('viewer');
        expect(defaultRoleName([{ name: 'root', is_superuser: true }])).toBe('root');
        expect(defaultRoleName([])).toBe('');
    });
});

describe('roleBadgeClass', () => {
    const lava = 'bg-lava/15 text-lava border-lava/30';

    test('the superuser role stands out', () => {
        expect(roleBadgeClass({ name: 'root', is_superuser: true })).toBe(lava);
        expect(roleBadgeClass('admin')).toBe(lava);
    });

    test('every other role shares one style', () => {
        const other = roleBadgeClass({ name: 'moderator', is_superuser: false });
        expect(other).not.toBe(lava);
        expect(roleBadgeClass('viewer')).toBe(other);
        expect(roleBadgeClass({ name: 'user' })).toBe(other);
    });
});
