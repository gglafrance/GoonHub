package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMarkerListOptionsQueryAndLiveOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/markers/all?q=+Kiss+&live_only=true&labels=a,,b&sort=recent", nil)

	opts := markerListOptions(c, "label_asc")
	if opts.Query != "Kiss" {
		t.Fatalf("query: got %q", opts.Query)
	}
	if !opts.LiveScenesOnly {
		t.Fatal("live_only=true not read")
	}
	if opts.SortBy != "recent" || len(opts.Labels) != 2 {
		t.Fatalf("unexpected options: %+v", opts)
	}

	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/markers/all", nil)
	opts = markerListOptions(c, "label_asc")
	if opts.Query != "" || opts.LiveScenesOnly || opts.SortBy != "label_asc" {
		t.Fatalf("defaults: %+v", opts)
	}
}
