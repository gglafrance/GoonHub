package request

import "goonhub/internal/core"

// SendMessageRequest is the body of POST /messages/conversations/:id/messages and
// POST /messages/direct/:userId/messages: a body, an attachment, or both.
type SendMessageRequest struct {
	Body       string                `json:"body"`
	Attachment *core.AttachmentInput `json:"attachment"`
}

// EditMessageRequest is the body of PATCH /messages/conversations/:id/messages/:messageId.
type EditMessageRequest struct {
	Body string `json:"body"`
}

// ShareRequest is the body of POST /messages/share.
type ShareRequest struct {
	UserIDs []uint                 `json:"user_ids" binding:"required"`
	Body    string                 `json:"body"`
	Items   []core.AttachmentInput `json:"items" binding:"required"`
}

// MarkReadRequest is the body of POST /messages/conversations/:id/read.
type MarkReadRequest struct {
	MessageID uint `json:"message_id" binding:"required"`
}

// MuteConversationRequest is the body of PUT /messages/conversations/:id/mute.
type MuteConversationRequest struct {
	Muted bool `json:"muted"`
}

// UpdateMessageSettingsRequest is the body of PUT /messages/settings (absent = unchanged).
type UpdateMessageSettingsRequest struct {
	Policy *string `json:"policy"`
	Toasts *bool   `json:"toasts"`
}
