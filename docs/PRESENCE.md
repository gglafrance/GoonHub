# Presence

Who among my friends and the people I follow is online, idle or offline, and what they are
watching. Built on the social graph and its privacy framework (`docs/SOCIAL.md`).

Code: `internal/core/presence_service.go` (state, reports), `internal/core/presence_push.go`
(loop, pushes, snapshot), `internal/core/presence_types.go` (types, JSON shapes, activity types),
`internal/data/presence_repository.go`, `internal/api/v1/handler/presence.go`. Frontend:
`stores/presence.ts`, `composables/usePresenceActivity.ts`, `utils/presence.ts`,
`components/header/FriendsPresence*.vue`, `components/social/Presence*.vue`.

## Model

Everything lives in memory (one process), except `user_presence.last_seen_at` and the invisible
mode (migration 100, `docs/DATABASE.md`).

| Status | When |
|---|---|
| `online` | the user has at least one live connection (an SSE stream `/api/v1/events`), or the last one closed less than `disconnect_grace` ago |
| `idle` | online, but no activity for `idle_after`: no input heartbeat and no playing scene |
| `offline` | otherwise; shown with `last_seen_at` |

- **Connections.** The SSE handler calls `PresenceService.Connect` when a stream opens and
  `Disconnect` when it closes. Connections are counted per user (several devices or browsers;
  the tabs of one browser share one stream through the shared worker or the leader tab). When
  the count drops to 0 the user stays online for `disconnect_grace` (a page reload reconnects
  within it: nothing is pushed).
- **Activity (not idle).** Each tab sends `POST /social/presence/heartbeat {active: true}` on
  user input (pointer, key, wheel, touch, tab shown), at most every `heartbeat_interval`. A
  playing (not paused) scene report counts as activity too.
- **Last seen.** Saved when the grace period ends (the disconnection time), every
  `last_seen_interval` while online (cheap, survives a crash) and for every online user at
  shutdown. `SaveLastSeen` never moves it back. Not saved while invisible.
- **Invisible mode** (`PUT /social/presence/settings {invisible}`, Settings > Privacy): the user
  appears offline to everyone, without activity; their last seen stays at the moment they turned
  it on. Turning it off shows them again at the next flush.

### Activities

An activity is reported per tab (`tab_id`, random per page load), so several tabs do not erase
each other. The activity shown is a playing one before a paused one, then the most recently
reported. Activities are only shown while the user is online (not offline, not invisible).

`watching_scene` (watch page, also its playlist playback mode; not share links): the player
(`usePresenceActivity`, enabled by the `presence` prop of `ScenePlayer`) reports
`PUT /social/presence/activity {tab_id, type: "watching_scene", scene_id, position, paused}`
once playback started, on play, pause, seek, end and scene change, every `activity_interval`
otherwise (also while paused, to keep it alive), and `DELETE /social/presence/activity?tab_id=`
when the player is destroyed or the page is hidden (`pagehide`, keepalive). An activity not
reported for `activity_ttl` ends (closed tab, crash). Reporting needs `scenes:view`; the scene
must exist and not be trashed (`404` otherwise). The scene card (title, duration, thumbnail URL)
is read once when the activity starts.

- **Paused**: shown as paused only once paused for `paused_after` (short pauses are not pushed).
- **Progress**: the view carries `position` at `position_at`; while not paused the client
  extrapolates it. A normal progress is not pushed; a jump farther than `seek_threshold` from the
  expected position (a seek) is.

## Privacy

Items of the catalog (`core/privacy_catalog.go`), default `friends`:

- `online_status`: the presence at all (status, last seen). Without it the viewer gets nothing
  (`{visible: false}`).
- `now_watching`: the activity; it also needs `online_status`.

There is no administrator bypass. On top of that, the scene of an activity is redacted with the
**viewer's** permissions: without `scenes:view`, the activity only says its type ("watching
something": no scene, position nor pause state). A presence never carries file information
(no path, size, codec), only title, duration and `/thumbnails/:id?v=` .

Changes apply at the next flush (at most `flush_interval`), without reconnecting:

- a privacy change of `online_status` or `now_watching` (`PrivacyService.OnChange`);
- a follow, unfollow, friendship made or removed (`SocialService.OnRelationChange`, both users);
- the invisible mode.

The service remembers, per live viewer, the last view it sent of each owner (pushes and
snapshot); when a viewer may no longer see an owner, it is sent `{visible: false}` (no stale
presence). A viewer is forgotten `disconnect_grace` + 1 minute after their last connection.

## Pushes (SSE)

`presence:update`, targeted (`EventBus.PublishToUser`, `presence:` is in
`sseTargetedPrefixes`), `data` = a presence view:

```json
{
  "user": {"id": 2, "username": "bob"},
  "visible": true,
  "status": "online",
  "last_seen_at": "2026-11-26T12:00:00Z",
  "activity": {
    "type": "watching_scene",
    "started_at": "2026-11-26T12:01:00Z",
    "scene": {"id": 10, "title": "...", "duration": 600, "thumbnail_url": "/thumbnails/10?v=..."},
    "position": 123.4,
    "position_at": "2026-11-26T12:03:00Z",
    "paused": false
  },
  "seq": 42
}
```

