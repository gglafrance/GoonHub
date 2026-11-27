# Social graph and privacy

This document is the reference for the social layer of GoonHub: friends, follows, mutuals, the
privacy framework, the API and the live events. Features that show information about a user
to other users (profiles, presence...) build on it: read "Privacy framework" and "Service API"
before writing one.

## Model

Two separate relations:

- **Follow** (Letterboxd style): one way, no approval. `A follows B` is one row of
  `user_follows`. When A follows B and B follows A, they are **mutuals** (a badge, and an
  audience). Following is idempotent.
- **Friendship** (Steam style): by request and acceptance. A sends B a request
  (`friend_requests`, pending only); B accepts (they become friends: one row of
  `user_friendships`, the request is deleted) or declines (the request is deleted, A is not
  told). A can cancel a request still pending. Either friend can remove the friendship (the
  other is not told). Friendship and follows are independent: removing a friend keeps the
  follows, unfollowing keeps the friendship.

Edge cases:

| Case | Answer |
|---|---|
| Follow, request or remove oneself | `400 SELF_RELATION` |
| Unknown user | `404` |
| Request to a friend | `409 ALREADY_FRIENDS` |
| Second request while one is pending | `409 FRIEND_REQUEST_PENDING` |
| Request to someone who already sent me one | accepted: we become friends (`201`, `status: "accepted"`), whatever their policy |
| Two crossed requests at the same time | one friendship, no pending request (both run under a per-pair advisory lock) |
| Request refused by the receiver's policy | `403 FRIEND_REQUESTS_CLOSED` |
| Accept, decline, cancel a request that is not pending | `404 FRIEND_REQUEST_NOT_FOUND` |
| Remove a friendship that does not exist | `404` |
| Too many pending outgoing requests (`social.max_pending_requests`, default 50) | `429 TOO_MANY_PENDING_REQUESTS` |
| Too many requests or follows per hour (`social.friend_requests_per_hour` 30, `social.follows_per_hour` 60) | `429 SOCIAL_RATE_LIMITED` |

The rate limits are per user, in memory (token buckets: the burst is a third of the hourly
budget), so they reset when the server restarts. They only count actions that notify someone
(a new request, a new follow); accepting a request one received does not count. 0 disables a
limit.

Deleting a user deletes their follows, friendships, requests and privacy settings (`ON DELETE
CASCADE`).

### Friend request policy

Each user chooses who may send them a friend request (`user_social_settings`):
`everyone` (default), `mutuals` (users they follow who follow them back) or `nobody`. A user
whose policy is `nobody` can still send requests, and a request they received before can still
be accepted. `relationship.can_send_friend_request` tells the UI whether the button makes sense.

## Privacy framework

Each user chooses, per **information item**, who can see it: an **audience**.

### Audiences

Ordered from the widest to the narrowest. A viewer who passes an audience passes every wider
one.

| Audience | Who passes (the owner always does) |
|---|---|
| `public` | every signed-in user of the instance |
| `followers` | users who follow the owner, mutuals, friends |
| `mutuals` | users who follow the owner and are followed back, friends |
| `friends` | friends of the owner |
| `private` | nobody but the owner |

So: a friend passes every audience except `private`, even when no follow exists between them;
a mutual passes `followers` and `mutuals`; a follower passes `followers`; being followed by the
owner (without following back) gives nothing. Since anyone can follow without approval,
`followers` restricts little: it means "people who chose to follow me".

The owner always sees everything of their own. **There is no administrator bypass**: a
superuser is a viewer like any other (the privacy service does not look at permissions).

Implementation: `core.Relationship.Passes(audience)`; the viewer's tier (none < follower <
mutual < friend < self) must reach the audience's rank.

### Catalog

Declared in code (`internal/core/privacy_catalog.go`, `PrivacyCatalog`), like the RBAC
catalog. Each item has a key, a label, a description, a group (for the UI) and a default
audience.

