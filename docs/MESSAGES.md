# Messages and shares

Private messages between users, and sharing videos, markers, playlists and Goon2gether lobbies
inside the platform. Built on the social graph (`docs/SOCIAL.md`) and its live events.

- Backend: `internal/core/message_service.go` (conversations, sending, edits, read state,
  typing, shares, settings), `internal/core/message_cards.go` (share cards resolved for the
  viewer), `internal/core/message_reactions.go` (reactions), `internal/core/reaction_keys.go`
  (emoji and emote keys, shared with Goon2gether), `internal/core/message_types.go` (views,
  inputs, events), `internal/data/message_*.go` (models, repository),
  `internal/api/v1/handler/message.go`, `internal/together/invites.go` (lobby invites,
  `core.LobbyInvites`).
- Frontend: `pages/messages/[[username]].vue`, `components/messages/` (conversation list and
  items, thread, share cards, toasts), `components/header/Messages*.vue`, `components/share/`
  (the one Share dialog, see below), `components/chat/RichText.vue` (shared with the
  Goon2gether chat), `components/chat/Reactions.vue`, `ReactionPicker.vue`,
  `ReactionEmote.vue` (reactions, shared with the Goon2gether chat), `stores/messages.ts`,
  `composables/api/useApiMessages.ts`, `composables/useMessageReactions.ts`,
  `utils/messages.ts`, `utils/reactions.ts` (tests: `web/tests/messages.test.ts`,
  `web/tests/reactions.test.ts`).
- Schema: migration 102 (`conversations`, `conversation_participants`, `messages`, two columns
  of `user_social_settings`, the range columns of `share_links`) and 104
  (`message_reactions`), see `docs/DATABASE.md`.

## Model

- **Conversation**: `conversations` + `conversation_participants`. Only **direct** (one-to-one)
  conversations exist: one per pair of users (`direct_low_id`, `direct_high_id`, unique), created
  with its two participants on the **first message** (never empty). The participants table holds
  the per-user state (last read message, muted, joined at), so group conversations can be added
  later (a `kind`, a title, more participants) without changing the messages.
- **Message**: a body (text, at most `messages.max_length` characters, trimmed), an
  **attachment** (a share, below), or both. Edited messages keep `edited_at`; deleted ones are
  **soft deleted** (`deleted_at`, body and attachment cleared): shown as "message deleted".
  Only the author edits or deletes a message; deleting is always allowed, editing follows the
  same policy as sending.
- **Read state**: `last_read_message_id` per participant. Unread = messages of others, not
  deleted, after it. The header badge sums the unread messages of the conversations that are
  not muted. "Seen" under my last message when the other's last read reaches it. Sending marks
  the conversation read for the sender.
- **Typing**: ephemeral, never stored. Relayed to the other participant at most once per
  `messages.typing_interval` per user and conversation; the client shows it for 6 s after the
  last event, and clears it when a message arrives.

- **Reactions**: emoji and server emotes under a message, see "Reactions" below.

## Who may write to whom

Each user chooses who may send them messages (`user_social_settings.message_policy`,
Settings > Privacy > Messages):

| Policy | Who may write |
|---|---|
| `everyone` | any signed-in user |
| `mutuals` | friends, and users who follow the user and are followed back |
| `friends` (default) | friends only |
| `nobody` | nobody (the user still reads their conversations) |

The policy of the **recipient** is checked **at send time** (starting a conversation, every
message, every edit, typing). So unfriending makes the thread read-only for the side that no
longer passes: the history stays, the input is replaced by an explanation
(`read_only_reason`: `policy`; `no_permission` without `messages:use`; `deleted_user`). The
policy is symmetric only if both users chose the same one: with A `everyone` and B `friends`, B
can write to A but A cannot answer until they are friends (shown read-only on A's side).

`core.Relationship.CanMessage` (`relationship.can_message` in every social payload) tells the UI
whether "Message" makes sense. It does not look at `messages:use`.

A relation change (follow, unfollow, friendship made or removed) sends
`chat:conversation_updated` to both users of their conversation, so open threads update
whether one can still write. A policy change applies to the next message (a refused sender
reloads the thread, which shows it read-only).

Blocking a user is **not** implemented (follow-up); the `nobody` policy exists.

## Privacy and permissions

- `messages:use` (Social group, default `user` and every role with `scenes:view`): send
  messages, edits, shares, typing, `POST /messages/direct/...`, `POST /messages/share`. Reading
  one's conversations, marking read, muting and deleting one's own messages only need a
  signed-in user.
- **Messages are private.** Only the participants of a conversation read it; there is **no
  administrator bypass** and no admin endpoint to read other users' messages (a superuser is a
  participant like any other). A conversation one does not take part in answers `404
  CONVERSATION_NOT_FOUND`, like an unknown id. The SSE events are personal
  (`EventBus.PublishToUser`, the `chat:` prefix is in `sseTargetedPrefixes`): never broadcast,
  superusers included.
