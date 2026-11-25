# Roles and permissions (RBAC)

This document is the reference for GoonHub access control: the model, how permissions are
resolved, the permission catalog, the API and what each permission shows or hides.

## Model

- **Permission**: a named capability (`scenes:view`, `files:view_paths`...). The catalog is
  defined in code (`internal/core/rbac_catalog.go`) and synced to the `permissions` table at
  startup. Each entry has a group (for the UI), a description and the default grants applied
  the first time the permission is inserted (see "Catalog sync").
- **Role**: a named set of permission settings. A role can have **several parent roles** and
  inherits their permissions. Cycles are refused.
  - `is_superuser`: the role has every permission, nothing can be denied to it (`admin`).
  - `is_system`: the role cannot be deleted (`admin`, and `user`, the default role of new
    accounts). The superuser role cannot be renamed nor lose its superuser flag.
- **Setting** (per role and permission, and per user and permission): `allow` or `deny`. No
  row means "inherit".
- **User**: has exactly one role (`users.role`, the role name, foreign key to `roles.name`
  with `ON UPDATE CASCADE`, so renaming a role follows) and optional **per-user overrides**
  (`allow` / `deny` per permission).

## Resolution

For a user and a permission, the first rule that applies wins:

1. The user's role is a superuser role: **granted**.
2. The user has an override for the permission: `allow` grants, `deny` refuses.
3. The role's effective value (below).

Effective value of a role for a permission:

1. The role has its own setting: `allow` grants, `deny` refuses.
2. Otherwise, from its parents' effective values: **deny wins** (one parent denies: refused),
   else one parent allows: granted.
3. Otherwise: not granted.

"Deny wins among parents" lets small restrictive roles be used as mixins (for example a
`no-file-info` role that denies the `files:*` permissions, added as a parent). A role's own
setting always overrides its parents, and a user override always overrides the role.

The effective permissions are computed live: a change of role, of role settings, of parents or
of overrides applies to the next request of every affected user, without logging in again (the
role stored in the session token is not used for authorization).

## Catalog

Default grants are those applied when a permission is first inserted. `from X` means "every
role that has X allowed at that time". `admin` is a superuser and needs no grant.

| Permission | Group | Gives | Default grant |
|---|---|---|---|
| `scenes:view` | Scenes | Browse and watch scenes | existing |
| `scenes:upload` | Scenes | Upload scenes (upload block, upload indicator) | existing (revoked from `user` by migration 094) |
| `scenes:edit` | Scenes | Edit scene details, tags, actors, studio, thumbnail | moderator |
| `scenes:trash` | Scenes | Move scenes to the trash | existing |
| `scenes:delete` | Scenes | Delete scenes and their files (explorer bulk delete) | existing |
| `scenes:reprocess` | Scenes | Reprocess a scene | existing |
| `scenes:share` | Scenes | Create and delete share links | from `scenes:view` |
| `markers:create` | Markers | Create, edit and delete one's own markers and marker tags | from `scenes:view` |
| `tags:manage` | Library | Create, rename, recolor, merge and delete tags | moderator |
| `actors:manage` | Library | Create, edit and delete actors and their images | none |
| `actors:merge` | Library | Merge actors | existing |
| `studios:manage` | Library | Create, edit and delete studios and their logos | none |
| `studios:merge` | Library | Merge studios | existing |
| `playlists:create`, `playlists:edit`, `playlists:delete`, `playlists:view_public` | Playlists | as today | existing |
| `files:view_size` | File info | File sizes, size filters and sorts, selection and library totals, biggest file | moderator |
| `files:view_paths` | File info | Stored paths, original file names, storage paths, copies of a scene | moderator |
| `files:view_technical` | File info | Codecs, bit rate, frame rate, file hash, fingerprint, processing errors | moderator |
| `explorer:access` | File info | Folder explorer (pages and API) | moderator |
| `jobs:view_status` | Processing | Job status in the header and its live events | moderator |
| `jobs:manage` | Admin | Jobs, pools, processing, triggers, retry, DLQ, workers, transcode cache, scene phases | none |
| `users:manage` | Admin | Users, their role, overrides, password, marker sharing | existing |
| `roles:manage` | Admin | Roles, parents and role permissions | existing |
| `settings:manage` | Admin | App settings, search config and reindex | existing |
| `storage:manage` | Admin | Storage paths and library scans | none |
| `duplicates:manage` | Admin | Duplicates, copies, marker deduplication | none |
| `trash:manage` | Admin | Trash page: restore and permanent deletion | none |
| `library:health` | Admin | Library health | none |
| `library:insights` | Admin | Curation and processing homepage widgets, trashed scenes in Similar | none |
| `porndb:use` | Admin | PornDB search and applying scene metadata | none |
| `import:use` | Admin | Import API | none |
| `stats:view_streams` | Admin | Stream statistics | none |
| `users:create`, `users:delete` | (deprecated) | folded into `users:manage`, kept in the table, not shown | |

