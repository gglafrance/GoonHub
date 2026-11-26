# Database Architecture

This document provides a comprehensive reference of the GoonHub database schema.

**Database:** PostgreSQL 18
**Migration Tool:** golang-migrate (embedded, runs automatically on startup)
**Migrations Path:** `internal/infrastructure/persistence/migrator/migrations/`

---

## Table of Contents

1. [Core Tables](#core-tables)
2. [RBAC (Role-Based Access Control)](#rbac-role-based-access-control)
3. [Content Organization](#content-organization)
4. [Sharing](#sharing)
5. [Playlists](#playlists)
6. [Social Graph](#social-graph) (and [Messages](#messages))
7. [User Profiles](#user-profiles)
8. [User Interactions](#user-interactions)
9. [Job System](#job-system)
10. [Storage & Scanning](#storage--scanning)
11. [Duplicate Detection](#duplicate-detection)
12. [Goon2gether](#goon2gether)
13. [Application Settings](#application-settings)
14. [Entity Relationship Diagram](#entity-relationship-diagram)
15. [Key Patterns & Conventions](#key-patterns--conventions)

---

## Core Tables

### `scenes`

Main content table storing video/scene metadata and processing state. Originally named `videos`, renamed in migration 034.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `deleted_at` | TIMESTAMPTZ | YES | NULL | Soft delete timestamp |
| `trashed_at` | TIMESTAMPTZ | YES | NULL | When moved to trash (for retention) |
| `title` | VARCHAR(255) | YES | NULL | Display title |
| `description` | TEXT | NO | '' | Scene description |
| `original_filename` | VARCHAR(255) | YES | NULL | Original uploaded filename |
| `stored_path` | VARCHAR(512) | YES | NULL | Full filesystem path |
| `storage_path_id` | INTEGER | YES | NULL | FK to `storage_paths.id` |
| `size` | BIGINT | YES | 0 | File size in bytes |
| `view_count` | BIGINT | YES | 0 | Total view count |
| `duration` | INTEGER | YES | 0 | Duration in seconds |
| `width` | INTEGER | YES | 0 | Video width in pixels |
| `height` | INTEGER | YES | 0 | Video height in pixels |
| `frame_rate` | DOUBLE PRECISION | NO | 0 | Frames per second |
| `bit_rate` | BIGINT | NO | 0 | Bit rate in bps |
| `video_codec` | TEXT | NO | '' | Video codec (e.g., h264) |
| `audio_codec` | TEXT | NO | '' | Audio codec (e.g., aac) |
| `file_hash` | TEXT | NO | '' | SHA256 file hash |
| `file_created_at` | TIMESTAMPTZ | YES | NULL | Original file creation date |
| `release_date` | DATE | YES | NULL | Scene release date |
| `thumbnail_path` | VARCHAR(512) | YES | NULL | Path to thumbnail image |
| `thumbnail_width` | INTEGER | YES | 0 | Thumbnail width |
| `thumbnail_height` | INTEGER | YES | 0 | Thumbnail height |
| `sprite_sheet_path` | VARCHAR(512) | YES | NULL | Path to sprite sheet |
| `sprite_sheet_count` | INTEGER | YES | 0 | Number of sprites |
| `vtt_path` | VARCHAR(512) | YES | NULL | Path to VTT file |
| `cover_image_path` | TEXT | NO | '' | Path to cover image |
| `studio` | TEXT | NO | '' | Legacy studio name (deprecated) |
| `studio_id` | BIGINT | YES | NULL | FK to `studios.id` |
| `tags` | TEXT[] | NO | '{}' | Legacy tags array (deprecated) |
| `actors` | TEXT[] | NO | '{}' | Legacy actors array (deprecated) |
| `origin` | VARCHAR(100) | YES | NULL | Content origin (upload, scan, import) |
| `type` | VARCHAR(50) | YES | NULL | Content type classification |
| `porndb_scene_id` | TEXT | NO | '' | PornDB external scene ID |
| `source_url` | TEXT | NO | '' | Web page the scene comes from (http/https; '' = unknown). Migration 074 moved descriptions that were only a URL here |
| `processing_status` | VARCHAR(50) | YES | 'pending' | Processing pipeline status |
| `processing_error` | TEXT | YES | NULL | Last processing error message |
| `is_corrupted` | BOOLEAN | NO | FALSE | Video file failed integrity check |

**Indexes:**
- `idx_scenes_deleted_at` on `deleted_at`
- `idx_scenes_trashed_at` on `trashed_at` WHERE trashed_at IS NOT NULL
- `idx_scenes_origin` on `origin` WHERE origin IS NOT NULL
- `idx_scenes_type` on `type` WHERE type IS NOT NULL
- `idx_scenes_stored_path` on `stored_path` WHERE deleted_at IS NULL
- `idx_scenes_size_filename` on `(size, original_filename)`
- `idx_scenes_studio_id` on `studio_id`

---

### `users`

User accounts with authentication credentials.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Account creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `username` | VARCHAR(255) | NO | - | Unique username |
| `password` | VARCHAR(255) | NO | - | Bcrypt hashed password |
| `role` | VARCHAR(50) | NO | 'user' | Role name, FK to `roles.name` (`ON UPDATE CASCADE ON DELETE RESTRICT`, migration 094) |
| `last_login_at` | TIMESTAMPTZ | YES | NULL | Last successful login |
| `share_markers` | BOOLEAN | NO | FALSE | The user's markers are visible (read-only) to every user (set by an admin) |

**Indexes:**
- `idx_users_role` on `role`

**Constraints:**
- `uni_users_username` UNIQUE on `username`
- `fk_users_role` FK `role` -> `roles(name)` ON UPDATE CASCADE (renaming a role follows) ON DELETE RESTRICT (a role held by users cannot be deleted)

---

### `user_settings`

Per-user application preferences.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `autoplay` | BOOLEAN | NO | false | Auto-play next scene |
| `default_volume` | INTEGER | NO | 100 | Default player volume (0-100) |
| `loop` | BOOLEAN | NO | true | Loop video playback |
| `default_marker_view` | VARCHAR(10) | NO | 'grouped' | Initial view of the markers page (`grouped` or `all`) |
| `advanced_marker_filters` | BOOLEAN | NO | false | Show the advanced filter panel (labels, scene tags/actors/studio, ranges only) on the marker pages |
| `pagination_position` | VARCHAR(10) | NO | 'bottom' | Page controls position on paginated pages: `top`, `bottom` or `both` |
| `grid_density` | VARCHAR(12) | NO | 'comfortable' | Card size of the scene grids (and actor/studio grids): `compact`, `comfortable` or `large` |
| `content_width` | VARCHAR(10) | NO | 'default' | Maximum width of the scene list pages (search, actor, studio, explorer, history, markers): `default` (usual width), `wide` or `full` (edge to edge); the grids add columns as it grows (migration 087) |
| `infinite_scroll` | BOOLEAN | NO | false | Scene list pages (search, actor, studio, explorer) load the next page on scroll instead of showing page controls |
| `date_format` | VARCHAR(20) | NO | 'auto' | Date display: `auto` (browser locale), `DD/MM/YYYY`, `MM/DD/YYYY`, `YYYY-MM-DD`, `DD.MM.YYYY`, `D MMM YYYY` or `MMM D, YYYY` |
| `player_gestures` | BOOLEAN | NO | true | Touch gestures on the video player: double-tap seek, swipes (scrub, volume, brightness) and long-press 2x speed |
| `gesture_seek_step` | INTEGER | NO | 10 | Seconds skipped by a double-tap on the sides of the player: 5, 10, 15 or 30 |
| `show_replay_heatmap` | BOOLEAN | NO | true | Show the "most replayed" heatmap (from `user_scene_heatmaps`) above the player progress bar |
| `record_replay_heatmap` | BOOLEAN | NO | true | Record which parts of a scene are watched into `user_scene_heatmaps` |
| `show_scrubber` | BOOLEAN | NO | false | Show the thumbnail scrubber strip (sprite sheet thumbnails and markers, drag or wheel to scroll, click to seek) under the player on the watch page (migration 084) |
| `scrubber_mode` | VARCHAR(10) | NO | 'free' | Behaviour of the scrubber strip: `free` (scrolled on its own, click to seek, follows playback) or `centered` (like Stash: the centre line is the playback position, dragging or wheeling the strip seeks the video) (migration 086) |
| `marker_loop` | BOOLEAN | NO | false | Loop the marker range when clicking a marker that has an `end_timestamp` |
| `videos_per_page` | INTEGER | NO | 20 | Grid pagination size |
| `default_sort_order` | VARCHAR(50) | NO | 'created_at_desc' | Default sort order |
| `default_tag_sort` | VARCHAR(10) | NO | 'az' | Tag sorting (az, za, count) |
| `marker_thumbnail_cycling` | BOOLEAN | NO | true | Enable marker thumbnail cycling |
| `homepage_config` | JSONB | NO | (see below) | Homepage section configuration |
| `scene_list_view` | JSONB | NO | (see below) | Scene list display: `pages` maps a page type (`search`, `actor`, `studio`, `playlist`, `explorer`) to `grid` or `table` (missing = `grid`); `columns` are the visible table columns in order (`thumbnail`, `title`, `duration`, `resolution`, `file_size`, `rating`, `views`, `studio`, `actors`, `release_date`, `added_at`) |

**Default `homepage_config`:**
```json
{
  "show_upload": true,
  "sections": [{
    "id": "default-latest",
    "type": "latest",
    "title": "Latest Uploads",
    "enabled": true,
    "limit": 12,
    "order": 0,
    "sort": "created_at_desc",
    "config": {}
  }]
}
```

**Default `scene_list_view`:**
```json
{
  "pages": {},
  "columns": ["thumbnail", "title", "duration", "resolution", "file_size", "rating", "views", "studio", "actors", "added_at"]
}
```

**Constraints:**
- `uni_user_settings_user_id` UNIQUE on `user_id`
- CHECK `chk_user_settings_scrubber_mode`: `scrubber_mode IN ('free', 'centered')`
- CHECK `chk_user_settings_content_width`: `content_width IN ('default', 'wide', 'full')`
- FK to `users(id)` ON DELETE CASCADE

---

### `user_nav_layouts`

Per-account layout of the two header bars (migration 106, `docs/NAVIGATION.md`): where each
header item goes (bar and zone, or a folder), the user's folders and the hidden items. One row
per user who customized the header; no row = the default layout (today's header), so those users
follow the defaults of later versions. Reset to default deletes the row.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | Primary key, FK to `users.id` (CASCADE) |
| `layout` | JSONB | NO | - | `{version: 1, bars: {top, nav: {left, center, right: [entry]}}, folders: [{id, name, icon, items: [item id]}], hidden: [item id]}`; an entry is an item id or `folder:<id>` |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last save |

The layout is validated by the application (`core.NavLayoutService`): known item ids, each item
and folder once, items in the bars they allow, required items (logo, settings, logout) never
hidden, at most 12 folders, folder names of 1 to 32 characters. Items the user lacks the
permission for are never stored, and are left out when a layout is read.

**Constraints:**
- CHECK `chk_user_nav_layouts_layout`: `jsonb_typeof(layout) = 'object'`
- FK to `users(id)` ON DELETE CASCADE

---

## RBAC (Role-Based Access Control)

Roles with several parent roles, `allow`/`deny` settings, per-user overrides and a superuser
role. The resolution rules, the permission catalog and the API are described in
[`docs/RBAC.md`](RBAC.md). Effective permissions are resolved live by the application
(`core.RBACService`), never stored.

### `roles`

Role definitions for access control.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `name` | VARCHAR(50) | NO | - | Unique role name (`users.role` references it) |
| `description` | VARCHAR(255) | YES | NULL | Role description |
| `is_system` | BOOLEAN | NO | FALSE | System role: cannot be deleted nor renamed (`admin`, `user`; migration 094) |
| `is_superuser` | BOOLEAN | NO | FALSE | Superuser role: every permission, nothing can be denied (`admin`; migration 094) |

**Seed Data:**
- `admin` - Full system access (system, superuser)
- `moderator` - Video management access
- `user` - Regular viewer, default role of new accounts (system)

**Constraints:**
- `uni_roles_name` UNIQUE on `name`

---

### `role_parents`

Parent roles: `role_id` inherits the permissions of `parent_id` (several parents per role,
cycles refused by the application). Migration 094.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `role_id` | BIGINT | NO | - | FK to `roles.id` (CASCADE) |
| `parent_id` | BIGINT | NO | - | FK to `roles.id` (CASCADE: a deleted role is removed from its children's parents) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |

**Indexes:**
- `idx_role_parents_parent_id` on `parent_id`

**Constraints:**
- PRIMARY KEY `(role_id, parent_id)`
- CHECK `chk_role_parents_not_self`: `role_id <> parent_id`

---

### `permissions`

Permission definitions for granular access control. Synced at startup from the catalog
(`internal/core/rbac_catalog.go`): missing permissions are inserted with their default grants,
the group and description of the others are kept up to date. Permissions that left the catalog
stay in the table (deprecated: `users:create`, `users:delete`).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `name` | VARCHAR(100) | NO | - | Unique permission identifier |
| `description` | VARCHAR(255) | YES | NULL | Permission description |
| `group_name` | VARCHAR(50) | NO | '' | Group shown by the UI (`Scenes`, `Markers`, `Library`, `Playlists`, `File info`, `Processing`, `Admin`; migration 094) |

**Permissions:** see the catalog in [`docs/RBAC.md`](RBAC.md#catalog) (`scenes:*`,
`markers:create`, `tags:manage`, `actors:*`, `studios:*`, `playlists:*`, `files:view_size`,
`files:view_paths`, `files:view_technical`, `explorer:access`, `jobs:*`, `users:manage`,
`roles:manage`, `settings:manage`, `storage:manage`, `duplicates:manage`, `trash:manage`,
`library:health`, `library:insights`, `porndb:use`, `import:use`, `stats:view_streams`).

**Constraints:**
- `uni_permissions_name` UNIQUE on `name`

---

### `role_permissions`

A role's own setting for a permission: `allow` or `deny`. No row means "inherit from the
parents".

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `role_id` | BIGINT | NO | - | FK to `roles.id` (CASCADE) |
| `permission_id` | BIGINT | NO | - | FK to `permissions.id` (CASCADE) |
| `effect` | VARCHAR(5) | NO | 'allow' | `allow` or `deny` (migration 094; the rows before it are grants) |

**Indexes:**
- `idx_role_permissions_role_id` on `role_id`

**Constraints:**
- `uni_role_permissions` UNIQUE on `(role_id, permission_id)`
- CHECK `chk_role_permissions_effect`: `effect IN ('allow', 'deny')`

---

### `user_permission_overrides`

Per-user overrides: beat the role's effective value (except for a superuser role, which has
everything). Migration 094.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `permission_id` | BIGINT | NO | - | FK to `permissions.id` (CASCADE) |
| `effect` | VARCHAR(5) | NO | - | `allow` or `deny` |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |

**Indexes:**
- `idx_user_permission_overrides_permission_id` on `permission_id`

**Constraints:**
- PRIMARY KEY `(user_id, permission_id)`
- CHECK `chk_user_permission_overrides_effect`: `effect IN ('allow', 'deny')`

---

### `revoked_tokens`

Tracks revoked authentication tokens for logout/invalidation.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Revocation timestamp |
| `token_hash` | VARCHAR(64) | NO | - | SHA256 hash of token |
| `expires_at` | TIMESTAMPTZ | NO | - | Token expiration (for cleanup) |
| `reason` | VARCHAR(255) | YES | NULL | Revocation reason |

**Indexes:**
- `idx_revoked_tokens_expires_at` on `expires_at`

**Constraints:**
- `uni_revoked_tokens_token_hash` UNIQUE on `token_hash`

---

## Content Organization

### `tags`

Tag definitions for categorizing scenes.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `name` | VARCHAR(100) | NO | - | Unique tag name |
| `color` | VARCHAR(7) | NO | '#6B7280' | Hex color code |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |

**Constraints:**
- UNIQUE on `name`

---

### `scene_tags`

Junction table linking scenes to tags.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `tag_id` | BIGINT | NO | - | FK to `tags.id` (CASCADE) |

**Indexes:**
- `idx_scene_tags_scene_id` on `scene_id`
- `idx_scene_tags_tag_id` on `tag_id`

**Constraints:**
- UNIQUE on `(scene_id, tag_id)`

---

### `actors`

Actor/performer profiles with biographical data.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `uuid` | UUID | NO | gen_random_uuid() | Public identifier |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `deleted_at` | TIMESTAMPTZ | YES | NULL | Soft delete timestamp |
| `name` | VARCHAR(255) | NO | - | Actor name |
| `aliases` | TEXT[] | NO | '{}' | Other names of the actor (NOT NULL since migration 083) |
| `image_url` | VARCHAR(512) | YES | NULL | Profile image URL |
| `gender` | VARCHAR(50) | YES | NULL | Gender identity |
| `birthday` | DATE | YES | NULL | Date of birth |
| `date_of_death` | DATE | YES | NULL | Date of death (if applicable) |
| `astrology` | VARCHAR(50) | YES | NULL | Astrological sign |
| `birthplace` | VARCHAR(255) | YES | NULL | Place of birth |
| `ethnicity` | VARCHAR(100) | YES | NULL | Ethnicity |
| `nationality` | VARCHAR(100) | YES | NULL | Nationality |
| `career_start_year` | INTEGER | YES | NULL | Year career began |
| `career_end_year` | INTEGER | YES | NULL | Year career ended |
| `height_cm` | INTEGER | YES | NULL | Height in centimeters |
| `weight_kg` | INTEGER | YES | NULL | Weight in kilograms |
| `measurements` | VARCHAR(50) | YES | NULL | Body measurements |
| `cupsize` | VARCHAR(10) | YES | NULL | Cup size |
| `hair_color` | VARCHAR(50) | YES | NULL | Hair color |
| `eye_color` | VARCHAR(50) | YES | NULL | Eye color |
| `tattoos` | TEXT | YES | NULL | Tattoo descriptions |
| `piercings` | TEXT | YES | NULL | Piercing descriptions |
| `fake_boobs` | BOOLEAN | NO | false | Enhanced breasts flag |
| `same_sex_only` | BOOLEAN | NO | false | Same-sex content only flag |

**Indexes:**
- `idx_actors_uuid` on `uuid`
- `idx_actors_name` on `name`
- `idx_actors_name_lower` on `lower(name)` WHERE deleted_at IS NULL
- `idx_actors_aliases` GIN on `aliases`
- `idx_actors_deleted_at` on `deleted_at`

**Names and aliases:** names are not unique (import clients create same-name actors). Aliases
are trimmed, deduplicated case-insensitively and never equal the actor's own name; the
application keeps an alias from being another actor's name or alias (rejected with 409 in
strict mode, dropped otherwise), there is no database constraint because existing data may
already overlap. Resolution by name (`GET /actors/resolve`, PornDB matching) is an exact,
case-insensitive match on the name or an alias.

**Actors split by alias (migration 083):** the up migration merges every actor B into actor A
when B's name (trimmed, case-insensitive) equals an alias of exactly one other non-deleted
actor A, the way the merge tool does (scene credits, ratings and likes, homepage sections,
saved filters, aliases, photo; B is soft-deleted). The plan is computed once on the data
before any merge: a name that is an alias of several actors is skipped; with mutual aliases
the oldest actor (lowest id) is kept; a chain (C -> B -> A) is merged into its final target A;
loops of three or more actors, and chains leading into them, are skipped. Studios follow the
same rule (also scenes and child/network studios). The down migration only reverts the
schema: the merges are not undone.

**Constraints:**
- UNIQUE on `uuid`

---

### `scene_actors`

Junction table linking scenes to actors.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `actor_id` | BIGINT | NO | - | FK to `actors.id` (CASCADE) |

**Indexes:**
- `idx_scene_actors_scene_id` on `scene_id`
- `idx_scene_actors_actor_id` on `actor_id`

**Constraints:**
- UNIQUE on `(scene_id, actor_id)`

---

### `studios`

Studio/network entities for content organization.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `uuid` | UUID | NO | gen_random_uuid() | Public identifier |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `deleted_at` | TIMESTAMPTZ | YES | NULL | Soft delete timestamp |
| `name` | VARCHAR(255) | NO | - | Studio name |
| `aliases` | TEXT[] | NO | '{}' | Other names of the studio (migration 083), same rules as `actors.aliases` |
| `short_name` | VARCHAR(100) | YES | NULL | Abbreviated name |
| `url` | VARCHAR(512) | YES | NULL | Studio website |
| `description` | TEXT | YES | NULL | Studio description |
| `rating` | DECIMAL(3,1) | YES | NULL | Average rating |
| `logo` | VARCHAR(512) | YES | NULL | Logo image path |
| `favicon` | VARCHAR(512) | YES | NULL | Favicon path |
| `poster` | VARCHAR(512) | YES | NULL | Poster image path |
| `porndb_id` | VARCHAR(100) | YES | NULL | PornDB external ID |
| `parent_id` | BIGINT | YES | NULL | FK to parent `studios.id` |
| `network_id` | BIGINT | YES | NULL | FK to network `studios.id` |

**Indexes:**
- `idx_studios_uuid` on `uuid`
- `idx_studios_name` on `name`
- `idx_studios_name_lower` on `lower(name)` WHERE deleted_at IS NULL
- `idx_studios_aliases` GIN on `aliases`
- `idx_studios_deleted_at` on `deleted_at`
- `idx_studios_porndb_id` on `porndb_id`

**Constraints:**
- UNIQUE on `uuid`

---

## Sharing

### `share_links`

Shareable links for scenes with configurable visibility and expiration.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `token` | VARCHAR(32) | NO | - | URL-safe random token (22 chars, base64url) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `share_type` | VARCHAR(20) | NO | 'public' | Link visibility type |
| `expires_at` | TIMESTAMPTZ | YES | NULL | Expiration timestamp (NULL = never) |
| `view_count` | BIGINT | NO | 0 | Number of times link was accessed |
| `start_s` | INTEGER | YES | NULL | Where the shared player starts, seconds (migration 102, `docs/MESSAGES.md` "Share links") |
| `end_s` | INTEGER | YES | NULL | End of a shared marker range (the share page loops it) |
| `label` | VARCHAR(100) | YES | NULL | Label of a shared marker (copied) |
| `marker_id` | BIGINT | YES | NULL | FK to `user_scene_markers.id` (SET NULL): the marker the range came from (never sent) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Link creation timestamp |

**Valid `share_type` values:** `public`, `auth_required`

**Indexes:**
- `idx_share_links_token` UNIQUE on `token`
- `idx_share_links_scene_user` on `(scene_id, user_id)`
- `idx_share_links_user_id` on `user_id`

**Constraints:**
- CHECK `share_type IN ('public', 'auth_required')`
- CHECK `chk_share_links_start` (`start_s >= 0`), `chk_share_links_range` (`end_s > start_s`, needs a start)
- FK to `scenes(id)` ON DELETE CASCADE
- FK to `users(id)` ON DELETE CASCADE

---

## Playlists

Playlists of scenes or of markers (migrations 046, 096; see `docs/MARKER_PLAYLISTS.md`). The
kind is fixed at creation; the items live in `playlist_scenes` or `playlist_markers`.

### `playlists`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `uuid` | UUID | NO | gen_random_uuid() | Public identifier (unique) |
| `user_id` | BIGINT | NO | - | Owner, FK to `users.id` (CASCADE) |
| `name` | VARCHAR(255) | NO | - | Name |
| `description` | TEXT | YES | NULL | Description |
| `visibility` | VARCHAR(20) | NO | 'private' | `private` or `public` |
| `kind` | VARCHAR(20) | NO | 'scenes' | `scenes` or `markers` (migration 096) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Indexes:** `idx_playlists_uuid` UNIQUE on `uuid`, `idx_playlists_user_id` on `user_id`,
`idx_playlists_visibility` on `visibility` WHERE `visibility = 'public'`, `idx_playlists_kind`
on `kind`.

**Constraints:** CHECK `visibility IN ('private', 'public')`, CHECK `chk_playlists_kind`:
`kind IN ('scenes', 'markers')`.

---

### `playlist_scenes`

Scenes of a scene playlist.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `playlist_id` | BIGINT | NO | - | FK to `playlists.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `position` | INT | NO | - | Order in the playlist |
| `added_at` | TIMESTAMPTZ | NO | NOW() | When the scene was added |

**Indexes:** UNIQUE `(playlist_id, scene_id)`, `(playlist_id)`, `(playlist_id, position)`.

---

### `playlist_markers`

Markers of a marker playlist (migration 096). A marker is in a playlist once; a deleted marker
leaves every playlist. Markers of trashed or deleted scenes are kept but not listed.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `playlist_id` | BIGINT | NO | - | FK to `playlists.id` (CASCADE) |
| `marker_id` | BIGINT | NO | - | FK to `user_scene_markers.id` (CASCADE) |
| `position` | INT | NO | - | Order in the playlist |
| `added_at` | TIMESTAMPTZ | NO | NOW() | When the marker was added |

**Indexes:** `idx_playlist_markers_unique` UNIQUE on `(playlist_id, marker_id)`,
`idx_playlist_markers_position` on `(playlist_id, position)`, `idx_playlist_markers_marker_id`
on `marker_id`.

---

### `playlist_tags`

Tags of a playlist (the `tags` table). UNIQUE `(playlist_id, tag_id)`, both FK CASCADE.

### `playlist_likes`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `playlist_id` | BIGINT | NO | - | FK to `playlists.id` (CASCADE) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Like timestamp |

**Indexes:** UNIQUE `(user_id, playlist_id)`.

### `playlist_progress`

Resume point of a user in a playlist (one row per user and playlist).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `playlist_id` | BIGINT | NO | - | FK to `playlists.id` (CASCADE) |
| `last_scene_id` | BIGINT | YES | NULL | Scene played, FK to `scenes.id` (SET NULL) |
| `last_marker_id` | BIGINT | YES | NULL | Marker played (marker playlists), FK to `user_scene_markers.id` (SET NULL; migration 096) |
| `last_position_s` | DOUBLE PRECISION | NO | 0 | Time in the scene, in seconds |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last save |

**Indexes:** UNIQUE `(user_id, playlist_id)` (upsert target).

---

## Social Graph

Follows, friendships, friend requests and privacy settings (migration 095, `docs/SOCIAL.md`),
presence (migration 098, `docs/PRESENCE.md`). Every table cascades on user deletion.

### `user_follows`

One-way follows (no approval). Two users following each other are "mutuals".

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `follower_id` | BIGINT | NO | - | FK to `users.id` (CASCADE), the user who follows |
| `followed_id` | BIGINT | NO | - | FK to `users.id` (CASCADE), the user followed |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Follow date |

**Indexes:**
- `idx_user_follows_followed_id` on `(followed_id, created_at DESC)` (followers of a user)

**Constraints:**
- PRIMARY KEY `(follower_id, followed_id)`
- CHECK `chk_user_follows_not_self`: `follower_id <> followed_id`

---

### `user_friendships`

Friendships, one row per pair with the smaller user id first.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_low_id` | BIGINT | NO | - | FK to `users.id` (CASCADE), the smaller id of the pair |
| `user_high_id` | BIGINT | NO | - | FK to `users.id` (CASCADE), the larger id |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Friendship date (request accepted) |

**Indexes:**
- `idx_user_friendships_user_high_id` on `user_high_id`

**Constraints:**
- PRIMARY KEY `(user_low_id, user_high_id)`
- CHECK `chk_user_friendships_ordered`: `user_low_id < user_high_id` (a pair is stored once)

**View `user_friends`:** `(user_id, friend_id, created_at)`, both directions of every
friendship (`UNION ALL` of the two orders): `SELECT friend_id FROM user_friends WHERE user_id
= ?` lists the friends of a user.

---

### `friend_requests`

Pending friend requests only: accepting, declining or cancelling deletes the row. A request
from B to A when A already asked B is an acceptance (application rule, under a per-pair
advisory lock).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `sender_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `receiver_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Request date |

**Indexes:**
- `idx_friend_requests_receiver_id` on `(receiver_id, created_at DESC)`

**Constraints:**
- `uni_friend_requests_pair` UNIQUE on `(sender_id, receiver_id)`
- CHECK `chk_friend_requests_not_self`: `sender_id <> receiver_id`

---

### `user_privacy_settings`

Explicit privacy choices: who can see an information item of the user. The items are declared
in code (`core/privacy_catalog.go`); no row = the item's catalog default (follows a change of
default), rows of unknown items are ignored.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `item` | VARCHAR(64) | NO | - | Privacy item key (`profile`, `friends_list`, `stats`...) |
| `audience` | VARCHAR(16) | NO | - | `public`, `followers`, `mutuals`, `friends` or `private` |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last change |

**Indexes:**
- `idx_user_privacy_settings_item` on `item`

**Constraints:**
- PRIMARY KEY `(user_id, item)`
- CHECK `chk_user_privacy_settings_audience`: `audience IN ('public', 'followers', 'mutuals', 'friends', 'private')`

---

### `user_presence`

What outlives the in-memory presence of a user (migration 098, `docs/PRESENCE.md`). The online
state itself is never stored. No row = never seen online, visible.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | Primary key, FK to `users.id` (CASCADE) |
| `last_seen_at` | TIMESTAMPTZ | YES | NULL | Last moment the user was seen online by the others: saved on disconnection (after the grace period), every `presence.last_seen_interval` while online and at shutdown; never moved back, not updated while invisible |
| `invisible` | BOOLEAN | NO | false | Invisible mode: the user appears offline to everyone |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last change |

---

### `user_social_settings`

Social preferences of a user. No row = the defaults.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | Primary key, FK to `users.id` (CASCADE) |
| `friend_request_policy` | VARCHAR(16) | NO | 'everyone' | Who may send friend requests: `everyone`, `mutuals`, `nobody` |
| `message_policy` | VARCHAR(16) | NO | 'friends' | Who may send messages (migration 102, `docs/MESSAGES.md`): `everyone`, `mutuals` (friends and mutuals), `friends`, `nobody` |
| `message_toasts` | BOOLEAN | NO | true | Show a toast for incoming messages (migration 102) |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last change |

**Constraints:**
- CHECK `chk_user_social_settings_friend_request_policy`: `friend_request_policy IN ('everyone', 'mutuals', 'nobody')`
- CHECK `chk_user_social_settings_message_policy`: `message_policy IN ('everyone', 'mutuals', 'friends', 'nobody')`

---

## Messages

Conversations between users and shares (migration 102, `docs/MESSAGES.md`), reactions on the
messages (migration 104). Only direct
(one-to-one) conversations exist; the participants table lets group conversations come later.
Messages are only read by the participants (no admin access).

### `conversations`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `kind` | VARCHAR(10) | NO | 'direct' | `direct` only for now |
| `direct_low_id` | BIGINT | YES | NULL | FK to `users.id` (SET NULL): smaller id of the pair of a direct conversation |
| `direct_high_id` | BIGINT | YES | NULL | FK to `users.id` (SET NULL): larger id |
| `last_message_at` | TIMESTAMPTZ | YES | NULL | Date of the last message (orders the lists) |
| `created_at`, `updated_at` | TIMESTAMPTZ | NO | NOW() | |

**Indexes:** `idx_conversations_direct_pair` UNIQUE on `(direct_low_id, direct_high_id)` WHERE `kind = 'direct'` (one conversation per pair; NULLs after a user deletion never conflict).
**Constraints:** CHECK `chk_conversations_kind` (`direct`), `chk_conversations_direct_pair` (`direct_low_id < direct_high_id`).

A conversation is created with its two participants on the first message. Once nobody takes
part in it anymore (both users deleted), the periodic cleanup deletes it.

### `conversation_participants`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `conversation_id` | BIGINT | NO | - | FK to `conversations.id` (CASCADE) |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `last_read_message_id` | BIGINT | YES | NULL | FK to `messages.id` (SET NULL): last message read (unread counts, "seen"); only moves forward |
| `muted` | BOOLEAN | NO | false | No header badge count nor toast |
| `joined_at` | TIMESTAMPTZ | NO | NOW() | |

**Indexes:** PRIMARY KEY `(conversation_id, user_id)`, `idx_conversation_participants_user_id`.

### `messages`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key (orders the history) |
| `conversation_id` | BIGINT | NO | - | FK to `conversations.id` (CASCADE) |
| `sender_id` | BIGINT | YES | NULL | FK to `users.id` (SET NULL): NULL = deleted user, the other side keeps the history |
| `body` | TEXT | NO | '' | Text (at most `messages.max_length` characters, enforced by the app) |
| `attachment_kind` | VARCHAR(16) | YES | NULL | What is shared: `scene`, `marker`, `playlist`, `lobby` |
| `attachment_scene_id` | BIGINT | YES | NULL | FK to `scenes.id` (SET NULL) |
| `attachment_marker_id` | BIGINT | YES | NULL | FK to `user_scene_markers.id` (SET NULL) |
| `attachment_playlist_id` | BIGINT | YES | NULL | FK to `playlists.id` (SET NULL) |
| `attachment_lobby_uuid` | UUID | YES | NULL | Lobby invited to (no FK: lobbies are deleted when they close) |
| `attachment_time_s` | DOUBLE PRECISION | YES | NULL | A scene shared at a time (seconds) |
| `edited_at` | TIMESTAMPTZ | YES | NULL | Last edit |
| `deleted_at` | TIMESTAMPTZ | YES | NULL | Soft delete: body and attachment cleared, shown as "message deleted" |
| `created_at` | TIMESTAMPTZ | NO | NOW() | |

**Indexes:** `idx_messages_conversation_id` on `(conversation_id, id DESC)` (history pages, unread counts), `idx_messages_sender_id`, `idx_messages_created_at` (retention).
**Constraints:** CHECK `chk_messages_attachment_kind`, `chk_messages_attachment_time` (`>= 0`), `chk_messages_body_length` (`<= 10000`).

The attachment stores references, resolved for each reader at read time: an item deleted later
leaves the kind with a NULL id ("no longer exists"). Retention: none by default
(`messages.retention_days`).

### `message_reactions`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `message_id` | BIGINT | NO | - | FK to `messages.id` (CASCADE) |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE): who reacted |
| `emoji` | VARCHAR(64) | NO | - | The key: a Unicode emoji, or `:name:` of a server emote (text: a deleted emote shows as its `:name:`) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Orders the users of a key, and the keys by their first reaction |

**Indexes:** PRIMARY KEY `(message_id, user_id, emoji)` (one reaction per user and key; also the lookups by message), `idx_message_reactions_user_id`.
**Constraints:** CHECK `chk_message_reactions_emoji` (`char_length(emoji) BETWEEN 1 AND 64`).

The soft delete of a message deletes its reactions (same transaction); the caps (distinct keys
per message, reactions per user) are checked by the application with the message row locked
(`SELECT ... FOR UPDATE`). Goon2gether chat reactions are not stored (in memory).

---

## User Profiles

Profile pages (migration 100, `docs/PROFILES.md`). Every table cascades on user deletion. Who
sees what is decided by the privacy settings (`user_privacy_settings`), not here.

### `user_profiles`

One row per user who edited their profile; no row = the defaults (no display name nor bio,
initials avatar, no banner, default layout).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | Primary key, FK to `users.id` (CASCADE) |
| `display_name` | VARCHAR(64) | NO | '' | Shown instead of the username ('' = the username) |
| `bio` | TEXT | NO | '' | Plain text (links rendered by the frontend), length capped by `profiles.bio_max_length` |
| `avatar_version` | BIGINT | NO | 0 | Unix ms of the avatar upload, appended to its URLs; 0 = no avatar. Files `<user_id>_<size>.jpg` in `profiles.image_dir/avatars` |
| `banner_type` | VARCHAR(16) | NO | 'none' | `none`, `scene` (thumbnail of `banner_scene_id`) or `upload` |
| `banner_scene_id` | BIGINT | YES | NULL | FK to `scenes.id` (SET NULL), for the `scene` banner |
| `banner_version` | BIGINT | NO | 0 | Version of the uploaded banner (files in `profiles.image_dir/banners`) |
| `layout` | JSONB | YES | NULL | Ordered blocks `[{type, visible, locked_placeholder, config}]`, validated by the server (`core/profile_blocks.go`); NULL = the default layout. Read only while `tabs` is NULL |
| `tabs` | JSONB | YES | NULL | Migration 103: tabs `[{id, kind, title, icon, slug, visible, sections: [{id, template, columns: [[block]]}]}]`, validated by the server (`core/profile_tabs.go`); NULL = derived from `layout` |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Row creation |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last change |

**Indexes:**
- `idx_user_profiles_banner_scene_id` on `banner_scene_id` WHERE NOT NULL

**Constraints:**
- CHECK `chk_user_profiles_banner_type`: `banner_type IN ('none', 'scene', 'upload')`
- CHECK `chk_user_profiles_bio_length`: `char_length(bio) <= 5000`
- CHECK `chk_user_profiles_layout_array`: `layout IS NULL OR jsonb_typeof(layout) = 'array'`
- CHECK `chk_user_profiles_tabs_array`: `tabs IS NULL OR jsonb_typeof(tabs) = 'array'`

---

### `user_profile_lists`

Custom lists of the profiles (migration 103, `docs/PROFILES.md`): search criteria run with the
owner's interactions. At most `profiles.max_lists` per user (application rule).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `title` | VARCHAR(80) | NO | - | 1-80 characters |
| `filters` | JSONB | NO | '{}' | The saved search filters shape (`data.Filters`), validated by the server |
| `saved_search_uuid` | UUID | YES | NULL | A saved search of the owner the list follows (its current filters while it exists); no FK, a missing one falls back to `filters` |
| `audience` | VARCHAR(16) | NO | 'public' | Who may see the list (besides the items its criteria depend on) |
| `figures` | VARCHAR(16) | NO | 'auto' | Owner figures under the cards (migration 105): `auto` (from the sort and filters), `none`, `jizz`, `rating`, `liked`, `views`; each one is sent only to viewers passing its privacy item |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Creation |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last change |

**Indexes:**
- `idx_user_profile_lists_user_id` on `user_id`

**Constraints:**
- CHECK `chk_user_profile_lists_audience`: `audience IN ('public', 'followers', 'mutuals', 'friends', 'private')`
- CHECK `chk_user_profile_lists_filters_object`: `jsonb_typeof(filters) = 'object'`
- CHECK `chk_user_profile_lists_title`: `char_length(title) BETWEEN 1 AND 80`
- CHECK `chk_user_profile_lists_figures`: `figures IN ('auto', 'none', 'jizz', 'rating', 'liked', 'views')`

---

### `user_profile_favorite_scenes`

The favorite videos showcase, ordered (`profiles.max_showcase_items` at most, application rule).
Trashed and deleted scenes stay stored but are skipped when read.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `position` | SMALLINT | NO | - | Order, 0 first |

**Indexes:**
- `idx_user_profile_favorite_scenes_scene_id` on `scene_id`

**Constraints:**
- PRIMARY KEY `(user_id, scene_id)`
- `uni_user_profile_favorite_scenes_position` UNIQUE on `(user_id, position)`
- CHECK `chk_user_profile_favorite_scenes_position`: `position >= 0`

---

### `user_profile_favorite_markers`

The favorite markers showcase, ordered: markers the user sees (their own or shared with them,
checked when saved and when read).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `marker_id` | INTEGER | NO | - | FK to `user_scene_markers.id` (CASCADE) |
| `position` | SMALLINT | NO | - | Order, 0 first |

**Indexes:**
- `idx_user_profile_favorite_markers_marker_id` on `marker_id`

**Constraints:**
- PRIMARY KEY `(user_id, marker_id)`
- `uni_user_profile_favorite_markers_position` UNIQUE on `(user_id, position)`
- CHECK `chk_user_profile_favorite_markers_position`: `position >= 0`

---

## User Interactions

### `user_scene_ratings`

User ratings for scenes (0.5 to 5.0 stars).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `rating` | DECIMAL(2,1) | NO | - | Rating value (0.5-5.0) |
| `created_at` | TIMESTAMP | NO | NOW() | Rating creation timestamp |
| `updated_at` | TIMESTAMP | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_user_scene_ratings_scene` on `scene_id`

**Constraints:**
- UNIQUE on `(user_id, scene_id)`
- CHECK `rating >= 0.5 AND rating <= 5.0`

---

### `user_scene_likes`

User likes/favorites for scenes.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `created_at` | TIMESTAMP | NO | NOW() | Like timestamp |

**Indexes:**
- `idx_user_scene_likes_scene` on `scene_id`

**Constraints:**
- UNIQUE on `(user_id, scene_id)`

---

### `user_scene_jizzed`

Tracks "jizz count" per user per scene.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `count` | INTEGER | NO | 0 | Number of times |
| `created_at` | TIMESTAMP | NO | NOW() | First occurrence timestamp |
| `updated_at` | TIMESTAMP | NO | NOW() | Last occurrence timestamp |

**Indexes:**
- `idx_user_scene_jizzed_scene` on `scene_id`

**Constraints:**
- UNIQUE on `(user_id, scene_id)`

---

### `user_scene_watches`

Watch history with position tracking for resume functionality.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `watched_at` | TIMESTAMPTZ | NO | NOW() | Watch session timestamp |
| `watch_duration` | INTEGER | NO | 0 | Duration watched (seconds) |
| `last_position` | INTEGER | NO | 0 | Last playback position (seconds) |
| `completed` | BOOLEAN | NO | false | Watched to completion flag |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_user_scene_watches_user_scene` on `(user_id, scene_id)`
- `idx_user_scene_watches_user_date` on `(user_id, watched_at DESC)`

---

### `user_scene_heatmaps`

"Most replayed" histogram per user and scene (migration 079). The scene is split into 100
equal buckets; each element of `buckets` is the watched time in milliseconds that fell into
that bucket, summed over every viewing. The player reports the ranges it actually played
(seeking over a part does not count it) and the server adds their overlap with each bucket in
a single upsert (element-wise sum, saturating at the INTEGER maximum). Bucket bounds use
`scenes.duration`, or the player's duration while it is unknown.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `buckets` | INTEGER[] | NO | - | Watched milliseconds per bucket (100 elements) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last report |

**Indexes:** primary key `(user_id, scene_id)`, `idx_user_scene_heatmaps_scene` on `scene_id`.

---

### `user_scene_view_counts`

Deduplication table for 24-hour view count increment logic.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `last_counted_at` | TIMESTAMPTZ | NO | NOW() | Last view count increment |
| `created_at` | TIMESTAMPTZ | NO | NOW() | First view timestamp |

**Indexes:**
- `idx_user_scene_view_counts_user_id` on `user_id`
- `idx_user_scene_view_counts_scene_id` on `scene_id`

**Constraints:**
- UNIQUE on `(user_id, scene_id)`

---

### `user_scene_jizz_events`

Dated "o" (jizz) events; `user_scene_jizzed.count` stays the per-scene total.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `jizzed_at` | TIMESTAMPTZ | NO | - | When it happened |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation |

**Indexes:** `(user_id, jizzed_at)`, `scene_id`. **Constraints:** UNIQUE `(user_id, scene_id, jizzed_at)`.

---

### `user_scene_markers`

Video bookmarks/markers created by users. A user sees their own markers plus those of the users with `users.share_markers`; only the owner can edit a marker.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `timestamp` | INTEGER | NO | - | Position in seconds |
| `end_timestamp` | INTEGER | YES | NULL | Optional end of the marked range in seconds (looped on click when `user_settings.marker_loop` is on; played up to it in a marker playlist) |
| `label_id` | BIGINT | YES | NULL | FK to `marker_labels.id` (SET NULL): the marker's label, NULL without label. Source of truth (migration 097) |
| `label` | VARCHAR(100) | NO | '' | Name of the label, a copy kept by the trigger `trg_user_scene_markers_sync_label` (`''` without label). A write of the text alone is resolved to the label of that name (ignoring case), created when missing |
| `color` | VARCHAR(7) | NO | '#FFFFFF' | Hex color code |
| `thumbnail_path` | VARCHAR(255) | YES | NULL | Generated thumbnail path |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Marker creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_user_scene_markers_user_scene` on `(user_id, scene_id)`
- `idx_user_scene_markers_timestamp` on `(scene_id, timestamp)`
- `idx_user_scene_markers_user_label` on `(user_id, label)` (named `idx_user_video_markers_user_label`)
- `idx_user_scene_markers_label_id` on `label_id`

**Constraints:**
- CHECK `timestamp >= 0`
- CHECK `chk_user_scene_markers_end_after_start`: `end_timestamp IS NULL OR end_timestamp > timestamp`

---

### `saved_marker_filters`

Named filters of the marker pages (advanced marker filters), per user.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `uuid` | UUID | NO | - | Public identifier (unique) |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `name` | VARCHAR(100) | NO | - | Display name |
| `filters` | JSONB | NO | '{}' | `labels`, `marker_tags`, `tags`, `actors`, `studio`, `ranges_only`, `sort`, `view` |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_saved_marker_filters_user_id` on `user_id`

---

### `marker_tags`

Tags associated with individual markers.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `marker_id` | BIGINT | NO | - | FK to `user_scene_markers.id` (CASCADE) |
| `tag_id` | BIGINT | NO | - | FK to `tags.id` (CASCADE) |
| `is_from_label` | BOOLEAN | NO | false | Inherited from label defaults |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Association timestamp |

**Indexes:**
- `idx_marker_tags_marker_id` on `marker_id`
- `idx_marker_tags_tag_id` on `tag_id`

**Constraints:**
- UNIQUE on `(marker_id, tag_id)`

---

### `marker_labels`

Server-wide marker labels (migration 097, `docs/MARKER_LABELS.md`), managed with
`marker_labels:manage`.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `name` | VARCHAR(100) | NO | - | Label name, trimmed, unique ignoring case |
| `color` | VARCHAR(7) | NO | '#FFFFFF' | Hex color |
| `description` | TEXT | NO | '' | Optional description |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_marker_labels_name_lower` UNIQUE on `lower(name)`

**Constraints:**
- CHECK `chk_marker_labels_name_not_blank`: `btrim(name) <> ''`

**Triggers:** `trg_marker_labels_propagate_name` (AFTER UPDATE OF name) renames the
`user_scene_markers.label` copy of the label's markers.

---

### `marker_label_tags`

Default tags of the marker labels, the same for every user (migration 097; per user before).
Every marker of the label carries them as `marker_tags.is_from_label = true`.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `label_id` | BIGINT | NO | - | FK to `marker_labels.id` (CASCADE) |
| `tag_id` | BIGINT | NO | - | FK to `tags.id` (CASCADE) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Association timestamp |

**Indexes:**
- `idx_marker_label_tags_tag_id` on `tag_id`

**Constraints:**
- UNIQUE `uq_marker_label_tags_label_tag` on `(label_id, tag_id)`

---

### `user_actor_ratings`

User ratings for actors (0.5 to 5.0 stars).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `actor_id` | BIGINT | NO | - | FK to `actors.id` (CASCADE) |
| `rating` | DECIMAL(2,1) | NO | - | Rating value (0.5-5.0) |
| `created_at` | TIMESTAMP | NO | NOW() | Rating creation timestamp |
| `updated_at` | TIMESTAMP | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_user_actor_ratings_actor` on `actor_id`

**Constraints:**
- UNIQUE on `(user_id, actor_id)`
- CHECK `rating >= 0.5 AND rating <= 5.0`

---

### `user_actor_likes`

User likes/favorites for actors.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `actor_id` | BIGINT | NO | - | FK to `actors.id` (CASCADE) |
| `created_at` | TIMESTAMP | NO | NOW() | Like timestamp |

**Indexes:**
- `idx_user_actor_likes_actor` on `actor_id`

**Constraints:**
- UNIQUE on `(user_id, actor_id)`

---

### `user_studio_ratings`

User ratings for studios (0.5 to 5.0 stars).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `studio_id` | BIGINT | NO | - | FK to `studios.id` (CASCADE) |
| `rating` | DECIMAL(2,1) | NO | - | Rating value (0.5-5.0) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Rating creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_user_studio_ratings_studio` on `studio_id`

**Constraints:**
- UNIQUE on `(user_id, studio_id)`
- CHECK `rating >= 0.5 AND rating <= 5.0`

---

### `user_studio_likes`

User likes/favorites for studios.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `studio_id` | BIGINT | NO | - | FK to `studios.id` (CASCADE) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Like timestamp |

**Indexes:**
- `idx_user_studio_likes_studio` on `studio_id`

**Constraints:**
- UNIQUE on `(user_id, studio_id)`

---

### `saved_searches`

User-saved search filter configurations.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `uuid` | UUID | NO | gen_random_uuid() | Public identifier |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `name` | VARCHAR(255) | NO | - | Saved search name |
| `filters` | JSONB | NO | '{}' | Search filter configuration |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_saved_searches_uuid` UNIQUE on `uuid`
- `idx_saved_searches_user_id` on `user_id`

---

## Job System

### `job_history`

Processing job records for the worker pool system.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `job_id` | VARCHAR(36) | NO | - | UUID job identifier |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` |
| `scene_title` | VARCHAR(255) | NO | '' | Scene title at job creation |
| `phase` | VARCHAR(20) | NO | - | Processing phase |
| `status` | VARCHAR(20) | NO | 'running' | Job status |
| `priority` | INTEGER | NO | 0 | Job priority (higher = first) |
| `error_message` | TEXT | YES | NULL | Error details if failed |
| `progress` | INTEGER | NO | 0 | Progress percentage (0-100) |
| `retry_count` | INTEGER | NO | 0 | Number of retries attempted |
| `max_retries` | INTEGER | NO | 0 | Maximum retries allowed |
| `next_retry_at` | TIMESTAMPTZ | YES | NULL | Scheduled retry time |
| `is_retryable` | BOOLEAN | NO | true | Whether job can be retried |
| `started_at` | TIMESTAMPTZ | NO | NOW() | Job start timestamp |
| `completed_at` | TIMESTAMPTZ | YES | NULL | Job completion timestamp |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `worker_id` | BIGINT | YES | NULL | FK to `workers.id` (ON DELETE SET NULL): remote worker holding (or that held) the lease; NULL = local job (migration 088) |
| `lease_generation` | BIGINT | NO | 0 | Fencing token of the remote lease: + 1 on every claim, expiry, release and cancel of a remote lease |
| `lease_expires_at` | TIMESTAMPTZ | YES | NULL | Remote lease expiry (renewed by heartbeats) |
| `last_heartbeat_at` | TIMESTAMPTZ | YES | NULL | Last heartbeat of the lease (set at claim) |
| `committed_generation` | BIGINT | YES | NULL | Generation whose complete was committed (idempotent complete) |
| `commit_started_generation` | BIGINT | YES | NULL | Generation whose commit started (fingerprint indexed at most once) |
| `input_mode` | VARCHAR(10) | YES | NULL | `direct` or `indirect`, reported by the worker |
| `worker_stage` | VARCHAR(20) | YES | NULL | `downloading`, `computing`, `uploading`, `completing`, reported by the worker |
| `lease_params` | JSONB | YES | NULL | Phase parameters prepared at claim; the complete is validated against them |
| `startup_requeues` | INTEGER | NO | 0 | Times the job was put back to pending at startup after an interruption while it ran (migration 092) |

**Valid `phase` values:** `metadata`, `thumbnail`, `sprites`, `animated_thumbnails`, `fingerprint`, `scan`

**Valid `status` values:** `pending`, `running`, `completed`, `failed`

**Indexes:**
- `idx_job_history_job_id` UNIQUE on `job_id`
- `idx_job_history_started_at` on `started_at`
- `idx_job_history_status` on `status`
- `idx_job_history_next_retry` on `next_retry_at` WHERE next_retry_at IS NOT NULL AND status = 'failed'
- `idx_job_history_pending_poll` on `(phase, priority DESC, created_at ASC)` WHERE status = 'pending'
- `idx_job_history_scene_phase_active` UNIQUE on `(scene_id, phase)` WHERE status IN ('pending', 'running')
- `idx_job_history_remote_lease_expiry` on `lease_expires_at` WHERE status = 'running' AND worker_id IS NOT NULL (expiry sweep)
- `idx_job_history_worker_id` on `(worker_id, status)` WHERE worker_id IS NOT NULL
- `idx_job_history_scene_phase_created` on `(scene_id, phase, created_at)` (latest job of a (scene, phase), startup re-queue)

**Startup re-queue (migration 092):** at startup, after the orphan and stuck pending recovery, the
failed jobs the previous run interrupted (`error_message` "Job interrupted by server shutdown",
"Orphaned job recovered after server restart" or "Stuck pending job recovered after server
restart") go back to `pending`, with their retry count, priority and `created_at` kept. Only the
latest job of its (scene, phase), never when the pair has an active job, never when
`is_retryable` is false (retried by hand) or the scene is deleted or in the trash. A job that ran
is put back once (`startup_requeues`): interrupted again, it stays failed. During the shutdown, the
jobs that fail or are cancelled are recorded with the shutdown message.

The attempt of a job (remote workers) is its `retry_count`. See `docs/REMOTE_WORKERS.md`
section 6 for the lease state machine.

---

### `workers`

Remote workers (goonhub-worker), migration 088. The token itself is never stored.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key (part of the token `ghw_<id>_<secret>`) |
| `name` | VARCHAR(100) | NO | - | Display name |
| `token_hash` | VARCHAR(64) | YES | NULL | sha256 hex of the token; NULL = revoked |
| `enabled` | BOOLEAN | NO | true | Disabled workers are refused (403) |
| `draining` | BOOLEAN | NO | false | Admin asked the worker to stop claiming |
| `connection_mode` | VARCHAR(10) | NO | 'pull' | `pull` or `push` (CHECK) |
| `push_url` | VARCHAR(500) | NO | '' | Push endpoint of the worker |
| `push_token` | VARCHAR(200) | NO | '' | Secret the server sends to the push worker (given by the worker at register) |
| `session_id` | VARCHAR(64) | NO | '' | Current session (register) |
| `server_url` | VARCHAR(500) | NO | '' | Base URL the worker used (signed media URLs) |
| `capabilities` | JSONB | NO | '{}' | `workerproto.Capabilities` (phases, slots, input modes, accel...) |
| `path_mappings` | JSONB | NO | '[]' | Verified server prefix -> local mount mappings |
| `allowed_phases` | TEXT[] | NO | all three | Phases the admin allows |
| `max_concurrency` | INTEGER | NO | 0 | Jobs held at once across phases (0 = the worker's own slots, CHECK >= 0) |
| `version`, `hostname`, `os`, `arch` | VARCHAR | NO | '' | Reported at register |
| `free_disk_bytes` | BIGINT | NO | 0 | Reported by heartbeats |
| `worker_draining` | BOOLEAN | NO | false | The worker said it is shutting down |
| `last_seen_at` | TIMESTAMPTZ | YES | NULL | Last call or push status (online/offline is computed from it) |
| `registered_at` | TIMESTAMPTZ | YES | NULL | Last register |
| `jobs_completed`, `jobs_failed`, `leases_expired`, `leases_released` | BIGINT | NO | 0 | Counters |
| `last_error` | TEXT | NO | '' | Last failure reported or push error |
| `last_error_at` | TIMESTAMPTZ | YES | NULL | |
| `created_at`, `updated_at` | TIMESTAMPTZ | NO | NOW() | |

**Indexes:**
- `idx_workers_token_hash` UNIQUE on `token_hash` WHERE token_hash IS NOT NULL

---

### `worker_settings`

Remote worker settings (singleton table, migration 088).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | INTEGER | NO | 1 | Primary key (always 1, CHECK) |
| `media_secret` | VARCHAR(128) | NO | '' | HMAC secret of the signed media URLs, generated at the first start (unless `workers.media_secret` is set) |
| `phase_routing` | JSONB | NO | '{}' | Admin routing overrides: `{"sprites": "local" \| "remote" \| "both"}` |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | |

---

### `scene_phase_failures`

Consecutive failures of a processing phase on a scene (migration 092). A job failure of the
phase adds one (shutdown interruptions and cancellations do not count), a success deletes the
row. When the file changed (size or modification time) since the last failure, the counter
restarts at 1. A (scene, phase) is **failing** when `failures` >= `processing.failing_threshold`
(default 3, 0 = never) and the file still has `file_size` and `file_mod_time`: the "missing" bulk
runs (unless `include_failing`) and the automatic retries skip it. Listed, retried and cleared in
Settings > Jobs > Failing (`/api/v1/admin/jobs/failing`). Bookkeeping only, no file is touched.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (ON DELETE CASCADE) |
| `phase` | VARCHAR(30) | NO | - | Processing phase |
| `failures` | INTEGER | NO | 1 | Failures in a row |
| `last_error` | TEXT | NO | '' | Error of the last failure |
| `last_failed_at` | TIMESTAMPTZ | NO | NOW() | Date of the last failure |
| `file_size` | BIGINT | YES | NULL | File size at the last failure (NULL: file unreadable) |
| `file_mod_time` | TIMESTAMPTZ | YES | NULL | File modification time at the last failure (microseconds) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | First failure of the streak |

**Indexes:**
- `uq_scene_phase_failures_scene_phase` UNIQUE on `(scene_id, phase)`
- `idx_scene_phase_failures_phase_failures` on `(phase, failures)`

---

### `dead_letter_queue`

Failed jobs that exceeded retry limits for manual review.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `job_id` | VARCHAR(36) | NO | - | Original job UUID |
| `scene_id` | BIGINT | NO | - | Scene ID |
| `scene_title` | VARCHAR(255) | NO | '' | Scene title |
| `phase` | VARCHAR(20) | NO | - | Processing phase |
| `original_error` | TEXT | NO | - | First failure error message |
| `failure_count` | INTEGER | NO | 1 | Total failure count |
| `last_error` | TEXT | NO | - | Most recent error message |
| `status` | VARCHAR(20) | NO | 'pending_review' | DLQ entry status |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Entry creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `abandoned_at` | TIMESTAMPTZ | YES | NULL | When marked abandoned |

**Valid `status` values:** `pending_review`, `retrying`, `abandoned`

**Indexes:**
- UNIQUE on `job_id`
- `idx_dlq_status` on `status`
- `idx_dlq_scene_id` on `scene_id`

---

### `pool_config`

Worker pool configuration (singleton table).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | INTEGER | NO | 1 | Primary key (always 1) |
| `metadata_workers` | INTEGER | NO | 3 | Metadata extraction workers |
| `thumbnail_workers` | INTEGER | NO | 1 | Thumbnail generation workers |
| `sprites_workers` | INTEGER | NO | 1 | Sprite sheet generation workers |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Constraints:**
- CHECK `id = 1` (singleton enforcement)

---

### `processing_config`

Processing quality settings (singleton table).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | INTEGER | NO | 1 | Primary key (always 1) |
| `max_frame_dimension_sm` | INTEGER | NO | 320 | Small thumbnail max dimension |
| `max_frame_dimension_lg` | INTEGER | NO | 1280 | Large thumbnail max dimension |
| `frame_quality_sm` | INTEGER | NO | 85 | Small thumbnail JPEG quality |
| `frame_quality_lg` | INTEGER | NO | 85 | Large thumbnail JPEG quality |
| `frame_quality_sprites` | INTEGER | NO | 75 | Sprite sheet JPEG quality |
| `sprites_concurrency` | INTEGER | NO | 0 | Parallel sprite generation (0=auto) |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Constraints:**
- CHECK `id = 1` (singleton enforcement)

---

### `trigger_config`

Job trigger configuration (when phases execute).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `phase` | VARCHAR(20) | NO | - | Processing phase |
| `trigger_type` | VARCHAR(20) | NO | 'on_import' | Trigger type |
| `after_phase` | VARCHAR(20) | YES | NULL | Phase to trigger after |
| `cron_expression` | VARCHAR(100) | YES | NULL | Cron schedule (if scheduled) |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Valid `phase` values:** `metadata`, `thumbnail`, `sprites`, `scan`

**Valid `trigger_type` values:** `on_import`, `after_job`, `manual`, `scheduled`

**Constraints:**
- UNIQUE on `phase`
- CHECK `phase IN ('metadata', 'thumbnail', 'sprites', 'scan')`
- CHECK `trigger_type IN ('on_import', 'after_job', 'manual', 'scheduled')`
- CHECK `after_phase IS NULL OR after_phase IN ('metadata', 'thumbnail', 'sprites')`
- CHECK `(trigger_type = 'after_job' AND after_phase IS NOT NULL) OR (trigger_type != 'after_job')`
- CHECK `(trigger_type = 'scheduled' AND cron_expression IS NOT NULL) OR (trigger_type != 'scheduled')`
- CHECK `phase != after_phase` (no self-reference)

**Default Configuration:**
| Phase | Trigger Type | After Phase |
|-------|--------------|-------------|
| metadata | on_import | NULL |
| thumbnail | after_job | metadata |
| sprites | manual | NULL |
| scan | manual | NULL |

---

### `retry_config`

Per-phase retry policy configuration.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `phase` | VARCHAR(20) | NO | - | Processing phase |
| `max_retries` | INTEGER | NO | 3 | Maximum retry attempts |
| `initial_delay_seconds` | INTEGER | NO | 30 | First retry delay |
| `max_delay_seconds` | INTEGER | NO | 3600 | Maximum retry delay |
| `backoff_factor` | DECIMAL(3,1) | NO | 2.0 | Exponential backoff multiplier |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Constraints:**
- UNIQUE on `phase`
- CHECK `phase IN ('metadata', 'thumbnail', 'sprites', 'scan')`

**Default Configuration:**
| Phase | Max Retries | Initial Delay | Max Delay | Backoff |
|-------|-------------|---------------|-----------|---------|
| metadata | 3 | 30s | 3600s | 2.0 |
| thumbnail | 3 | 60s | 3600s | 2.0 |
| sprites | 2 | 120s | 7200s | 2.0 |
| scan | 3 | 60s | 3600s | 2.0 |

---

### `job_schedule_config`

Global job scheduling settings (singleton table). Scheduling only decides when pending jobs start; job creation is not affected.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | INTEGER | NO | 1 | Primary key (always 1) |
| `timezone` | VARCHAR(64) | NO | 'UTC' | IANA timezone of the time windows and quiet hours |
| `quiet_hours_start` | VARCHAR(5) | NO | '01:00' | Quiet hours start (`HH:MM`) |
| `quiet_hours_end` | VARCHAR(5) | NO | '07:00' | Quiet hours end (`HH:MM`, before the start = crosses midnight) |
| `max_concurrent_jobs` | INTEGER | NO | 0 | Maximum jobs running at once across all phases (0 = no limit) |
| `new_scenes_first` | BOOLEAN | NO | false | Claim the jobs of recently created scenes first |
| `new_scenes_window_hours` | INTEGER | NO | 24 | What "recently created" means (1-720 hours, on `scenes.created_at`) |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Constraints:**
- CHECK `id = 1` (singleton enforcement)
- CHECK `quiet_hours_start`, `quiet_hours_end` match `HH:MM` (00:00-23:59)
- CHECK `max_concurrent_jobs >= 0`
- CHECK `new_scenes_window_hours BETWEEN 1 AND 720`

---

### `job_phase_schedules`

Per-phase job scheduling settings.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `phase` | VARCHAR(20) | NO | - | Processing phase |
| `priority` | INTEGER | NO | 0 | 0-100, higher gets the next free slot when `max_concurrent_jobs` is reached |
| `window_enabled` | BOOLEAN | NO | false | Restrict the phase to its daily window |
| `window_start` | VARCHAR(5) | NO | '01:00' | Window start (`HH:MM`) |
| `window_end` | VARCHAR(5) | NO | '07:00' | Window end (`HH:MM`, before the start = crosses midnight) |
| `quiet_workers` | INTEGER | NO | 0 | Pool size during the quiet hours (0 = the `pool_config` size) |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Constraints:**
- UNIQUE on `phase`
- CHECK `phase IN ('metadata', 'thumbnail', 'sprites', 'animated_thumbnails', 'fingerprint')`
- CHECK `priority BETWEEN 0 AND 100`
- CHECK `window_start`, `window_end` match `HH:MM`, and `window_start <> window_end`
- CHECK `quiet_workers BETWEEN 0 AND 32`

**Default Configuration:** no window, no quiet hours size; priorities metadata 50, thumbnail 40, animated_thumbnails 30, sprites 20, fingerprint 10.

---

## Storage & Scanning

### `storage_paths`

Configured storage locations for video files.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `name` | VARCHAR(100) | NO | - | Display name |
| `path` | VARCHAR(500) | NO | - | Filesystem path |
| `is_default` | BOOLEAN | NO | false | Default storage flag |
| `read_only` | BOOLEAN | NO | false | GoonHub never deletes, moves, renames or writes media files under this path (migration 075) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Read-only paths:** a scene's file is protected when its `storage_path_id` points to a read-only path or when its `stored_path` lies under one (prefix check). Permanently deleting such a scene (trash, empty trash, explorer bulk delete, resolved duplicates) deletes the row and the generated files (thumbnails, sprites, VTT) but keeps the video file, so the next scan adds it again. Uploads into a read-only path are refused. The decision is made in one place, `core.MediaFileGuard`.

**Indexes:**
- `idx_storage_paths_single_default` UNIQUE on `is_default` WHERE is_default = TRUE

**Constraints:**
- UNIQUE on `path`

**Default Data:**
- `Default` at `./data/videos` (is_default=true)

---

### `scan_history`

File scan operation records.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `status` | VARCHAR(20) | NO | 'running' | Scan status |
| `started_at` | TIMESTAMPTZ | NO | NOW() | Scan start timestamp |
| `completed_at` | TIMESTAMPTZ | YES | NULL | Scan completion timestamp |
| `paths_scanned` | INT | NO | 0 | Number of paths scanned |
| `files_found` | INT | NO | 0 | Total files discovered |
| `videos_added` | INT | NO | 0 | New scenes added |
| `videos_skipped` | INT | NO | 0 | Duplicate files skipped |
| `videos_removed` | INT | NO | 0 | Missing files removed |
| `videos_moved` | INT | NO | 0 | Relocated files updated |
| `copies_found` | INT | NO | 0 | Files recognised as copies of an existing scene, new or already recorded in `scene_copies` (migration 091) |
| `errors` | INT | NO | 0 | Error count |
| `error_message` | TEXT | YES | NULL | Error details |
| `current_path` | VARCHAR(500) | YES | NULL | Currently scanning path |
| `current_file` | VARCHAR(500) | YES | NULL | Currently processing file |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |

**Valid `status` values:** `running`, `completed`, `failed`

**Indexes:**
- `idx_scan_history_status` on `status`
- `idx_scan_history_started_at` on `started_at DESC`

---

### `scene_copies`

Files with the same content as a scene's file at another path (migration 091). A Stash import
kept one file per scene, so the other files of a Stash scene were seen as new videos by the
library scan; the scan now records such a file here instead of creating a scene for it.

How the scan uses it (`ScanService.runScan`):
1. A path that is a scene's `stored_path` is skipped; a path recorded here is skipped too and
   counted in `scan_history.copies_found` (its `last_seen_at` is updated in one batched
   `UPDATE` at the end of the scan).
2. Move detection (same size + same filename, original path gone) keeps priority.
3. Otherwise, when live scenes (not trashed, not soft-deleted) have exactly the same size, the
   oshash of the new file and of each candidate's file is computed (each file at most once per
   scan); on a match the file is recorded here for that scene. A candidate whose file is missing
   or unreadable is never matched: the file becomes a scene as before. A file identical to a
   scene created earlier in the same scan becomes a copy of that scene.

Rows are bookkeeping only: GoonHub never deletes, moves or writes the file of a copy. Trashing a
scene keeps its rows (the copies stay skipped); permanently deleting it removes them through the
cascade, never the files, so the next scan sees those files as new videos again. Listed
read-only by `GET /api/v1/scenes/:id/copies`.

Triage (Duplicates page > Copies, `/api/v1/admin/duplicates/copies...`, `SceneCopyService`):
- **Make main file**: in one transaction the scene takes the copy's `stored_path` and storage
  path (size kept, a scene marked missing is restored) and the copy row takes the old main
  path (same `oshash`). The scene ID does not change; the scene is re-indexed.
- **Separate scene**: the copy row is deleted and a scene is created for the file (as the scan
  creates scenes), then indexed and submitted for processing.
- **Forget**: the row is deleted; the file stays (the next scan records it again).
- **Delete file**: the only action that deletes a file. Refused for a read-only storage path
  (`MediaFileGuard`), for the scene's main file (same path or same file on disk), and unless
  the copy and the scene's main file still have the same oshash, both re-read right before
  (an unreadable main file refuses). Then the file is removed through
  `MediaFileGuard.RemoveMediaFile` and the row deleted. Bulk forget/delete: 500 copies max,
  per-copy results.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE), the scene the file is a copy of |
| `stored_path` | TEXT | NO | - | Full filesystem path of the copy |
| `storage_path_id` | INTEGER | YES | NULL | FK to `storage_paths.id` (SET NULL), the storage path the copy was found under |
| `size` | BIGINT | NO | - | File size in bytes |
| `oshash` | VARCHAR(16) | NO | - | OpenSubtitles hash as Stash computes it: 64-bit sum of the size and of the little-endian uint64 words of the first and last 64 KiB, 16 hex digits (`pkg/oshash`) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | When the scan recorded the copy |
| `last_seen_at` | TIMESTAMPTZ | NO | NOW() | Last scan that saw the file |

**Indexes:**
- `idx_scene_copies_scene_id` on `scene_id`

**Constraints:**
- UNIQUE `uq_scene_copies_stored_path` on `stored_path` (a path is recorded once)

---

### `library_health_checks`

Runs of the background tasks of the admin library health page (migration 077): the
missing-file check (`missing_files`, stats every active scene file) and the size fix
(`fix_sizes`, re-reads from disk the size of scenes stored with size 0). Both only read the
media files.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `kind` | VARCHAR(30) | NO | - | `missing_files` or `fix_sizes` |
| `status` | VARCHAR(20) | NO | 'running' | Task status |
| `started_at` | TIMESTAMPTZ | NO | NOW() | Start timestamp |
| `completed_at` | TIMESTAMPTZ | YES | NULL | End timestamp |
| `total` | INT | NO | 0 | Scenes to look at |
| `checked` | INT | NO | 0 | Scenes looked at so far |
| `missing` | INT | NO | 0 | Files that do not exist |
| `unreadable` | INT | NO | 0 | Files that could not be stat'ed (permission, I/O error, not a regular file) |
| `fixed` | INT | NO | 0 | `fix_sizes`: scenes whose size was updated |
| `unchanged` | INT | NO | 0 | `fix_sizes`: files found but still 0 bytes, or scene changed meanwhile |
| `error_message` | TEXT | YES | NULL | Failure details |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |

**Valid `status` values:** `running`, `completed`, `cancelled`, `failed` (a task left
`running` by a restart is marked `failed` on startup)

**Indexes:**
- `idx_library_health_checks_kind_started` on `(kind, started_at DESC)`

**Constraints:**
- CHECK `kind IN ('missing_files', 'fix_sizes')`
- CHECK `status IN ('running', 'completed', 'cancelled', 'failed')`

---

### `library_health_check_items`

Files a missing-file check found missing or unreadable. Only the items of the latest
completed check are kept: completing a check drops the items of the earlier ones, and a
cancelled or failed check drops its own partial items. The health page only lists the items
whose scene is still active and still at the checked path.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `check_id` | BIGINT | NO | - | FK to library_health_checks (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to scenes (CASCADE) |
| `stored_path` | TEXT | NO | - | Path that was checked |
| `reason` | VARCHAR(20) | NO | - | `missing` or `unreadable` |
| `detail` | TEXT | NO | '' | Error text for `unreadable` |

**Indexes:**
- `idx_library_health_check_items_check_reason` on `(check_id, reason, scene_id)`
- `idx_library_health_check_items_scene_id` on `scene_id`

**Constraints:**
- CHECK `reason IN ('missing', 'unreadable')`

---

## Duplicate Detection

Scenes whose audio (chromaprint) or visual (dHash) fingerprints match are grouped; the
fingerprints themselves live in ClickHouse. A scene belongs to at most one group.

### `duplicate_groups`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `status` | VARCHAR(20) | NO | 'unresolved' | `unresolved`, `resolved` or `dismissed` |
| `scene_count` | INTEGER | NO | 0 | Number of members |
| `best_scene_id` | BIGINT | YES | NULL | FK to scenes (SET NULL): the variant kept at resolution |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `resolved_at` | TIMESTAMPTZ | YES | NULL | When the group was resolved |

**Indexes:**
- `idx_duplicate_groups_status` on `status`

---

### `duplicate_group_members`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `group_id` | BIGINT | NO | - | FK to duplicate_groups (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to scenes (CASCADE) |
| `is_best` | BOOLEAN | NO | false | Member chosen as the variant to keep |
| `confidence_score` | DOUBLE PRECISION | NO | 0 | Match confidence |
| `match_type` | VARCHAR(10) | NO | 'audio' | `audio` or `visual` |
| `match_start`, `match_end` | REAL | YES | NULL | Matching segment in this scene, seconds (migration 066). Migration 089 cleared the segments a bug stored as "0 to one fingerprint step"; members without segment are recomputed at startup from the stored fingerprints (`MatchingService.BackfillMemberSegments`) |
| `ref_scene_id` | BIGINT | YES | NULL | FK to scenes (SET NULL): the scene this member was matched against |
| `ref_start`, `ref_end` | REAL | YES | NULL | Corresponding segment in `ref_scene_id`, seconds |
| `relation` | VARCHAR(10) | NO | 'unknown' | Relation to `ref_scene_id` (migration 085, see below) |
| `delete_after` | TIMESTAMPTZ | YES | NULL | Scheduled permanent deletion of a scene not kept at resolution |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |

**Relation (`relation`):** derived from the two segments and the scene durations by
`core.ClassifyDuplicateRelation`. A scene is "mostly covered" by the match when the match spans
at least 90 % of it, or leaves at most 45 s of it out while spanning at least 75 % (intros,
outros, trimmed ends). Values: `identical` (both scenes mostly covered), `contained` (this scene
is mostly covered, the reference is not: this scene is a clip of it), `contains` (the reference
is mostly covered, this scene is not: e.g. a compilation), `partial` (neither) and `unknown` (no
segment, a missing duration or an empty segment). It is written with the segment at fingerprint
time; rows that have a segment but are still `unknown` (recorded before migration 085, or whose
durations were missing) are classified by the application at startup.

**Group kind (computed, not stored):** `identical` when every member is identical, else
`containment` when a member is `contained`/`contains`, else `partial`, else `unverified`. Only
`identical` groups are resolved by the automatic actions (auto-select best, bulk auto-resolve);
the others are resolved by hand.

**Indexes:**
- `idx_duplicate_group_members_unique_scene` UNIQUE on `scene_id` (one group per scene)
- `idx_duplicate_group_members_group_id` on `group_id`
- `idx_duplicate_group_members_scene_id` on `scene_id`
- `idx_duplicate_group_members_delete_after` on `delete_after` WHERE delete_after IS NOT NULL
- `idx_duplicate_group_members_group_relation` on `(group_id, relation)`

**Constraints:**
- UNIQUE on `(group_id, scene_id)`
- CHECK `relation IN ('identical', 'contained', 'contains', 'partial', 'unknown')`

---

### `duplicate_exclusions`

Pairs of scenes marked as **not duplicates** of each other (migration 090). Written by
`POST /admin/duplicates/:id/exclude`: each scene removed from the group gets a pair with each
member that stays. Fingerprint matching (`MatchingService.ProcessMatches`) ignores a match
between the two scenes of a pair, and does not add to a group a new scene excluded with one of
its members, so a later fingerprint run never puts them back together; the segment backfill
(`BackfillMemberSegments`) never takes its segment from an excluded scene. Two scenes removed
from the same group have no pair between them and can group again.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `scene_id_a` | BIGINT | NO | - | FK to scenes (CASCADE), the lower scene ID of the pair |
| `scene_id_b` | BIGINT | NO | - | FK to scenes (CASCADE), the higher scene ID of the pair |
| `created_at` | TIMESTAMPTZ | NO | NOW() | When the pair was recorded |

**Indexes:**
- PRIMARY KEY `(scene_id_a, scene_id_b)` (a pair is stored once)
- `idx_duplicate_exclusions_scene_id_b` on `scene_id_b`

**Constraints:**
- CHECK `chk_duplicate_exclusions_ordered`: `scene_id_a < scene_id_b`

---

## Goon2gether

Synchronized watch lobbies and custom emotes (migrations 099, 101 and 107, `docs/GOON2GETHER.md`). The live
state (connections, buffering, holds, chat) is in memory; these tables keep what must survive a
restart.

### `lobbies`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `uuid` | UUID | NO | - | Public identifier (unique) |
| `invite_code` | VARCHAR(16) | NO | - | Code of the invite link `/together/join/<code>` (unique, 8 characters without ambiguous ones; can be changed by the host) |
| `name` | VARCHAR(100) | NO | - | Display name |
| `visibility` | VARCHAR(10) | NO | 'private' | `public` (listed) or `private` (invite only) |
| `host_user_id` | BIGINT | YES | NULL | FK to `users.id` (SET NULL): the host (handed over when the host leaves or is away) |
| `created_by` | BIGINT | YES | NULL | FK to `users.id` (SET NULL) |
| `control_open` | BOOLEAN | NO | FALSE | Every member controls playback and the queue |
| `queue_open` | BOOLEAN | NO | FALSE | Every member adds to the queue (else suggestions) |
| `wait_for_all` | BOOLEAN | NO | FALSE | Hold playback while members buffer |
| `max_members` | INTEGER | NO | 0 | Most online members (0 = `goon2gether.max_members`) |
| `current_item_id` | BIGINT | YES | NULL | FK to `lobby_queue_items.id` (SET NULL): item playing, last saved |
| `position_s` | DOUBLE PRECISION | NO | 0 | Last saved position in the current item (seconds) |
| `playback_rate` | DOUBLE PRECISION | NO | 1 | Last saved playback rate |
| `auto_next` | VARCHAR(16) | NO | 'next' | What happens at the end of an item: `stop`, `next`, `repeat_one`, `repeat_queue` (migration 101) |
| `anon_seq` | INTEGER | NO | 0 | Last anonymous number given in the lobby ("Anonymous #N", never reused; migration 101) |
| `play_started_at` | TIMESTAMPTZ | YES | NULL | When the current item first played (NULL: not yet), saved with the playback; its history entry is written when the item changes (migration 107) |
| `play_completed` | BOOLEAN | NO | FALSE | The current item reached its end at least once (history outcome; migration 107) |
| `last_active_at` | TIMESTAMPTZ | NO | NOW() | Last time someone was connected; unloaded lobbies with nobody connected for `goon2gether.empty_close_after` (else `idle_timeout`) are deleted, also after a restart |
| `created_at`, `updated_at` | TIMESTAMPTZ | NO | NOW() | |

**Indexes:** `idx_lobbies_uuid` UNIQUE, `idx_lobbies_invite_code` UNIQUE, `idx_lobbies_public_active` on `last_active_at DESC` WHERE `visibility = 'public'`, `idx_lobbies_host_user_id`, `idx_lobbies_last_active_at`.

**Constraints:** CHECK `chk_lobbies_visibility` (`public`, `private`), `chk_lobbies_max_members` (`>= 0`), `chk_lobbies_playback_rate` (`> 0`), `chk_lobbies_auto_next`, `chk_lobbies_anon_seq` (`>= 0`); `fk_lobbies_current_item` ON DELETE SET NULL.

A closed lobby (by its host, a moderator, or automatically for inactivity) is deleted with its
members, queue and history (CASCADE): it is lobby state, never a media file.

### `lobby_members`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `lobby_id` | BIGINT | NO | - | FK to `lobbies.id` (CASCADE) |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `can_control` | BOOLEAN | NO | FALSE | Control given by the host |
| `anonymous` | BOOLEAN | NO | FALSE | Joined anonymously (public lobbies): the other members see "Anonymous #`anon_number`" (migration 101) |
| `anon_number` | INTEGER | YES | NULL | Number of an anonymous member, from `lobbies.anon_seq` (migration 101) |
| `banned_at` | TIMESTAMPTZ | YES | NULL | Banned from the lobby (the row stays, the user cannot join again) |
| `joined_at` | TIMESTAMPTZ | NO | NOW() | Join date (orders the host hand-over) |

**Indexes:** PRIMARY KEY `(lobby_id, user_id)`, `idx_lobby_members_user_id`, `idx_lobby_members_anon_number` UNIQUE on `(lobby_id, anon_number)` WHERE `anon_number IS NOT NULL`. **Constraints:** CHECK `chk_lobby_members_anon` (`anonymous` with a positive `anon_number`, else no number).

### `lobby_queue_items`

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `lobby_id` | BIGINT | NO | - | FK to `lobbies.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `status` | VARCHAR(10) | NO | 'queued' | `queued` (in the queue; the first one is playing) or `suggested` (waiting for the host or a controller) |
| `position` | INTEGER | NO | 0 | Order within its status |
| `start_s`, `end_s` | DOUBLE PRECISION | YES | NULL | Optional range (a marker) in seconds |
| `label` | VARCHAR(100) | YES | NULL | Optional label (e.g. the marker label) |
| `added_by` | BIGINT | YES | NULL | FK to `users.id` (SET NULL) |
| `added_by_anon` | INTEGER | YES | NULL | Anonymous number of the member who added it (NULL: added under their name), so the adder stays hidden after leaving (migration 101) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | |

An item that finished playing is deleted (auto-next `next`; with `repeat_queue` it goes back to the end of the queue). **Indexes:** `idx_lobby_queue_items_lobby_status_position` on `(lobby_id, status, position)`, `idx_lobby_queue_items_scene_id`. **Constraints:** CHECK `chk_lobby_queue_items_status`, `chk_lobby_queue_items_start` (`start_s >= 0`), `chk_lobby_queue_items_range` (`end_s > COALESCE(start_s, 0)`), `chk_lobby_queue_items_added_by_anon` (`> 0`).

### `lobby_history`

The items a lobby played, one row per play (migration 107, `docs/GOON2GETHER.md` "History"):
the newest `goon2gether.history_max` rows per lobby are kept (each insert trims the older ones).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key (insertion order = order of the plays) |
| `lobby_id` | BIGINT | NO | - | FK to `lobbies.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `start_s`, `end_s` | DOUBLE PRECISION | YES | NULL | Range of the item (a marker) in seconds |
| `label` | VARCHAR(100) | YES | NULL | Label of the item |
| `added_by` | BIGINT | YES | NULL | FK to `users.id` (SET NULL): who added the item to the queue |
| `added_by_anon` | INTEGER | YES | NULL | Anonymous number of the adder (NULL: added under their name) |
| `started_at` | TIMESTAMPTZ | NO | - | When the play started |
| `ended_at` | TIMESTAMPTZ | NO | - | When another item took its place |
| `outcome` | VARCHAR(10) | NO | - | `completed` (reached its end at least once) or `skipped` |

**Indexes:** `idx_lobby_history_lobby_id` on `(lobby_id, id DESC)`, `idx_lobby_history_scene_id`. **Constraints:** CHECK `chk_lobby_history_outcome`, `chk_lobby_history_start` (`start_s >= 0`), `chk_lobby_history_range` (`end_s > COALESCE(start_s, 0)`), `chk_lobby_history_added_by_anon` (`> 0`).


### `emotes`

Server custom emotes (`:name:` in the chat). The image is stored in `goon2gether.emote_dir`
under `filename` (random hex name, never a media directory) and served at `/emotes/<filename>`.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `name` | VARCHAR(32) | NO | - | `^[A-Za-z0-9_]{2,32}$`, unique without regard to case |
| `filename` | VARCHAR(64) | NO | - | Stored file name (unique) |
| `content_type` | VARCHAR(32) | NO | - | `image/png`, `image/gif` or `image/webp` |
| `width`, `height` | INTEGER | NO | - | Pixels |
| `size_bytes` | INTEGER | NO | - | File size |
| `animated` | BOOLEAN | NO | FALSE | Animated GIF, WebP or APNG |
| `created_by` | BIGINT | YES | NULL | FK to `users.id` (SET NULL) |
| `created_at`, `updated_at` | TIMESTAMPTZ | NO | NOW() | |

**Indexes:** `idx_emotes_name_lower` UNIQUE on `LOWER(name)`, `idx_emotes_filename` UNIQUE. **Constraints:** CHECK `chk_emotes_name`, `chk_emotes_content_type`.

---

## Application Settings

### `app_settings`

Global application settings (singleton table).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | INTEGER | NO | 1 | Primary key (always 1) |
| `trash_retention_days` | INTEGER | NO | 7 | Days before trash auto-delete |
| `serve_og_metadata` | BOOLEAN | NO | true | Serve OpenGraph tags to link preview crawlers |
| `transcoding_enabled` | BOOLEAN | NO | true | Convert scenes browsers can't play (AVI, MKV, HEVC, ...) on the fly with ffmpeg (migration 076) |
| `updated_at` | TIMESTAMPTZ | YES | NOW() | Last update timestamp |

**Constraints:**
- CHECK `id = 1` (singleton enforcement)

---

### `search_config`

Meilisearch configuration (singleton table).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | INTEGER | NO | 1 | Primary key (always 1) |
| `max_total_hits` | INTEGER | NO | 100000 | Maximum search results |
| `updated_at` | TIMESTAMPTZ | YES | NOW() | Last update timestamp |

**Constraints:**
- CHECK `id = 1` (singleton enforcement)

---

## Entity Relationship Diagram

```
                                    +------------------+
                                    |      users       |
                                    +------------------+
                                    | id (PK)          |
                                    | username         |
                                    | password         |
                                    | role             |
                                    +------------------+
                                           |
          +--------------------------------+--------------------------------+
          |                                |                                |
          v                                v                                v
+------------------+            +------------------+            +--------------------+
|  user_settings   |            |  saved_searches  |            | revoked_tokens     |
+------------------+            +------------------+            +--------------------+
| user_id (FK)     |            | user_id (FK)     |
| autoplay         |            | name             |
| homepage_config  |            | filters (JSONB)  |
+------------------+            +------------------+

+------------------+                    +------------------+
|     scenes       |<------------------>|     studios      |
+------------------+                    +------------------+
| id (PK)          |                    | id (PK)          |
| title            |                    | uuid             |
| stored_path      |                    | name             |
| studio_id (FK)   |                    | parent_id (FK)   |
| processing_status|                    | network_id (FK)  |
+------------------+                    +------------------+
     |      |
     |      +----------------+
     |                       |
     v                       v
+------------------+   +------------------+
|   scene_tags     |   |  scene_actors    |
+------------------+   +------------------+
| scene_id (FK)    |   | scene_id (FK)    |
| tag_id (FK)      |   | actor_id (FK)    |
+------------------+   +------------------+
     |                       |
     v                       v
+------------------+   +------------------+
|      tags        |   |     actors       |
+------------------+   +------------------+
| id (PK)          |   | id (PK)          |
| name             |   | uuid             |
| color            |   | name             |
+------------------+   +------------------+

Sharing:
+------------------+
|   share_links    |
+------------------+
| token (unique)   |
| scene_id (FK)    |
| user_id (FK)     |
| share_type       |
| expires_at       |
| view_count       |
+------------------+

User Interactions (scenes):
+------------------------+   +------------------------+   +------------------------+
| user_scene_ratings     |   | user_scene_likes       |   | user_scene_jizzed      |
+------------------------+   +------------------------+   +------------------------+
| user_id (FK)           |   | user_id (FK)           |   | user_id (FK)           |
| scene_id (FK)          |   | scene_id (FK)          |   | scene_id (FK)          |
| rating                 |   +------------------------+   | count                  |
+------------------------+                                +------------------------+

+------------------------+   +------------------------+
| user_scene_watches     |   | user_scene_markers     |-------> marker_tags
+------------------------+   +------------------------+         marker_labels <- marker_label_tags
| user_id (FK)           |   | user_id (FK)           |
| scene_id (FK)          |   | scene_id (FK)          |
| last_position          |   | timestamp              |
| completed              |   | label_id (FK), label   |
+------------------------+   +------------------------+

Job System:
+------------------+   +------------------+   +------------------+
|   job_history    |   | dead_letter_queue|   |   retry_config   |
+------------------+   +------------------+   +------------------+
| scene_id (FK)    |   | job_id           |   | phase            |
| phase            |   | scene_id         |   | max_retries      |
| status           |   | phase            |   | backoff_factor   |
| priority         |   | status           |   +------------------+
+------------------+   +------------------+

Configuration (Singletons):
+------------------+   +------------------+   +------------------+
|   pool_config    |   | processing_config|   |  trigger_config  |
+------------------+   +------------------+   +------------------+
| id=1             |   | id=1             |   | phase (unique)   |
| *_workers        |   | frame_quality_*  |   | trigger_type     |
+------------------+   +------------------+   +------------------+

+------------------+   +------------------+
|   app_settings   |   |  search_config   |
+------------------+   +------------------+
| id=1             |   | id=1             |
| trash_retention_ |   | max_total_hits   |
+------------------+   +------------------+
```

---

## Key Patterns & Conventions

### Naming Conventions

- **Tables:** snake_case, plural nouns (`scenes`, `users`, `scene_tags`)
- **Junction tables:** `{entity1}_{entity2}` (e.g., `scene_tags`, `role_permissions`)
- **User interaction tables:** `user_{entity}_{interaction}` (e.g., `user_scene_ratings`)
- **Columns:** snake_case (e.g., `created_at`, `storage_path_id`)
- **Foreign keys:** `{entity}_id` (e.g., `user_id`, `scene_id`)
- **Indexes:** `idx_{table}_{columns}` (e.g., `idx_scenes_deleted_at`)

### Timestamps

- All tables use `TIMESTAMPTZ` for timezone-aware timestamps
- Standard columns: `created_at`, `updated_at`
- Soft delete: `deleted_at` (NULL = active)
- Trash feature: `trashed_at` (separate from soft delete)

### Soft Delete Pattern

Tables with soft delete support:
- `scenes` - `deleted_at` column
- `actors` - `deleted_at` column
- `studios` - `deleted_at` column

Queries should filter `WHERE deleted_at IS NULL` unless specifically querying deleted records.

### Singleton Tables

Configuration tables use `id = 1` with CHECK constraint:
- `pool_config`
- `processing_config`
- `job_schedule_config`
- `app_settings`
- `search_config`
- `worker_settings`

Use `INSERT ... ON CONFLICT DO UPDATE` or `UPDATE WHERE id = 1`.

### UUID Public Identifiers

Entities exposed via API use UUID for public identification:
- `actors.uuid`
- `studios.uuid`
- `saved_searches.uuid`

Internal references still use BIGSERIAL `id` for performance.

### Foreign Key Cascade Rules

- User-owned data: `ON DELETE CASCADE` (settings, header layouts, interactions, markers, share_links, playlists, follows, friendships, friend requests, privacy settings, presence, conversation participants, message reactions)
- Messages: `ON DELETE SET NULL` on the sender and the shared items (the other side keeps the history and the card says "not available")
- Playlist items: `ON DELETE CASCADE` (`playlist_scenes`, `playlist_markers`: a deleted scene or marker leaves the playlists)
- Content associations: `ON DELETE CASCADE` (scene_tags, scene_actors, share_links)
- Optional references: `ON DELETE SET NULL` (scenes.studio_id)

### JSONB Columns

Used for flexible, user-configurable data:
- `user_settings.homepage_config` - Homepage section configuration
- `user_nav_layouts.layout` - Header bars layout (`docs/NAVIGATION.md`)
- `saved_searches.filters` - Search filter parameters

### Partial Indexes

Performance optimization for common query patterns:
- `idx_scenes_stored_path` WHERE deleted_at IS NULL
- `idx_scenes_trashed_at` WHERE trashed_at IS NOT NULL
- `idx_job_history_pending_poll` WHERE status = 'pending'
- `idx_job_history_scene_phase_active` WHERE status IN ('pending', 'running')

### Job Queue Pattern

The `job_history` table serves as a persistent job queue:
1. Jobs created with `status = 'pending'`
2. `JobQueueFeeder` claims jobs using `FOR UPDATE SKIP LOCKED`
3. Unique index on `(scene_id, phase)` for active jobs prevents duplicates
4. Priority ordering: `priority DESC, created_at ASC`; with "new scenes first" (`job_schedule_config`), jobs of scenes created recently come right after the higher priorities: `priority DESC, (scene created recently) DESC, created_at ASC`
5. A phase outside its time window (`job_phase_schedules`) claims nothing; its pending jobs stay pending and are not failed by the startup stuck-pending recovery
6. Bulk cancel (`POST /admin/jobs/cancel-bulk`): one `UPDATE` sets every pending job of the phase(s) to `cancelled` (`is_retryable = false`) while the feeder of those phases is held, then the jobs claimed but not started (rows `running`, still in a worker pool queue) are taken out of the queue and set to `cancelled` too
7. Remote workers (`docs/REMOTE_WORKERS.md`): a worker claims with the same `FOR UPDATE SKIP LOCKED` statement shape (plus scene affinity, path prefixes and the no-audio filter), sets `worker_id`, `lease_generation + 1` and `lease_expires_at`. Every worker call is a compare-and-set on `(job_id, lease_generation, worker_id, status = 'running')`. Expired leases go back to pending with `retry_count + 1` (or fail when the retries are exhausted). The orphan recovery, the shutdown interruption and the reset to pending only touch `worker_id IS NULL` rows
8. History purge: finished rows are deleted by `id` batches (`processing.job_history_purge_batch_size`, default 5000 rows per `DELETE`), `completed`/`cancelled` after `processing.job_history_retention` (default 30d) and `failed`/`timed_out` after `processing.job_history_failed_retention` (default 90d), by `started_at`. `pending` and `running` rows, failed rows with a retry still scheduled (`next_retry_at`), `dead_letter_queue` and other tables are never touched
