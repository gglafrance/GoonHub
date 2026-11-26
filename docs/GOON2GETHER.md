# Goon2gether: synchronized watch lobbies

Goon2gether lets several users watch the same scenes at the same time, in a **lobby**: a shared
playing queue, playback kept in sync by the server, a chat with emoji and custom emotes and
reactions on its messages, and floating reactions over the video. It is like Watch2Gether or
Teleparty, inside GoonHub.

- Backend: `internal/together` (hub, lobbies, playback, emotes), `internal/data/together_*.go`
  (models and repositories), `internal/api/v1/handler/together*.go` (REST and WebSocket).
- Frontend: `web/app/pages/together/` (lobby browser, room, join by code),
  `web/app/components/together/`, `web/app/composables/useTogether*.ts`; the chat message
  reactions use `web/app/components/chat/Reactions.vue` (shared with the messages,
  `docs/MESSAGES.md`) and `web/app/utils/reactions.ts`.
- Schema: migration 099 (`lobbies`, `lobby_members`, `lobby_queue_items`, `emotes`) and 101
  (auto-next mode, anonymous members), see `docs/DATABASE.md`.

## Concepts

- **Lobby**: named room, `public` (listed on `/together`) or `private` (not listed, joined with
  its invite code or link `/together/join/<code>`). Identified by a UUID in the API.
- **Host**: one member (the creator at first). The host controls playback and the queue,
  changes the settings, gives or revokes control, transfers the host role and kicks or bans.
- **Controllers**: members the host gave control to. With `control_open`, every member controls.
  Controlling = play, pause, seek, rate, skip, jump to an item, report the end of an item, and
  manage the queue (add, remove, reorder, accept or reject suggestions).
- **Queue**: ordered scene items (a scene, optionally a `start`/`end` range and a label, e.g. a
  marker). The first item of the queue is the item playing. What happens when it ends is the
  lobby's **auto-next mode** (below). Members without control **suggest** items instead (the
  host or a controller accepts or rejects them), unless the lobby has `queue_open` (everyone
  adds).
- **Auto-next mode** (`auto_next`, changed by anyone with control, saved with the lobby):
  `next` (default: the item leaves the queue and the next one starts), `stop` (playback pauses
  at the end of the item; playing again starts it over), `repeat_one` (the item, or its range
  for a marker item, starts over), `repeat_queue` (the item goes to the end of the queue and
  the next one starts: after the last item, the first one comes back). The server decides
  (see "End of an item").
- **Members**: users who joined (a row in `lobby_members`). A banned user stays as a row with
  `banned_at` and cannot join again. "Online" members have at least one open WebSocket.
- **Anonymous members**: a signed-in user can join a **public** lobby anonymously (lobby
  browser, join screen of the room). The other members see "Anonymous #N" (N stable for the
  membership, never reused in the lobby: `lobbies.anon_seq`) with a generic avatar and no
  profile, in the member list, the chat, the system messages, the reactions and "suggested by" /
  "added by". See "Anonymous members" below for what the server sends.

## Permissions

Declared in `internal/core/rbac_catalog.go` (see `docs/RBAC.md`):

| Permission | Gives | Default grant |
|---|---|---|
| `goon2gether:use` | Join lobbies, chat, react, suggest; every `/together` route | from `scenes:view` |
| `goon2gether:host` | Create lobbies | from `scenes:view` |
| `goon2gether:moderate` | See and close any lobby (also private), kick and ban in any lobby, delete any chat message, see who is behind an anonymous member | moderator |
| `emotes:manage` | Upload, rename and delete the server custom emotes | moderator |

Adding to a queue also needs `scenes:view`. Queue items carry the scene title and duration
only: no size, path or technical field is ever sent (the room loads the scene with
`GET /api/v1/scenes/:id`, which applies the usual `files:view_*` redaction). Users without
`scenes:view` get no title in the lobby list.

## REST API

All routes are under `/api/v1`, need a session (cookie) and `goon2gether:use`. Lobby `:id` is
the lobby UUID. Errors use the usual `{error, code}` body.

### Lobby summary

