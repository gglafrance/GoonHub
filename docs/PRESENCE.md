# Presence

Who among my friends and the people I follow is online, away or offline, and what they are
watching. Built on the social graph and its privacy framework (`docs/SOCIAL.md`).

Code: `internal/core/presence_service.go` (state, reports), `internal/core/presence_push.go`
(loop, pushes, snapshot), `internal/core/presence_types.go` (types, JSON shapes, activity types),
`internal/data/presence_repository.go`, `internal/api/v1/handler/presence.go`. Frontend:
`stores/presence.ts`, `utils/presenceTracker.ts` (what counts as activity in a tab),
`composables/usePresenceActivity.ts`, `utils/presence.ts`,
`components/header/FriendsPresence*.vue`, `components/social/Presence*.vue`.

## Model

Everything lives in memory (one process), except `user_presence.last_seen_at` and the invisible
mode (migration 098, `docs/DATABASE.md`).

| Status | Shown as | When |
|---|---|---|
| `online` | Online (green), or Watching (red) with an activity | connected, and a real input or a playing video less than `idle_after` ago |
| `away` | Away (amber), with the activity left open if any ("Paused on X") | connected, but no real input and no playing video for `idle_after` |
| `offline` | Offline, "last seen" | no live connection (after `disconnect_grace`), or invisible |

Connected: at least one live connection (an SSE stream `/api/v1/events`), or the last one closed
less than `disconnect_grace` ago. The API value `away` replaces the former `idle` (the server and
the web app ship together).

- **Connections.** The SSE handler calls `PresenceService.Connect` when a stream opens and
  `Disconnect` when it closes. Connections are counted per user (several devices or browsers;
  the tabs of one browser share one stream through the shared worker or the leader tab). When
  the count drops to 0 the user stays online for `disconnect_grace` (a page reload reconnects
  within it: nothing is pushed). A connection is **not** an activity, except as a fallback: a
  user coming online who never sent a heartbeat counts as active until their first one.

### Online and away: the exact rules

The server keeps, per user, the time of the last activity (`lastActive`); the user is away once
it is `idle_after` old (checked at every flush, so the switch shows within `idle_after` +
`flush_interval`). It moves forward on:

1. **A heartbeat** `POST /social/presence/heartbeat {active: true, idle_seconds}`: the last
   real input in that tab was `idle_seconds` ago. The latest of all the tabs and devices counts
   (a heartbeat never moves it back). The first heartbeat after a connection also ends the
   connection fallback above.
2. **A playing report** `PUT /social/presence/activity` with `paused: false` whose position
   moved since the previous report of the tab (or that starts or resumes the playback). A paused
   or ended playback, or a stalled one (same position), does not count.

Nothing else counts: no SSE (re)connection (but the fallback), no other API call, no tab
shown or focused, no paused video kept alive by its periodic reports.

The client (`PresenceTracker`, one per tab, started with the header) decides what it reports:

- **Real input**: `pointerdown`, `keydown`, `wheel`, `touchstart` (window, capture phase), only
  when trusted (`isTrusted`, not dispatched by a script) and the tab visible; `mousemove` only
  once the pointer moved `PRESENCE_MOUSE_MOVE_MIN_PX` (12 px, screen coordinates) from where it
  was last counted (the browser fires mousemove without any move when the page changes under a
  still pointer; jitter). `scroll` does not count (programmatic scrolls fire it too: a scroll by
  the user comes with a wheel, key, touch or pointer down). A tab shown again
  (`visibilitychange`), a focus or a reconnection is not an input.
- **Playing video**: every `ScenePlayer` (watch page, playlist, Goon2gether lobby, share page
  when signed in) registers a probe (`presenceStore.registerPlayback`): playing = not paused, not
  ended and the position moved less than `stalled_after` ago (`isPlaybackActive`). A playing
  video counts in any tab, visible or not; the moment it stops counts too (away is measured from
  the pause). The probe is checked every `PRESENCE_TICK_MS` (5 s).
- **Throttle**: the first activity is sent at once, the next ones within the same period
  (`min(heartbeat_interval, idle_after / 2)`) at the end of it (trailing heartbeat with the time
  since the last activity), so the server always knows the real last input within one period.
- **Back from away**: when the last activity this tab reported is about `idle_after` old (the
  server may show the user away), the next input is sent at once, without the throttle: the user
  is back online at the next flush.
