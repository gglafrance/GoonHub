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
  - `is_system`: the role cannot be deleted nor renamed (`admin`, and `user`, the default
    role of new accounts: `users.role` defaults to `'user'`). The superuser role cannot lose
    its superuser flag (no API changes the flags).
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

A superuser parent counts as allowing every permission (the child's own deny still wins).

"Deny wins among parents" lets small restrictive roles be used as mixins (for example a
`no-file-info` role that denies the `files:*` permissions, added as a parent). A role's own
setting always overrides its parents, and a user override always overrides the role.

The effective permissions are computed live: a change of role, of role settings, of parents or
of overrides applies to the next request of every affected user, without logging in again (the
role stored in the session token is not used for authorization). The auth middleware resolves
the user's current role and permissions on every request (`RBACService.ResolveUser`). The role
graph (roles, parents, settings) and each user's access are cached and dropped on every change
made through the API; both are also reloaded once older than one minute, so a change made
outside the API (SQL on `users`, `roles`, `role_parents`, `role_permissions`,
`user_permission_overrides`) applies within a minute. The token of a deleted user is refused
(`401`, also by `/events`, which closes an open stream, and by the share links that require
an account).

To keep at least one account able to manage everything, the last user holding a superuser role
cannot be moved to another role nor deleted (`400`). The check is atomic: the repository locks
the rows of the superuser accounts (`SELECT ... FOR UPDATE`) in the same transaction as the
change, so two concurrent demotions cannot remove the last two.

## Delegation

An actor who is not a superuser can only hand out what they hold. These rules are enforced by
the services (`AdminService`, `RBACService`, actor passed in), answer `403`, and a superuser
actor bypasses them all:

- **Assigning a role** (user creation, role change): not a superuser role, and the role's
  effective permissions must be a subset of the actor's.
- **Acting on a user** (role change, password reset, deletion, overrides): not an account
  holding a superuser role. Changing one's own role or overrides is refused (one's own password
  is changed in the account settings).
- **Overrides**: `allow` only for permissions the actor holds (or already allowed by an
  override of that user); `deny` is always possible.
- **Roles**: the actor cannot change the settings, parents or name of their own role nor of
  any role it inherits from, nor delete them; nor change a superuser role. A setting can only
  `allow` a permission the actor holds (or one the role already allows). A parent can only be
  added (role creation or update) if its effective permissions are a subset of the actor's
  (parents already present are kept).

## Catalog

Default grants are those applied when a permission is first inserted. `from X` means "every
role that has X allowed by its own setting at that time" (inherited values are not looked at:
the children inherit the grant from their parent anyway). `admin` is a superuser and needs no
grant (superuser roles never get a grant row).

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
| `marker_labels:manage` | Markers | Create, rename, recolor, merge and delete the server-wide marker labels and set their default tags (`/marker-labels`, `PUT /markers/label-tags`); a marker with an unknown label creates it (without the permission: `400`). See `docs/MARKER_LABELS.md` | moderator |
| `tags:manage` | Library | Create, rename, recolor, merge and delete tags | moderator |
| `actors:manage` | Library | Create, edit and delete actors and their images | none |
| `actors:merge` | Library | Merge actors | existing |
| `studios:manage` | Library | Create, edit and delete studios and their logos | none |
| `studios:merge` | Library | Merge studios | existing |
| `playlists:create`, `playlists:edit`, `playlists:delete`, `playlists:view_public` | Playlists | as today; they also cover the marker playlists (create, add/remove/reorder markers, delete, like; `docs/MARKER_PLAYLISTS.md`) | existing |
| `social:use` | Social | Follow users, send and accept friend requests (`docs/SOCIAL.md`). Reading the directory, lists and profiles, unfollowing, declining, cancelling and removing a friend only need a signed-in user | `user`, from `scenes:view` |
| `files:view_size` | File info | File sizes, size filters and sorts, selection and library totals, biggest file | moderator |
| `files:view_paths` | File info | Stored paths, original file names, storage paths, copies of a scene | moderator |
| `files:view_technical` | File info | Codecs, bit rate, frame rate, file hash, fingerprint, processing errors | moderator |
| `explorer:access` | File info | Folder explorer (pages and API); bulk tags/actors/studio also need `scenes:edit`, bulk delete `scenes:delete` | moderator |
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
  playlists, actor/studio scenes, homepage, explorer and its folders' `total_size`, explorer
  match-info), `min_size`/`max_size` filters and `size_asc`/`size_desc` sorts of
  `GET /scenes` and of the explorer search (refused), `total_size` of the stats widget,
  `biggest_file` record. Saving size filters or sorts is refused (`403`): saved searches
  (`min_size`, `max_size`, size `sort`) and settings (`default_sort_order`, homepage section
  sorts, `sort_preferences.actor_scenes`/`studio_scenes`). Those stored before migration 094
  are ignored, not failed: the saved search and settings responses hide them (default sort,
  no bound) and the homepage sections run without them.
