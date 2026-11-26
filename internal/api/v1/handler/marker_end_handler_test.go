package handler

import (
	"net/http"
	"testing"

	"goonhub/internal/api/middleware"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

// newMarkerUpdateTestRouter mounts PUT /scenes/:id/markers/:markerID as user 1, with marker 3
// (start 10, end 25) of user 1 on scene 2.
func newMarkerUpdateTestRouter(t *testing.T) (*gin.Engine, *mocks.MockMarkerRepository, *data.UserSceneMarker) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	markers := mocks.NewMockMarkerRepository(ctrl)
	scenes := mocks.NewMockSceneRepository(ctrl)
	scenes.EXPECT().GetByID(uint(2)).Return(&data.Scene{ID: 2, Duration: 100}, nil).AnyTimes()
	svc := core.NewMarkerService(markers, mocks.NewMockMarkerLabelRepository(ctrl), scenes,
		mocks.NewMockTagRepository(ctrl), &config.Config{}, zap.NewNop())
	h := NewMarkerHandler(svc, 100)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &core.UserPayload{UserID: 1, Username: "u"})
		middleware.SetAccess(c, core.NewUserAccess(1, "user", false, core.PermMarkersCreate))
		c.Next()
	})
	r.PUT("/scenes/:id/markers/:markerID", h.UpdateMarker)

	end := 25
	marker := &data.UserSceneMarker{ID: 3, UserID: 1, SceneID: 2, Timestamp: 10, EndTimestamp: &end, Color: "#FFFFFF"}
	markers.EXPECT().GetByID(uint(3)).Return(marker, nil)
	return r, markers, marker
}

func TestUpdateMarker_EndTimestampBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantEnd any // float64 in the JSON response, nil when cleared
	}{
		{"clear_end_timestamp removes the end", `{"clear_end_timestamp":true}`, nil},
		{"clear wins over a new end", `{"clear_end_timestamp":true,"end_timestamp":40}`, nil},
		{"field absent keeps the end", `{"color":"#00FF00"}`, float64(25)},
		{"null end keeps the end (absent)", `{"end_timestamp":null}`, float64(25)},
		{"clear false keeps the end", `{"clear_end_timestamp":false}`, float64(25)},
		{"new end", `{"end_timestamp":40}`, float64(40)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, markers, _ := newMarkerUpdateTestRouter(t)
			markers.EXPECT().Update(gomock.Any()).Return(nil)

			w, body := doAliasRequest(t, r, http.MethodPut, "/scenes/2/markers/3", tt.body)
			if w.Code != http.StatusOK {
				t.Fatalf("status %d body %v", w.Code, body)
			}
			got, present := body["end_timestamp"]
			if !present {
				t.Fatalf("no end_timestamp in %v", body)
			}
			if got != tt.wantEnd {
				t.Fatalf("end_timestamp = %v, want %v", got, tt.wantEnd)
			}
		})
	}
}
