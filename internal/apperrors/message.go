package apperrors

import (
	"errors"
	"net/http"
)

// Message error codes (docs/MESSAGES.md).
const (
	CodeMessagesClosed       = "MESSAGES_CLOSED"
	CodeMessageRateLimited   = "MESSAGE_RATE_LIMITED"
	CodeConversationNotFound = "CONVERSATION_NOT_FOUND"
	CodeMessageNotFound      = "MESSAGE_NOT_FOUND"
	CodeNotMessageAuthor     = "NOT_MESSAGE_AUTHOR"
	CodeShareNotAvailable    = "SHARE_NOT_AVAILABLE"
	CodeReactionLimit        = "REACTION_LIMIT"
	CodeReactionRateLimited  = "REACTION_RATE_LIMITED"
)

// MessageError is an error of the messages, with its own code and HTTP status.
type MessageError struct {
	baseError
}

func newMessageError(code string, status int, message string) *MessageError {
	return &MessageError{baseError: baseError{message: message, code: code, httpStatus: status}}
}

// ErrMessagesClosed: the recipient's message policy refuses the sender, or the other user of
// the conversation no longer exists.
func ErrMessagesClosed(message string) *MessageError {
	if message == "" {
		message = "this user does not accept messages from you"
	}
	return newMessageError(CodeMessagesClosed, http.StatusForbidden, message)
}

// ErrMessageRateLimited: the sender sent too many messages recently.
func ErrMessageRateLimited() *MessageError {
	return newMessageError(CodeMessageRateLimited, http.StatusTooManyRequests, "you are sending messages too fast, try again in a moment")
}

// ErrConversationNotFound: no such conversation, or the user does not take part in it (the
// same answer, so that conversation ids cannot be probed).
func ErrConversationNotFound() *MessageError {
	return newMessageError(CodeConversationNotFound, http.StatusNotFound, "conversation not found")
}

// ErrMessageNotFound: no such message in the conversation.
func ErrMessageNotFound() *MessageError {
	return newMessageError(CodeMessageNotFound, http.StatusNotFound, "message not found")
}

// ErrNotMessageAuthor: editing or deleting a message of someone else.
func ErrNotMessageAuthor() *MessageError {
	return newMessageError(CodeNotMessageAuthor, http.StatusForbidden, "you can only change your own messages")
}

// ErrShareNotAvailable: the shared item does not exist or the sender may not see it.
func ErrShareNotAvailable(message string) *MessageError {
	if message == "" {
		message = "this item cannot be shared"
	}
	return newMessageError(CodeShareNotAvailable, http.StatusBadRequest, message)
}

// ErrReactionLimit: the message already has the most distinct reactions, or the user the most
// reactions on it.
func ErrReactionLimit(message string) *MessageError {
	if message == "" {
		message = "this message has too many reactions"
	}
	return newMessageError(CodeReactionLimit, http.StatusConflict, message)
}

// ErrReactionRateLimited: the user reacted too many times recently.
func ErrReactionRateLimited() *MessageError {
	return newMessageError(CodeReactionRateLimited, http.StatusTooManyRequests, "you are reacting too fast, try again in a moment")
}

// IsMessagesClosed checks if an error is ErrMessagesClosed.
func IsMessagesClosed(err error) bool {
	var m *MessageError
	return errors.As(err, &m) && m.code == CodeMessagesClosed
}
