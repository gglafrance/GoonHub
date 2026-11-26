package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

// Social SSE events, always targeted at one user (EventBus.PublishToUser, docs/SOCIAL.md).
const (
	// EventSocialFriendRequest: to the receiver of a new friend request.
	EventSocialFriendRequest = "social:friend_request"
	// EventSocialFriendRequestCancelled: to the receiver, when the sender cancels.
	EventSocialFriendRequestCancelled = "social:friend_request_cancelled"
	// EventSocialFriendRequestAccepted: to the sender, when the receiver accepts.
	EventSocialFriendRequestAccepted = "social:friend_request_accepted"
	// EventSocialFollowed: to the followed user.
	EventSocialFollowed = "social:followed"
	// EventSocialRequestsChanged: to the user whose incoming requests changed by their own
	// action (accept, decline), to sync their other tabs and devices.
	EventSocialRequestsChanged = "social:friend_requests_changed"
)

// SocialService manages follows and friendships (docs/SOCIAL.md).
type SocialService struct {
	repo     data.SocialRepository
	privacy  *PrivacyService
	eventBus *EventBus
	cfg      config.SocialConfig
	logger   *zap.Logger

	mu              sync.Mutex
	requestLimiters map[uint]*rate.Limiter
	followLimiters  map[uint]*rate.Limiter
}

func NewSocialService(repo data.SocialRepository, privacy *PrivacyService, eventBus *EventBus, cfg config.SocialConfig, logger *zap.Logger) *SocialService {
	return &SocialService{
		repo:            repo,
		privacy:         privacy,
		eventBus:        eventBus,
		cfg:             cfg,
		logger:          logger.With(zap.String("service", "social")),
		requestLimiters: make(map[uint]*rate.Limiter),
		followLimiters:  make(map[uint]*rate.Limiter),
	}
}

// SocialUserEntry is a user in a social list, with the viewer's relation toward them.
type SocialUserEntry struct {
	User         data.SocialUser
	Relationship Relationship
}

// SocialUserPage is a page of a social list.
type SocialUserPage struct {
	Entries []SocialUserEntry
	Total   int64
}

// SocialProfile is the social summary of a user as a viewer sees it. Counts and lists are
// only filled when the viewer may see them.
type SocialProfile struct {
	User         data.SocialUser
	Relationship Relationship
	// ProfileVisible: the viewer can see the profile page (privacy item profile).
	ProfileVisible bool
	// FriendsVisible, FollowersVisible: the viewer can see the friends list and the
	// followers/following lists (each also needs the profile).
	FriendsVisible   bool
	FollowersVisible bool
	// Counts: Friends is set when FriendsVisible, Followers and Following when
	// FollowersVisible (nil otherwise).
	Friends   *int64
	Followers *int64
	Following *int64
}

// SocialSummary is the social state of the signed-in user (header badge, /friends tabs).
type SocialSummary struct {
	// User is the card of the user (nil if it could not be read)
	User             *data.SocialUser
	IncomingRequests int64
	OutgoingRequests int64
	Friends          int64
	Followers        int64
	Following        int64
}

// SocialList is a list of a user's relations.
type SocialList string

const (
	SocialListFriends   SocialList = "friends"
	SocialListFollowers SocialList = "followers"
	SocialListFollowing SocialList = "following"
)

// privacyItem is the privacy item that hides the list.
func (l SocialList) privacyItem() string {
	if l == SocialListFriends {
		return PrivacyFriendsList
	}
	return PrivacyFollowersList
}

// FriendRequestList is one side of the pending requests.
type FriendRequestList string

const (
	FriendRequestsIncoming FriendRequestList = "incoming"
	FriendRequestsOutgoing FriendRequestList = "outgoing"
)

