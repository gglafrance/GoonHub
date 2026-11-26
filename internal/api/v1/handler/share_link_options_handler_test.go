package handler

import (
	"net/http"
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// The start time of a share link goes through the API, is validated, and comes back with the
// link (docs/MESSAGES.md, "Share links").
func TestCreateShareLinkWithStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	links := mocks.NewMockShareLinkRepository(ctrl)
	scenes := mocks.NewMockSceneRepository(ctrl)
	h := &ShareHandler{ShareService: core.NewShareService(links, scenes, zap.NewNop())}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Username: "alice"})
		c.Next()
	})
	r.POST("/scenes/:id/shares", h.CreateShareLink)

	scenes.EXPECT().GetByID(uint(5)).Return(&data.Scene{ID: 5, Duration: 100}, nil).Times(3)
	links.EXPECT().Create(gomock.Any()).Return(nil)

	w, body := doAliasRequest(t, r, http.MethodPost, "/scenes/5/shares", `{"share_type":"public","expires_in":"7d","start":42}`)
	if w.Code != http.StatusCreated || body["start"] != float64(42) || body["end"] != nil {
		t.Fatalf("create with start: %d %v", w.Code, body)
	}
	if _, has := body["marker_id"]; has {
		t.Fatalf("the marker reference is sent: %v", body)
	}
	if w, _ := doAliasRequest(t, r, http.MethodPost, "/scenes/5/shares", `{"share_type":"public","expires_in":"7d","start":100}`); w.Code != http.StatusBadRequest {
		t.Fatalf("start past the end: %d", w.Code)
	}
	// no marker source wired: marker links are refused
	if w, _ := doAliasRequest(t, r, http.MethodPost, "/scenes/5/shares", `{"share_type":"public","expires_in":"7d","marker_id":3}`); w.Code != http.StatusBadRequest {
		t.Fatalf("marker link without markers: %d", w.Code)
	}
}
