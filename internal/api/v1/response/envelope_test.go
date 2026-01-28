package response

import (
	"encoding/json"
	"goonhub/internal/apperrors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewPagination(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		totalItems    int64
		expectedPages int
	}{
		{"exact pages", 1, 10, 100, 10},
		{"partial page", 1, 10, 95, 10},
		{"single page", 1, 10, 5, 1},
		{"zero items", 1, 10, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.limit, tt.totalItems)
			if p.Page != tt.page {
				t.Fatalf("expected page %d, got %d", tt.page, p.Page)
			}
			if p.Limit != tt.limit {
				t.Fatalf("expected limit %d, got %d", tt.limit, p.Limit)
			}
			if p.TotalItems != tt.totalItems {
				t.Fatalf("expected totalItems %d, got %d", tt.totalItems, p.TotalItems)
			}
			if p.TotalPages != tt.expectedPages {
				t.Fatalf("expected totalPages %d, got %d", tt.expectedPages, p.TotalPages)
			}
		})
	}
}

func TestNewPaginatedResponse(t *testing.T) {
	data := []string{"a", "b", "c"}
	resp := NewPaginatedResponse(data, 2, 10, 25)

	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Data))
	}
	if resp.Pagination.Page != 2 {
		t.Fatalf("expected page 2, got %d", resp.Pagination.Page)
	}
	if resp.Pagination.TotalPages != 3 {
		t.Fatalf("expected 3 total pages, got %d", resp.Pagination.TotalPages)
	}
}

func TestNewPaginatedResponse_NilData(t *testing.T) {
	resp := NewPaginatedResponse[string](nil, 1, 10, 0)

	if resp.Data == nil {
		t.Fatal("expected non-nil data slice")
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(resp.Data))
	}
}

func TestNewDataResponse(t *testing.T) {
	data := map[string]string{"foo": "bar"}
	resp := NewDataResponse(data)

	if resp.Data["foo"] != "bar" {
		t.Fatalf("expected foo=bar, got %v", resp.Data)
	}
}

func TestErrorResponse(t *testing.T) {
	resp := NewErrorResponse("something went wrong")
	if resp.Error != "something went wrong" {
		t.Fatalf("expected error message, got %q", resp.Error)
	}

	resp = NewErrorResponseWithCode("not found", "NOT_FOUND")
	if resp.Code != "NOT_FOUND" {
		t.Fatalf("expected code NOT_FOUND, got %q", resp.Code)
	}

	details := map[string]string{"field": "invalid"}
	resp = NewErrorResponseWithDetails("validation failed", details)
	if resp.Details["field"] != "invalid" {
		t.Fatalf("expected details, got %v", resp.Details)
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "not found error",
			err:            apperrors.NewNotFoundError("video", 1),
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name:           "validation error",
			err:            apperrors.NewValidationError("invalid input"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "VALIDATION_ERROR",
		},
		{
			name:           "conflict error",
			err:            apperrors.NewConflictError("user", "already exists"),
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONFLICT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			Error(c, tt.err)

			if w.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if resp.Code != tt.expectedCode {
				t.Fatalf("expected code %q, got %q", tt.expectedCode, resp.Code)
			}
		})
	}
}

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	OK(c, gin.H{"message": "success"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Created(c, gin.H{"id": 1})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	NoContent(c)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	BadRequest(c, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	NotFound(c, "resource not found")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	InternalError(c, "internal error")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestConflict(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Conflict(c, "resource already exists")

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", w.Code)
	}
}