// allow takes a token from the user's limiter of the action (perHour 0 = unlimited).
func (s *SocialService) allow(limiters map[uint]*rate.Limiter, userID uint, perHour int) bool {
	if perHour <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := limiters[userID]
	if !ok {
		burst := perHour / 3
		if burst < 1 {
			burst = 1
		}
		l = rate.NewLimiter(rate.Every(time.Hour/time.Duration(perHour)), burst)
		limiters[userID] = l
	}
	return l.Allow()
}

func (s *SocialService) userByID(ctx context.Context, userID uint) (*data.SocialUser, error) {
	u, err := s.repo.GetUser(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrUserNotFound(userID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read user: %w", err)
	}
	return u, nil
}

// UserByUsername returns the card of a user (not found error when unknown).
func (s *SocialService) UserByUsername(ctx context.Context, username string) (*data.SocialUser, error) {
	u, err := s.repo.GetUserByUsername(ctx, username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrUserNotFoundByUsername(username)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read user: %w", err)
	}
	return u, nil
}

// Relationship returns the relation of the viewer toward the owner.
func (s *SocialService) Relationship(ctx context.Context, viewerID, ownerID uint) (Relationship, error) {
	rels, err := loadRelationships(ctx, s.repo, viewerID, []uint{ownerID})
	if err != nil {
		return Relationship{}, err
	}
	return rels[ownerID], nil
}

// Relationships returns the relation of the viewer toward each user (one query); every
// requested id is a key of the map.
func (s *SocialService) Relationships(ctx context.Context, viewerID uint, userIDs []uint) (map[uint]Relationship, error) {
	rels, err := loadRelationships(ctx, s.repo, viewerID, userIDs)
	if err != nil {
		return nil, err
	}
	for _, id := range userIDs {
		if _, ok := rels[id]; !ok {
			rels[id] = Relationship{}
		}
	}
	return rels, nil
}

// FriendIDs returns the ids of the user's friends.
func (s *SocialService) FriendIDs(ctx context.Context, userID uint) ([]uint, error) {
	return s.repo.FriendIDs(ctx, userID)
}

// FollowerIDs returns the ids of the users following the user.
func (s *SocialService) FollowerIDs(ctx context.Context, userID uint) ([]uint, error) {
	return s.repo.FollowerIDs(ctx, userID)
}

func (s *SocialService) withRelationships(ctx context.Context, viewerID uint, users []data.SocialUser, total int64) (*SocialUserPage, error) {
	ids := make([]uint, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	rels, err := s.Relationships(ctx, viewerID, ids)
	if err != nil {
		return nil, err
	}
	page := &SocialUserPage{Entries: make([]SocialUserEntry, len(users)), Total: total}
	for i, u := range users {
		page.Entries[i] = SocialUserEntry{User: u, Relationship: rels[u.ID]}
	}
	return page, nil
}

// SearchUsers lists the other users whose username contains the query (every user when
// empty), with the viewer's relation toward each. The directory is open to every signed-in
// user (usernames are not private).
func (s *SocialService) SearchUsers(ctx context.Context, viewerID uint, query string, page, limit int) (*SocialUserPage, error) {
	users, total, err := s.repo.SearchUsers(ctx, viewerID, query, page, limit)
	if err != nil {
		return nil, err
	}
	return s.withRelationships(ctx, viewerID, users, total)
}

// Profile returns the social summary of the owner as the viewer sees it.
func (s *SocialService) Profile(ctx context.Context, viewerID uint, username string) (*SocialProfile, error) {
	owner, err := s.UserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	rel, err := s.Relationship(ctx, viewerID, owner.ID)
	if err != nil {
		return nil, err
	}
	visible, err := s.privacy.Visibility(ctx, viewerID, owner.ID, []string{PrivacyProfile, PrivacyFriendsList, PrivacyFollowersList})
	if err != nil {
		return nil, err
	}
	p := &SocialProfile{
		User:             *owner,
		Relationship:     rel,
		ProfileVisible:   visible[PrivacyProfile],
		FriendsVisible:   visible[PrivacyProfile] && visible[PrivacyFriendsList],
		FollowersVisible: visible[PrivacyProfile] && visible[PrivacyFollowersList],
	}
	if p.FriendsVisible || p.FollowersVisible {
		counts, err := s.repo.Counts(ctx, owner.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to count relations: %w", err)
		}
		if p.FriendsVisible {
			p.Friends = &counts.Friends
		}
		if p.FollowersVisible {
			p.Followers = &counts.Followers
			p.Following = &counts.Following
		}
	}
	return p, nil
}

// ListRelations returns a list (friends, followers, following) of the owner, if the viewer can
// see it (the profile and the list's privacy item), with the viewer's relation toward each
// listed user. ErrPrivacyRestricted when hidden.
func (s *SocialService) ListRelations(ctx context.Context, viewerID uint, username string, list SocialList, page, limit int) (*SocialUserPage, error) {
	owner, err := s.UserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	ok, err := s.privacy.CanViewAll(ctx, viewerID, owner.ID, PrivacyProfile, list.privacyItem())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperrors.ErrPrivacyRestricted()
	}
	var users []data.SocialUser
	var total int64
	switch list {
	case SocialListFriends:
		users, total, err = s.repo.ListFriends(ctx, owner.ID, page, limit)
	case SocialListFollowers:
		users, total, err = s.repo.ListFollowers(ctx, owner.ID, page, limit)
	case SocialListFollowing:
		users, total, err = s.repo.ListFollowing(ctx, owner.ID, page, limit)
	default:
		return nil, apperrors.NewValidationError("unknown list")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list %s: %w", list, err)
	}
	return s.withRelationships(ctx, viewerID, users, total)
}

// ListFriendRequests returns the pending requests of the user (incoming: their senders,
// outgoing: their receivers).
func (s *SocialService) ListFriendRequests(ctx context.Context, userID uint, side FriendRequestList, page, limit int) (*SocialUserPage, error) {
	var users []data.SocialUser
	var total int64
	var err error
	switch side {
	case FriendRequestsIncoming:
		users, total, err = s.repo.ListIncomingRequests(ctx, userID, page, limit)
	case FriendRequestsOutgoing:
		users, total, err = s.repo.ListOutgoingRequests(ctx, userID, page, limit)
	default:
		return nil, apperrors.NewValidationErrorWithField("direction", "must be incoming or outgoing")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list friend requests: %w", err)
	}
	return s.withRelationships(ctx, userID, users, total)
}

// Summary returns the counts of the user (pending requests and relations).
func (s *SocialService) Summary(ctx context.Context, userID uint) (*SocialSummary, error) {
	in, err := s.repo.CountIncomingRequests(ctx, userID)
	if err != nil {
		return nil, err
	}
	out, err := s.repo.CountOutgoingRequests(ctx, userID)
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.Counts(ctx, userID)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read user: %w", err)
	}
	return &SocialSummary{User: user, IncomingRequests: in, OutgoingRequests: out, Friends: counts.Friends, Followers: counts.Followers, Following: counts.Following}, nil
}

