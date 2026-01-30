package apperrors

// Error codes for API responses.
// These codes provide machine-readable error identification.
const (
	// General errors
	CodeNotFound       = "NOT_FOUND"
	CodeValidation     = "VALIDATION_ERROR"
	CodeConflict       = "CONFLICT"
	CodeInternal       = "INTERNAL_ERROR"
	CodeForbidden      = "FORBIDDEN"
	CodeUnauthorized   = "UNAUTHORIZED"
	CodeTooManyRequests = "TOO_MANY_REQUESTS"

	// Auth errors
	CodeInvalidCredentials = "INVALID_CREDENTIALS"
	CodeTokenExpired       = "TOKEN_EXPIRED"
	CodeTokenInvalid       = "TOKEN_INVALID"
	CodeTokenRevoked       = "TOKEN_REVOKED"
	CodeAccountLocked      = "ACCOUNT_LOCKED"

	// Video errors
	CodeInvalidFileExtension        = "INVALID_FILE_EXTENSION"
	CodeInvalidImageExtension       = "INVALID_IMAGE_EXTENSION"
	CodeVideoDimensionsNotAvailable = "VIDEO_DIMENSIONS_NOT_AVAILABLE"
	CodeVideoFileNotFound           = "VIDEO_FILE_NOT_FOUND"

	// Processing errors
	CodeProcessingFailed = "PROCESSING_FAILED"
	CodeProcessingQueued = "PROCESSING_QUEUED"
)