- Rate limit: `messages.rate_per_minute` per user (token bucket, burst `messages.rate_burst`,
  in memory), counting messages, edits, and a share once per recipient. `429
  MESSAGE_RATE_LIMITED`.

## Reactions

Like Discord or Slack: hovering a message (a long press on touch screens) shows "Add
reaction", which opens the emoji and emote picker of the chats (Unicode emoji and the server
custom emotes of Goon2gether, `:name:`). The reactions show under the message as pills (the
emoji or emote and the count), highlighted when I reacted; clicking a pill adds or removes my
reaction; hovering it tells who reacted ("You and Bobby reacted with 🔥").

- **Stored** in `message_reactions` (migration 104): one row per message, user and key (an
  emoji, or `:name:` of a server emote, kept as text). The keys are listed in the order of their
  first reaction, the users in reaction order.
- **Who may react**: the participants of the conversation only (`404 CONVERSATION_NOT_FOUND`
  otherwise, like every message route). **Adding** follows the rules of sending: `messages:use`
  and the other participant's message policy (`canSendTo`), so a read-only conversation (no
  longer friends, deleted user, no permission) takes no new reaction (`403 MESSAGES_CLOSED`).
  **Removing** one's own reaction is always allowed, like deleting one's own message.
- **Keys**: a short Unicode emoji sequence (no ASCII text, no whitespace or markup; keycaps,
  skin tones, joiners allowed; at most 32 bytes), or `:name:` of an existing server emote
  (Goon2gether's emotes, `together.EmoteService`; refused when Goon2gether is disabled). An
  emote deleted later stays as a key and renders as its `:name:` text.
- **Caps** (`messages.max_reaction_kinds`, default 20 distinct keys per message;
  `messages.max_reactions_per_user`, default 10 reactions of one user on one message): `409
  REACTION_LIMIT`. They are checked with the message row locked, so concurrent reactions
  respect them. Reacting again with the same key changes nothing.
- **Rate limit**: `messages.reaction_rate_per_minute` (default 60, burst
  `messages.reaction_rate_burst` 20) per user, adds and removals, in memory: `429
  REACTION_RATE_LIMITED`.
- **Deleted messages lose their reactions** (deleted with the soft delete, none can be added:
  `404 MESSAGE_NOT_FOUND`); a hard deleted message (retention) and a deleted account take
  theirs with them (`CASCADE`).
- **In the payloads**: every `message` carries `reactions: [{emoji, count, users: [{id,
  username, display_name}]}]` (empty for a deleted message, and for a message just sent). Only
  participants read them, like the messages.
- **Live**: `chat:reaction` to every participant (targeted, never broadcast) with the whole
  list of the message; the open thread replaces it. The client applies my toggle at once and
  replaces it with the server's answer (reverted on an error).

## Shares

A message's attachment stores **references** (ids), never copies; the card is **resolved at
read time for the reader**, with the reader's permissions (history, live events: each recipient
gets the event resolved with their own access). A card never carries file information (size,
path, codecs) whatever the permissions: only what showing and opening it needs. When the reader
may not see the item, the card is "not available" with a reason (`not_found`, `trashed`,
`no_permission`, `private`, `not_joinable`) and nothing else.

| Kind | Stored | Sender may share | Reader sees it when | Card | Opens |
|---|---|---|---|---|---|
| `scene` | `attachment_scene_id` (+ `attachment_time_s`) | `scenes:view`, the scene exists and is not trashed | `scenes:view`, the scene is neither trashed nor deleted | title, duration, thumbnail, "From m:ss" | `/watch/:id` (`?t=` the shared time) |
| `marker` | `attachment_marker_id` | `scenes:view`, a marker they see (`VisibleMarkersSQL`: their own or shared by its owner) | `scenes:view`, its scene is neither trashed nor deleted | label, scene title, range, thumbnail | `/watch/:scene?t=start&end=end` (the watch page loops the range) |
| `playlist` | `attachment_playlist_id` | `scenes:view`, their own playlist or a public one | `scenes:view`; their own, or a public one with `playlists:view_public` | name, owner, item count, first thumbnail | `/playlists/:uuid` |
| `lobby` | `attachment_lobby_uuid` | `goon2gether:use`, a member of the lobby | `goon2gether:use`, the lobby exists, the reader is a member or can join (below) | name, online count, host, playing title (needs `scenes:view`) | "Join" |