`last_seen_at` only when offline; `activity` only when online with one and allowed;
`scene`/`position`/`position_at`/`paused` only when the viewer may see the scene. `visible:
false` views have only `user` and `seq`. `seq` grows with every view sent (pushes and
snapshots): a client keeps the highest per user (`applyPresenceView`), so a late snapshot cannot
override a newer push and an old push cannot bring a revoked presence back.

Coalescing: reports only change the memory state. One goroutine (lifecycle manager, started and
stopped by the server) runs every `flush_interval`: it ends the grace periods, expires the
activities, detects idle and paused transitions, then, for each user whose visible state changed
(or marked by a privacy/relation change), computes the view of every live viewer and pushes only
the ones that differ from what that viewer was last sent. Heartbeats and normal progress push
nothing. Per flushed user: two audience queries (`AudienceMembers`), plus the permissions of the
viewers (cached by the RBAC service) when there is an activity.

Memory: one entry per online user (dropped once pushed offline), at most
`max_tabs_per_user` activities each (the oldest is dropped), one "sent" map per live viewer.

## API

Every route needs a signed-in user.

| Route | Answer |
|---|---|
| `GET /api/v1/social/presence` | `{users: [view], invisible, heartbeat_interval_seconds, activity_interval_seconds}`: my friends and the users I follow whose presence I may see (hidden ones left out), by username |
| `GET /api/v1/social/presence/users/:userId` | one view (`{visible: false}` when hidden; `404` unknown user) |
| `POST /api/v1/social/presence/heartbeat` `{active}` | `204` |
| `PUT /api/v1/social/presence/activity` `{tab_id?, type, scene_id, position, paused}` | `204`; `400` invalid (type, scene_id, negative position, tab_id not `[A-Za-z0-9_-]{1,64}`), `403` without `scenes:view`, `404` unknown or trashed scene |
| `DELETE /api/v1/social/presence/activity?tab_id=` | `204` |
| `GET /api/v1/social/presence/settings` | `{invisible}` |
| `PUT /api/v1/social/presence/settings` `{invisible}` | `{invisible}` |

## Frontend

- Header: `HeaderFriendsPresence` (count of contacts online or idle, popover "Friends online"
  grouped Watching / Online / Idle / Offline with last seen). A watching entry shows the
  thumbnail, title, progress, "Paused", and "Watch this too": from the start or at their
  position (`/watch/:id?t=`), not synchronized.
- `SocialPresenceDot` on the avatars of the user rows (friends page, lists) and of the profile
  header; `SocialPresenceBadge` in the profile header (loads `GET .../users/:id` once).
- Settings > Privacy: the "Invisible" toggle at the top of the Presence group.
- The store (`usePresenceStore`) loads the snapshot at start, after an SSE reconnection and
  after a social change (`useSocialStore().version`), and applies the `presence:update` events.

## Configuration

```yaml
presence:
  disconnect_grace: 30s     # still online this long after the last live connection closed
  idle_after: 5m            # no input nor playback for this long = idle
  heartbeat_interval: 60s   # the client reports input at most this often
  activity_interval: 15s    # the client reports the playback position this often
  activity_ttl: 60s         # an activity not reported for this long ends (> activity_interval)
  paused_after: 20s         # a playback paused this long shows as paused
  seek_threshold: 15s       # a jump this big in the position is pushed
  flush_interval: 2s        # changes are coalesced and pushed at this interval
  last_seen_interval: 5m    # last seen of online users saved this often
  max_tabs_per_user: 8      # activities kept per user (one per tab)
```

`heartbeat_interval` and `activity_interval` are sent to the clients by the snapshot.

## Adding an activity type

For example `in_lobby` (goon2gether) or a marker playlist playback:

1. Add a constant next to `ActivityWatchingScene` (`core/presence_types.go`) and the fields it
   needs to `PresenceActivityInput` and `presenceActivity` (e.g. `LobbyID`), and to
   `request.PresenceActivityRequest`.
2. Validate it in `validateActivity`; check in the handler that the user may do it (like
   `scenes:view` for `watching_scene`); read what it shows once when it starts (as `sceneCard`).
3. Show it in `activityView`, redacted for the viewer (who may join the lobby? who may see its
   name?), and add what changes visibly to `presenceKey` in `viewFor` (only what must be pushed).
   If it needs its own audience, add a privacy item (`docs/SOCIAL.md`) and read its
   `AudienceMembers` in `flushOwner` and `VisibleTo` in the snapshot.
4. Frontend: report it with `useApiPresence().reportActivity` (its own `tab_id` scheme is fine),
   clear it when leaving, and render it in `SocialPresenceEntry` / `activityLabel`
   (`utils/presence.ts`) and `presenceGroupOf` (a new group if needed).

## Limits

- Single process: presence is lost on restart (users reconnect within seconds; last seen is
  saved at shutdown).
- The scene card is read when the activity starts: a scene renamed or trashed meanwhile keeps
  its card until the next scene.
- A user who closed every tab without `pagehide` (crash) keeps their activity for
  `activity_ttl` and stays online for `disconnect_grace`.