Migration 094 also sets the `user` role to the "regular viewer" profile: `scenes:upload` is
revoked from `user` (it keeps `scenes:view` and playlists; it gets `scenes:share` and
`markers:create` from the sync). `moderator` keeps what it has.

## Information shown per permission

Without the permission, the API **omits** the fields (absent from the JSON, not zeroed) and
refuses the matching parameters (`403`); the UI hides the matching blocks.

- `files:view_size`: `size` in every scene payload (detail, list items, similar, history,
  playlists, actor/studio scenes, triage, explorer), `min_size`/`max_size` filters and
  `size_asc`/`size_desc` sorts (refused), `total_size` of the stats widget, `biggest_file` record.
- `files:view_paths`: `stored_path`, `original_filename`, `storage_path_id` in scene payloads;
  `GET /scenes/:id/copies` (refused).
- `files:view_technical`: `video_codec`, `audio_codec`, `bit_rate`, `frame_rate`, `file_hash`,
  `fingerprint_type`, `fingerprint_at`, `processing_error` in scene payloads (and the
  `frame_rate` card field). The `playback` object (needed by the player) stays.
- `explorer:access`: every `/explorer` route.
- `jobs:view_status`: job events on `/events` and the header job indicator.

Resolution (width/height) and duration stay visible to everyone who can view scenes.

## API

- `GET /api/v1/auth/me`: `{id, username, role}`, role read live.
- `GET /api/v1/auth/permissions`: `{role, superuser, permissions: [names]}`, effective, live.
- `GET /api/v1/admin/permissions` (`roles:manage` or `users:manage`):
  `{permissions: [{id, name, group, description}]}` (deprecated ones excluded), catalog order.
- `GET /api/v1/admin/roles` (`roles:manage` or `users:manage`): `{roles: [{id, name,
  description, is_system, is_superuser, parent_ids: [], settings: [{permission_id, effect}],
  user_count}]}`.
- `POST /api/v1/admin/roles` (`roles:manage`): `{name, description, parent_ids}`; name
  `^[a-z0-9][a-z0-9_-]{1,49}$`, unique (`409`).
- `PUT /api/v1/admin/roles/:id` (`roles:manage`): `{name, description, parent_ids}`; cycle
  (`400`), superuser role rename (`400`).
- `DELETE /api/v1/admin/roles/:id` (`roles:manage`): system role (`400`), role still held by
  users (`409` with the count). Its child roles lose it as a parent.
- `PUT /api/v1/admin/roles/:id/permissions` (`roles:manage`): `{settings: [{permission_id,
  effect}]}` replaces the role's own settings (absent = inherit). Refused on a superuser role.
- `GET /api/v1/admin/roles/:id/effective` (`roles:manage`): `{permissions: [{name, granted,
  source}]}` where `source` is `superuser`, `role` (own setting), `parent:<role name>` or `none`.
- `GET /api/v1/admin/users/:id/permissions` (`users:manage`): `{role, overrides:
  [{permission_id, effect}], effective: [{name, granted, source}]}` (`source` adds `override`).
- `PUT /api/v1/admin/users/:id/permissions` (`users:manage`): `{overrides: [{permission_id,
  effect}]}` replaces the user's overrides.
- Existing user routes (`/admin/users...`) need `users:manage`.

The rest of `/admin` needs the permission of its area (see the catalog). There is no
role-name check anymore: `RequireRole("admin")` is gone, a superuser passes every check.

## Catalog sync

At startup, for each catalog entry: insert the permission if missing (then apply its default
grants as `allow` settings: named roles that exist, or roles having the `from` permission
allowed), else update its group and description. Permissions in the table but not in the
catalog are left alone (deprecated).
