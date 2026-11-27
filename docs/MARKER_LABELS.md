# Server-wide marker labels

Marker labels (the name of a marker: "Kiss", "Cumshot"...) are a server referential, the same
for every user, managed by the users holding `marker_labels:manage` (default: `moderator`; a
superuser has it). Markers themselves stay per user (`user_scene_markers.user_id`, shared ones
through `users.share_markers`).

## Model

- `marker_labels`: `id`, `name` (unique ignoring case: unique index on `lower(name)`, stored
  trimmed), `color` (hex, default `#FFFFFF`), `description`, `created_at`, `updated_at`.
- `user_scene_markers.label_id`: the marker's label, `NULL` for a marker without label
  (allowed). `ON DELETE SET NULL`: deleting a label keeps its markers, without label.
- `user_scene_markers.label`: a copy of the label name kept by the database (`''` without
  label), so the listings, filters and the other features read the name without a join.
  `label_id` is the source of truth. Trigger `trg_user_scene_markers_sync_label`
  (`BEFORE INSERT OR UPDATE`):
  - a write that sets or clears `label_id` wins: `label` becomes the label name (or `''`);
  - a write that only gives the label text (insert without `label_id`, or update of `label`
    alone, as legacy writers and older code do) is resolved to the label of that name,
    ignoring case and surrounding spaces, created when missing. The application checks the
    permission before writing (below); this only keeps the copy consistent.
  - Trigger `trg_marker_labels_propagate_name` renames the copy on the markers when a label is
    renamed (`updated_at` of the markers is not touched: it versions the thumbnails).
  - In Go, `UserSceneMarker.Label` is written on create only (`<-:create`): `Save()` of a
    marker loaded before a rename cannot bring the old name back.
- `marker_label_tags (label_id, tag_id)`: the default tags of a label, the same for everyone
  (unique pair, cascade on label and tag deletion). Every marker of the label carries them as
  `marker_tags.is_from_label = true`. A tag the marker also had by hand keeps
  `is_from_label = false` and is never removed by a label change.

Label matching is case-insensitive everywhere a name comes from outside (filters, saved
filters, homepage widget configs, URLs, import): `label_id IN (SELECT id FROM marker_labels
WHERE lower(name) IN ...)` with lowercased, trimmed names.

## Rules

- Creating or editing a marker (`POST /scenes/:id/markers`, `PUT /scenes/:id/markers/:id`)
  with a label: an existing label is matched ignoring case (the marker gets its canonical
  spelling). An unknown label is a `400` (`unknown marker label "X": pick an existing label`)
  for a user without `marker_labels:manage`, and is created on the fly for a user with it. An
  empty label means no label. Without color, a new marker takes the color of its label.
- Changing the label of a marker replaces its tags from label by the default tags of the new
  label (tags set by hand stay).
- The import API (`POST /import/markers`, admin-only) creates an unknown label instead of
  failing (`ImportRepository.EnsureMarkerLabel`); the dedup key `(user, scene, timestamp,
  label)` matches the label ignoring case.
- Setting the default tags of a label syncs the tags from label of every marker of the label,
  for every user: the removed ones are dropped, the new ones added.
- Rename: a name used by another label (ignoring case) answers `409` with that label in
  `existing` (the UI offers to merge into it). A different case of the label's own name is
  allowed. The rename also renames the references to the old name (ignoring case): saved
  searches (`selected_marker_labels`), saved marker filters (`labels`) and the homepage widgets
  (`labels` of moments, marker_compilation, recent_markers; `items` of browse_tags on labels).
- Merge: the markers of the sources get the target, the target gets the union of their
  default tags (then the markers of the target are synced), the references are renamed and the
  sources are deleted. `dry_run` only counts.
- Delete: the markers (of every user) lose the label and the tags they had from it. Stored
  references to the name are left as they are (they match nothing).
- The duplicate marker merge (`POST /admin/markers/deduplicate`) gives the kept marker the label
  of another marker of the set (an existing label) through `label_id`.

## API

Everyone (`/markers`, as before):

- `GET /markers/labels`: `{labels: [{id, label, color, count}]}`, the whole referential
  (unused labels too, `count` 0), most used first by the markers the user sees, then by name.
  The marker label pickers use it. `GET /scenes/filter-options` keeps `marker_labels` with the
  labels in use only.
- `GET /markers/label-tags?label=X` (ignoring case, `404` for an unknown label): `{tags}`.

With `marker_labels:manage`:

- `PUT /markers/label-tags?label=X` `{tag_ids}`: sets the default tags (was per user and
  `markers:create` before).
- `GET /marker-labels`: `{data: [{id, name, color, description, created_at, updated_at,
  marker_count, user_count, default_tags}]}` (counts over every user's markers), by name.
- `POST /marker-labels` `{name, color?, description?}`: `201` with the label; `409` (with
  `existing`) when the name is used ignoring case.
- `PUT /marker-labels/:id` `{name?, color?, description?}`: `409` as above, `404` unknown.
- `DELETE /marker-labels/:id`, `POST /marker-labels/delete` `{ids}`: `{deleted,
  markers_cleared}`.
- `POST /marker-labels/merge` `{target_id, source_ids, dry_run}`: `{target, sources, dry_run,
  markers_moved, default_tags_added, references_updated, sources_deleted}`.
- `GET|PUT /marker-labels/:id/tags` (`PUT` body `{tag_ids}`): `{tags}`.

## UI

- Settings > Marker Labels (shown with `marker_labels:manage`): create, list with search,
  inline rename (a `409` offers "Merge into"), recolor, description, default tags, usage counts,
  merge of a selection, delete (one or a selection).
- Marker label inputs (Markers tab of the watch page, quick marker M): existing labels only; a
  label manager gets a "Create label" row for a new name; anyone else gets "No label X: pick an
  existing label" and the save is refused before the request.
- The label page (`/markers/<label>`) shows the default tags; editing them there needs
  `marker_labels:manage` too.

## Migration 097

Up, on the existing data:

1. Every distinct label of every user (markers and per-user default tags), trimmed, merged
   ignoring case, becomes one `marker_labels` row. The name kept is the most used spelling by
   markers (ties: the first in byte order, so `Kiss` before `kiss`). Its color is the most used
   marker color of the label, its creation date the first marker (or default tag).
2. Markers get `label_id` and their `label` becomes the canonical name. Empty or blank labels
   become `''` with `label_id` `NULL`. `label` becomes `NOT NULL DEFAULT ''`.
3. Default tags: the per-user rows are merged into one row per (label, tag), the union of all
   users' default tags. Then every marker of a label gets the default tags it lacks as tags
   from label (a tag it already had keeps its origin). So a user's markers may get tags that
   another user had set as defaults of the same label.
4. The triggers are installed.

Down: the per-user default tags are rebuilt for every user having markers with the label.
Lost: the spelling variants (markers keep the canonical name), the label colors and
descriptions, the labels without markers and their default tags; the marker tags added by
the union stay.

After deployment: look at Settings > Marker Labels for near-duplicates the case-insensitive
merge did not catch (e.g. "Blow job" / "Blowjob") and merge them; check the default tags of
the merged labels (union of every user's).
