# Profile pages

Letterboxd-like profile pages: `/u/<username>` shows a header (avatar, display name, bio,
banner, join date, relation counts and actions) and the **blocks** its owner composed (favorite
videos, favorite markers, recent activity, stats...). It builds on the social graph and the
privacy framework (`docs/SOCIAL.md`): read "Privacy framework" there first.

## Model

- **Profile** (`user_profiles`, migration 100, one row per user who edited theirs; no row = the
  defaults): display name, bio, avatar version, banner (none, a scene thumbnail, or an uploaded
  image), block layout.
- **Showcases**: the favorite videos (`user_profile_favorite_scenes`) and the favorite markers
  (`user_profile_favorite_markers`), ordered, at most `profiles.max_showcase_items` (default 4)
  each. The owner picks scenes still in the library and markers they see (their own, or those of
  users who share theirs, `users.share_markers`).
- **Tabs** (`user_profiles.tabs`, migration 103): the ordered tabs, each made of **sections**
  (rows with a column template) holding **blocks** (`{id, type, visible, locked_placeholder,
  config}`), see "Tabs and sections". NULL = derived from the older **layout** column (one list
  of blocks, migration 100), itself NULL for the default layout.
- **Custom lists** (`user_profile_lists`, migration 103): search criteria run with the owner's
  interactions, see "Custom lists".

Display names and avatars are identity, like usernames: every signed-in user sees them (lists,
directory, header), whatever the privacy settings. The bio and the banner are part of the
profile page and follow the `profile` item.

## What a viewer sees

`ProfileService.Get` (`internal/core/profile_service.go`), for every viewer (superusers
included, no administrator bypass):

1. The social summary (identity, relationship, counts, `visibility`) as `GET
   /social/users/:username` gives it.
2. If the viewer may not see the `profile` item: nothing else (no bio, banner nor block).
3. Else the bio, the banner (a scene banner only for viewers with `scenes:view` and a scene still
   in the library), the tabs the viewer may open, and the visible blocks of the asked tab
   (`?tab=`), section by section, column by column:
   - a block whose `ViewerPermission` the viewer lacks is omitted (e.g. no scene block without
     `scenes:view`);
   - a block whose privacy item the viewer may not see is omitted, or, when the owner turned on
     `locked_placeholder`, sent as `{locked: true}` without any data (the UI shows "Private");
   - otherwise the block is fetched. A block that fails is sent as `{failed: true}` (logged), the
     page still loads.

The privacy items of all the blocks are read in one `PrivacyService.Visibility` call (two
queries at most). The server never fetches, let alone sends, the data of a hidden block. The
owner always sees all their visible blocks; the UI hides empty blocks from other viewers.

Choice made: hidden blocks are omitted by default (the existence of a block says something), the
placeholder is opt-in per block.

### Redaction

Scenes in blocks are `SceneListItem` converted with the viewer's file visibility
(`response.ToSceneListItem`, `docs/RBAC.md`): no size without `files:view_size`, no path
without `files:view_paths`. Every scene query skips trashed and deleted scenes, so a viewer only
gets scenes they could browse anyway (and the scene blocks need `scenes:view`). The stats block
never carries file information. Playlist covers need `scenes:view`.

### Markers

A profile exposes the owner's markers in the blocks the owner made visible: that is their point.
`recent_markers` lists the owner's own markers; `favorite_markers` the markers the owner picked
among those they see (theirs or shared with them; a marker whose owner stops sharing disappears
from the showcase). The audience of these blocks sees those markers even if the owner does not
share their markers (`users.share_markers`). Marker thumbnails use the versioned URLs
(`/marker-thumbnails/:id?v=`); a click plays the marker range on the watch page (`?t=&end=`).

## Blocks