```json
{
  "id": "6f1c...uuid",
  "name": "Friday night",
  "visibility": "public",
  "host": {"id": 3, "username": "alice"},
  "online_count": 2,
  "queue_length": 4,
  "now_playing": {"scene_id": 12, "title": "Scene title", "label": null},
  "control_open": false,
  "queue_open": false,
  "wait_for_all": false,
  "max_members": 0,
  "auto_next": "next",
  "is_member": true,
  "anonymous": false,
  "invite_code": "K7QW2MZP",
  "created_at": "2026-11-25T20:00:00Z"
}
```

`host` is null while nobody holds it (host account deleted, or only anonymous members left).
`now_playing` is null when the queue is empty; its `title` is omitted without `scenes:view`.
`invite_code` is only sent to members (and to `goon2gether:moderate`). `max_members` 0 means the
server default (`goon2gether.max_members`). `anonymous`: the user is an anonymous member. The
summary never lists the members.

### Routes

| Method and path | Permission | Body | Answer |
|---|---|---|---|
| `GET /together/lobbies` | use | query `all=true` (moderate only: every lobby) | `{lobbies: [summary]}`: public lobbies and the lobbies the user is a member of, most recently active first |
| `GET /together/public-count` | use | | `{count}`: number of public lobbies (the badge of the Together nav link; banned users count them too). Kept live by the `together:public_count` SSE event, see below |
| `POST /together/lobbies` | host | `{name, visibility, control_open, queue_open, wait_for_all, max_members, auto_next, items: [item]}` (all optional) | `201` summary |
| `GET /together/lobbies/:id` | use | | summary; `404` for a private lobby the user is not a member of |
| `POST /together/lobbies/:id/join` | use | `{anonymous}` (optional) | summary; joins a public lobby (`404` private non-member, `403` `LOBBY_BANNED`). `anonymous: true`: public lobbies only (`400` for a private lobby or for its creator), `409` when the user is already a member under their name (leave first); an anonymous member joining again stays anonymous |
| `POST /together/join` | use | `{code}` | summary; joins the lobby of the invite code (`404` unknown code, `403` `LOBBY_BANNED`) |
| `POST /together/lobbies/:id/leave` | use | | `204`; a leaving host hands the role to the next member, or the lobby closes if nobody is left |
| `DELETE /together/lobbies/:id` | host of the lobby, or moderate | | `204`; open connections get `bye` (`closed`) |
| `POST /together/lobbies/:id/invite-code` | host of the lobby | | `{invite_code}`: a new code, the old one stops working |
| `POST /together/lobbies/:id/queue` | use + `scenes:view`, member | `{items: [item]}` | `{queued, suggested}`: added to the queue if the user may, else as suggestions |
| `GET /together/emotes` | use | | `{emotes: [emote]}` sorted by name |
| `POST /together/emotes` | `emotes:manage` | multipart `name`, `file` | `201` emote |
| `PUT /together/emotes/:id` | `emotes:manage` | `{name}` | emote |
| `DELETE /together/emotes/:id` | `emotes:manage` | | `204` (the image file is removed) |
| `GET /api/v1/together/lobbies/:id/ws` | use, member (a public lobby is joined on connect; `anonymous=1`: anonymously) | | WebSocket, see below |

