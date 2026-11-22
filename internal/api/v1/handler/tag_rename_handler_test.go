package handler

import (
	"net/http"
	"strings"
	"testing"

	"goonhub/internal/data"

	"gorm.io/gorm"
)

func TestRenameTagEndpoint(t *testing.T) {
	r, tagRepo := newTagMergeTestRouter(t)

	for _, body := range []string{`{}`, `{"name":""}`, `{"name":"   "}`, `{"name":"` + strings.Repeat("a", 101) + `"}`, `{"name":5}`} {
		if w, _ := doAliasRequest(t, r, http.MethodPut, "/tags/1", body); w.Code != http.StatusBadRequest {
			t.Fatalf("body %.40s: status %d, want 400", body, w.Code)
		}
	}

	tagRepo.EXPECT().GetByID(uint(9)).Return(nil, gorm.ErrRecordNotFound)
	if w, _ := doAliasRequest(t, r, http.MethodPut, "/tags/9", `{"name":"Anal"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown tag: status %d, want 404", w.Code)
	}

	// another tag has the name ignoring case: 409 with that tag to merge into
	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "cumshot", Color: "#6B7280"}, nil)
	tagRepo.EXPECT().List().Return([]data.Tag{{ID: 1, Name: "cumshot"}, {ID: 2, Name: "Anal"}}, nil)
	w, body := doAliasRequest(t, r, http.MethodPut, "/tags/1", `{"name":"ANAL"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("conflict: status %d: %s", w.Code, w.Body.String())
	}
	existing, _ := body["existing"].(map[string]any)
	if existing["id"] != float64(2) || body["field"] != "name" || body["value"] != "ANAL" ||
		!strings.Contains(body["error"].(string), "merge into it") {
		t.Fatalf("conflict body = %v", body)
	}

	// a different case of its own name is allowed
	tagRepo.EXPECT().GetByID(uint(1)).Return(&data.Tag{ID: 1, Name: "cumshot", Color: "#6B7280"}, nil)
	tagRepo.EXPECT().List().Return([]data.Tag{{ID: 1, Name: "cumshot"}, {ID: 2, Name: "Anal"}}, nil)
	tagRepo.EXPECT().RenameTag(data.Tag{ID: 1, Name: "Cumshot", Color: "#6B7280"}, "cumshot").Return(int64(0), nil, nil)
	w, body = doAliasRequest(t, r, http.MethodPut, "/tags/1", `{"name":"Cumshot"}`)
	if w.Code != http.StatusOK || body["name"] != "Cumshot" || body["color"] != "#6B7280" {
		t.Fatalf("own-case rename: status %d body %v", w.Code, body)
	}
}
