package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/lifecycle"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

// MessageAccessResolver resolves the live permissions of a user (RBACService), to resolve the
// share cards of a live event for each recipient.
type MessageAccessResolver interface {
	ResolveUser(userID uint) (*UserAccess, error)
}

// MessageService manages the conversations between users and the shares (docs/MESSAGES.md).
// Messages are private: only the participants of a conversation read it, there is no
// administrator bypass. Who may write to whom is the recipient's message policy, checked at
// send time.
type MessageService struct {
	repo     data.MessageRepository
	social   data.SocialRepository
	access   MessageAccessResolver
	lobbies  LobbyInvites
	eventBus *EventBus
	cfg      config.MessagesConfig
	logger   *zap.Logger
	now      func() time.Time

	mu       sync.Mutex
	limiters map[uint]*rate.Limiter
	typing   map[typingKey]time.Time

	lifecycle *lifecycle.Manager
	stopOnce  sync.Once
}

type typingKey struct {
	conversationID uint
	userID         uint
}

// Defaults of the messages settings when the configuration leaves them unset.
const (
	defaultMessageMaxLength     = 2000
	defaultMessagePageSize      = 50
	defaultMessageMaxPageSize   = 100
	defaultTypingInterval       = 3 * time.Second
	defaultMaxShareRecipients   = 20
	defaultMaxShareItems        = 10
	defaultMessagePurgeInterval = time.Hour
	// messageBodyHardLimit is the database limit (chk_messages_body_length)
	messageBodyHardLimit = 10000
)

func normalizeMessagesConfig(cfg config.MessagesConfig) config.MessagesConfig {
	if cfg.MaxLength <= 0 {
		cfg.MaxLength = defaultMessageMaxLength
	}
	if cfg.MaxLength > messageBodyHardLimit {
		cfg.MaxLength = messageBodyHardLimit
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = defaultMessagePageSize
	}
	if cfg.MaxPageSize <= 0 {
		cfg.MaxPageSize = defaultMessageMaxPageSize
	}
	if cfg.PageSize > cfg.MaxPageSize {
		cfg.PageSize = cfg.MaxPageSize
	}
	if cfg.TypingInterval <= 0 {
		cfg.TypingInterval = defaultTypingInterval
	}
	if cfg.MaxShareRecipients <= 0 {
		cfg.MaxShareRecipients = defaultMaxShareRecipients
	}
	if cfg.MaxShareItems <= 0 {
		cfg.MaxShareItems = defaultMaxShareItems
	}
	if cfg.RateBurst <= 0 {
		cfg.RateBurst = 10
	}
	if cfg.PurgeInterval <= 0 {
		cfg.PurgeInterval = defaultMessagePurgeInterval
	}
	return cfg
}

// NewMessageService creates the message service. lobbies may be nil (Goon2gether disabled:
// lobby invites are refused and shown as not available). It listens to the relation changes
// to tell open conversations whether one can still write.
func NewMessageService(repo data.MessageRepository, social data.SocialRepository, socialService *SocialService, access MessageAccessResolver, lobbies LobbyInvites, eventBus *EventBus, cfg config.MessagesConfig, logger *zap.Logger) *MessageService {
	logger = logger.With(zap.String("service", "messages"))
	s := &MessageService{
		repo:      repo,
		social:    social,
		access:    access,
		lobbies:   lobbies,
		eventBus:  eventBus,
		cfg:       normalizeMessagesConfig(cfg),
		logger:    logger,
		now:       time.Now,
		limiters:  make(map[uint]*rate.Limiter),
		typing:    make(map[typingKey]time.Time),
		lifecycle: lifecycle.NewManager(logger),
	}
	if socialService != nil {
		socialService.OnRelationChange(s.onRelationChange)
	}
	return s
}

// Config returns the effective settings.
func (s *MessageService) Config() config.MessagesConfig { return s.cfg }

// --- lifecycle: retention ---

// Start runs the periodic cleanup: messages older than retention_days (when set) and the
// conversations nobody takes part in anymore.
func (s *MessageService) Start() {
	s.lifecycle.Go("messages-purge", func(done <-chan struct{}) {
		ticker := time.NewTicker(s.cfg.PurgeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				s.Purge(ctx)
				cancel()
			}
		}
	})
}

