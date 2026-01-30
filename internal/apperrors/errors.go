// Package apperrors provides typed application errors for consistent error handling.
// Use errors.Is() and errors.As() to check error types in handlers and services.
package apperrors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError is the interface for all application errors.
// It provides HTTP status code, error code for API responses, and error wrapping.
type AppError interface {
	error
	Code() string
	HTTPStatus() int
	Unwrap() error
}

// baseError implements common error functionality.
type baseError struct {
	message    string
	code       string
	httpStatus int
	cause      error
}

func (e *baseError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

func (e *baseError) Code() string {
	return e.code
}

func (e *baseError) HTTPStatus() int {
	return e.httpStatus
}

func (e *baseError) Unwrap() error {
	return e.cause
}

// NotFoundError represents a resource not found error.
type NotFoundError struct {
	baseError
	Resource string
	ID       any
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(resource string, id any) *NotFoundError {
	return &NotFoundError{
		baseError: baseError{
			message:    fmt.Sprintf("%s not found", resource),
			code:       "NOT_FOUND",
			httpStatus: http.StatusNotFound,
		},
		Resource: resource,
		ID:       id,
	}
}

// NewNotFoundErrorWithCause creates a NotFoundError wrapping another error.
func NewNotFoundErrorWithCause(resource string, id any, cause error) *NotFoundError {
	e := NewNotFoundError(resource, id)
	e.cause = cause
	return e
}

// IsNotFound checks if an error is a NotFoundError.
func IsNotFound(err error) bool {
	var notFound *NotFoundError
	return errors.As(err, &notFound)
}

// ValidationError represents a validation failure.
type ValidationError struct {
	baseError
	Field   string
	Details map[string]string
}

// NewValidationError creates a new ValidationError with a message.
func NewValidationError(message string) *ValidationError {
	return &ValidationError{
		baseError: baseError{
			message:    message,
			code:       "VALIDATION_ERROR",
			httpStatus: http.StatusBadRequest,
		},
	}
}

// NewValidationErrorWithField creates a ValidationError for a specific field.
func NewValidationErrorWithField(field, message string) *ValidationError {
	return &ValidationError{
		baseError: baseError{
			message:    message,
			code:       "VALIDATION_ERROR",
			httpStatus: http.StatusBadRequest,
		},
		Field: field,
	}
}

// NewValidationErrorWithDetails creates a ValidationError with multiple field errors.
func NewValidationErrorWithDetails(message string, details map[string]string) *ValidationError {
	return &ValidationError{
		baseError: baseError{
			message:    message,
			code:       "VALIDATION_ERROR",
			httpStatus: http.StatusBadRequest,
		},
		Details: details,
	}
}

// IsValidation checks if an error is a ValidationError.
func IsValidation(err error) bool {
	var validation *ValidationError
	return errors.As(err, &validation)
}

// ConflictError represents a resource conflict (e.g., duplicate).
type ConflictError struct {
	baseError
	Resource string
}

// NewConflictError creates a new ConflictError.
func NewConflictError(resource, message string) *ConflictError {
	return &ConflictError{
		baseError: baseError{
			message:    message,
			code:       "CONFLICT",
			httpStatus: http.StatusConflict,
		},
		Resource: resource,
	}
}

// IsConflict checks if an error is a ConflictError.
func IsConflict(err error) bool {
	var conflict *ConflictError
	return errors.As(err, &conflict)
}

// InternalError represents an internal server error.
type InternalError struct {
	baseError
}

// NewInternalError creates a new InternalError.
func NewInternalError(message string, cause error) *InternalError {
	return &InternalError{
		baseError: baseError{
			message:    message,
			code:       "INTERNAL_ERROR",
			httpStatus: http.StatusInternalServerError,
			cause:      cause,
		},
	}
}

// IsInternal checks if an error is an InternalError.
func IsInternal(err error) bool {
	var internal *InternalError
	return errors.As(err, &internal)
}

// ForbiddenError represents an authorization failure.
type ForbiddenError struct {
	baseError
}

// NewForbiddenError creates a new ForbiddenError.
func NewForbiddenError(message string) *ForbiddenError {
	return &ForbiddenError{
		baseError: baseError{
			message:    message,
			code:       "FORBIDDEN",
			httpStatus: http.StatusForbidden,
		},
	}
}

// IsForbidden checks if an error is a ForbiddenError.
func IsForbidden(err error) bool {
	var forbidden *ForbiddenError
	return errors.As(err, &forbidden)
}

// UnauthorizedError represents an authentication failure.
type UnauthorizedError struct {
	baseError
}

// NewUnauthorizedError creates a new UnauthorizedError.
func NewUnauthorizedError(message string) *UnauthorizedError {
	return &UnauthorizedError{
		baseError: baseError{
			message:    message,
			code:       "UNAUTHORIZED",
			httpStatus: http.StatusUnauthorized,
		},
	}
}

// IsUnauthorized checks if an error is an UnauthorizedError.
func IsUnauthorized(err error) bool {
	var unauthorized *UnauthorizedError
	return errors.As(err, &unauthorized)
}

// GetHTTPStatus returns the HTTP status code for an error.
// Returns 500 for non-AppError errors.
func GetHTTPStatus(err error) int {
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr.HTTPStatus()
	}
	return http.StatusInternalServerError
}

// GetCode returns the error code for an error.
// Returns "INTERNAL_ERROR" for non-AppError errors.
func GetCode(err error) string {
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr.Code()
	}
	return "INTERNAL_ERROR"
}
