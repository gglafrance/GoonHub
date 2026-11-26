# Marker playlists

A playlist holds either **scenes** or **markers** (moments of scenes: `user_scene_markers`). The
kind is chosen at creation and never changes; mixed playlists do not exist. Everything else is
shared with the scene playlists: name, description, visibility, tags, likes, resume progress,
list pages, filters, the playlist page and the playlist mode of the watch page.

Why one feature rather than a parallel one: a marker playlist only differs by what it lists
and by how an item plays. Reusing `playlists` keeps one set of routes, permissions, list
filters, cards and player panels, and the homepage playlist sections show both kinds for free.

## Model

- `playlists.kind`: `scenes` (default, every playlist created before migration 096) or
  `markers` (CHECK constraint).
- `playlist_markers` (migration 096): `playlist_id`, `marker_id`, `position`, `added_at`.
  A marker is in a playlist at most once (unique `(playlist_id, marker_id)`); adding it again
  is skipped, not an error. `ON DELETE CASCADE` on both: a marker deleted by its owner (or with
  its scene, or its user) leaves every playlist.
- `playlist_progress.last_marker_id` (`ON DELETE SET NULL`): the last marker played in a marker
  playlist; `last_scene_id` holds its scene and `last_position_s` the time in that scene.
- Markers of a scene in the trash or deleted are kept in the table but not listed, counted nor
  played (they come back with the scene when it is restored).

## Size limit and settings

Config (`internal/config`, YAML `playlists:` or `GOONHUB_PLAYLISTS_*` env vars):

| Key | Default | Meaning |
|---|---|---|
| `playlists.max_markers` | `1000` | Most markers in a marker playlist (`0` = no limit; adding from a listing then takes at most 10000 markers) |
| `playlists.marker_default_duration` | `30` | Seconds a marker without an end plays |

An add that would exceed the limit is refused as a whole (`400 PLAYLIST_FULL`), nothing is
added. The check and the insert run in one transaction that locks the playlist row
(`SELECT ... FOR UPDATE`), so concurrent adds cannot pass the limit.

## Access

Markers are per user; a user reads the markers of others only when they share them
(`users.share_markers`, `data.VisibleMarkersSQL`). For marker playlists:

- **Viewing**: whoever can view the playlist (its owner; anyone for a public playlist) sees all
  its markers, including markers of users who do not share theirs. Sharing a playlist shares the
  moments it lists: that is its point. What is exposed is limited to what playing needs (marker
  id, owner id, scene id, times, label, color, thumbnail version; scene id, title, duration).
  No file size, path nor technical information (no `files:*` data at all).
- **Adding**: only the owner, and only markers they see themselves (their own and the shared
  ones), of scenes neither trashed nor deleted. A marker that does not exist and a marker the
  user cannot see answer the same `404 PLAYLIST_MARKER_NOT_FOUND`, and the whole request fails.
- A marker added while its owner shared markers stays in the playlist if they stop sharing.
- Changing (add, remove, reorder) needs ownership plus `playlists:edit`; creating needs
  `playlists:create`, deleting `playlists:delete`, liking `playlists:view_public`, reading a
  playlist `scenes:view` (the existing playlist permissions, no new one).

## API

All under `/api/v1`, authenticated. Existing routes gain the kind:

- `GET /playlists?kind=scenes|markers` (plus the existing `owner`, `visibility`, `tag_ids`,
  `search`, `sort`, `page`, `limit`). `sort=scene_count_desc` counts the items of either kind.
  Unknown kind: `400 PLAYLIST_INVALID_KIND`.
- List items and details have `kind`, `marker_count`, `thumbnail_markers: [{id, version}]`
  (marker playlists; `thumbnail_scenes` is empty then). For a marker playlist `scene_count` is
  the number of distinct scenes its markers come from and `total_duration` the sum of the play
  lengths.
- `POST /playlists` `{name, description?, visibility?, kind?, tag_ids?, scene_ids?, marker_ids?}`
  -> `201 {uuid, name, kind}`. `scene_ids` with `kind=markers` or `marker_ids` with
  `kind=scenes`: `400 PLAYLIST_KIND_MISMATCH`.