- A **shared marker is shown to the recipient even when its owner does not share their markers**
  (`users.share_markers`): sharing it is the point. Only that marker is exposed, through the
  card (range, label, scene), not the owner's other markers.
- A referenced row that goes away (scene deleted, marker deleted, playlist deleted) leaves the
  kind with a NULL id (`ON DELETE SET NULL`): "no longer exists". Lobbies are deleted when they
  close: no foreign key, "this lobby is closed".
- **Lobby invites** use the Goon2gether hook (`docs/GOON2GETHER.md`, "Service hooks"): nobody is
  added at send time. "Join" calls `POST .../messages/:id/join-lobby`: the server checks that the
  reader takes part in the conversation and that the message is a lobby invite, then
  `Hub.AcceptInvite(lobby, sender, reader)`: a member opens the room, a public lobby is joined,
  a private one only while the sender is still a member (the message is the ticket, like the
  invite link), bans apply. The card is "not joinable" otherwise.

### Share dialog (frontend)

There is one Share dialog (`components/share/ShareModal.vue`, opened by `ShareButton`), with two
tabs:

- **Send to friends** (`messages:use`): pick users (recent conversations first, then friends,
  then a directory search; presence dots; users whose policy refuses me are disabled), an
  optional message, send (`POST /messages/share`: one message per item and user, the text on
  the first one; per-user results).
- **Link** (`scenes:share`, scenes and markers): the share links of the scene (public or for
  signed-in users, expiry), unchanged otherwise.

A scene has an optional **Start at** (checkbox and time field, `m:ss` or `h:mm:ss`), prefilled
with the player position on the watch page: the message card and the link start there. A marker
shares its range in both tabs.

Entry points: the watch page bar (Share, with the current time), the selection toolbar (Send,
up to 10 videos), the marker rows of the watch Markers tab and the marker cards of `/markers`,
the playlist page (public playlists), the Goon2gether room ("Invite friends", lobby invite), and
"Message" on profile headers and user rows.

### Share links

Migration 102 adds to `share_links`: `start_s` (where the shared player starts), `end_s` and
`label` (a marker range), `marker_id` (the source marker, `SET NULL`, never sent).
`POST /scenes/:id/shares` accepts `start` (seconds, `0 <= start < duration` when the duration is
known, `400` otherwise) or `marker_id` (a marker of that scene the creator sees: its start, its
end or `playlists.marker_default_duration` seconds, cut at the scene end, and its label are
copied; `404` unknown or not visible, `400` another scene). The existing rules stay
(`scenes:share`, share types, expiry). `GET /shares/:token` returns `share_link.start`, `end`,
`label`: the share page starts there and loops the range like a marker link, with the label. A
marker link exposes that scene, the range and the label only.

## API

All under `/api/v1/messages`, signed-in users. `conversation` = `{id, kind, other: {id,
username, display_name, avatar_version} | null, last_message: {id, sender_id, body,
attachment_kind, deleted, created_at} | null, unread_count, muted, last_read_message_id,
other_last_read_message_id, can_send, read_only_reason?, updated_at}`. `message` = `{id,
conversation_id, sender_id | null, body, attachment: {kind, available, reason?, time?,
scene?|marker?|playlist?|lobby?} | null, reactions: [reaction], edited_at, deleted,
created_at}`. `reaction` = `{emoji, count, users: [{id, username, display_name}]}`.

| Route | Permission | Answer |
|---|---|---|
| `GET /conversations?page&limit` | | paginated `conversation`s, most recent first |
| `GET /unread` | | `{messages, conversations}` (muted excluded) |
| `GET /with/:username` | | `{user, conversation | null, can_send, read_only_reason?}` |
| `GET /conversations/:id` | | `conversation` |
| `GET /conversations/:id/messages?before=&limit=` | | `{messages (oldest first), has_more}` (`limit` default `page_size`, max `max_page_size`) |
| `POST /direct/:userId/messages` `{body, attachment?}` | `messages:use` | `201 {conversation, message}` (creates the conversation) |
| `POST /conversations/:id/messages` `{body, attachment?}` | `messages:use` | `201 message` |
| `PATCH /conversations/:id/messages/:messageId` `{body}` | `messages:use` | `message` |
| `DELETE /conversations/:id/messages/:messageId` | | `204` |
| `POST /conversations/:id/messages/:messageId/join-lobby` | | `{lobby_id}` |
| `POST /conversations/:id/messages/:messageId/reactions` `{emoji}` | `messages:use` | `{conversation_id, message_id, reactions}` (adds mine) |
| `DELETE /conversations/:id/messages/:messageId/reactions?emoji=` | | `{conversation_id, message_id, reactions}` (removes mine) |
| `POST /conversations/:id/read` `{message_id}` | | `204` (only moves forward) |
| `POST /conversations/:id/typing` | `messages:use` | `204` |
| `PUT /conversations/:id/mute` `{muted}` | | `conversation` |
| `POST /share` `{user_ids, items: [attachment], body}` | `messages:use` | `{results: [{user_id, ok, conversation_id?, code?, error?}]}` |
| `GET /settings`, `PUT /settings` `{policy?, toasts?}` | | `{policy, toasts, policies, max_length, typing_interval_ms, max_reaction_kinds, max_reactions_per_user}` |

