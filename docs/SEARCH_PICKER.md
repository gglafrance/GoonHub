# Search picker

`PickerModal` (`web/app/components/picker/`) is a large modal (full screen on mobile) to pick
videos and markers with the interface of the search page. It is used by the Goon2gether queue
("Add to queue", see `docs/GOON2GETHER.md`) and the profile showcases (favorite videos and
markers, see `docs/PROFILES.md`). Its props and events are documented at the top of
`PickerModal.vue`; the selection rules and the mapping of picked markers to queue items are in
`utils/picker.ts` (tests in `web/tests/picker.test.ts`).

## Tabs

- **Videos** (`PickerScenes`): the search page components (`SearchBar`, `SearchFilters` and
  its filters, `SearchActiveFilters`), the result grid with the user's card template
  (`SceneCard` in select mode, hover preview and scrubber included), pages or infinite scroll
  as the user setting says. RBAC is the search page's: no size filter or sort without
  `files:view_size`.
- **Markers** (`PickerMarkers`): the "All" listing of the markers page (`GET /markers/all`)
  with its filters (`useMarkerFilters`, always on here, whatever the "Advanced Marker Filters"
  setting), sorts, a text search and the marker cards (`MarkersMarkerCard`, versioned
  thumbnails).

## Search state

The search store is a setup function shared by two Pinia stores: `useSearchStore` (the search
page, synced with the URL by `pages/search.vue`) and `usePickerSearchStore` (the picker, never in
the URL). The search components get their store with `useSearchContext()`: the one provided by
an ancestor (`provideSearchStore`, done by `PickerScenes`), else the search page store. So
opening the picker changes neither the search page state nor the URL. The picker's filters stay
between two openings (same session). `useMarkerFilters(sort, withLabels, { url: false })` keeps
the marker filters out of the URL the same way.

## Marker listing parameters

`GET /api/v1/markers/all` (and the other marker listings sharing `markerListOptions`) also take:

| Parameter | Meaning |
|---|---|
| `q` | label or scene title contains the text (ignoring case) |
| `live_only=true` | only markers of scenes still in the library (not trashed, not deleted) |

The picker always sends `live_only=true`: trashed scenes cannot be queued, nor showcased, and
the markers it lists are then exactly the ones `GET /profiles/me/marker-candidates` allows
(the markers the user sees, `VisibleMarkersSQL`, on live scenes).