`item` = `{scene_id, start, end, label}`: `start`/`end` in seconds (optional, `end > start`),
`label` optional (max 100 characters, e.g. the marker label). The scene must exist and not be in
the trash. An `end` past the scene duration (when known) is cut to it (no end at all when the
start is already past it): a marker without an end is queued with a default length (30 s, like
the marker playlists' `playlists.marker_default_duration`) that may pass the end of the scene.

`emote` = `{id, name, url, width, height, animated, size}` (`size` in bytes); `url` is `/emotes/<file>` (static,
cached, served from `goon2gether.emote_dir`, outside every media directory). Names match
`^[A-Za-z0-9_]{2,32}$` and are unique without regard to case. Accepted files: PNG, GIF, WebP
(animated GIF and WebP allowed), at most `goon2gether.emote_max_bytes` and
`goon2gether.emote_max_dimension` pixels on each side.

### Public lobby count (SSE)

`together:public_count` is a broadcast event of `GET /api/v1/events` with `data: {count}`, the
number of public lobbies. It carries no personal data; the SSE filter sends it to users with
`goon2gether:use` only. The hub marks the count as possibly changed when a public lobby is
created or closed (host, last member leaving, inactivity, empty close), when a lobby changes
visibility, and when the idle cleanup deletes lobbies; the next tick (`tick_interval`) counts
the public lobbies again (one `COUNT` query) and publishes the event only when the count
changed. Clients load the initial value with `GET /together/public-count` and again after an
SSE reconnect (`stores/togetherLobbies.ts`); the lobby browser reloads its list when the count
changes.

### Service hooks for later features

- `together.Hub.ActiveLobbiesForUser(userID)` lists the lobbies where the user is online (for
  "watching in lobby X" in the friends presence). Anonymous memberships are never listed.
- `together.Hub.InviteUser(lobbyUUID, inviterID, userID)` adds a user as a member of a lobby
  (pre-authorized, like opening the invite link).
- Lobby invites sent as messages (`docs/MESSAGES.md`): the hub implements `core.LobbyInvites`
  (`internal/together/invites.go`): `CanInvite` (members only), `InviteCard` (the card shown to
  the invited user: joinable when the lobby is public or the inviter is still a member, not when
  banned) and `AcceptInvite` (joins when the invite is still valid). The room's "Invite friends"
  button sends them.

## WebSocket protocol (version 3)

Endpoint: `GET /api/v1/together/lobbies/:id/ws?v=3[&anonymous=1]`, same origin, authenticated
by the session cookie on the upgrade (the same auth middleware as every route). The `Origin`
header must match the host or one of `server.allowed_origins`. Frames are JSON text messages
with a type field `t`. The server accepts versions 2 and 3 (`MinProtocolVersion` to
`ProtocolVersion`: each version only adds fields and message types, which older clients
ignore; `welcome.v` is always the server's version); other versions are refused (`400`
`BAD_VERSION`).

Version 3 (2026-11-27) adds reactions on chat messages: `ChatMessage.reactions`, the
`chat_react` and `chat_unreact` commands, the `chat_reactions` message, the error code
`reaction_limit`, and `chat_max_reaction_kinds` / `chat_max_reactions_per_user` in
`welcome.config`. See "Chat message reactions" below.

Version 2 (2026-11-26) changes from version 1: `user_id` (members, chat, reactions) and
`added_by.id` may be null (anonymous members), new fields `anonymous`/`anon_number`, `auto_next`
in `LobbyInfo`, `closing_at` and `you.anonymous`/`you.anon_number` in `welcome`, the `closing`
message, the `auto_next` and `keep_open` commands, `anon` to name a member in commands, the bye
reason `inactive`.
A client message may carry a `ref` (string, up to 64 characters) echoed back in an `error`.
Messages larger than `goon2gether.max_message_bytes` close the connection.

The server pings at the WebSocket level every `goon2gether.ping_interval` (works through Traefik
and auth proxies that drop idle connections); the client's own `ping` messages (clock sync) also
keep it busy. A client that cannot keep up (its send buffer of `goon2gether.send_buffer`
messages is full) is disconnected (close code 4008) and resyncs on reconnect.

### Close codes

| Code | Meaning | Client reconnects |
|---|---|---|
| 1000 / 1001 | normal close, server shutdown | yes (1001) |
| 4001 | kicked | no |
| 4003 | banned, no longer allowed (permission removed), or an anonymous join that is not possible | no |
| 4004 | lobby closed (also for inactivity, or not found) | no |
| 4005 | the user left the lobby (from another tab) | no |
| 4008 | too slow (send buffer full) or protocol violation | yes |
| 4009 | lobby full, or too many connections of the user | no |

The client reconnects with an exponential backoff (1 s, doubling, max 30 s, with jitter) and
gets a full `welcome` snapshot each time (state resync).

### Shapes

```text
LobbyInfo     {id, name, visibility, host_id, control_open, queue_open, wait_for_all,
               max_members, auto_next, invite_code}
Member        {user_id, username, anonymous, anon_number, host, control, online, connections,
               buffering, joined_at}
QueueItem     {id, scene_id, title, duration, start, end, label,
               added_by: {id, username, anonymous?, anon_number?} | null}
State         {item_id, playing, hold, position, server_time, rate, seq}
ChatMessage   {id, user_id, username, anonymous?, anon_number?, text, at, system,
               reactions?: [ChatReaction]}
ChatReaction  {emote, count, users: [{id, username, anonymous?, anon_number?}]}
```

- An anonymous member (see "Anonymous members"): `user_id` / `id` null, `username`
  "Anonymous #N", `anonymous` true, `anon_number` N; moderators get the real `user_id` and
  `username` with `anonymous` and `anon_number`.

- `Member.control`: the member may control (host, given control, or `control_open`).
  `buffering`: one of its connections reported buffering.
- `QueueItem.duration` is the scene duration in seconds (0 if unknown); `start`, `end`, `label`
  are null when not set.
- `State`: `item_id` is the item playing (`null`: empty queue). `position` (seconds) is the
  position at `server_time` (Unix milliseconds of the server clock). `playing` is the wanted
  state, `hold` is true while the lobby waits for buffering members (`wait_for_all`): the
  position does not move during a hold. `rate` is the playback rate. `seq` increases on every
  change.
- `ChatMessage.at` is Unix milliseconds; `system` messages (joined, left, now playing, host
  changed...) have a null `user_id`. `reactions` is omitted when the message has none; the keys
  come in the order of their first reaction, the users in reaction order (an anonymous reactor
  like an anonymous member: null `id`, "Anonymous #N", except for moderators).

### Server to client

| `t` | Fields | When |
|---|---|---|
| `welcome` | `v`, `server_time`, `you: {user_id, username, moderate, anonymous, anon_number}`, `config: {drift_threshold, drift_nudge, hold_timeout_ms, chat_max_length, end_grace, chat_max_reaction_kinds, chat_max_reactions_per_user}`, `lobby`, `members`, `queue`, `suggestions`, `state`, `chat` (with their reactions), `closing_at` (ms or null) | first message of each connection (full snapshot) |
| `state` | `state` | playback changed (play, pause, seek, rate, item change, hold) |
| `queue` | `queue`, `suggestions` | queue or suggestions changed. `queue[0]` is the item playing when `state.item_id` is set |
| `members` | `members` | join, leave, connection change, buffering, control, host |
| `lobby` | `lobby` | settings or host changed |
| `chat` | `message` | new chat or system message |
| `chat_deleted` | `id` | a message was deleted (its reactions with it) |
| `chat_reactions` | `id`, `reactions: [ChatReaction]` | the reactions of a chat message changed: the whole list (empty: none left) |
| `reaction` | `user_id`, `username`, `anonymous?`, `anon_number?`, `emote` | floating reaction over the video (not a chat message reaction) |
| `closing` | `at` (Unix ms, or null), `reason` (`inactive`) | the lobby closes at `at` for inactivity unless something happens; `at` null: called off |
| `emotes` | | the server emotes changed: reload `GET /together/emotes` |
| `pong` | `id`, `c`, `s` | answer to `ping`: `c` echoed, `s` server time (ms) |
| `error` | `code`, `message`, `ref` | a message was refused |
| `bye` | `reason` (`kicked`, `banned`, `closed`, `forbidden`, `left`, `inactive`) | just before the server closes |

Error codes: `not_allowed` (no control, not host, no permission), `invalid` (bad payload),
`not_found` (unknown item, member, chat message or emote), `rate_limited`, `too_long`,
`queue_full`, `reaction_limit` (caps of the chat message reactions), `unknown_type`,
`internal`.

### Client to server

| `t` | Fields | Who |
|---|---|---|
| `ping` | `id` (number), `c` (client time, ms) | anyone (clock sync) |
| `play` | `position` (optional) | control |
| `pause` | `position` (optional: where the controller paused) | control |
| `seek` | `position` | control |
| `rate` | `rate` (0.25 to 4) | control |
| `skip` | | control: ends the current item, the next one starts |
| `play_item` | `item_id` | control: plays that queue item now (the current one is dropped) |
| `ended` | `item_id` | anyone: the player reached the end of the item |
| `status` | `buffering` (bool), `seq` (last state applied), `item_id` | anyone: buffering or ready |
| `queue_add` | `items: [{scene_id, start, end, label}]`, `play_now` (bool, optional) | `scenes:view`; without queue rights they become suggestions. `play_now` needs control: the items go to the front of the queue and the first one plays at once (the item playing is dropped, like `play_item`) |
| `queue_remove` | `item_id` | control (also: one's own suggestion) |
| `queue_move` | `item_id`, `index` (position among the upcoming items, 0 = next) | control |
| `suggestion_accept` | `item_id` | control |
| `suggestion_reject` | `item_id` | control (also: one's own suggestion) |
| `chat` | `text` | anyone (rate limited, max `chat_max_length` characters) |
| `chat_delete` | `id` | host, `goon2gether:moderate`, or the author |
| `chat_react` | `id` (chat message), `emote`: an emoji, or `:name:` of a server emote | anyone: adds my reaction to the message (rate limited, capped) |
| `chat_unreact` | `id`, `emote` | anyone: removes my reaction (rate limited) |
| `reaction` | `emote`: an emoji, or `:name:` of a server emote | anyone (rate limited): a floating reaction over the video |
| `auto_next` | `mode` (`stop`, `next`, `repeat_one`, `repeat_queue`) | control |
| `keep_open` | | anyone: activity only (calls an announced inactivity close off) |
| `settings` | `name`, `visibility`, `control_open`, `queue_open`, `wait_for_all`, `max_members` (each optional) | host |
| `grant_control` / `revoke_control` | `user_id` or `anon` | host |
| `transfer_host` | `user_id` or `anon` (an online, named member) | host |
| `kick` | `user_id` or `anon`, `ban` (bool) | host, `goon2gether:moderate` (never the host); banning a user who is not a member: moderators only |

`anon` names an anonymous member by number. Only moderators may name an anonymous member by
`user_id`: for anyone else it answers `not_found`, like a user who is not in the lobby, so that
trying user IDs never tells who is behind an anonymous member.

A command from a member without the right is refused with `error` `not_allowed` and changes
nothing: the server state stays the reference, clients that applied a local action revert to it.

## Chat message reactions

Members react to the chat messages like in Discord (hover a message, or a long press on touch
screens, then "Add reaction": the emoji and emote picker of the chat; pills under the message
with the count, highlighted when I reacted; a click toggles my reaction; hovering tells who).
They are distinct from the floating reactions over the video (`reaction`, unchanged).

- In memory with the message (`ChatMessage.reactions` in the hub), so bounded by
  `chat_history` and lost with the chat (unload, restart); deleting a message deletes them.
  Late joiners and reconnects get them in the `welcome` chat snapshot.
- Only members react (every connection is one; kicked and banned users are disconnected), on
  messages still in the history (`not_found` otherwise), not on system messages
  (`not_allowed`). The key is an emoji or `:name:` of an existing server emote (`invalid`,
  `not_found`), the same check as the messages (`core.CheckReactionKey`).
- Caps: `chat_max_reaction_kinds` distinct keys per message (default 20) and
  `chat_max_reactions_per_user` reactions of one member on one message (default 10):
  `reaction_limit`. Rate limit: `chat_reaction_rate_limit` adds and removals per
  `rate_window` (default 10 per 10 s), separate from the chat and floating reaction limits:
  `rate_limited`. Adding a reaction that is there, or removing one that is not, changes
  nothing and sends nothing. A reaction counts as activity (inactivity close).
- Every change broadcasts `chat_reactions` with the whole list of the message.
- **Anonymous reactors** are redacted like the rest of the protocol: members get a null `id`,
  "Anonymous #N" and `anon_number` (the anonymous user recognizes their own reactions by
  number), moderators get the real `id` and `username` with `anonymous: true` (shown as
  "Anonymous #N (alice)"). A lobby with anonymous reactions sends two variants of the payloads
  (see "Anonymous members"). The reactor keeps the number of the membership they reacted
  under.

## Sync algorithm

The server is authoritative: it holds, per lobby, the current item, `playing`, `hold`, the
position at a server timestamp and the rate. Clients never trust each other's clocks.

**Clock offset.** The client sends `ping {id, c}` (5 times, 400 ms apart, on connect, then every
15 s); the server answers `pong {id, c, s}`. With `r` the reception time, `rtt = r - c` and the
offset sample is `s - (c + rtt / 2)`. The client keeps the offset of the sample with the lowest
RTT among the last 8 (the least delayed one is the most precise).

**Expected position.** `server_now = Date.now() + offset`;
`expected = state.position + (state.playing && !state.hold ? (server_now - state.server_time) / 1000 * state.rate : 0)`,
clamped to `[0, end]` (`end` = item end, else the scene duration when known).

**Drift correction** (every 500 ms and on each `state`): `drift = player_time - expected`.

- `|drift| > drift_threshold` (default 0.75 s; at least 2.5 s while the scene is converted on
  the fly, where a seek restarts the conversion): seek to `expected`.
- else, with `drift_nudge`, `|drift| > 0.1 s`: play at `rate * (1 - clamp(drift * 0.1, -0.05, 0.05))`
  until the drift is under 0.05 s, then back to `rate`. Tiny rate changes are inaudible and
  avoid seeking for small drifts.
- the paused state and the rate follow the state (`playing && !hold` plays, else pauses).

**Local actions.** The room plays with the usual player (`ScenePlayer`: video.js, stream and
on-the-fly conversion, quality and keyboard shortcuts of the watch page). Player events coming
from the user (play, pause, seeking, rate) are told apart from the ones caused by applying the
state (those are expected), then:

- with control: turned into commands (`play`/`pause` with the local position, `seek`, `rate`);
  the player stays where the user put it, and the server answers with the new state;
- without control: reverted to the expected state at once.

**Buffering and "wait for everyone".** Clients send `status {buffering: true}` on `waiting` and
`status {buffering: false, seq}` once they can play after applying a state. With `wait_for_all`,
an item change or a seek is a *sync point*: every online connection must report ready for that
sync point, and any buffering report while playing puts the lobby on `hold` (the position stops,
everyone pauses) until all are ready. A hold lasts at most `goon2gether.hold_timeout`
(default 10 s): then playback resumes without the late members, whose reports are ignored until
the next sync point (no hold flapping on a slow connection). Without `wait_for_all`, buffering
is only shown in the member list.

**Late joiners and reconnects.** The `welcome` snapshot carries the state: the client loads the
item, seeks to the expected position and plays or pauses as the state says.

**End of an item.** The server advances on its own: on each tick (`goon2gether.tick_interval`,
500 ms) it checks whether the expected position reached the item's `end` (a range item), or the
scene duration plus `end_grace` (1.5 s, the duration is rounded). Client `ended` reports also
advance, so that items whose duration is unknown or shorter than recorded move on: a report for
another item than the current one is ignored; when the end is known, a report earlier than 5 s
before it is ignored (a broken stream must not skip for everyone); when the end is unknown, a
report from a member with control advances, other reports advance once half of the online
members sent one. One item is advanced once (the item id is checked), so clients that disagree
cannot skip twice. The next item starts playing at its `start`; an empty queue stops playback.

What "advance" does is the auto-next mode: `next` deletes the item and starts the next one;
`stop` pauses at the end (a later `play` at the end starts the item over); `repeat_one` starts
the item over at its `start` (a range item loops its range; `ended` reports of the previous loop
are ignored for 3 s); `repeat_queue` moves the item to the end of the queue and starts the next
one (a single item loops). `skip` always moves on (in `repeat_queue` the skipped item goes to
the end of the queue), `play_item` in `repeat_queue` turns the queue until the item is first
(nothing is dropped), and removing the item playing always drops it.

**Host away.** When the host has no open connection for `goon2gether.host_grace` (60 s), the role
goes to the next online member (members with control first, then by join date), with a system
message. A host who leaves the lobby hands it over at once. Anonymous members are never host
(skipped, and `transfer_host` to one is refused), but they can be given control. When only
anonymous members are left, the lobby has no host and every member controls it, until a named
member connects (they get the role at the next tick).

## Anonymous members

- Joining: `POST /together/lobbies/:id/join {anonymous: true}` or the WebSocket with
  `anonymous=1`, public lobbies only, never by the lobby creator. The flag and the number are on
  the membership (`lobby_members.anonymous`, `anon_number`); leaving ends it (joining again gives
  a new number). A member under their name cannot turn anonymous (`409`), and an anonymous
  member joining again stays anonymous.
- What the server sends: to every member but the moderators, an anonymous member has a null
  `user_id` (members, chat, reactions) or `id` (queue and suggestions `added_by`, chat message
  reactions `users`), "Anonymous #N" as
  name, and system messages say "Anonymous #N joined" (moderators: "Anonymous #N (alice)
  joined"). Queue items keep the adder's number (`lobby_queue_items.added_by_anon`), so the
  adder stays hidden after leaving or a restart. The anonymous user gets their own identity in
  `welcome.you` and recognizes their own entries by `anon_number`.
- `goon2gether:moderate` gets the real `user_id` and `username` with `anonymous: true` (the
  room shows them with an "Anonymous #N" badge). The payloads are marshaled twice (moderators,
  others) only while the lobby has anonymous content.
- Lobby summaries never list members; presence (`ActiveLobbiesForUser`) never lists an
  anonymous membership; commands cannot be used to probe identities (see the commands table).

**Upgrade through gin.** gin refuses to hijack a response whose header was written, while the
WebSocket library writes the `101` before hijacking: the handler passes it a small writer
(`upgradeWriter`) that sends the `101` to the `net/http` writer and hijacks through gin. The
WebSocket path is excluded from the gzip middleware.

## Adding to the queue (frontend)

The queue panel's "Add to queue" button (or "Suggest" without queue rights) opens the search
picker (`components/picker/PickerModal.vue`): the search page interface with a Videos tab and a
Markers tab, multi-select. Picked videos are queued whole; picked markers as range items (marker
start, its end or 30 s, its label; `pickedToQueueItems` in `utils/picker.ts`). Videos already
queued whole are marked "In queue" (they can be queued again). Members with control also get
"Play now" (`queue_add` with `play_now`). Members without queue rights get the same picker and
their picks become suggestions.

## Watch progress

Watching in a lobby counts like watching alone for views and the watch history (the view is
recorded after 5 s, the history entry gets the watched duration), and the replay heatmap follows
the user's setting. It does **not** change the resume position: the player sends `keep_resume:
true` with `POST /scenes/:id/watch`, and the server keeps the position and completed flag of the
user's previous watch (`WatchHistoryService.RecordWatch`), so opening the scene alone later
still offers to resume where the user was, not where the lobby was.

## Persistence and lifetime

- Lobbies, settings, members (control, ban), the queue and suggestions are in PostgreSQL. The
  current item, the position and the rate are saved on every item change, pause, seek and rate
  change, and when a lobby is unloaded: after a restart a lobby comes back paused where it was.
- The live state (connections, buffering, holds, chat) is in memory, in one process. The chat
  keeps the last `goon2gether.chat_history` messages per lobby (sent to late joiners); it is
  lost when the lobby is unloaded or the server restarts.
- A lobby without connections is unloaded from memory after `goon2gether.unload_after` (1 min).
- **Auto-close.** Closing deletes the lobby rows (lobby, members, queue: lobby state, never a
  media file), so it leaves the lobby list and its invite link stops working; connected members
  get `bye` `inactive` (close 4004) and the room shows "closed for inactivity" with a way back to
  the lobby browser.
  - **Nobody connected** for `goon2gether.empty_close_after` (15 min): closed. `last_active_at`
    is the last time someone was connected (touched on connect, on the last disconnect, every
    `cleanup_interval` while connected, and at shutdown for the lobbies with connections);
    unloaded lobbies are deleted by the cleanup from it, loaded ones by their tick. It is in
    the database, so it holds across restarts: a lobby restored after a restart with nobody
    connected closes on time.
  - **Members connected but nothing happening** for `goon2gether.inactive_close_after` (60 min):
    closed, announced `goon2gether.close_warning` (5 min) before with a `closing` message and a
    system message. Activity is a playback running (playing, also during a hold), and every
    command of a member but `ping`, `status` and `ended` (chat, reactions, queue changes,
    control actions, settings, `keep_open`), and a REST queue add; a member connecting to an
    empty lobby starts the count over. Any activity calls the announce off (`closing` with
    `at: null`). This timer is in memory: a restart starts it over.
  - The old `goon2gether.idle_timeout` (24 h) stays as a safety net when `empty_close_after` is
    0 (the cleanup uses the shorter of the two).
- One goroutine (lifecycle manager) runs the ticks and the cleanup for every lobby; each
  connection has its reader (the HTTP handler) and one writer goroutine with a bounded send
  buffer. No goroutine or channel is created per message.

## Configuration (`goon2gether.*`)

| Key | Default | Meaning |
|---|---|---|
| `enabled` | `true` | Registers the routes (off: no Goon2gether API) |
| `emote_dir` | `./data/emotes` | Emote images directory (GoonHub data, never a media directory) |
| `emote_max_bytes` | `524288` | Largest emote file (bytes) |
| `emote_max_dimension` | `256` | Largest emote width and height (pixels) |
| `emote_max_count` | `500` | Most emotes on the server |
| `max_members` | `20` | Most online members per lobby (also the cap of a lobby's own `max_members`) |
| `max_lobbies_per_user` | `5` | Most lobbies a user hosts |
| `max_queue_items` | `200` | Most queue items per lobby |
| `max_suggestions` | `50` | Most pending suggestions per lobby |
| `max_connections_per_user` | `4` | Most connections of one user to one lobby (tabs) |
| `chat_history` | `100` | Chat messages kept per lobby |
| `chat_max_length` | `500` | Longest chat message (characters) |
| `chat_rate_limit` | `5` | Chat messages per user per `rate_window` |
| `reaction_rate_limit` | `10` | Reactions per user per `rate_window` |
| `rate_window` | `10s` | Window of the two rate limits |
| `drift_threshold` | `0.75` | Drift (seconds) over which clients seek |
| `drift_nudge` | `true` | Correct small drifts with tiny playback rate changes |
| `hold_timeout` | `10s` | Longest "wait for everyone" hold |
| `end_grace` | `1.5s` | Time after the scene duration before the server advances |
| `host_grace` | `60s` | Host absence before the role is handed over |
| `tick_interval` | `500ms` | Server tick (auto-advance, holds, host grace) |
| `unload_after` | `1m` | A lobby without connections leaves memory after this |
| `idle_timeout` | `24h` | Safety net: a lobby nobody connected to for this long is deleted (used when `empty_close_after` is 0) |
| `empty_close_after` | `15m` | A lobby with nobody connected closes after this (0: off) |
| `inactive_close_after` | `60m` | A lobby with members connected but no activity closes after this (0: off) |
| `close_warning` | `5m` | Warning (`closing` message) before an inactivity close (0: none) |

| `cleanup_interval` | `1m` | Idle lobby cleanup period |
| `send_buffer` | `64` | Messages queued per connection before it is dropped |
| `ping_interval` | `25s` | WebSocket ping period |
| `write_timeout` | `10s` | Longest write of one message |
| `max_message_bytes` | `16384` | Largest client message |
| `chat_reaction_rate_limit` | `10` | Chat message reactions added or removed per member per `rate_window` |
| `chat_max_reaction_kinds` | `20` | Distinct reactions per chat message |
| `chat_max_reactions_per_user` | `10` | Reactions of one member on one chat message |

## Limits

- One process: the hub is in memory, several GoonHub instances behind a load balancer would each
  have their own lobbies.
- The chat history is not persisted (nor its reactions).
- Sync precision depends on the browsers: usually under 0.2 s with direct streams; with
  on-the-fly conversion, seeks are slower (the conversion restarts).
- The scene stream endpoints are not authenticated (unchanged, see `docs/RBAC.md`): being in a
  lobby gives nothing more than the scene IDs.