| Type | Privacy item | Viewer permission | Options | Default layout |
|---|---|---|---|---|
| `favorite_scenes` | `favorites` | `scenes:view` | | shown |
| `favorite_markers` | `favorite_markers` | `scenes:view` | | shown |
| `recent_watches` | `activity` | `scenes:view` | `count` 2-24 (8) | shown |
| `most_watched` | `activity` | `scenes:view` | `count` (8), `period` week/month/year/all (month) | hidden |
| `recent_markers` | `recent_markers` | `scenes:view` | `count` (8) | hidden |
| `liked_scenes` | `liked` | `scenes:view` | `count` (8) | hidden |
| `stats` | `stats` | `scenes:view` | `period` (all) | shown |
| `ratings` | `ratings` | `scenes:view` | | shown |
| `jizz` | `jizz` | `scenes:view` | `count` (6) | shown |
| `activity_calendar` | `activity` | | `weeks` 4-104 (26), `metric` views/jizz/both (views) | shown |
| `playlists` | `playlists` | `playlists:view_public` | `count` (6) | shown |
| `friends` | `friends_list` | | `count` (8) | shown |
| `custom_list` | the list's (below) | `scenes:view` | `list_id`, `count` 2-24 (6), `display` grid/row (grid) | never (placed by the owner, several times) |

Periods: `week` = last 7 days, `month` = last 30 days, `year` = last 365 days, `all`.

- `stats`: watch time (sum of the watch durations), views, distinct videos watched, O (dated O
  of the period; all time also counts the undated totals), and as totals liked videos, markers,
  ratings and the average rating; the top 5 tags, actors and studios by views of the period.
- `ratings`: the Letterboxd-like chart of the owner's ratings: one bar per half star from 0.5
  to 5 (empty bars included), the number of ratings (`total`) and their average. A bar opens
  the ratings list filtered on that value. Payload: `{buckets: [{rating, count}], total,
  average}`.
- `jizz`: the O total and the number of videos (`{total, scenes}`, block `total` = videos), and
  the videos jizzed to last with their O count (`jizz_count`) and the date of the last O (`at`).
- `liked_scenes`, `ratings` and `jizz` have a "See all" link to their full list (below).
- `activity_calendar` reuses the `activity_calendar` homepage widget for the owner (daily
  counts and streaks only) and its frontend component.

A stored layout is normalized when read: unknown types and invalid options are dropped, block
types added later are appended hidden (they do not appear on customized profiles by
themselves). Saving a layout refuses unknown types, duplicates, unknown options and out of range
values (`400`), and appends the missing types hidden.

### Adding a block

1. Backend: create `internal/core/profile_block_<type>.go` with an `init()` calling
   `registerProfileBlock(ProfileBlockSpec{...})`: type, label, description, privacy item (an
   existing one or a new one, see `docs/SOCIAL.md`, "Adding a privacy item"), the viewer
   permission if the block shows library content, options (`ProfileOptionInt` with min, max,
   default, or `ProfileOptionChoice` with choices; validated generically), default visibility,
   order, and `Fetch(ctx, deps, req)`. Fetch only reads data of `req.OwnerID` (the service already
   checked the viewer), returns `ProfileBlockData` (scenes as `data.Scene`, redacted by the API
   layer; markers as `data.WidgetMarker`; any JSON payload in `Widget`; `Total`; `Empty`).
   Repository queries go in `data.ProfileRepository` (regenerate the mock). Add a test.
2. Frontend: create `web/app/components/profile/blocks/<PascalType>.vue` with the props
   `{ block: ProfileBlock, owner: boolean }` (`ProfileBlockRenderer` finds it by name and frames
   it: title, locked, failed and empty states), the payload type in `types/profile.ts` and the
   icon in `PROFILE_BLOCK_ICONS` (`utils/profile.ts`).
3. Document it in the table above. The edit UI lists it from `block_types` (no change).

## Lists

Next to the blocks, a profile has full lists, opened from tabs under the header (Profile, Likes,
Ratings, Jizz) and from the "See all" links of their blocks:

| List | Page | Privacy item | Sorts (default first) | Figures on each item |
|---|---|---|---|---|
| Liked videos | `/u/<name>/likes` | `liked` | `liked_desc`, `liked_asc` | `at` (like date) |
| Rated videos | `/u/<name>/ratings?rating=` | `ratings` | `rating_desc` (then most recently rated), `rating_asc`, `rated_desc`, `rated_asc` | `rating`, `at` (rating date) |
| Jizz | `/u/<name>/jizz` | `jizz` | `recent` (last O), `count_desc` (most O, then most recent) | `jizz_count`, `at` (last O) |

