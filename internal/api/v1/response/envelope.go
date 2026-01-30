// Package response provides standardized response types for the API.
package response

import (
	"goonhub/internal/apperrors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Pagination contains pagination metadata for list responses.
type Pagination struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

// NewPagination creates a new Pagination struct with calculated total pages.
func NewPagination(page, limit int, totalItems int64) Pagination {
	totalPages := int(totalItems) / limit
	if int(totalItems)%limit != 0 {
		totalPages++
	}
	return Pagination{
		Page:       page,
		Limit:      limit,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}

// PaginatedResponse is a generic paginated response envelope.
type PaginatedResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// NewPaginatedResponse creates a new paginated response.
func NewPaginatedResponse[T any](data []T, page, limit int, totalItems int64) PaginatedResponse[T] {
	if data == nil {
		data = []T{}
	}
	return PaginatedResponse[T]{
		Data:       data,
		Pagination: NewPagination(page, limit, totalItems),
	}
}

// DataResponse is a simple data envelope for non-paginated responses.
type DataResponse[T any] struct {
	Data T `json:"data"`
}

// NewDataResponse creates a new data response.
func NewDataResponse[T any](data T) DataResponse[T] {
	return DataResponse[T]{Data: data}
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// NewErrorResponse creates a new error response from a message.
func NewErrorResponse(message string) ErrorResponse {
	return ErrorResponse{Error: message}
}

// NewErrorResponseWithCode creates an error response with an error code.
func NewErrorResponseWithCode(message, code string) ErrorResponse {
	return ErrorResponse{Error: message, Code: code}
}

// NewErrorResponseWithDetails creates an error response with validation details.
func NewErrorResponseWithDetails(message string, details map[string]string) ErrorResponse {
	return ErrorResponse{Error: message, Details: details}
}

// Error sends an error response based on the error type.
// It uses the apperrors package to determine the appropriate HTTP status and error code.
func Error(c *gin.Context, err error) {
	status := apperrors.GetHTTPStatus(err)
	code := apperrors.GetCode(err)

	resp := ErrorResponse{
		Error: err.Error(),
		Code:  code,
	}

	// Add validation details if available
	var validationErr *apperrors.ValidationError
	if apperrors.IsValidation(err) {
		if vErr, ok := err.(*apperrors.ValidationError); ok {
			validationErr = vErr
			if validationErr.Details != nil {
				resp.Details = validationErr.Details
			}
		}
	}

	c.JSON(status, resp)
}

// OK sends a 200 OK response with the given data.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// Created sends a 201 Created response with the given data.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}

// NoContent sends a 204 No Content response.
func NoContent(c *gin.Context) {
	c.JSON(http.StatusNoContent, nil)
}

// BadRequest sends a 400 Bad Request response with the given message.
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, NewErrorResponse(message))
}

// NotFound sends a 404 Not Found response with the given message.
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, NewErrorResponse(message))
}

// InternalError sends a 500 Internal Server Error response with the given message.
func InternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, NewErrorResponse(message))
}

// Conflict sends a 409 Conflict response with the given message.
func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, NewErrorResponse(message))
}
