package core

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// EmoteLookup tells whether a server custom emote exists (together.EmoteService), to accept
// ":name:" reactions. nil: only Unicode emoji are accepted.
type EmoteLookup interface {
	HasEmote(name string) bool
}

// maxReactionKeyLength is the longest reaction key (bytes, the column is 64 characters).
const maxReactionKeyLength = 64

// ValidEmoji accepts a short emoji sequence: non-ASCII, no whitespace nor markup (keycaps,
// zero width joiners and variation selectors are allowed). Shared by the message reactions
// and the Goon2gether reactions.
func ValidEmoji(s string) bool {
	if s == "" || len(s) > 32 || !utf8.ValidString(s) {
		return false
	}
	nonASCII := false
	for _, r := range s {
		switch {
		case r >= 0x80:
			nonASCII = true
		case r == '#' || r == '*' || (r >= '0' && r <= '9'):
			// keycap emoji
		default:
			return false
		}
		if unicode.IsSpace(r) || (unicode.IsControl(r) && r != 0x200d) {
			return false
		}
	}
	return nonASCII
}

// EmoteName returns the name of a ":name:" emote key (ok false for anything else).
func EmoteName(key string) (string, bool) {
	if len(key) <= 2 || !strings.HasPrefix(key, ":") || !strings.HasSuffix(key, ":") {
		return "", false
	}
	name := key[1 : len(key)-1]
	if len(name) > 32 {
		return "", false
	}
	for _, r := range name {
		if !(r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return "", false
		}
	}
	return name, true
}

// ReactionKeyError tells why a reaction key is refused.
type ReactionKeyError int

const (
	// ReactionKeyOK: the key is accepted.
	ReactionKeyOK ReactionKeyError = iota
	// ReactionKeyInvalid: neither an emoji nor an emote key.
	ReactionKeyInvalid
	// ReactionKeyUnknownEmote: a ":name:" key of an emote that does not exist.
	ReactionKeyUnknownEmote
)

// CheckReactionKey checks a reaction key: a Unicode emoji, or ":name:" of an existing server
// emote (emotes nil: no emote accepted).
func CheckReactionKey(key string, emotes EmoteLookup) ReactionKeyError {
	if len(key) > maxReactionKeyLength {
		return ReactionKeyInvalid
	}
	if name, ok := EmoteName(key); ok {
		if emotes == nil || !emotes.HasEmote(name) {
			return ReactionKeyUnknownEmote
		}
		return ReactionKeyOK
	}
	if !ValidEmoji(key) {
		return ReactionKeyInvalid
	}
	return ReactionKeyOK
}