Rules (`internal/core/profile_lists.go`, for every viewer, superusers included):

- The viewer needs `scenes:view`, and the owner's `profile` item and the list's item must let
  them see it (`CanViewAll`), else `403 PRIVACY_RESTRICTED`. Nothing is read before. The ratings
  chart follows the `ratings` item too: its counts are never sent to someone who may not see it.
- `GET /profiles/users/:username` tells which lists the viewer may open (`lists: {liked,
  ratings, jizz}`, the tabs), in the same privacy call as the blocks.
- Scenes are the live ones only (trashed and deleted ones skipped, in the counts too), redacted
  with the viewer's file visibility. With `?card_fields=` (the user's card template, as on `GET
  /scenes`) the items get the template fields and the response the **viewer's** sidecar maps
  (`ratings`, `likes`, `jizz_counts`, `watched`), so the card badges mean "me" as everywhere
  else; the owner's figures are on the items (`rating`, `jizz_count`, `at`).
- Ratings are half stars from 0.5 to 5 (`InteractionService.SetRating`). The chart rounds a
  rating to its nearest half star (`ROUND(rating * 2) / 2`, for imported ratings with one
  decimal) and the `rating` filter selects the same bucket, so a bar and its list always match.
  `rating` must be a half step in 0.5..5 and is only accepted by the ratings list (`400`).
- Jizz: the videos with a count above 0 (`user_scene_jizzed`); the last O is the last dated
  event (`user_scene_jizz_events`), else the last change of the count.
- Pagination: `page`, `limit` (default 24, max `pagination.max_items_per_page`); the UI uses the
  user's page size and infinite scroll setting.

Choice made: `ratings` and `jizz` default to `friends`, like the other activity items. Jizz is
the most sensitive one: users who want it for themselves set it to "Only me" in Settings >
Privacy (or the block's audience in Edit profile > Blocks).

## Tabs and sections

A profile has tabs, in the owner's order:

- built-in: `profile` (the main tab, always shown, made of sections), `likes`, `ratings`, `jizz`
  (the full lists above; each can be hidden). Each built-in tab appears once, its id is its kind,
  its title and icon are fixed;
- custom tabs (`kind: custom`): a title (1-40 characters), an optional Nuxt icon name
  (`set:name`), a slug (from the title when empty; unique, `[a-z0-9-]`), shown or hidden, made of
  sections like the profile tab.

A section is `{id, template, columns}`: the template is one of `1` (one column), `1-1` (two
equal), `1-1-1` (three equal), `2-1` (2/3 + 1/3), `1-2` (1/3 + 2/3) (`core.ProfileSectionTemplates`,
widths in twelfths); `columns` holds the blocks of each column. Columns stack on small screens.
A block id is unique in the profile (`[a-z0-9][a-z0-9_-]{0,31}`, like tab and section ids); the
same block type may appear several times (one stats block per period, a custom list block per
list).

Saving (`PUT /profiles/me/tabs`) refuses (`400`) unknown kinds, templates, block types or
options, invalid or duplicate ids or slugs, a built-in tab twice or with another id, a custom tab
without title, sections on a list tab, a column count that does not match the template, a
custom list block without one of the owner's lists, more than `profiles.max_tabs` tabs (8, the
built-in ones included), `profiles.max_sections` sections per tab (12) or
`profiles.max_blocks_per_tab` blocks per tab (24); missing built-in tabs are appended. Reading
never fails: unknown kinds, blocks and options are dropped, extra columns join the last one.

Migration of older profiles (in code, `NormalizeProfileTabs`): a profile without tabs is read as
the profile tab holding one full-width section with the blocks of its layout in order (the
older normalization: missing types appended hidden, block ids = their type), followed by the
list tabs: it renders exactly as before. `PUT /profiles/me/layout` (the older API) still works:
it replaces the profile tab with one such section.

Who sees a tab (choice made: tabs have no audience of their own, they follow their blocks):

- the profile tab: every viewer of the profile;
- a list tab: shown by its owner and visible to the viewer (its privacy item and
  `scenes:view`);