| Key | Group | Default | What it hides |
|---|---|---|---|
| `profile` | Profile | public | the profile page at all; what a profile shows also needs its own item |
| `favorites` | Profile | friends | the favorite videos showcase |
| `favorite_markers` | Profile | friends | the favorite markers showcase |
| `stats` | Profile | friends | watch statistics (stats block) |
| `activity` | Profile | friends | recent watches, most watched, activity calendar |
| `recent_markers` | Profile | friends | the markers created recently |
| `liked` | Profile | friends | the liked videos (block and Likes list) |
| `ratings` | Profile | friends | the ratings chart and the rated videos with their rating (block and Ratings list) |
| `jizz` | Profile | friends | the videos the user came to, their O count and dates (block and Jizz list). Sensitive: "Only me" keeps it to oneself |
| `playlists` | Profile | public | the public playlists listed on the profile |
| `friends_list` | Connections | public | the friends list (and its count) |
| `followers_list` | Connections | public | followers and following lists (and their counts) |
| `online_status` | Presence | friends | whether the user is online, away or offline, and their last seen (`docs/PRESENCE.md`) |
| `now_watching` | Presence | friends | what the user is watching (also needs `online_status`) |

Custom lists of a profile (`docs/PROFILES.md`, "Custom lists") add no item: a list has its own
audience and depends on the items of its criteria (`liked`, `ratings`, `jizz`, `activity`,
`recent_markers`), all checked for every viewer. The figures shown on its cards (O count,
rating, like date, views) are each checked against their own item (`jizz`, `ratings`, `liked`,
`activity`) and dropped when hidden.

`profile` is a gate: a block shown on the profile page needs `profile` and its own item (the
lists endpoints check both, `CanViewAll`; the profile blocks and their items are listed in
`docs/PROFILES.md`). Items shown elsewhere (presence in the header, for
example) only need their own item.

### Storage

`user_privacy_settings` has one row per user and item the user explicitly chose. No row = the
item's catalog default, so changing a default in the catalog applies to every user who did not
choose. Choosing the default value in the UI resets the item (`"default"`, the row is deleted)
rather than pinning it. Rows of items removed from the catalog are ignored. Unknown items and
audiences are refused by the API (`400`); the database checks the audience values.

### Adding a privacy item

1. Add a key constant and an entry to `PrivacyCatalog` (`internal/core/privacy_catalog.go`):
   label, description, group (an existing one or a new `PrivacyGroup*`), default audience.
   Keep keys short (`VARCHAR(64)`), never rename one (the stored choices use it).
2. Check it where the information is served: `CanView` for one owner, `VisibleTo` /
   `FilterVisible` for lists, `Visibility` for several items of one owner, `AudienceMembers`
   to push a change only to who may see it. Never return the information before checking.
3. Document it in the table above. Nothing to do in the UI: Settings > Privacy renders the
   catalog sent by the server. No migration.

## Service API (for other features)

Wire `*core.PrivacyService` and `*core.SocialService` into your service. All methods take the
request context; ids are user ids.

```go
// internal/core/privacy_service.go
func (s *PrivacyService) CanView(ctx context.Context, viewerID, ownerID uint, item string) (bool, error)
func (s *PrivacyService) CanViewAll(ctx context.Context, viewerID, ownerID uint, items ...string) (bool, error)
func (s *PrivacyService) Visibility(ctx context.Context, viewerID, ownerID uint, items []string) (map[string]bool, error)
func (s *PrivacyService) VisibleTo(ctx context.Context, viewerID uint, ownerIDs []uint, item string) (map[uint]bool, error)
func (s *PrivacyService) FilterVisible(ctx context.Context, viewerID uint, ownerIDs []uint, item string) ([]uint, error)
func (s *PrivacyService) AudienceOf(ctx context.Context, ownerID uint, item string) (Audience, error)
func (s *PrivacyService) AudienceMembers(ctx context.Context, ownerID uint, item string) (AudienceMembers, error)
func (s *PrivacyService) GetSettings(ctx context.Context, userID uint) (*PrivacySettings, error)
func (s *PrivacyService) UpdateSettings(ctx context.Context, userID uint, in UpdatePrivacyInput) (*PrivacySettings, error)

// AudienceMembers: Everyone (public) or the sorted UserIDs in the audience (owner excluded)
func (m AudienceMembers) Contains(userID uint) bool

// Change listeners (called synchronously after the change): re-evaluate what someone sees
func (s *PrivacyService) OnChange(fn func(userID uint, items []string))
func (s *SocialService) OnRelationChange(fn func(a, b uint)) // follow, unfollow, friendship made or removed

// internal/core/social_service.go
func (s *SocialService) Relationship(ctx context.Context, viewerID, ownerID uint) (Relationship, error)
func (s *SocialService) Relationships(ctx context.Context, viewerID uint, userIDs []uint) (map[uint]Relationship, error)
func (s *SocialService) FriendIDs(ctx context.Context, userID uint) ([]uint, error)
func (s *SocialService) FollowerIDs(ctx context.Context, userID uint) ([]uint, error)
func (s *SocialService) UserByUsername(ctx context.Context, username string) (*data.SocialUser, error)
func (s *SocialService) Profile(ctx context.Context, viewerID uint, username string) (*SocialProfile, error)

// internal/core/social_relationship.go
type Relationship struct {
	Self, Following, FollowedBy, Mutual, Friend, RequestSent, RequestReceived, CanSendFriendRequest bool
}
func (r Relationship) Passes(a Audience) bool

// internal/core/event_bus.go: personal live events
func (eb *EventBus) PublishToUser(userID uint, event SceneEvent)
func (eb *EventBus) PublishToUsers(userIDs []uint, event SceneEvent)
```