// checkTarget refuses self relations and unknown users.
func (s *SocialService) checkTarget(ctx context.Context, userID, targetID uint) (*data.SocialUser, error) {
	if userID == targetID {
		return nil, apperrors.ErrSelfRelation()
	}
	return s.userByID(ctx, targetID)
}

// Follow makes the user follow the target (idempotent) and notifies the target the first
// time. Returns the new relation.
func (s *SocialService) Follow(ctx context.Context, userID, targetID uint) (Relationship, error) {
	if _, err := s.checkTarget(ctx, userID, targetID); err != nil {
		return Relationship{}, err
	}
	rel, err := s.Relationship(ctx, userID, targetID)
	if err != nil {
		return Relationship{}, err
	}
	if rel.Following {
		return rel, nil
	}
	if !s.allow(s.followLimiters, userID, s.cfg.FollowsPerHour) {
		return Relationship{}, apperrors.ErrSocialRateLimited()
	}
	created, err := s.repo.Follow(ctx, userID, targetID)
	if err != nil {
		return Relationship{}, fmt.Errorf("failed to follow: %w", err)
	}
	if created {
		s.notify(ctx, targetID, EventSocialFollowed, userID, nil)
	}
	return s.Relationship(ctx, userID, targetID)
}

// Unfollow stops following (idempotent; always allowed). Returns the new relation.
func (s *SocialService) Unfollow(ctx context.Context, userID, targetID uint) (Relationship, error) {
	if _, err := s.checkTarget(ctx, userID, targetID); err != nil {
		return Relationship{}, err
	}
	if _, err := s.repo.Unfollow(ctx, userID, targetID); err != nil {
		return Relationship{}, fmt.Errorf("failed to unfollow: %w", err)
	}
	return s.Relationship(ctx, userID, targetID)
}