- a custom tab: shown by its owner and holding at least one block the viewer gets (visible, or
  sent as a locked placeholder). Its owner always sees it. A hidden or unknown tab asked with
  `?tab=` answers the profile with `tab_missing: true` and no sections.

One `Visibility` call reads every privacy item of all the tabs (blocks, custom list
dependencies, list tabs); only the blocks of the served tab are fetched.

## Custom lists

A custom list is a title, search criteria and an audience. The criteria are the saved search
filters (`data.Filters`: text, tags, actors, studio, resolution, duration, size, dates, status,
liked, rating, O and marker filters) and a sort among the search sorts, which include sorts on
the user's own interactions (`core/search_user_sorts.go`): `my_rating_desc`, `my_rating_asc`,
`liked_at_desc`, `my_views_desc`, `last_watched_desc` besides the O sorts (`jizz_count_*`,
`last_jizz_*`). A list may also follow one of the owner's saved searches (`saved_search_uuid`):
while it exists its current filters are used (a copy is kept in `filters`). The criteria are
stored typed and validated (lengths, known sorts, resolutions, match types, dates, ranges; no
query text reaches SQL). At most `profiles.max_lists` lists per user (20).

The list runs with the **owner's** interactions (their likes, ratings, O, views, markers) through
the search (`SearchService`), so a criterion on them reveals owner data. Each one depends on a
privacy item:

| Criterion | Privacy item |
|---|---|
| liked, `liked_at_desc` | `liked` |
| rating range, rated / unrated, `my_rating_*` | `ratings` |
| O range, `jizz_count_*`, `last_jizz_*` | `jizz` |
| watched / unwatched, `my_views_desc`, `last_watched_desc` | `activity` |
| with / without markers, marker labels | `recent_markers` |

A viewer gets a list (its block, or `GET .../lists/:id`) only when they have `scenes:view`, pass
the `profile` item, the list's own audience (default `public`) and every item its criteria
depend on (no administrator bypass), and have `files:view_size` when the criteria use sizes
(size bounds or sorts). Otherwise the block is omitted or locked and the page answers `403
PRIVACY_RESTRICTED`. The search runs with the viewer's file permissions: without
`files:view_paths` the text is not matched on paths. Trashed scenes are skipped and the scenes
redacted for the viewer. A viewer never gets the criteria, only the title. Tags that do not
exist make the list empty (never wider). The editor shows the dependencies of a list and their
current audiences (`dependencies` of `GET /profiles/me/lists`).