- `files:view_paths`: `stored_path`, `original_filename`, `storage_path_id` in scene payloads
  (and `original_filename` of the explorer match-info); `GET /scenes/:id/copies` (refused). The
  explorer itself (folder names, storage paths) is governed by `explorer:access`. Path oracles
  are closed too: without it, the text search (`GET /scenes` `q`, saved search sections) is
  matched on title, description, actors and tags only (Meilisearch `attributesToSearchOn`,
  not on `path`/`original_filename`), the `folder_latest` homepage widget gives nothing
  (`{"forbidden": true}`) and the `storage_path_id`/`folder` filters of the other scene
  widgets are ignored.
- `files:view_technical`: `video_codec`, `audio_codec`, `bit_rate`, `frame_rate`, `file_hash`,
  `fingerprint_type`, `fingerprint_at`, `processing_error` in scene payloads (and the
  `frame_rate` card field). The `playback` object (needed by the player) stays.
- `explorer:access`: every `/explorer` route.
- `jobs:view_status`: job events on `/events` (`jobs:*`, `scene:retry_scheduled`,
  `scene:dlq_*`) and the header job indicator.
- `storage:manage`: scan events on `/events` (`scan:*`, they carry absolute paths).
- `library:insights`: the trashed scenes in `GET /scenes/:id/similar` and the details of the
  curation and processing homepage widgets.
- Marker playlists carry no file information (no size, path nor technical field): a viewer
  of a public marker playlist gets its markers (also those of users who do not share theirs)
  with only what playing needs. Only markers the owner sees can be added
  (`docs/MARKER_PLAYLISTS.md`).

The redaction is done in one place, `internal/api/v1/response/file_visibility.go`:
`FileVisibility` is built from the request's permissions and passed to every scene DTO
converter (`ToSceneListItem*`, `NewSceneDetail`, homepage, explorer, playlist, history
responses). Admin-only payloads (duplicates, trash, library health, jobs) are not redacted.

Resolution (width/height) and duration stay visible to everyone who can view scenes.

Presence (`docs/PRESENCE.md`): reporting a `watching_scene` activity needs `scenes:view`, and the
scene of an activity (title, thumbnail, position) is only shown to viewers with `scenes:view`
(the others see "watching something"); a presence never carries file information.

### Live events (`/events`)

Every event type has an allowlist (`internal/api/v1/handler/sse_filter.go`): who receives
it and which `data` fields each permission shows. Fields that are not listed are never sent, so
a field added later stays private until it is listed. An unknown event type is sent without its
data. Superusers get every event as is. Notable rules: `scan:*` need `storage:manage`; job
events need `jobs:view_status`; `fingerprint_type` of `scene:fingerprint_complete` needs
`files:view_technical`; the `error` of `scene:failed` needs `files:view_technical` and
`files:view_paths`.

### Social privacy

