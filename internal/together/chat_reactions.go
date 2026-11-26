package together

import (
	"encoding/json"
	"strconv"
	"time"

	"goonhub/internal/core"
)

// maxReactionKeyBytes is the longest reaction key accepted.
const maxReactionKeyBytes = 64

// chatIndex finds a chat message of the history by id (-1: not there anymore).
func (l *lobby) chatIndex(id int64) int {
	for i := range l.chat {
		if l.chat[i].ID == id {
			return i
		}
	}
	return -1
}

// cmdChatReact adds (add) or removes the member's reaction to a chat message of the history:
// an emoji or ":name:" of a server emote. Members only (every connection is one), not on
// system messages, rate limited (chat_reaction_rate_limit per rate_window), capped
// (chat_max_reaction_kinds distinct keys per message, chat_max_reactions_per_user per member).
// Adding a reaction that is there, or removing one that is not, changes nothing. The whole
// reaction list of the message is broadcast, anonymous reactors redacted for non-moderators.
func (l *lobby) cmdChatReact(c *Client, in *inbound, now time.Time, add bool) error {
	var id int64
	if err := json.Unmarshal(in.ID, &id); err != nil || id <= 0 {
		return refuse(ErrCodeInvalid, "id is required")
	}
	if in.Emote == nil || *in.Emote == "" || len(*in.Emote) > maxReactionKeyBytes {
		return refuse(ErrCodeInvalid, "emote is required")
	}
	key := *in.Emote
	if add {
		switch core.CheckReactionKey(key, l.hub.emotes) {
		case core.ReactionKeyUnknownEmote:
			return refuse(ErrCodeNotFound, "no such emote")
		case core.ReactionKeyInvalid:
			return refuse(ErrCodeInvalid, "not an emoji")
		}
	}
	i := l.chatIndex(id)
	if i < 0 {
		return refuse(ErrCodeNotFound, "no such message")
	}
	msg := &l.chat[i]
	if msg.System {
		return refuse(ErrCodeNotAllowed, "system messages take no reaction")
	}

	// the member's reactions on the message, and the key's group
	group := -1
	mine := 0
	reacted := false
	for gi, r := range msg.reactions {
		if r.emote == key {
			group = gi
		}
		for _, u := range r.users {
			if u.userID == c.userID {
				mine++
				if r.emote == key {
					reacted = true
				}
			}
		}
	}
	if add == reacted {
		return nil // already there / not there
	}
	cfg := l.hub.cfg
	if add {
		if group < 0 && cfg.ChatMaxReactionKinds > 0 && len(msg.reactions) >= cfg.ChatMaxReactionKinds {
			return refuse(ErrCodeReactionLimit, "a message can have at most "+strconv.Itoa(cfg.ChatMaxReactionKinds)+" different reactions")
		}
		if cfg.ChatMaxReactionsPerUser > 0 && mine >= cfg.ChatMaxReactionsPerUser {
			return refuse(ErrCodeReactionLimit, "you can add at most "+strconv.Itoa(cfg.ChatMaxReactionsPerUser)+" reactions to a message")
		}
	}
	if !l.limiter(l.chatReactionLimiters, c.userID, cfg.ChatReactionRateLimit).AllowN(now, 1) {
		return refuse(ErrCodeRateLimited, "slow down")
	}

	if add {
		who := reactor{userID: c.userID, username: c.username}
		if m := l.members[c.userID]; m != nil {
			who.anon = m.anon
		}
		if group < 0 {
			msg.reactions = append(msg.reactions, &chatReaction{emote: key, users: []reactor{who}})
		} else {
			msg.reactions[group].users = append(msg.reactions[group].users, who)
		}
	} else {
		r := msg.reactions[group]
		users := make([]reactor, 0, len(r.users))
		for _, u := range r.users {
			if u.userID != c.userID {
				users = append(users, u)
			}
		}
		if len(users) == 0 {
			msg.reactions = append(msg.reactions[:group:group], msg.reactions[group+1:]...)
		} else {
			r.users = users
		}
	}

	list := msg.reactions
	l.broadcastSplit(func(full bool) any {
		views := reactionViews(list, full)
		if views == nil {
			views = []ChatReaction{}
		}
		return chatReactionsMsg{T: msgChatReactions, ID: id, Reactions: views}
	})
	return nil
}