// SendFriendRequest sends a friend request to the target. When the target already sent one
// to the user, it is accepted instead (accepted = true). Refused: self, unknown user, already
// friends, already pending, the target's policy (accepting the target's own request is always
// possible), too many pending requests, rate limit.
func (s *SocialService) SendFriendRequest(ctx context.Context, userID, targetID uint) (accepted bool, rel Relationship, err error) {
	if _, err := s.checkTarget(ctx, userID, targetID); err != nil {
		return false, Relationship{}, err
	}
	current, err := s.Relationship(ctx, userID, targetID)
	if err != nil {
		return false, Relationship{}, err
	}
	switch {
	case current.Friend:
		return false, Relationship{}, apperrors.ErrAlreadyFriends()
	case current.RequestSent:
		return false, Relationship{}, apperrors.ErrFriendRequestPending()
	case !current.CanSendFriendRequest:
		return false, Relationship{}, apperrors.ErrFriendRequestsClosed()
	}
	if !current.RequestReceived {
		if s.cfg.MaxPendingRequests > 0 {
			n, err := s.repo.CountOutgoingRequests(ctx, userID)
			if err != nil {
				return false, Relationship{}, fmt.Errorf("failed to count requests: %w", err)
			}
			if n >= int64(s.cfg.MaxPendingRequests) {
				return false, Relationship{}, apperrors.ErrTooManyPendingRequests()
			}
		}
		if !s.allow(s.requestLimiters, userID, s.cfg.FriendRequestsPerHour) {
			return false, Relationship{}, apperrors.ErrSocialRateLimited()
		}
	}
	outcome, err := s.repo.SendFriendRequest(ctx, userID, targetID)
	if err != nil {
		return false, Relationship{}, err
	}
	switch outcome {
	case data.FriendRequestAlreadyFriends:
		return false, Relationship{}, apperrors.ErrAlreadyFriends()
	case data.FriendRequestAlreadySent:
		return false, Relationship{}, apperrors.ErrFriendRequestPending()
	case data.FriendRequestAccepted:
		accepted = true
		s.notify(ctx, targetID, EventSocialFriendRequestAccepted, userID, nil)
		s.notifyRequestsChanged(ctx, userID)
	default:
		s.notify(ctx, targetID, EventSocialFriendRequest, userID, s.incomingCount(ctx, targetID))
	}
	rel, err = s.Relationship(ctx, userID, targetID)
	return accepted, rel, err
}

// AcceptFriendRequest accepts the request the sender sent to the user.
func (s *SocialService) AcceptFriendRequest(ctx context.Context, userID, senderID uint) (Relationship, error) {
	if _, err := s.checkTarget(ctx, userID, senderID); err != nil {
		return Relationship{}, err
	}
	ok, err := s.repo.AcceptFriendRequest(ctx, userID, senderID)
	if err != nil {
		return Relationship{}, err
	}
	if !ok {
		return Relationship{}, apperrors.ErrFriendRequestNotFound()
	}
	s.notify(ctx, senderID, EventSocialFriendRequestAccepted, userID, nil)
	s.notifyRequestsChanged(ctx, userID)
	return s.Relationship(ctx, userID, senderID)
}