`attachment` (request) = `{kind: scene, scene_id, time?}`, `{kind: marker, marker_id}`,
`{kind: playlist, playlist_uuid}`, `{kind: lobby, lobby_id}`.

Errors: `403 MESSAGES_CLOSED` (policy, deleted user), `403` without permission, `404
CONVERSATION_NOT_FOUND`, `404 MESSAGE_NOT_FOUND`, `403 NOT_MESSAGE_AUTHOR`, `400
SHARE_NOT_AVAILABLE` (an item the sender may not share, a stale lobby invite), `429
MESSAGE_RATE_LIMITED`, `409 REACTION_LIMIT`, `429 REACTION_RATE_LIMITED`, `400` validation
(empty, too long, unknown kind or policy, not an emoji or emote).

## Live events (SSE)

Targeted (`chat:` in `sseTargetedPrefixes`), listed in `LISTENER_EVENT_TYPES`
(`MESSAGE_SSE_EVENTS`) and `sse-worker.js`. The client reloads its conversations and the open
thread after an SSE reconnection.

| Event | To | `data` |
|---|---|---|
| `chat:message` | every participant (the sender's other tabs too) | `{conversation_id, message}` resolved for each |
| `chat:message_updated` | every participant | `{conversation_id, message}` (edit, or `deleted: true`) |
| `chat:read` | every participant | `{conversation_id, user_id, last_read_message_id}` |
| `chat:typing` | the other participant | `{conversation_id, user_id}` |
| `chat:conversation_updated` | the user(s) concerned | `{conversation_id}`: reload it (mute, relation change) |
| `chat:reaction` | every participant | `{conversation_id, message_id, reactions}`: the whole list of the message |

## Users deleted, retention

- Deleting a user **keeps the other side's history**: the user's participant rows go
  (`CASCADE`), their messages stay with `sender_id` NULL ("Deleted user"), the pair columns are
  set NULL; the conversation is read-only (`deleted_user`). Once nobody takes part in a
  conversation anymore it is deleted by the periodic cleanup.
- Retention: none by default. `messages.retention_days` > 0 deletes the messages older than that
  every `messages.purge_interval` (hard delete).

## Configuration (`messages.*`)

| Key | Default | Meaning |
|---|---|---|
| `max_length` | `2000` | Longest body (characters, at most 10000) |
| `rate_per_minute` | `30` | Messages, edits and share recipients per user per minute (0: unlimited) |
| `rate_burst` | `10` | Sent at once before the rate applies |
| `page_size`, `max_page_size` | `50`, `100` | History pages |
| `typing_interval` | `3s` | Typing relayed at most this often (also sent to the client) |
| `max_share_recipients` | `20` | Users per share |
| `max_share_items` | `10` | Items per share |
| `retention_days` | `0` | Delete older messages (0: keep) |
| `purge_interval` | `1h` | Retention and orphan cleanup period |
| `max_reaction_kinds` | `20` | Distinct reactions per message |
| `max_reactions_per_user` | `10` | Reactions of one user on one message |
| `reaction_rate_per_minute` | `60` | Reactions added or removed per user per minute (0: unlimited) |
| `reaction_rate_burst` | `20` | Reactions at once before the rate applies |

## Limits and follow-ups

- No blocking (the `nobody` policy exists), no group conversations, no attachments other than
  shares (no file upload), no message search.
- The rate limits and the typing throttle are in memory (one process).
- Reactions are not shown in the conversation lists nor notified (no toast, no unread count).
- Rendering a lobby invite card loads the lobby in the Goon2gether hub (unloaded again after
  `goon2gether.unload_after`).
- The header toast setting is per user (`message_toasts`); a muted conversation never toasts.