Costs: `VisibleTo` makes two queries whatever the number of owners (the explicit audiences of
the item, then the relations of the viewer toward the owners whose audience is neither public
nor private, in one `unnest` query); `Visibility` two queries whatever the number of items;
`CanView` is `VisibleTo` for one owner. Self is answered without a query.

Unknown item keys are programming errors: the methods return an error (not a `false`).
`ErrPrivacyRestricted()` (`403`, code `PRIVACY_RESTRICTED`, `apperrors.IsPrivacyRestricted`) is
the error to answer when a whole resource is hidden.

`response.NewRelationshipResponse` converts a `Relationship` to its JSON shape (below).

## API

Every route needs a signed-in user. The routes that create a relation need `social:use`
(`docs/RBAC.md`); undoing one is always allowed. Privacy is checked by the services. Users are
designated by id in actions and by exact username in reads.

`relationship` (the signed-in user toward the listed user): `{self, following, followed_by,
mutual, friend, request_sent, request_received, can_send_friend_request, can_message}`
(`can_message`: the user's message policy lets me write to them, `docs/MESSAGES.md`).

A list item: `{id, username, created_at, display_name, avatar_version, since, relationship}`
(`display_name` and `avatar_version` come from the profile, `docs/PROFILES.md`; '' and 0 when
not set) (`since`: date of the listed
relation: follow, friendship, request; the account creation in the directory). Lists are
paginated (`page`, `limit`, default 30, max `pagination.max_items_per_page`) in the usual
envelope `{data: [...], pagination: {page, limit, total_items, total_pages}}`, most recent
first.

| Route | Permission | Answer |
|---|---|---|
| `GET /api/v1/social/me` | | `{user, incoming_requests, outgoing_requests, friends, followers, following}` (`user`: the card of the signed-in user) |
| `GET /api/v1/social/users?q=` | | directory: the other users whose username contains `q` (all when empty), prefix matches first |
| `GET /api/v1/social/users/:username` | | `{user: {id, username, created_at, display_name, avatar_version}, relationship, visibility: {profile, friends_list, followers_list}, counts: {friends?, followers?, following?}}` (counts absent when hidden; the lists also need `profile`) |
| `GET /api/v1/social/users/:username/friends` | | list; `403 PRIVACY_RESTRICTED` unless `profile` and `friends_list` allow it |
| `GET /api/v1/social/users/:username/followers` | | list; needs `profile` and `followers_list` |
| `GET /api/v1/social/users/:username/following` | | list; needs `profile` and `followers_list` |
| `POST /api/v1/social/follows/:userId` | `social:use` | `{user_id, relationship}` |
| `DELETE /api/v1/social/follows/:userId` | | `{user_id, relationship}` |
| `GET /api/v1/social/friend-requests?direction=incoming\|outgoing` | | list (incoming: the senders, outgoing: the receivers) |
| `POST /api/v1/social/friend-requests` `{user_id}` | `social:use` | `201 {status: "sent"\|"accepted", user_id, relationship}` |
| `POST /api/v1/social/friend-requests/:userId/accept` | `social:use` | `{user_id, relationship}` (`:userId` = the sender) |
| `POST /api/v1/social/friend-requests/:userId/decline` | | `{user_id, relationship}` |
| `DELETE /api/v1/social/friend-requests/:userId` | | cancel my request to `:userId`: `{user_id, relationship}` |
| `DELETE /api/v1/social/friends/:userId` | | `{user_id, relationship}` |
| `GET /api/v1/social/privacy` | | `{items: [{key, label, description, group, default, audience, is_default}], audiences: [{value, label, description}], friend_request_policy, friend_request_policies: [{value, label, description}]}` |
| `PUT /api/v1/social/privacy` `{items?: {key: audience\|"default"}, friend_request_policy?}` | | the same as `GET`; absent items are unchanged |

The directory and the user cards only expose `id`, `username`, `created_at`, the display name
and the avatar version (no role, no login dates). Usernames, display names and avatars are not
private: every signed-in user can list them.

## Live events (SSE)

Social events are **personal**: published with `EventBus.PublishToUser(userID, event)`, they
reach only the streams of that user (every tab and device), never anyone else, superusers
included. The SSE filter (`internal/api/v1/handler/sse_filter.go`) drops a `social:*` event that
would be broadcast by mistake (`sseTargetedPrefixes`). The stream of a user subscribes with
`EventBus.SubscribeUser(userID)`; `Subscribe()` streams (no user) never get targeted events.

| Event | To | `data` |
|---|---|---|
| `social:friend_request` | the receiver | `{user: {id, username}, incoming_requests}` (sender, new pending count) |
| `social:friend_request_cancelled` | the receiver | `{user, incoming_requests}` |
| `social:friend_request_accepted` | the sender | `{user}` (who accepted) |
| `social:followed` | the followed user | `{user}` (the new follower) |
| `social:friend_requests_changed` | the user who accepted or declined | `{incoming_requests}` (syncs their other tabs) |

Presence has its own targeted family, `presence:update` (`docs/PRESENCE.md`), and the messages
theirs, `chat:*` (`docs/MESSAGES.md`).

To add a personal event family (presence...): publish with `PublishToUser` /
`PublishToUsers` (with `PrivacyService.AudienceMembers` to pick the users), add its prefix to
`sseTargetedPrefixes`, and in the frontend add the types to `LISTENER_EVENT_TYPES`
(`web/app/composables/useSSE.ts`) and to `SSE_EVENT_TYPES` (`web/public/workers/sse-worker.js`),
then listen with `onSSEEvent(type, fn)`.

## Frontend

- Header: `HeaderFriendsLink` (link to `/friends`, badge of pending incoming requests, kept live
  by the `social` store, refreshed after an SSE reconnection).
- `/friends`: tabs Friends, Requests (received: accept/decline, sent: cancel), Following,
  Followers, Find people (`?tab=`).
- `/u/<username>`: the profile page (`docs/PROFILES.md`): `ProfileHeader` (identity, badges,
  follow/friend buttons, counts), the blocks, and `SocialProfileLists`
  (friends/followers/following, each behind its privacy, opened from the counts).
- Settings > Privacy (`SettingsPrivacy`): one audience per catalog item, grouped, the
  friend request policy and the messages settings (`SettingsPrivacyMessages`: who may write to
  me, toasts; `docs/MESSAGES.md`); each change is saved at once.
- Messages (`docs/MESSAGES.md`): header entry `HeaderMessages` (unread badge, recent
  conversations), `/messages/<username>`, "Message" (`SocialMessageButton`) on profile headers
  and user rows when `relationship.can_message`. The Presence group also has the
  invisible mode toggle (`SettingsPrivacyInvisibleToggle`, `docs/PRESENCE.md`).
- Presence (`docs/PRESENCE.md`): `HeaderFriendsPresence` (who is online, watching), presence
  dots on `SocialUserRow` and the profile header (`SocialPresenceDot`, `SocialPresenceBadge`).
- Reusable pieces: `SocialUserAvatar` (avatar image or initials), `SocialUserRow`, `SocialUserList` (paginated, "load more"),
  `SocialRelationshipBadges`, `SocialRelationshipActions`, `useApiSocial()`, `useSocialStore()`,
  `utils/social.ts` (`relationshipBadges`, `friendAction`, `profilePath`). Usernames in lists
  link to `/u/<username>` (`profilePath`).

## Configuration

```yaml
social:
  friend_requests_per_hour: 30  # per user (0 = unlimited)
  follows_per_hour: 60          # per user (0 = unlimited)
  max_pending_requests: 50      # pending outgoing requests per user (0 = unlimited)
```

## Message policy

Who may send a user messages (`user_social_settings.message_policy`, migration 102):
`everyone`, `mutuals` (friends and mutuals), `friends` (default) or `nobody`, checked at send
time; see `docs/MESSAGES.md`.

## Known limitations

- No blocking: a user cannot prevent someone from following them (use `mutuals` or `friends`
  audiences for what matters), nor remove a follower, nor stop one user from writing (only the
  message policy: `friends`, `nobody`...).
- The rate limits are in memory (single process): they reset on restart.