The `custom_list` block shows the first `count` videos (grid or compact row) with a "Browse"
link to `/u/<name>/lists/<id>` (the whole list, paginated, the user's card template). A deleted
list leaves its blocks empty: they are shown to nobody.

## Images

Avatars and uploaded banners are GoonHub data, stored in `profiles.image_dir` (default
`./data/metadata/profiles`, in Docker `/app/data/metadata/profiles`, the data volume; never a
media directory): `avatars/<user_id>_<sm|md|lg>.jpg` (64, 160, 400 px squares) and
`banners/<user_id>_<md|lg>.jpg` (750x250, 1500x500).

Processing (`internal/core/profile_image.go`, standard library only):

1. The upload is read up to `profiles.max_upload_bytes` (default 8 MB; the request body is
   capped too).
2. The header is decoded first (`image.DecodeConfig`): only JPEG, PNG and GIF (first frame);
   at least 16x16, at most 20000 px a side and `profiles.max_image_pixels` (default 25 M)
   pixels, so a decompression bomb is refused before any decoding. Anything else is `400`.
3. The optional crop rectangle (`crop_x`, `crop_y`, `crop_w`, `crop_h`, source pixels, from the
   browser cropper) is clamped to the image, then reduced to the target aspect around its
   center (no crop = center crop).
4. Transparent pixels go on the app background, the image is resized with an area-averaging
   filter and re-encoded as JPEG (quality 88): metadata and extra frames are dropped.
5. Each size is written atomically (temporary file + rename); the version (Unix ms) is stored
   and appended to the URLs (`?v=`), so the files are cached for a year and a new upload is
   fetched at once. Removing an avatar or a banner deletes its files.

Uploads are limited per user (`profiles.image_uploads_per_hour`, default 20, in memory, burst
of half; `429 SOCIAL_RATE_LIMITED`). No permission gates them (a signed-in user edits their own
profile): on an instance of a few users, the size, pixel and rate limits are enough (a
`profiles:upload_images` permission can be added to the catalog later if abuse matters). EXIF orientation is not applied (a phone photo may need rotating first).

The images are served to signed-in users only (cookie auth also works for `<img>`):
`/api/v1/profiles/avatar/:userId/:size` (no privacy item) and
`/api/v1/profiles/banner/:userId/:size` (needs the `profile` item), `Cache-Control: private,
max-age=31536000, immutable`. A scene banner is the scene thumbnail (`/thumbnails/:id?size=lg`).

## API

Every route needs a signed-in user. Editing one's own profile needs nothing more; the routes
that pick scenes or markers need `scenes:view`.

| Route | Permission | Answer |
|---|---|---|
| `GET /api/v1/profiles/users/:username` | | the social summary (`user`, `relationship`, `visibility`, `counts`, as `GET /social/users/:username`) + `bio`, `banner: {type, url, url_md, scene_id?} \| null`, `blocks: [...]` (the blocks of the served tab), `lists: {liked, ratings, jizz}`, `tabs: [{id, kind, title, icon?, slug?}]` (those the viewer may open), `tab`, `tab_missing?`, `sections: [{id, template, columns: [[block]]}]`; `?tab=` a tab id or custom tab slug (default the profile tab) |
| `GET /api/v1/profiles/users/:username/ratings/histogram` | `scenes:view` | `{buckets: [{rating, count}] (10, ascending), total, average}`; `403 PRIVACY_RESTRICTED` unless `profile` and `ratings` |
| `GET /api/v1/profiles/users/:username/ratings?rating=&sort=&page=&limit=&card_fields=` | `scenes:view` | list (below); needs `profile` and `ratings`; `400` for an unknown sort or a rating off the scale |
| `GET /api/v1/profiles/users/:username/likes?sort=&page=&limit=&card_fields=` | `scenes:view` | list; needs `profile` and `liked` |
| `GET /api/v1/profiles/users/:username/jizz?sort=&page=&limit=&card_fields=` | `scenes:view` | list; needs `profile` and `jizz` |
| `GET /api/v1/profiles/users/:username/lists/:id?page=&limit=&card_fields=` | `scenes:view` | `{list: {id, title}, data: [{scene}], pagination, ratings?, likes?, jizz_counts?, watched?}`; `403 PRIVACY_RESTRICTED` unless visible (above); `404` unknown list |
| `PUT /api/v1/profiles/me/tabs` `{tabs}` or `{reset: true}` | | `{tabs}` normalized; `400` (see "Tabs and sections") |
| `GET /api/v1/profiles/me/lists` | `scenes:view` | `{lists: [list]}` |
| `POST /api/v1/profiles/me/lists` `{title, filters, saved_search_uuid?, audience?}` | `scenes:view` | `201` list; `400` invalid criteria, title, audience, unknown saved search or over the cap; `403` size criteria without `files:view_size` |
| `PUT /api/v1/profiles/me/lists/:id` (same body) | `scenes:view` | the list |
| `DELETE /api/v1/profiles/me/lists/:id` | | `204`; `404` |
| `GET /api/v1/profiles/me` | | edit state: `{user, display_name, bio, avatar_version, banner: {type, scene_id?, version, url?}, layout: [...] (the blocks of the profile tab), tabs: [...], lists: [list], section_templates: [{id, label, widths}], favorite_scenes: [SceneListItem], favorite_markers: [marker], block_types: [...], privacy: <GET /social/privacy>, limits: {max_showcase_items, bio_max_length, display_name_max_length, max_upload_bytes}}` |
| `PATCH /api/v1/profiles/me` `{display_name?, bio?}` | | the edit state; `400` over the limits |
| `PUT /api/v1/profiles/me/layout` `{blocks: [...]}` or `{reset: true}` | | `{blocks}` normalized |
| `PUT /api/v1/profiles/me/showcases/scenes` `{scene_ids}` | `scenes:view` | `{scenes: [SceneListItem]}`; `400` too many, duplicate, unknown or trashed scene |
| `PUT /api/v1/profiles/me/showcases/markers` `{marker_ids}` | `scenes:view` | `{markers}`; `400` for a marker the user does not see |
| `GET /api/v1/profiles/me/marker-candidates?q=&page=&limit=` | `scenes:view` | paginated markers the user sees, label or scene title containing `q`, newest first |
| `POST /api/v1/profiles/me/avatar` multipart `image` + `crop_*` | | `{version}` |
| `DELETE /api/v1/profiles/me/avatar` | | `204` |
| `POST /api/v1/profiles/me/banner` multipart `image` + `crop_*` | | `{version}` |
| `PUT /api/v1/profiles/me/banner` `{type: "scene", scene_id}` or `{type: "none"}` | (`scenes:view` for a scene) | `204` |
| `DELETE /api/v1/profiles/me/banner` | | `204` |
| `GET /api/v1/profiles/avatar/:userId/:size` | | JPEG (`sm`, `md`, `lg`) or `404` |
| `GET /api/v1/profiles/banner/:userId/:size` | | JPEG (`md`, `lg`); `403 PRIVACY_RESTRICTED` unless the profile is visible |

A custom list (owner): `{id, title, filters, saved_search_uuid, follows_saved_search, audience,
dependencies: [{key, label, audience}], needs_size, updated_at}`. The limits also carry
`max_tabs`, `max_sections`, `max_blocks_per_tab`, `max_lists`; a block type `multi: true`
(custom_list).

A list: `{data: [{scene, at?, rating?, jizz_count?}], pagination: {page, limit, total_items,
total_pages}, sort, ratings?, likes?, jizz_counts?, watched?}` (the sidecar maps: the viewer's
own interactions, with `card_fields`).

A block: `{type, label, privacy_item, config, locked, failed?, empty, scenes?: [{scene, at?,
views?, rating?, jizz_count?}], markers?, widget?, total?}` (`config` with the defaults filled in). A block type:
`{type, label, description, privacy_item, viewer_permission?, options: [{key, label, kind:
"int"|"choice", min?, max?, default, choices?}]}`.

The whole page is one request: the blocks are a handful of indexed queries each and the page
is shown at once (no block by block loading).

The social user cards (`/social/...` lists, directory, `GET /social/me` `user`) carry
`display_name` and `avatar_version` (0 = no avatar, initials).

## Frontend

- `pages/u/[username].vue` (orchestrator, parent route): loads the profile with the tab of the
  route (`tabKeyOfPath`), `ProfileHeader` (banner, avatar, names, `ProfileBio`, counts, badges
  and actions, `Edit profile` for the owner, a `presence` slot next to the name for an online /
  now watching badge), `ProfileTabs` (the tabs of `profile.tabs`, `tabPath`), the child page
  (`<NuxtPage>`, props `profile` and `owner`) and the relation lists (`SocialProfileLists`,
  opened from the counts).
- Child pages in `pages/u/[username]/`: `index.vue` and `t/[tab].vue` (a custom tab,
  `/u/<name>/t/<slug>`) render `ProfileSections` (the sections as 12-column grids from `md`,
  stacked below; the blocks: `ProfileBlockRenderer` + `components/profile/blocks/<Type>.vue`,
  sharing `ProfileScenesGrid`, `ProfileSceneFigure` and `ProfileMarkersGrid`; the frame shows
  "See all" for the blocks of `PROFILE_BLOCK_LISTS` and "Browse" for a custom list),
  `lists/[id].vue` (a custom list browsed in full),
  `likes.vue`, `ratings.vue` (the chart, a click on a bar toggles the `?rating=` filter, shown
  as a removable chip) and `jizz.vue`. The lists are `ProfileSceneList` (the user's card
  template and grid density, pagination or infinite scroll per the user setting, page and sort
  in the URL, the owner's figure under each card: `ProfileSceneFigure`); `ProfileListLocked`
  when the list is not visible.
- The ratings chart is `ProfileRatingHistogram` (muted bars, lava on hover and selection, a star
  at each end, tooltip "12 ratings of 3.5 stars (8%)", the average under it); its math is in
  `utils/ratingHistogram.ts` (tests in `web/tests/ratingHistogram.test.ts`).
- Edit mode (`?edit=1`, owner only): `ProfileEditPanel` with the tabs Profile
  (`ProfileEditIdentity`: display name, bio; `ProfileEditImages`: avatar and banner upload with
  `ProfileEditImageCropper`, remove, banner from a favorite video), Tabs & layout
  (`ProfileEditTabs`: `ProfileEditTabList` to reorder (drag and drop or arrows), show, rename,
  pick an icon, add and delete custom tabs; `ProfileEditSections` / `ProfileEditSection` for the
  sections of the selected tab: add, remove, reorder, template, and the blocks moved by drag and
  drop between columns and sections or with the arrows of `ProfileEditBlock` (up, down, left,
  right), shown toggle, "Private" placeholder, options, and who can see the block: its privacy
  item's audience, saved at once through `PUT /social/privacy`, shared by the blocks of the same
  item; a custom list block picks a list and shows its audience and dependencies; saved with the
  button), Custom lists (`ProfileEditLists`: presets Top O, Top rated, Recently liked, Most
  watched, Newest; `ProfileEditListForm`: title, audience, criteria or a saved search, the
  dependencies and their audiences; `ProfileEditListCriteria`: the search interface on its own
  store, `useProfileListSearchStore`, with a preview of the first results), Favorite videos and Favorite markers (`ProfileEditShowcase`:
  drag and drop, remove, replace, and "Add" with the search picker `PickerModal`, Videos or
  Markers tab, multi-select up to the free slots, single select to replace a slot; saved on each
  change). The picker's Markers tab lists `GET /markers/all?live_only=true`: the markers the user
  sees on scenes still in the library, the same rule as `marker-candidates` (still used by
  nothing else in the UI).
- Avatars everywhere a user is shown: `SocialUserAvatar` (`userId`, `avatarVersion`,
  `displayName`, initials fallback), used by the user rows, the friends block and the header,
  where the avatar and name link to "My profile" (`useSocialStore().me`, from `/social/me`).
- Helpers: `useApiProfiles()`, `useDragReorder()`, `utils/profile.ts` (`avatarUrl`,
  `bioSegments` (links as text nodes, http(s) only), `cropFromView`, `moveItem`...), tests in
  `web/tests/profile.test.ts`; `utils/profileLayout.ts` (templates and column classes, ids,
  `moveBlock`, `neighbourPosition`, `setSectionTemplate`, tab paths, `criteriaDependencies`
  mirroring the server, presets), tests in `web/tests/profileLayout.test.ts`.

## Configuration

```yaml
profiles:
  image_dir: ./data/metadata/profiles  # GoonHub data, never a media directory
  max_upload_bytes: 8388608
  max_image_pixels: 25000000
  image_uploads_per_hour: 20           # per user (0 = unlimited)
  max_showcase_items: 4                # 1-12
  bio_max_length: 1000                 # characters (5000 at most, the database check)
  display_name_max_length: 64          # characters (64 at most, the column)
  max_tabs: 8                          # tabs of a profile, built-in ones included (4-20)
  max_sections: 12                     # sections per tab (1-40)
  max_blocks_per_tab: 24               # blocks per tab (1-100)
  max_lists: 20                        # custom lists per user (1-200)
```

## Known limitations

- Customized layouts get the `ratings` and `jizz` blocks appended hidden (like every block type
  added later): their owners turn them on in Edit profile > Tabs & layout. The list tabs do not
  depend on the blocks.
- A custom list whose source saved search is deleted falls back to the copy of its filters taken
  when it was saved. A list with a random sort draws a new order on each load.
- Custom lists run a Meilisearch search per block on each profile view (a few per page).
- The stats block counts every rating (`rated`, `avg_rating`), trashed scenes included; the
  ratings chart and lists only count the scenes still in the library.

- No permission gates avatar and banner uploads (only the limits above).
- EXIF orientation is ignored; WebP uploads are refused (no decoder in the standard library).
- The image files of a deleted user are left in `image_dir` (the rows cascade); they are never
  served by a URL anyone has.
- The upload rate limit is in memory: it resets when the server restarts.
