package apperrors

import (
	"errors"
	"fmt"
	"strings"
)

// EntityConflictError is a ConflictError that carries the existing entity a name or
// an alias collides with, so that clients can reuse it instead of creating a duplicate.
type EntityConflictError struct {
	*ConflictError
	// Field is the request field that collides: "name" or "aliases".
	Field string
	// Value is the colliding name or alias.
	Value string
	// Existing is the entity that already uses Value as its name or one of its aliases.
	Existing any
}

// NewEntityConflictError creates an EntityConflictError. existingName is the name of the
// entity that already uses value.
func NewEntityConflictError(resource, field, value, existingName string, existing any) *EntityConflictError {
	msg := fmt.Sprintf("%s '%s' already exists", resource, existingName)
	if field == "aliases" || !strings.EqualFold(existingName, value) {
		msg = fmt.Sprintf("'%s' is already used by %s '%s'", value, resource, existingName)
	}
	return &EntityConflictError{
		ConflictError: NewConflictError(resource, msg),
		Field:         field,
		Value:         value,
		Existing:      existing,
	}
}

// AsEntityConflict returns the EntityConflictError in err's chain, if any.
func AsEntityConflict(err error) (*EntityConflictError, bool) {
	var conflict *EntityConflictError
	if errors.As(err, &conflict) {
		return conflict, true
	}
	return nil, false
}

// Unwrap exposes the ConflictError, so that IsConflict matches.
func (e *EntityConflictError) Unwrap() error {
	return e.ConflictError
}