// DeclineFriendRequest deletes the request the sender sent to the user (the sender is not
// notified).
func (s *SocialService) DeclineFriendRequest(ctx context.Context, userID, senderID uint) (Relationship, error) {
	if _, err := s.checkTarget(ctx, userID, senderID); err != nil {
		return Relationship{}, err
	}
	ok, err := s.repo.DeleteFriendRequest(ctx, senderID, userID)
	if err != nil {
		return Relationship{}, fmt.Errorf("failed to decline friend request: %w", err)
	}
	if !ok {
		return Relationship{}, apperrors.ErrFriendRequestNotFound()
	}
	s.notifyRequestsChanged(ctx, userID)
	return s.Relationship(ctx, userID, senderID)
}

// CancelFriendRequest deletes the request the user sent to the receiver (the receiver's
// pending count is updated live).
func (s *SocialService) CancelFriendRequest(ctx context.Context, userID, receiverID uint) (Relationship, error) {
	if _, err := s.checkTarget(ctx, userID, receiverID); err != nil {
		return Relationship{}, err
	}
	ok, err := s.repo.DeleteFriendRequest(ctx, userID, receiverID)
	if err != nil {
		return Relationship{}, fmt.Errorf("failed to cancel friend request: %w", err)
	}
	if !ok {
		return Relationship{}, apperrors.ErrFriendRequestNotFound()
	}
	s.notify(ctx, receiverID, EventSocialFriendRequestCancelled, userID, s.incomingCount(ctx, receiverID))
	return s.Relationship(ctx, userID, receiverID)
}

// RemoveFriend ends a friendship (the other user is not notified; follows are kept).
func (s *SocialService) RemoveFriend(ctx context.Context, userID, friendID uint) (Relationship, error) {
	if _, err := s.checkTarget(ctx, userID, friendID); err != nil {
		return Relationship{}, err
	}
	ok, err := s.repo.RemoveFriend(ctx, userID, friendID)
	if err != nil {
		return Relationship{}, fmt.Errorf("failed to remove friend: %w", err)
	}
	if !ok {
		return Relationship{}, apperrors.NewNotFoundError("friend", friendID)
	}
	return s.Relationship(ctx, userID, friendID)
}

func (s *SocialService) incomingCount(ctx context.Context, userID uint) *int64 {
	n, err := s.repo.CountIncomingRequests(ctx, userID)
	if err != nil {
		s.logger.Warn("Failed to count incoming friend requests", zap.Uint("user_id", userID), zap.Error(err))
		return nil
	}
	return &n
}

func (s *SocialService) notifyRequestsChanged(ctx context.Context, userID uint) {
	if s.eventBus == nil {
		return
	}
	payload := map[string]any{}
	if n := s.incomingCount(ctx, userID); n != nil {
		payload["incoming_requests"] = *n
	}
	s.eventBus.PublishToUser(userID, SceneEvent{Type: EventSocialRequestsChanged, Data: payload})
}

// notify sends a social event to the target only: data {user: {id, username}} for the actor,
// plus incoming_requests when given.
func (s *SocialService) notify(ctx context.Context, targetID uint, eventType string, actorID uint, incoming *int64) {
	if s.eventBus == nil {
		return
	}
	actor, err := s.repo.GetUser(ctx, actorID)
	if err != nil {
		s.logger.Warn("Failed to read the actor of a social event", zap.Uint("user_id", actorID), zap.Error(err))
		return
	}
	payload := map[string]any{"user": map[string]any{"id": actor.ID, "username": actor.Username}}
	if incoming != nil {
		payload["incoming_requests"] = *incoming
	}
	s.eventBus.PublishToUser(targetID, SceneEvent{Type: eventType, Data: payload})
}