// Stop ends the cleanup loop.
func (s *MessageService) Stop() {
	s.stopOnce.Do(func() {
		if err := s.lifecycle.Shutdown(5 * time.Second); err != nil {
			s.logger.Warn("Messages purge loop did not stop in time", zap.Error(err))
		}
	})
}

// Purge deletes the messages past the retention (none when retention_days is 0) and the
// orphan conversations.
func (s *MessageService) Purge(ctx context.Context) {
	if s.cfg.RetentionDays > 0 {
		before := s.now().Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
		if n, err := s.repo.PurgeBefore(ctx, before); err != nil {
			s.logger.Warn("Failed to purge old messages", zap.Error(err))
		} else if n > 0 {
			s.logger.Info("Purged old messages", zap.Int64("count", n), zap.Int("retention_days", s.cfg.RetentionDays))
		}
	}
	if _, err := s.repo.DeleteOrphanConversations(ctx); err != nil {
		s.logger.Warn("Failed to delete orphan conversations", zap.Error(err))
	}
}

// --- helpers ---

// allow takes a token from the user's send limiter (rate_per_minute 0 = unlimited).
func (s *MessageService) allow(userID uint, n int) bool {
	if s.cfg.RatePerMinute <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.limiters[userID]
	if !ok {
		l = rate.NewLimiter(rate.Every(time.Minute/time.Duration(s.cfg.RatePerMinute)), s.cfg.RateBurst)
		s.limiters[userID] = l
	}
	return l.AllowN(s.now(), n)
}

func (s *MessageService) user(ctx context.Context, userID uint) (*data.SocialUser, error) {
	u, err := s.social.GetUser(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrUserNotFound(userID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read user: %w", err)
	}
	return u, nil
}

func messageUserFrom(u *data.SocialUser) MessageUser {
	return MessageUser{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, AvatarVersion: u.AvatarVersion}
}

// participant checks that the user takes part in the conversation and returns its
// participants (ErrConversationNotFound otherwise: the same answer as an unknown id).
func (s *MessageService) participant(ctx context.Context, userID, conversationID uint) ([]uint, error) {
	ids, err := s.repo.ParticipantIDs(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if id == userID {
			return ids, nil
		}
	}
	return nil, apperrors.ErrConversationNotFound()
}

// otherOf returns the other participant of a direct conversation (0: deleted user).
func otherOf(participants []uint, userID uint) uint {
	for _, id := range participants {
		if id != userID {
			return id
		}
	}
	return 0
}

// canSendTo tells whether the sender may write to the recipient now, and why not.
func (s *MessageService) canSendTo(ctx context.Context, sender MessageActor, recipientID uint) (bool, string, error) {
	if !sender.Access.Has(PermMessagesUse) {
		return false, ReadOnlyNoPermission, nil
	}
	if recipientID == 0 {
		return false, ReadOnlyDeletedUser, nil
	}
	rels, err := loadRelationships(ctx, s.social, sender.ID, []uint{recipientID})
	if err != nil {
		return false, "", err
	}
	if !rels[recipientID].CanMessage {
		return false, ReadOnlyPolicy, nil
	}
	return true, "", nil
}

// closedError is the error of a refused send.
func closedError(reason string) error {
	switch reason {
	case ReadOnlyNoPermission:
		return apperrors.NewForbiddenError("you are not allowed to send messages")
	case ReadOnlyDeletedUser:
		return apperrors.ErrMessagesClosed("this user no longer exists")
	default:
		return apperrors.ErrMessagesClosed("")
	}
}

// cleanBody trims a body and checks its length (characters).
func (s *MessageService) cleanBody(body string) (string, error) {
	body = strings.TrimSpace(strings.ToValidUTF8(body, ""))
	if n := utf8.RuneCountInString(body); n > s.cfg.MaxLength {
		return "", apperrors.NewValidationErrorWithField("body", fmt.Sprintf("a message is at most %d characters", s.cfg.MaxLength))
	}
	return body, nil
}

// prepareAttachment checks that the sender may share the item and fills the message's
// attachment columns.
func (s *MessageService) prepareAttachment(ctx context.Context, sender MessageActor, in *AttachmentInput, msg *data.Message) error {
	if in == nil {
		return nil
	}
	kind := in.Kind
	switch kind {
	case data.AttachmentScene:
		if !sender.Access.Has(PermScenesView) {
			return apperrors.NewForbiddenError("you are not allowed to share videos")
		}
		if in.SceneID == 0 {
			return apperrors.NewValidationErrorWithField("scene_id", "required")
		}
		rows, err := s.repo.SceneCards(ctx, []uint{in.SceneID})
		if err != nil {
			return err
		}
		if len(rows) == 0 || rows[0].DeletedAt != nil || rows[0].TrashedAt != nil {
			return apperrors.ErrShareNotAvailable("this video does not exist")
		}
		if in.Time != nil && *in.Time < 0 {
			return apperrors.NewValidationErrorWithField("time", "must be positive")
		}
		id := in.SceneID
		msg.AttachmentSceneID = &id
		if in.Time != nil && *in.Time > 0 {
			t := *in.Time
			msg.AttachmentTimeS = &t
		}
	case data.AttachmentMarker:
		if !sender.Access.Has(PermScenesView) {
			return apperrors.NewForbiddenError("you are not allowed to share markers")
		}
		if in.MarkerID == 0 {
			return apperrors.NewValidationErrorWithField("marker_id", "required")
		}
		ok, err := s.repo.MarkerVisibleTo(ctx, in.MarkerID, sender.ID)
		if err != nil {
			return err
		}
		if !ok {
			return apperrors.ErrShareNotAvailable("this marker does not exist")
		}
		id := in.MarkerID
		msg.AttachmentMarkerID = &id
	case data.AttachmentPlaylist:
		if !sender.Access.Has(PermScenesView) {
			return apperrors.NewForbiddenError("you are not allowed to share playlists")
		}
		pid, err := uuid.Parse(in.PlaylistUUID)
		if err != nil {
			return apperrors.NewValidationErrorWithField("playlist_uuid", "invalid playlist id")
		}
		id, err := s.repo.PlaylistIDByUUID(ctx, pid)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.ErrShareNotAvailable("this playlist does not exist")
		}
		if err != nil {
			return err
		}
		rows, err := s.repo.PlaylistCards(ctx, []uint{id})
		if err != nil {
			return err
		}
		if len(rows) == 0 || (rows[0].UserID != sender.ID && rows[0].Visibility != "public") {
			return apperrors.ErrShareNotAvailable("this playlist does not exist")
		}
		msg.AttachmentPlaylistID = &id
	case data.AttachmentLobby:
		if s.lobbies == nil {
			return apperrors.ErrShareNotAvailable("watch lobbies are disabled")
		}
		if !sender.Access.Has(PermTogetherUse) {
			return apperrors.NewForbiddenError("you are not allowed to invite to lobbies")
		}
		lid, err := uuid.Parse(in.LobbyID)
		if err != nil {
			return apperrors.NewValidationErrorWithField("lobby_id", "invalid lobby id")
		}
		if err := s.lobbies.CanInvite(lid, sender.ID); err != nil {
			return err
		}
		msg.AttachmentLobbyUUID = &lid
	default:
		return apperrors.NewValidationErrorWithField("kind", fmt.Sprintf("unknown attachment kind %q", kind))
	}
	msg.AttachmentKind = &kind
	return nil
}

