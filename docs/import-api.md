# Import API (admin) — v2

Endpoints used by external importers (e.g. `Goonhub-stash-import`). All routes are under
`/api/v1/admin/import`, require an authenticated **admin**. "Current user" = the user of the JWT.

All v2 fields are optional and backward compatible with v1 clients.

## GET /api/v1/admin/import/capabilities

Lets a client detect v2 support (a v1 server returns 404).

```json
200 {"version": 2, "features": ["scene_associations", "scene_interactions", "scene_update_existing", "marker_dedup", "marker_tags", "marker_end", "scene_summary", "skip_index"]}
```

## POST /api/v1/admin/import/scenes

v1 fields unchanged (`title`, `stored_path`, `original_filename`, `size`, `duration`, `width`, `height`,
`frame_rate`, `bit_rate`, `video_codec`, `audio_codec`, `description`, `release_date`, `studio_id`,
`storage_path_id`, `origin`, `type`, `skip_file_check`).

New fields:

| field | type | meaning |
|---|---|---|
| `tag_ids` | `[]uint` or absent | absent/null = don't touch; `[]` = clear; replaces scene tags |
| `actor_ids` | `[]uint` or absent | same semantics for actors |
| `view_count` | `*int64` | sets `scenes.view_count` |
| `created_at` | `*string` RFC3339 | preserves the source "added" date on creation (ignored on update) |
| `file_created_at` | `*string` RFC3339 | sets `scenes.file_created_at` |
| `update_existing` | `bool` | if a non-trashed scene already has this `stored_path`: update it instead of 409 |
| `skip_index` | `bool` | don't touch the search index (client will trigger a full reindex) |
| `interactions` | object or absent | applied for the **current user**, see below |

`interactions`:

| field | type | meaning |
|---|---|---|
| `rating` | `*float64` | 0.5–5, rounded to nearest 0.5; upsert. `0`/absent = don't touch |
| `liked` | `*bool` | true = set like, false = remove like, absent = don't touch |
| `jizzed_count` | `*int` | sets the exact count (not an increment); absent = don't touch |
| `watches` | `[]{watched_at RFC3339, watch_duration int, last_position int, completed bool}` | inserted into watch history; an entry with the same (user, scene, watched_at to the second) is not inserted twice |

Behaviour:

- Validation happens before any write (release_date `YYYY-MM-DD`, origin, type, tag/actor/studio IDs exist,
  rating range, timestamps). Unknown tag/actor/studio IDs → 400 with an explicit message.
- Scene creation + tags + actors + studio + view_count + interactions run in **one DB transaction**: either
  everything is written or nothing (no half-imported scene).
- Denormalized `scenes.tags`, `scenes.actors`, `scenes.studio` text fields are kept in sync.
- Unless `skip_index`, the scene is (re)indexed in the search engine after commit (best effort, logged on failure).
- Existing scene and `update_existing=false` → `409 {"id", "title", "error"}` (unchanged v1 behaviour).
- Existing scene and `update_existing=true` → the provided metadata (`title`, `description`, `release_date`,
  `studio_id`, `origin`, `type`, `view_count`, `file_created_at`, technical fields if non-zero) + associations +
  interactions are applied → `200 {"id", "title", "created": false}`.
- Created → `201 {"id", "title", "created": true}`.

## POST /api/v1/admin/import/markers

| field | type | meaning |
|---|---|---|
| `scene_id` | uint, required | |
| `user_id` | uint, optional | defaults to the current user |
| `timestamp` | int ≥ 0 | seconds |
| `end_timestamp` | int or absent | end of the marker range in seconds, must be > `timestamp` (400 otherwise). Feature `marker_end` |
| `label` | string | ≤ 100 chars (400 otherwise) |
| `color` | string | default `#FFFFFF` |
| `tag_ids` | `[]uint` or absent | individual marker tags, applied after label tags (added, not replacing label tags) |
| `update_existing` | bool | see dedup |

Dedup on the natural key (user_id, scene_id, timestamp, label):
- exists and `update_existing=false` → `409 {"id", "scene_id", "error"}`
- exists and `update_existing=true` → color, `end_timestamp` (when present) and `tag_ids` applied → `200 {"id", "scene_id", "created": false}`
- created → `201 {"id", "scene_id", "created": true}`

## POST /api/v1/admin/import/scenes/summary

For post-import verification. Body `{"ids": [1,2,3]}` (max 1000, else 400).

```json
200 {
  "data": [
    {"id": 1, "title": "...", "stored_path": "...", "studio_id": 3, "tag_ids": [1,2], "actor_ids": [5],
     "view_count": 4, "marker_count": 2, "rating": 4.5, "liked": false, "jizzed_count": 1,
     "watch_count": 3, "trashed": false}
  ],
  "missing": [3]
}
```

`marker_count`, `rating`, `liked`, `jizzed_count`, `watch_count` are for the current user.
`missing` lists IDs that don't exist (or are soft-deleted).
