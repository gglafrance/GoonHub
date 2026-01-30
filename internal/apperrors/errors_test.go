package apperrors

import (
	"errors"
	"net/http"
	"testing"
)

func TestNotFoundError(t *testing.T) {
	err := NewNotFoundError("video", uint(123))

	if !IsNotFound(err) {
		t.Fatal("expected IsNotFound to return true")
	}

	if err.HTTPStatus() != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, err.HTTPStatus())
	}

	if err.Code() != "NOT_FOUND" {
		t.Fatalf("expected code NOT_FOUND, got %s", err.Code())
	}

	if err.Resource != "video" {
		t.Fatalf("expected resource 'video', got %s", err.Resource)
	}

	if err.ID != uint(123) {
		t.Fatalf("expected ID 123, got %v", err.ID)
	}

	expectedMsg := "video not found"
	if err.Error() != expectedMsg {
		t.Fatalf("expected message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNotFoundErrorWithCause(t *testing.T) {
	cause := errors.New("database error")
	err := NewNotFoundErrorWithCause("video", uint(123), cause)

	if !IsNotFound(err) {
		t.Fatal("expected IsNotFound to return true")
	}

	if err.Unwrap() != cause {
		t.Fatal("expected Unwrap to return cause")
	}

	if !errors.Is(err, cause) {
		t.Fatal("expected errors.Is to match cause")
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("invalid input")

	if !IsValidation(err) {
		t.Fatal("expected IsValidation to return true")
	}

	if err.HTTPStatus() != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.HTTPStatus())
	}

	if err.Code() != "VALIDATION_ERROR" {
		t.Fatalf("expected code VALIDATION_ERROR, got %s", err.Code())
	}
}

func TestValidationErrorWithField(t *testing.T) {
	err := NewValidationErrorWithField("email", "invalid email format")

	if !IsValidation(err) {
		t.Fatal("expected IsValidation to return true")
	}

	if err.Field != "email" {
		t.Fatalf("expected field 'email', got %s", err.Field)
	}
}

func TestValidationErrorWithDetails(t *testing.T) {
	details := map[string]string{
		"email":    "invalid format",
		"password": "too short",
	}
	err := NewValidationErrorWithDetails("validation failed", details)

	if !IsValidation(err) {
		t.Fatal("expected IsValidation to return true")
	}

	if len(err.Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(err.Details))
	}
}

func TestConflictError(t *testing.T) {
	err := NewConflictError("user", "username already exists")

	if !IsConflict(err) {
		t.Fatal("expected IsConflict to return true")
	}

	if err.HTTPStatus() != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, err.HTTPStatus())
	}

	if err.Resource != "user" {
		t.Fatalf("expected resource 'user', got %s", err.Resource)
	}
}

func TestInternalError(t *testing.T) {
	cause := errors.New("database connection failed")
	err := NewInternalError("operation failed", cause)

	if !IsInternal(err) {
		t.Fatal("expected IsInternal to return true")
	}

	if err.HTTPStatus() != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, err.HTTPStatus())
	}

	if err.Unwrap() != cause {
		t.Fatal("expected Unwrap to return cause")
	}
}

func TestForbiddenError(t *testing.T) {
	err := NewForbiddenError("access denied")

	if !IsForbidden(err) {
		t.Fatal("expected IsForbidden to return true")
	}

	if err.HTTPStatus() != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, err.HTTPStatus())
	}
}

func TestUnauthorizedError(t *testing.T) {
	err := NewUnauthorizedError("invalid token")

	if !IsUnauthorized(err) {
		t.Fatal("expected IsUnauthorized to return true")
	}

	if err.HTTPStatus() != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, err.HTTPStatus())
	}
}

func TestGetHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"NotFound", NewNotFoundError("video", 1), http.StatusNotFound},
		{"Validation", NewValidationError("invalid"), http.StatusBadRequest},
		{"Conflict", NewConflictError("user", "exists"), http.StatusConflict},
		{"Internal", NewInternalError("failed", nil), http.StatusInternalServerError},
		{"Forbidden", NewForbiddenError("denied"), http.StatusForbidden},
		{"Unauthorized", NewUnauthorizedError("invalid"), http.StatusUnauthorized},
		{"StandardError", errors.New("generic error"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := GetHTTPStatus(tt.err)
			if status != tt.expected {
				t.Fatalf("expected status %d, got %d", tt.expected, status)
			}
		})
	}
}

func TestGetCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"NotFound", NewNotFoundError("video", 1), "NOT_FOUND"},
		{"Validation", NewValidationError("invalid"), "VALIDATION_ERROR"},
		{"Conflict", NewConflictError("user", "exists"), "CONFLICT"},
		{"Internal", NewInternalError("failed", nil), "INTERNAL_ERROR"},
		{"Forbidden", NewForbiddenError("denied"), "FORBIDDEN"},
		{"Unauthorized", NewUnauthorizedError("invalid"), "UNAUTHORIZED"},
		{"StandardError", errors.New("generic error"), "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := GetCode(tt.err)
			if code != tt.expected {
				t.Fatalf("expected code %s, got %s", tt.expected, code)
			}
		})
	}
}

func TestIsNotFoundWithNonNotFoundError(t *testing.T) {
	err := NewValidationError("invalid")
	if IsNotFound(err) {
		t.Fatal("expected IsNotFound to return false for ValidationError")
	}
}

func TestIsValidationWithNonValidationError(t *testing.T) {
	err := NewNotFoundError("video", 1)
	if IsValidation(err) {
		t.Fatal("expected IsValidation to return false for NotFoundError")
	}
}

func TestErrorsAsWithAppError(t *testing.T) {
	err := NewNotFoundError("video", uint(123))

	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatal("expected errors.As to match NotFoundError")
	}

	var appErr AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected errors.As to match AppError interface")
	}
}