// view builds the view of a message for a viewer (attachment resolved).
func (s *MessageService) view(ctx context.Context, m data.Message, viewer *UserAccess) (MessageView, error) {
	views, err := s.views(ctx, []data.Message{m}, viewer)
	if err != nil {
		return MessageView{}, err
	}
	return views[0], nil
}

func (s *MessageService) views(ctx context.Context, msgs []data.Message, viewer *UserAccess) ([]MessageView, error) {
	attachments, err := s.resolveAttachments(ctx, msgs, viewer)
	if err != nil {
		return nil, err
	}
	out := make([]MessageView, len(msgs))
	for i, m := range msgs {
		v := MessageView{
			ID:             m.ID,
			ConversationID: m.ConversationID,
			SenderID:       m.SenderID,
			Body:           m.Body,
			EditedAt:       m.EditedAt,
			Deleted:        m.DeletedAt != nil,
			CreatedAt:      m.CreatedAt,
		}
		if v.Deleted {
			v.Body = ""
		} else {
			v.Attachment = attachments[m.ID]
		}
		out[i] = v
	}
	return out, nil
}

// publish sends an event to each participant, the message resolved with each one's access.
func (s *MessageService) publishMessage(ctx context.Context, eventType string, participants []uint, m data.Message, senderAccess *UserAccess) {
	if s.eventBus == nil {
		return
	}
	for _, uid := range participants {
		access := senderAccess
		if m.SenderID == nil || uid != *m.SenderID {
			var err error
			if access, err = s.accessOf(uid); err != nil {
				s.logger.Warn("Failed to resolve the access of a recipient", zap.Uint("user_id", uid), zap.Error(err))
				continue
			}
		}
		v, err := s.view(ctx, m, access)
		if err != nil {
			s.logger.Warn("Failed to build a message event", zap.Uint("user_id", uid), zap.Error(err))
			continue
		}
		s.eventBus.PublishToUser(uid, SceneEvent{Type: eventType, Data: map[string]any{"conversation_id": m.ConversationID, "message": v}})
	}
}