- `GET /playlists/:uuid` adds `markers` (marker playlists, `[]` otherwise),
  `marker_default_duration` and `resume.marker_id`. A marker entry:
  `{position, added_at, marker: {id, user_id, scene_id, timestamp, end_timestamp, label, color,
  has_thumbnail, thumbnail_version}, scene: {id, title, duration}, start, end}`.
  The thumbnail is `/marker-thumbnails/:id?v=<thumbnail_version>` (the marker's `updated_at` in
  Unix ms, like the other marker thumbnails).
- `POST /playlists/:uuid/markers` `{marker_ids}` -> `{added, skipped}` (skipped: already in it).
- `POST /playlists/:uuid/markers/from-filter` -> `{added, skipped}`: every marker of a marker
  listing, in its order. Body: `label` (a label page: that label, newest first) or the
  listing of `GET /markers/all` (`labels`, `tags`, `actors`, `studio`, `marker_tags` as
  comma-separated strings, `ranges_only` boolean, `sort`, `seed`). A listing larger than the
  limit is refused (`400 PLAYLIST_FULL`).
- `DELETE /playlists/:uuid/markers/:markerId`, `POST /playlists/:uuid/markers/remove`
  `{marker_ids}` -> `204`; none of them in the playlist: `400 PLAYLIST_MARKER_NOT_IN_PLAYLIST`.
- `PUT /playlists/:uuid/markers/reorder` `{marker_ids}` (new order, positions 0..n-1) -> `204`.
- `PUT /playlists/:uuid/progress` `{scene_id, position_s}` for a scene playlist,
  `{marker_id, position_s}` for a marker playlist (the marker must be in it; its scene is
  stored). Needs to be able to view the playlist.
- Scene routes (`/playlists/:uuid/scenes...`) on a marker playlist and marker routes on a scene
  playlist answer `400 PLAYLIST_KIND_MISMATCH`. Changes by another user: `403`.

## Playback

The watch page plays a playlist with `?playlist=<uuid>&pos=<index in the play order>`
(`&shuffle=<seed>` when shuffled). A marker item also has `t=<start>` and no `end` (a marker
link of the markers pages loops its range; a playlist plays it once).

- A marker plays from `start` to `end`: `end_timestamp`, or `timestamp +
  marker_default_duration` for a marker without an end, cut at the end of the scene when its
  duration is known (`data.MarkerPlayRange`, computed by the server).
- When playback reaches `end` (or the scene ends first), the next marker starts: on the same
  scene by a seek, else on its scene at its start. A seek past the end does not count, so the
  user can keep watching the scene after a marker. With the auto-advance setting on `manual`
  (Settings > Player) the player pauses at the end instead; `instant` and `countdown` both go
  on at once (no countdown between short clips). At the end of the playlist the player pauses.
- Next/previous (player buttons, queue panel, mobile bar), shuffle (seeded, kept in the URL)
  and the highlighted current item of the queue panel work like scene playlists. There is no
  loop mode (scene playlists have none either).
- Resume: the watch page saves the item played and the time in its scene when an item starts,
  every 10 s while playing and when the page is left (both kinds; scene playlists did not save
  their progress before). The playlist page Resume button reopens the last marker at the saved
  time when it is inside its range, else at its start.

Code: `internal/core/playlist_marker_service.go`, `internal/data/playlist_marker_repository.go`,
`internal/api/v1/handler/playlist_marker_handler.go`; front `web/app/utils/playlistQueue.ts`
(queue items, query, resume), `composables/usePlaylistPlayer.ts` (order, shuffle, advance),
`composables/usePlaylistItemPlayback.ts` (range end, resume saves),
`components/playlist/MarkerList.vue`, `components/markers/PlaylistAddButton.vue`.

## Where markers are added

"Add to playlist" opens the playlist picker (`PlaylistCreateModal`) on the marker playlists of
the user, with an inline "Create New":

- the Markers tab of the watch page: per marker, and "Add all to playlist" (every marker shown);
- the marker cards of `/markers` (All view) and `/markers/<label>`: per marker (hover button);
- "Add all to playlist" in the header of `/markers/<label>` (the label, with the current
  filters) and of the All view of `/markers` (the filtered list, in its sort order).

The marker pages have no multi-select, so a selection is the whole filtered list or label.
