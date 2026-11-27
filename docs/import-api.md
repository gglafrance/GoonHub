# Import API (admin) — v2

Endpoints used by external importers (e.g. `Goonhub-stash-import`). All routes are under
`/api/v1/admin/import`, require an authenticated **admin**. "Current user" = the user of the JWT.

All v2 fields are optional and backward compatible with v1 clients.

## GET /api/v1/admin/import/capabilities

Lets a client detect v2 support (a v1 server returns 404).

```json
200 {"version": 2, "features": ["scene_associations", "scene_interactions", "scene_update_existing", "marker_dedup", "marker_tags", "marker_end", "scene_summary", "jizz_history", "skip_index"]}
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
| `jizz_history` | `[]string` | dates (RFC 3339) of the "o", stored as dated events deduplicated on (user, scene, instant); the count is raised to at least their number. Feature `jizz_history` |
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

## Actors and studios: aliases, resolution, duplicates

Actors and studios are created with `POST /api/v1/admin/actors` / `POST /api/v1/admin/studios`
(admin) and updated with `PUT .../{id}`. Both accept `aliases` (`[]string`): other names of the
entity. Aliases are trimmed, deduplicated case-insensitively, and an alias equal to the entity's
own name is dropped.

**Default (import clients):** unchanged contract. Several actors/studios may have the same name
(e.g. a client that does not match by name creates one actor per source performer). An alias
already used by **another** actor (resp. studio) as its name or alias is dropped silently (logged);
the response body has the stored `aliases`.

**`?strict=true`** (what the GoonHub UI sends on its create/edit forms): a name, or an alias,
already used by another entity as its name or alias (case-insensitive) is refused:

```json
409 {"error": "actor 'Jane Doe' already exists", "code": "CONFLICT", "field": "name",
     "value": "jane doe", "existing": {"id": 3, "uuid": "...", "name": "Jane Doe", "aliases": ["JD"], ...}}
```

`field` is `name` or `aliases`, `value` the colliding value, `existing` the full entity. On update
the name is only checked when it changes.

### GET /api/v1/actors/resolve?name= and GET /api/v1/studios/resolve?name=

Exact, case-insensitive match on the name **or an alias** (any authenticated user). Use it to
find-or-create by name instead of `GET /actors?q=` (a substring search limited to 100 results).

```json
200 {"data": {<full actor or studio>}, "matched": "alias", "matched_alias": "JD", "candidates": 1}
404 {"error": "Actor not found"}
400 when name is empty
```

When several entities match (`candidates` > 1, possible with duplicate names), `data` is the best
one: a name match before an alias match, then the oldest.

`GET /actors?q=` and `GET /studios?q=` also match aliases (substring); each result has `aliases`
and, when the query matched an alias but not the name, `matched_alias`.

### Merge

`POST /api/v1/actors/{uuid}/merge` `{"source_uuid": "..."}` merges the source actor into the actor
of the URL (permission `actors:merge`: admin and moderator): the source's scenes, ratings and likes
move to the target (a user's own rating/like of the target is kept), homepage sections and saved
filters that referenced the source point to the target, the source's name and aliases become
aliases of the target, the target keeps its photo unless it has none, and the source is deleted.
`GET /api/v1/actors/{uuid}/merge-preview?source_uuid=` returns the same body without merging:

```json
200 {"target": {...}, "source": {...}, "scenes": 12, "shared_scenes": 1, "ratings": 2, "likes": 1,
     "aliases_added": ["Jane D."], "image_from_source": false}
```

Studios: `POST /api/v1/studios/{uuid}/merge` and `GET .../merge-preview` (permission
`studios:merge`), with `child_studios` (studios whose parent or network was the source, moved
to the target) and `logo_from_source` instead of `shared_scenes` and `image_from_source`.