func (s *MessageService) accessOf(userID uint) (*UserAccess, error) {
	if s.access == nil {
		return NewUserAccess(userID, "", false), nil
	}
	return s.access.ResolveUser(userID)
}

func (s *MessageService) publishTo(userIDs []uint, eventType string, payload map[string]any) {
	if s.eventBus == nil {
		return
	}
	for _, uid := range userIDs {
		s.eventBus.PublishToUser(uid, SceneEvent{Type: eventType, Data: payload})
	}
}

// onRelationChange tells both users that their conversation (if any) changed: whether one
// can still write depends on the relation.
func (s *MessageService) onRelationChange(a, b uint) {
	if a == b || s.eventBus == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conv, err := s.repo.FindDirect(ctx, a, b)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("Failed to find a conversation after a relation change", zap.Error(err))
		}
		return
	}
	s.publishTo([]uint{a, b}, EventChatConversationUpdated, map[string]any{"conversation_id": conv.ID})
}

// --- conversations ---

// conversationViews builds the views of conversation rows for the viewer: whether they may
// still write (one relation query for every other participant).
func (s *MessageService) conversationViews(ctx context.Context, viewer MessageActor, rows []data.ConversationRow) ([]ConversationView, error) {
	var others []uint
	for _, r := range rows {
		if r.OtherUserID != nil {
			others = append(others, *r.OtherUserID)
		}
	}
	rels := map[uint]Relationship{}
	if len(others) > 0 && viewer.Access.Has(PermMessagesUse) {
		var err error
		if rels, err = loadRelationships(ctx, s.social, viewer.ID, others); err != nil {
			return nil, err
		}
	}
	out := make([]ConversationView, len(rows))
	for i, r := range rows {
		v := ConversationView{
			ID:                     r.ID,
			Kind:                   r.Kind,
			UnreadCount:            r.UnreadCount,
			Muted:                  r.Muted,
			LastReadMessageID:      r.LastReadMessageID,
			OtherLastReadMessageID: r.OtherLastReadMessageID,
			UpdatedAt:              r.CreatedAt,
		}
		if r.LastMessageAt != nil {
			v.UpdatedAt = *r.LastMessageAt
		}
		if r.OtherUserID != nil {
			v.Other = &MessageUser{ID: *r.OtherUserID, Username: r.OtherUsername, DisplayName: r.OtherDisplayName, AvatarVersion: r.OtherAvatarVersion}
		}
		if r.LastID != nil && r.LastCreatedAt != nil {
			p := &MessagePreview{ID: *r.LastID, SenderID: r.LastSenderID, Body: r.LastBody, AttachmentKind: r.LastAttachmentKind, Deleted: r.LastDeletedAt != nil, CreatedAt: *r.LastCreatedAt}
			if p.Deleted {
				p.Body = ""
				p.AttachmentKind = nil
			}
			v.LastMessage = p
		}
		switch {
		case !viewer.Access.Has(PermMessagesUse):
			v.ReadOnlyReason = ReadOnlyNoPermission
		case r.OtherUserID == nil:
			v.ReadOnlyReason = ReadOnlyDeletedUser
		case !rels[*r.OtherUserID].CanMessage:
			v.ReadOnlyReason = ReadOnlyPolicy
		default:
			v.CanSend = true
		}
		out[i] = v
	}
	return out, nil
}

// ListConversations lists the conversations of the viewer, most recent first.
func (s *MessageService) ListConversations(ctx context.Context, viewer MessageActor, page, limit int) (*ConversationPage, error) {
	rows, total, err := s.repo.ListConversations(ctx, viewer.ID, page, limit)
	if err != nil {
		return nil, err
	}
	views, err := s.conversationViews(ctx, viewer, rows)
	if err != nil {
		return nil, err
	}
	return &ConversationPage{Conversations: views, Total: total}, nil
}

