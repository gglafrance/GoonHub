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
5. [User Interactions](#user-interactions)
6. [Job System](#job-system)
7. [Storage & Scanning](#storage--scanning)
8. [Application Settings](#application-settings)
9. [Entity Relationship Diagram](#entity-relationship-diagram)
10. [Key Patterns & Conventions](#key-patterns--conventions)

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
| `role` | VARCHAR(50) | NO | 'user' | Role name (admin, moderator, user) |
| `last_login_at` | TIMESTAMPTZ | YES | NULL | Last successful login |

**Constraints:**
- `uni_users_username` UNIQUE on `username`

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
| `videos_per_page` | INTEGER | NO | 20 | Grid pagination size |
| `default_sort_order` | VARCHAR(50) | NO | 'created_at_desc' | Default sort order |
| `default_tag_sort` | VARCHAR(10) | NO | 'az' | Tag sorting (az, za, count) |
| `marker_thumbnail_cycling` | BOOLEAN | NO | true | Enable marker thumbnail cycling |
| `homepage_config` | JSONB | NO | (see below) | Homepage section configuration |

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

**Constraints:**
- `uni_user_settings_user_id` UNIQUE on `user_id`
- FK to `users(id)` ON DELETE CASCADE

---

## RBAC (Role-Based Access Control)

### `roles`

Role definitions for access control.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |
| `name` | VARCHAR(50) | NO | - | Unique role name |
| `description` | VARCHAR(255) | YES | NULL | Role description |

**Seed Data:**
- `admin` - Full system access
- `moderator` - Video management access
- `user` - Basic view and upload access

**Constraints:**
- `uni_roles_name` UNIQUE on `name`

---

### `permissions`

Permission definitions for granular access control.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `name` | VARCHAR(100) | NO | - | Unique permission identifier |
| `description` | VARCHAR(255) | YES | NULL | Permission description |

**Seed Data:**
- `scenes:view` - View and stream scenes
- `scenes:upload` - Upload new scenes
- `scenes:delete` - Delete scenes
- `scenes:reprocess` - Reprocess scenes
- `scenes:trash` - Move scenes to trash
- `users:manage` - Manage users
- `users:create` - Create new users
- `users:delete` - Delete users
- `roles:manage` - Manage roles and permissions
- `settings:manage` - Manage application settings

**Constraints:**
- `uni_permissions_name` UNIQUE on `name`

---

### `role_permissions`

Junction table linking roles to permissions.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | BIGSERIAL | NO | auto | Primary key |
| `role_id` | BIGINT | NO | - | FK to `roles.id` (CASCADE) |
| `permission_id` | BIGINT | NO | - | FK to `permissions.id` (CASCADE) |

**Indexes:**
- `idx_role_permissions_role_id` on `role_id`

**Constraints:**
- `uni_role_permissions` UNIQUE on `(role_id, permission_id)`

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
- `idx_actors_deleted_at` on `deleted_at`

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
| `created_at` | TIMESTAMPTZ | NO | NOW() | Link creation timestamp |

**Valid `share_type` values:** `public`, `auth_required`

**Indexes:**
- `idx_share_links_token` UNIQUE on `token`
- `idx_share_links_scene_user` on `(scene_id, user_id)`
- `idx_share_links_user_id` on `user_id`

**Constraints:**
- CHECK `share_type IN ('public', 'auth_required')`
- FK to `scenes(id)` ON DELETE CASCADE
- FK to `users(id)` ON DELETE CASCADE

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

### `user_scene_markers`

Video bookmarks/markers created by users.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `scene_id` | BIGINT | NO | - | FK to `scenes.id` (CASCADE) |
| `timestamp` | INTEGER | NO | - | Position in seconds |
| `label` | VARCHAR(100) | YES | NULL | Marker label/name |
| `color` | VARCHAR(7) | NO | '#FFFFFF' | Hex color code |
| `thumbnail_path` | VARCHAR(255) | YES | NULL | Generated thumbnail path |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Marker creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

**Indexes:**
- `idx_user_scene_markers_user_scene` on `(user_id, scene_id)`
- `idx_user_scene_markers_timestamp` on `(scene_id, timestamp)`
- `idx_user_scene_markers_user_label` on `(user_id, label)`

**Constraints:**
- CHECK `timestamp >= 0`

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

### `marker_label_tags`

Default tags associated with marker labels (per user).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `user_id` | BIGINT | NO | - | FK to `users.id` (CASCADE) |
| `label` | VARCHAR(100) | NO | - | Marker label name |
| `tag_id` | BIGINT | NO | - | FK to `tags.id` (CASCADE) |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Association timestamp |

**Indexes:**
- `idx_marker_label_tags_user_label` on `(user_id, label)`
- `idx_marker_label_tags_tag_id` on `tag_id`

**Constraints:**
- UNIQUE on `(user_id, label, tag_id)`

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

**Valid `phase` values:** `metadata`, `thumbnail`, `sprites`, `scan`

**Valid `status` values:** `pending`, `running`, `completed`, `failed`

**Indexes:**
- `idx_job_history_job_id` UNIQUE on `job_id`
- `idx_job_history_started_at` on `started_at`
- `idx_job_history_status` on `status`
- `idx_job_history_next_retry` on `next_retry_at` WHERE next_retry_at IS NOT NULL AND status = 'failed'
- `idx_job_history_pending_poll` on `(phase, priority DESC, created_at ASC)` WHERE status = 'pending'
- `idx_job_history_scene_phase_active` UNIQUE on `(scene_id, phase)` WHERE status IN ('pending', 'running')

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

## Storage & Scanning

### `storage_paths`

Configured storage locations for video files.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | SERIAL | NO | auto | Primary key |
| `name` | VARCHAR(100) | NO | - | Display name |
| `path` | VARCHAR(500) | NO | - | Filesystem path |
| `is_default` | BOOLEAN | NO | false | Default storage flag |
| `created_at` | TIMESTAMPTZ | NO | NOW() | Record creation timestamp |
| `updated_at` | TIMESTAMPTZ | NO | NOW() | Last update timestamp |

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

## Application Settings

### `app_settings`

Global application settings (singleton table).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | INTEGER | NO | 1 | Primary key (always 1) |
| `trash_retention_days` | INTEGER | NO | 7 | Days before trash auto-delete |
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
+------------------------+   +------------------------+         marker_label_tags
| user_id (FK)           |   | user_id (FK)           |
| scene_id (FK)          |   | scene_id (FK)          |
| last_position          |   | timestamp              |
| completed              |   | label                  |
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
- `app_settings`
- `search_config`

Use `INSERT ... ON CONFLICT DO UPDATE` or `UPDATE WHERE id = 1`.

### UUID Public Identifiers

Entities exposed via API use UUID for public identification:
- `actors.uuid`
- `studios.uuid`
- `saved_searches.uuid`

Internal references still use BIGSERIAL `id` for performance.

### Foreign Key Cascade Rules

- User-owned data: `ON DELETE CASCADE` (settings, interactions, markers, share_links)
- Content associations: `ON DELETE CASCADE` (scene_tags, scene_actors, share_links)
- Optional references: `ON DELETE SET NULL` (scenes.studio_id)

### JSONB Columns

Used for flexible, user-configurable data:
- `user_settings.homepage_config` - Homepage section configuration
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
4. Priority ordering: `priority DESC, created_at ASC`
