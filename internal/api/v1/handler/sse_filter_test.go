package handler

import (
	"testing"

	"goonhub/internal/core"
	"goonhub/internal/data"
)

func TestSSEEventForUser(t *testing.T) {
	viewer := core.NewUserAccess(1, "user", false, core.PermScenesView)
	tech := core.NewUserAccess(2, "tech", false, core.PermFilesViewTech)
	techPaths := core.NewUserAccess(3, "techpaths", false, core.PermFilesViewTech, core.PermFilesViewPaths)
	jobs := core.NewUserAccess(4, "jobs", false, core.PermJobsViewStatus)
	storage := core.NewUserAccess(5, "storage", false, core.PermStorageManage)
	admin := core.NewUserAccess(6, "admin", true)

	scanProgress := core.SceneEvent{Type: "scan:progress", Data: map[string]any{"current_path": "/media/a", "current_file": "/media/a/x.mp4"}}
	scanStarted := core.SceneEvent{Type: "scan:started", Data: &data.ScanHistory{Status: "running"}}
	failed := core.SceneEvent{Type: "scene:failed", Data: map[string]any{"error": "ffmpeg: /media/x.mkv broken", "phase": "metadata"}}
	fingerprint := core.SceneEvent{Type: "scene:fingerprint_complete", Data: map[string]any{"fingerprint_type": "audio"}}
	metadata := core.SceneEvent{Type: "scene:metadata_complete", Data: map[string]any{"duration": 60, "width": 1920, "height": 1080, "stored_path": "/media/new-field.mp4"}}
	unknown := core.SceneEvent{Type: "scene:something_new", Data: map[string]any{"path": "/media/a.mp4"}}

	tests := []struct {
		name       string
		event      core.SceneEvent
		access     *core.UserAccess
		send       bool
		wantKeys   []string
		absentKeys []string
	}{
		{"job status needs jobs:view_status", core.SceneEvent{Type: "jobs:status"}, viewer, false, nil, nil},
		{"job status with jobs:view_status", core.SceneEvent{Type: "jobs:status"}, jobs, true, nil, nil},
		{"bulk cancel needs jobs:view_status", core.SceneEvent{Type: "jobs:bulk_cancelled"}, viewer, false, nil, nil},
		{"dlq needs jobs:view_status", core.SceneEvent{Type: "scene:dlq_added", Data: map[string]any{"job_id": "j"}}, viewer, false, nil, nil},
		{"dlq with jobs:view_status", core.SceneEvent{Type: "scene:dlq_added", Data: map[string]any{"job_id": "j"}}, jobs, true, []string{"job_id"}, nil},
		{"retry needs jobs:view_status", core.SceneEvent{Type: "scene:retry_scheduled"}, viewer, false, nil, nil},
		{"scan progress needs storage:manage", scanProgress, techPaths, false, nil, nil},
		{"scan progress with storage:manage", scanProgress, storage, true, []string{"current_path", "current_file"}, nil},
		{"scan history needs storage:manage", scanStarted, viewer, false, nil, nil},
		{"scene events stay", core.SceneEvent{Type: "scene:completed"}, viewer, true, nil, nil},
		{"failed: no error without technical", failed, viewer, true, []string{"phase"}, []string{"error"}},
		{"failed: no error without paths", failed, tech, true, []string{"phase"}, []string{"error"}},
		{"failed: error with technical and paths", failed, techPaths, true, []string{"phase", "error"}, nil},
		{"fingerprint type needs technical", fingerprint, viewer, true, nil, []string{"fingerprint_type"}},
		{"fingerprint type with technical", fingerprint, tech, true, []string{"fingerprint_type"}, nil},
		{"unlisted fields never leak", metadata, techPaths, true, []string{"duration", "width", "height"}, []string{"stored_path"}},
		{"unknown event without data", unknown, techPaths, true, nil, []string{"path"}},
		{"superuser gets everything", metadata, admin, true, []string{"stored_path"}, nil},
		{"no access, nothing", core.SceneEvent{Type: "scene:completed"}, nil, false, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ev, send := eventForUser(tc.event, tc.access)
			if send != tc.send {
				t.Fatalf("send = %v, want %v", send, tc.send)
			}
			if !send {
				return
			}
			got, _ := ev.Data.(map[string]any)
			for _, k := range tc.wantKeys {
				if _, ok := got[k]; !ok {
					t.Fatalf("key %q missing in %v", k, ev.Data)
				}
			}
			for _, k := range tc.absentKeys {
				if _, ok := got[k]; ok {
					t.Fatalf("key %q must be removed: %v", k, ev.Data)
				}
			}
		})
	}

	// the shared event is never modified
	if _, ok := failed.Data.(map[string]any)["error"]; !ok {
		t.Fatal("the published event was modified")
	}
	// scan events keep their non-map data for storage managers
	if ev, send := eventForUser(scanStarted, storage); !send || ev.Data == nil {
		t.Fatal("scan history must reach storage managers")
	}
	// an unknown type keeps no data
	if ev, _ := eventForUser(unknown, viewer); ev.Data != nil {
		t.Fatalf("unknown event data must be dropped: %v", ev.Data)
	}
}