What a user shows to other users (profile, friends and followers lists, presence, activity...)
is not governed by permissions but by that user's privacy settings (`docs/SOCIAL.md`). Privacy
has **no administrator bypass**: a superuser is a viewer like any other and does not see what a
user keeps private. Social live events (`social:*`, `presence:*`) are personal: they only reach the user they
are for, superusers included (`EventBus.PublishToUser`).

### Known limitations

- `GET /api/v1/scenes/:id/stream` and `/transcode` are not authenticated (unchanged by this
  rework): anyone who knows a scene ID can stream it.

## API

- `GET /api/v1/auth/me`: `{id, username, role}`, role read live.
- `GET /api/v1/auth/permissions`: `{role, superuser, permissions: [names]}`, effective, live
  (sorted; a superuser gets every catalog permission).
- `GET /api/v1/admin/permissions` (`roles:manage` or `users:manage`):
  `{permissions: [{id, name, group, description}]}` (deprecated ones excluded), catalog order.
- `GET /api/v1/admin/roles` (`roles:manage` or `users:manage`): `{roles: [{id, name,
  description, is_system, is_superuser, parent_ids: [], settings: [{permission_id, effect}],
  user_count}]}` (id order; `settings` are the role's own ones, catalog order).
- `POST /api/v1/admin/roles` (`roles:manage`): `{name, description, parent_ids}`; name
  `^[a-z0-9][a-z0-9_-]{1,49}$` (`400`), unique (`409`), parents must exist (`400`). Answers
  `201` with the role (same shape as a `GET /admin/roles` item).
- `PUT /api/v1/admin/roles/:id` (`roles:manage`): `{name, description, parent_ids}`; cycle or
  self parent (`400`), system role rename (`400`), name taken (`409`), unknown role (`404`).
  Answers the role.
- `DELETE /api/v1/admin/roles/:id` (`roles:manage`): system role (`400`), role still held by
  users (`409`: `{error, code: "ROLE_IN_USE", user_count}`), else `{message}`. Its child roles
  lose it as a parent.
- `PUT /api/v1/admin/roles/:id/permissions` (`roles:manage`): `{settings: [{permission_id,
  effect}]}` replaces the role's own settings (absent = inherit; `settings` is required, `[]`
  clears them). Effect `allow` or `deny`, each permission once, deprecated or unknown ones
  refused (`400`). Refused on a superuser role (`400`). Answers the role.
- `GET /api/v1/admin/roles/:id/effective` (`roles:manage`): `{permissions: [{name, granted,
  source}]}` (catalog order) where `source` is `superuser`, `role` (own setting),
  `parent:<role name>` (the ancestor whose own setting decided, possibly a grandparent) or
  `none`.
- `GET /api/v1/admin/users/:id/permissions` (`users:manage`): `{role, overrides:
  [{permission_id, effect}], effective: [{name, granted, source}]}` (`source` adds `override`).
  Unknown user: `404`.
- `PUT /api/v1/admin/users/:id/permissions` (`users:manage`): `{overrides: [{permission_id,
  effect}]}` replaces the user's overrides (required, `[]` clears them; same validation as the
  role settings). Answers the updated `GET` view.
- Existing user routes (`/admin/users...`) need `users:manage`.

The rest of `/admin` needs the permission of its area (see the catalog). There is no
role-name check anymore: `RequireRole("admin")` is gone, a superuser passes every check.

The `/social` routes need a signed-in user; those creating a relation need `social:use`
(`docs/SOCIAL.md`).

Routes listing scenes need `scenes:view`: `/scenes...`, `/actors/:uuid/scenes`,
`/studios/:uuid/scenes`, `/history...`, `/homepage...`, `GET /playlists/:uuid`.

The user and role changes answer `403` when they break the delegation rules above.

## Catalog sync

At startup, for each catalog entry: insert the permission if missing (then apply its default
grants as `allow` settings: named roles that exist, or roles having the `from` permission
allowed), else update its group and description. Permissions in the table but not in the
catalog are left alone (deprecated).