// Conversation returns one conversation of the viewer.
func (s *MessageService) Conversation(ctx context.Context, viewer MessageActor, conversationID uint) (*ConversationView, error) {
	row, err := s.repo.GetConversationRow(ctx, viewer.ID, conversationID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrConversationNotFound()
	}
	if err != nil {
		return nil, err
	}
	views, err := s.conversationViews(ctx, viewer, []data.ConversationRow{*row})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// WithUser returns the thread of the viewer with a user (by username): the conversation when
// one exists and whether the viewer may write.
func (s *MessageService) WithUser(ctx context.Context, viewer MessageActor, username string) (*DirectThread, error) {
	u, err := s.social.GetUserByUsername(ctx, username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrUserNotFoundByUsername(username)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read user: %w", err)
	}
	if u.ID == viewer.ID {
		return nil, apperrors.NewValidationError("you cannot write to yourself")
	}
	out := &DirectThread{User: messageUserFrom(u)}
	conv, err := s.repo.FindDirect(ctx, viewer.ID, u.ID)
	switch {
	case err == nil:
		if out.Conversation, err = s.Conversation(ctx, viewer, conv.ID); err != nil {
			return nil, err
		}
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, err
	}
	if out.Conversation != nil {
		out.CanSend, out.ReadOnlyReason = out.Conversation.CanSend, out.Conversation.ReadOnlyReason
		return out, nil
	}
	ok, reason, err := s.canSendTo(ctx, viewer, u.ID)
	if err != nil {
		return nil, err
	}
	out.CanSend, out.ReadOnlyReason = ok, reason
	return out, nil
}

// Unread counts the unread messages of the user (muted conversations excluded).
func (s *MessageService) Unread(ctx context.Context, userID uint) (UnreadView, error) {
	sum, err := s.repo.UnreadSummary(ctx, userID)
	if err != nil {
		return UnreadView{}, err
	}
	return UnreadView{Messages: sum.Messages, Conversations: sum.Conversations}, nil
}

// History returns a page of messages of a conversation, oldest first, older than beforeID
// (0: the latest).
func (s *MessageService) History(ctx context.Context, viewer MessageActor, conversationID, beforeID uint, limit int) (*MessageHistory, error) {
	if _, err := s.participant(ctx, viewer.ID, conversationID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = s.cfg.PageSize
	}
	if limit > s.cfg.MaxPageSize {
		limit = s.cfg.MaxPageSize
	}
	msgs, err := s.repo.ListMessages(ctx, conversationID, beforeID, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	// newest first from the repository: oldest first for the client
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	views, err := s.views(ctx, msgs, viewer.Access)
	if err != nil {
		return nil, err
	}
	return &MessageHistory{Messages: views, HasMore: hasMore}, nil
}

// --- sending ---

// SendDirect sends a message to a user, creating the conversation with them on the first
// message. Refused: oneself, unknown user, the recipient's policy, the rate limit, an empty
// message, an item the sender may not share.
func (s *MessageService) SendDirect(ctx context.Context, sender MessageActor, recipientID uint, in SendMessageInput) (*ConversationView, *MessageView, error) {
	if recipientID == sender.ID {
		return nil, nil, apperrors.NewValidationError("you cannot write to yourself")
	}
	if _, err := s.user(ctx, recipientID); err != nil {
		return nil, nil, err
	}
	ok, reason, err := s.canSendTo(ctx, sender, recipientID)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, closedError(reason)
	}
	msg, err := s.buildMessage(ctx, sender, in)
	if err != nil {
		return nil, nil, err
	}
	if !s.allow(sender.ID, 1) {
		return nil, nil, apperrors.ErrMessageRateLimited()
	}
	conv, _, err := s.repo.FindOrCreateDirect(ctx, sender.ID, recipientID)
	if err != nil {
		return nil, nil, err
	}
	view, err := s.insert(ctx, sender, conv.ID, []uint{sender.ID, recipientID}, msg)
	if err != nil {
		return nil, nil, err
	}
	cv, err := s.Conversation(ctx, sender, conv.ID)
	if err != nil {
		return nil, nil, err
	}
	return cv, view, nil
}

// Send sends a message in a conversation of the sender (the recipient's policy applies).
func (s *MessageService) Send(ctx context.Context, sender MessageActor, conversationID uint, in SendMessageInput) (*MessageView, error) {
	participants, err := s.participant(ctx, sender.ID, conversationID)
	if err != nil {
		return nil, err
	}
	ok, reason, err := s.canSendTo(ctx, sender, otherOf(participants, sender.ID))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, closedError(reason)
	}
	msg, err := s.buildMessage(ctx, sender, in)
	if err != nil {
		return nil, err
	}
	if !s.allow(sender.ID, 1) {
		return nil, apperrors.ErrMessageRateLimited()
	}
	return s.insert(ctx, sender, conversationID, participants, msg)
}

