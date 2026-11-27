package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

// Defaults of the reaction settings when the configuration leaves them unset.
const (
	defaultMaxReactionKinds    = 20
	defaultMaxReactionsPerUser = 10
	defaultReactionRateBurst   = 20
)

// SetEmotes sets the server emotes accepted as ":name:" reactions (nil: Unicode emoji only,
// Goon2gether disabled).
func (s *MessageService) SetEmotes(emotes EmoteLookup) { s.emotes = emotes }

// allowReaction takes a token from the user's reaction limiter (reaction_rate_per_minute 0 =
// unlimited).
func (s *MessageService) allowReaction(userID uint) bool {
	if s.cfg.ReactionRatePerMinute <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.reactionLimiters[userID]
	if !ok {
		l = rate.NewLimiter(rate.Every(time.Minute/time.Duration(s.cfg.ReactionRatePerMinute)), s.cfg.ReactionRateBurst)
		s.reactionLimiters[userID] = l
	}
	return l.AllowN(s.now(), 1)
}

// groupReactions groups reaction rows (oldest first) by message, then by key in the order of
// their first reaction.
func groupReactions(rows []data.MessageReactionRow) map[uint][]MessageReactionView {
	out := make(map[uint][]MessageReactionView)
	index := make(map[uint]map[string]int)
	for _, r := range rows {
		keys := index[r.MessageID]
		if keys == nil {
			keys = make(map[string]int)
			index[r.MessageID] = keys
		}
		user := ReactionUser{ID: r.UserID, Username: r.Username, DisplayName: r.DisplayName}
		i, ok := keys[r.Emoji]
		if !ok {
			keys[r.Emoji] = len(out[r.MessageID])
			out[r.MessageID] = append(out[r.MessageID], MessageReactionView{Emoji: r.Emoji, Count: 1, Users: []ReactionUser{user}})
			continue
		}
		g := &out[r.MessageID][i]
		g.Count++
		g.Users = append(g.Users, user)
	}
	return out
}

// reactionsOf reads the reactions of messages, grouped.
func (s *MessageService) reactionsOf(ctx context.Context, messageIDs []uint) (map[uint][]MessageReactionView, error) {
	if len(messageIDs) == 0 {
		return map[uint][]MessageReactionView{}, nil
	}
	rows, err := s.repo.ListReactions(ctx, messageIDs)
	if err != nil {
		return nil, err
	}
	return groupReactions(rows), nil
}

// messageReactions is the reaction list of one message.
func (s *MessageService) messageReactions(ctx context.Context, conversationID, messageID uint) (*MessageReactions, error) {
	groups, err := s.reactionsOf(ctx, []uint{messageID})
	if err != nil {
		return nil, err
	}
	list := groups[messageID]
	if list == nil {
		list = []MessageReactionView{}
	}
	return &MessageReactions{ConversationID: conversationID, MessageID: messageID, Reactions: list}, nil
}

// reactableMessage returns a message of a conversation of the user that is not deleted, with
// the participants (CONVERSATION_NOT_FOUND / MESSAGE_NOT_FOUND otherwise).
func (s *MessageService) reactableMessage(ctx context.Context, userID, conversationID, messageID uint) (*data.Message, []uint, error) {
	participants, err := s.participant(ctx, userID, conversationID)
	if err != nil {
		return nil, nil, err
	}
	m, err := s.repo.GetMessage(ctx, messageID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (m.ConversationID != conversationID || m.DeletedAt != nil)) {
		return nil, nil, apperrors.ErrMessageNotFound()
	}
	if err != nil {
		return nil, nil, err
	}
	return m, participants, nil
}

// cleanReactionKey trims a reaction key and checks it: a Unicode emoji or ":name:" of an
// existing server emote.
func (s *MessageService) cleanReactionKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	switch CheckReactionKey(key, s.emotes) {
	case ReactionKeyOK:
		return key, nil
	case ReactionKeyUnknownEmote:
		return "", apperrors.NewValidationErrorWithField("emoji", "no such emote")
	default:
		return "", apperrors.NewValidationErrorWithField("emoji", "not an emoji or a server emote")
	}
}

// publishReactions tells the participants that the reactions of a message changed.
func (s *MessageService) publishReactions(participants []uint, r *MessageReactions) {
	s.publishTo(participants, EventChatReaction, map[string]any{
		"conversation_id": r.ConversationID,
		"message_id":      r.MessageID,
		"reactions":       r.Reactions,
	})
}

// React adds the user's reaction to a message of their conversation. Like sending, it needs
// messages:use and the other participant's message policy to still allow the user (a
// read-only conversation takes no new reaction). Refused: a deleted message, an unknown
// emote, the caps (distinct keys per message, reactions per user), the rate limit. Reacting
// again with the same key changes nothing.
func (s *MessageService) React(ctx context.Context, user MessageActor, conversationID, messageID uint, key string) (*MessageReactions, error) {
	key, err := s.cleanReactionKey(key)
	if err != nil {
		return nil, err
	}
	m, participants, err := s.reactableMessage(ctx, user.ID, conversationID, messageID)
	if err != nil {
		return nil, err
	}
	ok, reason, err := s.canSendTo(ctx, user, otherOf(participants, user.ID))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, closedError(reason)
	}
	if !s.allowReaction(user.ID) {
		return nil, apperrors.ErrReactionRateLimited()
	}
	res, err := s.repo.AddReaction(ctx, m.ID, user.ID, key, s.cfg.MaxReactionKinds, s.cfg.MaxReactionsPerUser)
	if err != nil {
		return nil, err
	}
	switch res {
	case data.ReactionMessageGone:
		return nil, apperrors.ErrMessageNotFound()
	case data.ReactionTooManyKinds:
		return nil, apperrors.ErrReactionLimit(fmt.Sprintf("a message can have at most %d different reactions", s.cfg.MaxReactionKinds))
	case data.ReactionTooManyForUser:
		return nil, apperrors.ErrReactionLimit(fmt.Sprintf("you can add at most %d reactions to a message", s.cfg.MaxReactionsPerUser))
	}
	out, err := s.messageReactions(ctx, conversationID, m.ID)
	if err != nil {
		return nil, err
	}
	if res == data.ReactionAdded {
		s.publishReactions(participants, out)
	}
	return out, nil
}

// Unreact removes the user's reaction from a message of their conversation. Always allowed to
// a participant (like deleting one's own message), also in a read-only conversation; rate
// limited with the reactions. Removing a reaction that is not there changes nothing.
func (s *MessageService) Unreact(ctx context.Context, user MessageActor, conversationID, messageID uint, key string) (*MessageReactions, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > maxReactionKeyLength {
		return nil, apperrors.NewValidationErrorWithField("emoji", "invalid reaction")
	}
	m, participants, err := s.reactableMessage(ctx, user.ID, conversationID, messageID)
	if err != nil {
		return nil, err
	}
	if !s.allowReaction(user.ID) {
		return nil, apperrors.ErrReactionRateLimited()
	}
	removed, err := s.repo.RemoveReaction(ctx, m.ID, user.ID, key)
	if err != nil {
		return nil, err
	}
	out, err := s.messageReactions(ctx, conversationID, m.ID)
	if err != nil {
		return nil, err
	}
	if removed {
		s.publishReactions(participants, out)
	}
	return out, nil
}
