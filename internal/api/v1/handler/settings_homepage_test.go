package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"goonhub/internal/api/v1/request"

	"github.com/gin-gonic/gin"
)

func TestUpdateHomepageConfigRequest_KeepsTabsAndWidgets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := map[string]any{
		"show_upload":    false,
		"sections":       []any{},
		"default_tab_id": "stats",
		"open_on":        "default",
		"tabs": []any{
			map[string]any{"id": "home", "name": "Home", "icon": "heroicons:home", "sections": []any{
				map[string]any{"id": "s1", "type": "latest", "title": "Latest", "enabled": true, "limit": 12, "order": 0},
			}},
			map[string]any{"id": "stats", "name": "Stats", "icon": "heroicons:chart-bar", "sections": []any{
				map[string]any{"id": "s2", "type": "activity_calendar", "title": "Activity", "enabled": true, "limit": 1, "order": 0,
					"config": map[string]any{"weeks": 52}},
			}},
		},
	}
	raw, _ := json.Marshal(body)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("PUT", "/", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")

	var req request.UpdateHomepageConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		t.Fatalf("widget sections and tabs must bind: %v", err)
	}
	cfg := (&SettingsHandler{}).convertRequestToConfig(req)
	if len(cfg.Tabs) != 2 || cfg.Tabs[1].Sections[0].Type != "activity_calendar" || cfg.DefaultTabID != "stats" || cfg.OpenOn != "default" {
		t.Fatalf("tabs not converted: %+v", cfg)
	}
	if cfg.Tabs[1].Sections[0].Config["weeks"] != float64(52) {
		t.Fatalf("section config lost: %+v", cfg.Tabs[1].Sections[0].Config)
	}
}