// buildMessage validates a message input (body, attachment) for the sender.
func (s *MessageService) buildMessage(ctx context.Context, sender MessageActor, in SendMessageInput) (*data.Message, error) {
	body, err := s.cleanBody(in.Body)
	if err != nil {
		return nil, err
	}
	if body == "" && in.Attachment == nil {
		return nil, apperrors.NewValidationErrorWithField("body", "a message cannot be empty")
	}
	senderID := sender.ID
	msg := &data.Message{SenderID: &senderID, Body: body}
	if err := s.prepareAttachment(ctx, sender, in.Attachment, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// insert saves a message, publishes it to the participants and returns the sender's view.
func (s *MessageService) insert(ctx context.Context, sender MessageActor, conversationID uint, participants []uint, msg *data.Message) (*MessageView, error) {
	msg.ConversationID = conversationID
	msg.CreatedAt = s.now().UTC()
	if err := s.repo.InsertMessage(ctx, msg); err != nil {
		return nil, err
	}
	s.clearTyping(conversationID, sender.ID)
	s.publishMessage(ctx, EventChatMessage, participants, *msg, sender.Access)
	v, err := s.view(ctx, *msg, sender.Access)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Share sends items (scenes, markers, playlists, lobby invites) to several users: for each
// user, one message per item, the body on the first one. Each user is answered separately
// (policy, rate limit); the items are checked once for the sender (an item the sender may not
// share fails the whole share).
func (s *MessageService) Share(ctx context.Context, sender MessageActor, in ShareInput) ([]ShareResult, error) {
	if !sender.Access.Has(PermMessagesUse) {
		return nil, closedError(ReadOnlyNoPermission)
	}
	users := dedupeIDs(in.UserIDs)
	if len(users) == 0 {
		return nil, apperrors.NewValidationErrorWithField("user_ids", "pick at least one user")
	}
	if len(users) > s.cfg.MaxShareRecipients {
		return nil, apperrors.NewValidationErrorWithField("user_ids", fmt.Sprintf("at most %d users at once", s.cfg.MaxShareRecipients))
	}
	if len(in.Items) == 0 {
		return nil, apperrors.NewValidationErrorWithField("items", "nothing to share")
	}
	if len(in.Items) > s.cfg.MaxShareItems {
		return nil, apperrors.NewValidationErrorWithField("items", fmt.Sprintf("at most %d items at once", s.cfg.MaxShareItems))
	}
	body, err := s.cleanBody(in.Body)
	if err != nil {
		return nil, err
	}
	// The items, checked once: a template message per item
	templates := make([]data.Message, len(in.Items))
	for i := range in.Items {
		item := in.Items[i]
		if err := s.prepareAttachment(ctx, sender, &item, &templates[i]); err != nil {
			return nil, err
		}
	}
	rels, err := loadRelationships(ctx, s.social, sender.ID, users)
	if err != nil {
		return nil, err
	}
	results := make([]ShareResult, 0, len(users))
	for _, uid := range users {
		res := ShareResult{UserID: uid}
		fail := func(err error) {
			res.Code = apperrors.GetCode(err)
			res.Error = err.Error()
		}
		switch {
		case uid == sender.ID:
			fail(apperrors.NewValidationError("you cannot write to yourself"))
		case !rels[uid].CanMessage:
			fail(apperrors.ErrMessagesClosed(""))
		case !s.allow(sender.ID, 1):
			fail(apperrors.ErrMessageRateLimited())
		default:
			if _, err := s.user(ctx, uid); err != nil {
				fail(err)
				break
			}
			conv, _, err := s.repo.FindOrCreateDirect(ctx, sender.ID, uid)
			if err != nil {
				return nil, err
			}
			res.ConversationID = conv.ID
			for i, tpl := range templates {
				msg := tpl
				senderID := sender.ID
				msg.ID = 0
				msg.SenderID = &senderID
				msg.Body = ""
				if i == 0 {
					msg.Body = body
				}
				if _, err := s.insert(ctx, sender, conv.ID, []uint{sender.ID, uid}, &msg); err != nil {
					return nil, err
				}
			}
			res.OK = true
		}
		results = append(results, res)
	}
	return results, nil
}

func dedupeIDs(ids []uint) []uint {
	seen := make(map[uint]bool, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ownMessage returns a message of the conversation sent by the user.
func (s *MessageService) ownMessage(ctx context.Context, userID, conversationID, messageID uint) (*data.Message, []uint, error) {
	participants, err := s.participant(ctx, userID, conversationID)
	if err != nil {
		return nil, nil, err
	}
	m, err := s.repo.GetMessage(ctx, messageID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && m.ConversationID != conversationID) {
		return nil, nil, apperrors.ErrMessageNotFound()
	}
	if err != nil {
		return nil, nil, err
	}
	if m.SenderID == nil || *m.SenderID != userID {
		return nil, nil, apperrors.ErrNotMessageAuthor()
	}
	return m, participants, nil
}

// Edit changes the body of one of the sender's messages (not deleted). Like sending, it needs
// the recipient's policy to still allow the sender.
func (s *MessageService) Edit(ctx context.Context, sender MessageActor, conversationID, messageID uint, body string) (*MessageView, error) {
	m, participants, err := s.ownMessage(ctx, sender.ID, conversationID, messageID)
	if err != nil {
		return nil, err
	}
	if m.DeletedAt != nil {
		return nil, apperrors.ErrMessageNotFound()
	}
	ok, reason, err := s.canSendTo(ctx, sender, otherOf(participants, sender.ID))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, closedError(reason)
	}
	clean, err := s.cleanBody(body)
	if err != nil {
		return nil, err
	}
	if clean == "" && m.AttachmentKind == nil {
		return nil, apperrors.NewValidationErrorWithField("body", "a message cannot be empty (delete it instead)")
	}
	if clean == m.Body {
		v, err := s.view(ctx, *m, sender.Access)
		return &v, err
	}
	if !s.allow(sender.ID, 1) {
		return nil, apperrors.ErrMessageRateLimited()
	}
	at := s.now().UTC()
	updated, err := s.repo.UpdateMessageBody(ctx, m.ID, clean, at)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, apperrors.ErrMessageNotFound()
	}
	m.Body = clean
	m.EditedAt = &at
	s.publishMessage(ctx, EventChatMessageUpdated, participants, *m, sender.Access)
	v, err := s.view(ctx, *m, sender.Access)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Delete deletes one of the user's messages (soft: shown as "message deleted"). Always
// allowed for the author, even in a read-only conversation.
func (s *MessageService) Delete(ctx context.Context, user MessageActor, conversationID, messageID uint) error {
	m, participants, err := s.ownMessage(ctx, user.ID, conversationID, messageID)
	if err != nil {
		return err
	}
	if m.DeletedAt != nil {
		return nil
	}
	at := s.now().UTC()
	deleted, err := s.repo.SoftDeleteMessage(ctx, m.ID, at)
	if err != nil {
		return err
	}
	if !deleted {
		return nil
	}
	m.DeletedAt = &at
	s.publishMessage(ctx, EventChatMessageUpdated, participants, *m, user.Access)
	return nil
}

// MarkRead marks the conversation read up to a message for the user, and tells the
// participants (the user's other tabs, the "seen" of the other side).
func (s *MessageService) MarkRead(ctx context.Context, userID, conversationID, messageID uint) error {
	participants, err := s.participant(ctx, userID, conversationID)
	if err != nil {
		return err
	}
	if messageID == 0 {
		return apperrors.NewValidationErrorWithField("message_id", "required")
	}
	changed, err := s.repo.MarkRead(ctx, conversationID, userID, messageID)
	if err != nil {
		return err
	}
	if changed {
		s.publishTo(participants, EventChatRead, map[string]any{"conversation_id": conversationID, "user_id": userID, "last_read_message_id": messageID})
	}
	return nil
}

// Typing relays that the user is typing to the other participants, at most once per
// typing_interval (nothing is stored). Only who may write can type.
func (s *MessageService) Typing(ctx context.Context, user MessageActor, conversationID uint) error {
	participants, err := s.participant(ctx, user.ID, conversationID)
	if err != nil {
		return err
	}
	key := typingKey{conversationID: conversationID, userID: user.ID}
	now := s.now()
	s.mu.Lock()
	last, seen := s.typing[key]
	if seen && now.Sub(last) < s.cfg.TypingInterval {
		s.mu.Unlock()
		return nil
	}
	s.typing[key] = now
	// forget the stale entries now and then (bounded by the active typers)
	if len(s.typing) > 1000 {
		for k, t := range s.typing {
			if now.Sub(t) > time.Minute {
				delete(s.typing, k)
			}
		}
	}
	s.mu.Unlock()
	other := otherOf(participants, user.ID)
	ok, _, err := s.canSendTo(ctx, user, other)
	if err != nil || !ok {
		return err
	}
	s.publishTo([]uint{other}, EventChatTyping, map[string]any{"conversation_id": conversationID, "user_id": user.ID})
	return nil
}

func (s *MessageService) clearTyping(conversationID, userID uint) {
	s.mu.Lock()
	delete(s.typing, typingKey{conversationID: conversationID, userID: userID})
	s.mu.Unlock()
}

// SetMuted mutes or unmutes a conversation for the user.
func (s *MessageService) SetMuted(ctx context.Context, user MessageActor, conversationID uint, muted bool) (*ConversationView, error) {
	if _, err := s.participant(ctx, user.ID, conversationID); err != nil {
		return nil, err
	}
	if err := s.repo.SetMuted(ctx, conversationID, user.ID, muted); err != nil {
		return nil, err
	}
	s.publishTo([]uint{user.ID}, EventChatConversationUpdated, map[string]any{"conversation_id": conversationID})
	return s.Conversation(ctx, user, conversationID)
}

// JoinLobby joins the lobby a message invites the user to (respecting the lobby's rules: a
// private lobby needs the inviter to still be a member, bans apply) and returns its id.
func (s *MessageService) JoinLobby(ctx context.Context, user MessageActor, conversationID, messageID uint) (string, error) {
	if _, err := s.participant(ctx, user.ID, conversationID); err != nil {
		return "", err
	}
	m, err := s.repo.GetMessage(ctx, messageID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && m.ConversationID != conversationID) {
		return "", apperrors.ErrMessageNotFound()
	}
	if err != nil {
		return "", err
	}
	if m.DeletedAt != nil || m.AttachmentKind == nil || *m.AttachmentKind != data.AttachmentLobby || m.AttachmentLobbyUUID == nil {
		return "", apperrors.ErrShareNotAvailable("this message is not a lobby invite")
	}
	if s.lobbies == nil {
		return "", apperrors.ErrShareNotAvailable("watch lobbies are disabled")
	}
	if !user.Access.Has(PermTogetherUse) {
		return "", apperrors.NewForbiddenError("you are not allowed to join lobbies")
	}
	var inviter uint
	if m.SenderID != nil {
		inviter = *m.SenderID
	}
	if err := s.lobbies.AcceptInvite(*m.AttachmentLobbyUUID, inviter, user.ID, user.Username); err != nil {
		return "", err
	}
	return m.AttachmentLobbyUUID.String(), nil
}

// --- settings ---

// Settings returns the message settings of the user.
func (s *MessageService) Settings(ctx context.Context, userID uint) (*MessageSettingsView, error) {
	st, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !validMessagePolicy(st.Policy) {
		st.Policy = data.MessagePolicyFriends
	}
	return &MessageSettingsView{
		Policy:           st.Policy,
		Toasts:           st.Toasts,
		Policies:         MessagePolicies,
		MaxLength:        s.cfg.MaxLength,
		TypingIntervalMs: s.cfg.TypingInterval.Milliseconds(),
	}, nil
}

// UpdateSettings changes the message settings of the user (nil = unchanged). The policy
// applies to the next message sent to the user (a sender refused then reloads the
// conversation, which shows it read-only).
func (s *MessageService) UpdateSettings(ctx context.Context, userID uint, policy *string, toasts *bool) (*MessageSettingsView, error) {
	if policy != nil && !validMessagePolicy(*policy) {
		return nil, apperrors.NewValidationErrorWithField("policy", fmt.Sprintf("unknown message policy %q", *policy))
	}
	if policy != nil || toasts != nil {
		if err := s.repo.SaveSettings(ctx, userID, policy, toasts); err != nil {
			return nil, err
		}
	}
	return s.Settings(ctx, userID)
}