- **Sync**: at start and whenever the live stream connects again (`jobStatus.isConnected` back
  to true: server restart, network loss), the tab sends its real idle time at once, so that a
  reconnection is not taken for an activity.

Watching then pausing: the activity shows as paused after `paused_after`; the user stays
`online` (Watching, paused) until `idle_after` after the pause or the last input, then is `away`
with the paused activity ("Away, Paused on X"). Several tabs: the user is online while any tab
has input or plays; a background tab with nothing playing adds nothing.

Each transition (online to away, away to online) changes the pushed key: it is pushed once to
every viewer allowed to see it (targeted, privacy unchanged), never repeated while nothing
changes.
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
activities, detects away and paused transitions, then, for each user whose visible state changed
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
| `GET /api/v1/social/presence` | `{users: [view], invisible, heartbeat_interval_seconds, activity_interval_seconds, idle_after_seconds, stalled_after_seconds}`: my friends and the users I follow whose presence I may see (hidden ones left out), by username |
| `GET /api/v1/social/presence/users/:userId` | one view (`{visible: false}` when hidden; `404` unknown user) |
| `POST /api/v1/social/presence/heartbeat` `{active, idle_seconds?}` | `204` (`active: false` is ignored; `idle_seconds`: time since the last input, default 0, capped at a week); `400` negative `idle_seconds`. Accepted before the stream opens |
| `PUT /api/v1/social/presence/activity` `{tab_id?, type, scene_id, position, paused}` | `204`; `400` invalid (type, scene_id, negative position, tab_id not `[A-Za-z0-9_-]{1,64}`), `403` without `scenes:view`, `404` unknown or trashed scene |
| `DELETE /api/v1/social/presence/activity?tab_id=` | `204` |
| `GET /api/v1/social/presence/settings` | `{invisible}` |
| `PUT /api/v1/social/presence/settings` `{invisible}` | `{invisible}` |

## Frontend

- Groups (`presenceGroupOf`): offline; `away` (even with an activity: a video left paused);
  watching (online with an activity); online. Labels in `PRESENCE_GROUP_LABELS`, colors: watching
  red, online green, away amber, offline grey.
- Header: `HeaderFriendsPresence` (count of contacts online, watching or away; its dot is green
  when one is active, amber when all are away; the title says "N online, M away"), popover
  "Friends online" grouped Watching / Online / Away / Offline with last seen. A watching entry
  shows the thumbnail, title, progress, "Paused", and "Watch this too": from the start or at
  their position (`/watch/:id?t=`), not synchronized. An away entry shows "Away" and what was
  left open (`awayActivityLabel`: "Paused on X", linked to the scene).
- `SocialPresenceDot` on the avatars of the user rows (friends page, lists, messages list and
  thread, share recipients) and of the profile header (title "Away, Paused on X");
  `SocialPresenceBadge` in the profile header (loads `GET .../users/:id` once); the messages
  thread header says Watching / Online / Away.
- Settings > Privacy: the "Invisible" toggle at the top of the Presence group.
- The store (`usePresenceStore`) loads the snapshot at start, after an SSE reconnection and
  after a social change (`useSocialStore().version`), and applies the `presence:update` events.

## Configuration

```yaml
presence:
  disconnect_grace: 30s     # still online this long after the last live connection closed
  idle_after: 5m            # no real input nor playing video for this long = away
  heartbeat_interval: 60s   # the client reports input at most this often (and half of idle_after)
  activity_interval: 15s    # the client reports the playback position this often
  activity_ttl: 60s         # an activity not reported for this long ends (> activity_interval)
  paused_after: 20s         # a playback paused this long shows as paused
  stalled_after: 30s        # a playback not progressing (buffering) this long no longer counts as playing
  seek_threshold: 15s       # a jump this big in the position is pushed
  flush_interval: 2s        # changes are coalesced and pushed at this interval
  last_seen_interval: 5m    # last seen of online users saved this often
  max_tabs_per_user: 8      # activities kept per user (one per tab)
```

`heartbeat_interval`, `activity_interval`, `idle_after` and `stalled_after` are sent to the
clients by the snapshot. Client constants (`utils/presenceTracker.ts`): the mouse move threshold
(12 px) and the tick of the tracker (5 s). Timings of the switch, with the defaults: away 5 min
after the last input or playback (+ up to 2 s of flush); back online at the next flush after an
input; a stalled playback stops counting after 30 s (client) or at its next report with the same
position (server).

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
