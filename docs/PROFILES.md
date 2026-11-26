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
- **Layout**: the ordered list of blocks, each `{type, visible, locked_placeholder, config}`,
  stored as JSONB and validated by the server. NULL = the default layout.

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
   in the library), and the visible blocks of the layout, in order:
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
| `activity_calendar` | `activity` | | `weeks` 4-104 (26), `metric` views/jizz/both (views) | shown |
| `playlists` | `playlists` | `playlists:view_public` | `count` (6) | shown |
| `friends` | `friends_list` | | `count` (8) | shown |

Periods: `week` = last 7 days, `month` = last 30 days, `year` = last 365 days, `all`.

- `stats`: watch time (sum of the watch durations), views, distinct videos watched, O (dated O
  of the period; all time also counts the undated totals), and as totals liked videos, markers,
  ratings and the average rating; the top 5 tags, actors and studios by views of the period.
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
| `GET /api/v1/profiles/users/:username` | | the social summary (`user`, `relationship`, `visibility`, `counts`, as `GET /social/users/:username`) + `bio`, `banner: {type, url, url_md, scene_id?} \| null`, `blocks: [...]` |
| `GET /api/v1/profiles/me` | | edit state: `{user, display_name, bio, avatar_version, banner: {type, scene_id?, version, url?}, layout: [...], favorite_scenes: [SceneListItem], favorite_markers: [marker], block_types: [...], privacy: <GET /social/privacy>, limits: {max_showcase_items, bio_max_length, display_name_max_length, max_upload_bytes}}` |
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

A block: `{type, label, privacy_item, config, locked, failed?, empty, scenes?: [{scene, at?,
views?}], markers?, widget?, total?}` (`config` with the defaults filled in). A block type:
`{type, label, description, privacy_item, viewer_permission?, options: [{key, label, kind:
"int"|"choice", min?, max?, default, choices?}]}`.

The whole page is one request: the blocks are a handful of indexed queries each and the page
is shown at once (no block by block loading).

The social user cards (`/social/...` lists, directory, `GET /social/me` `user`) carry
`display_name` and `avatar_version` (0 = no avatar, initials).

## Frontend

- `pages/u/[username].vue` (orchestrator): `ProfileHeader` (banner, avatar, names, `ProfileBio`,
  counts, badges and actions, `Edit profile` for the owner, a `presence` slot next to the name
  for an online / now watching badge), the blocks (`ProfileBlockRenderer` +
  `components/profile/blocks/<Type>.vue`, sharing `ProfileScenesGrid` and `ProfileMarkersGrid`),
  and the relation lists (`SocialProfileLists`, opened from the counts).
- Edit mode (`?edit=1`, owner only): `ProfileEditPanel` with the tabs Profile
  (`ProfileEditIdentity`: display name, bio; `ProfileEditImages`: avatar and banner upload with
  `ProfileEditImageCropper`, remove, banner from a favorite video), Blocks (`ProfileEditLayout`:
  drag and drop or arrows, shown toggle, "Private" placeholder toggle, options, and who can see
  the block: its privacy item's audience, saved at once through `PUT /social/privacy`, shared by
  the blocks of the same item), Favorite videos and Favorite markers (`ProfileEditShowcase`:
  drag and drop, remove, search to add; saved on each change).
- Avatars everywhere a user is shown: `SocialUserAvatar` (`userId`, `avatarVersion`,
  `displayName`, initials fallback), used by the user rows, the friends block and the header,
  where the avatar and name link to "My profile" (`useSocialStore().me`, from `/social/me`).
- Helpers: `useApiProfiles()`, `useDragReorder()`, `utils/profile.ts` (`avatarUrl`,
  `bioSegments` (links as text nodes, http(s) only), `cropFromView`, `moveItem`...), tests in
  `web/tests/profile.test.ts`.

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
```

## Known limitations

- No permission gates avatar and banner uploads (only the limits above).
- EXIF orientation is ignored; WebP uploads are refused (no decoder in the standard library).
- The image files of a deleted user are left in `image_dir` (the rows cascade); they are never
  served by a URL anyone has.
- The upload rate limit is in memory: it resets when the server restarts.
