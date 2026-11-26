package handler

import (
	"strings"

	"goonhub/internal/core"
)

// sseEventRule says who receives an event type and which data fields they see. Fields not
// listed are never sent (allowlist): a field added later to an event stays private until
// it is listed here.
type sseEventRule struct {
	// require is the permission needed to receive the event at all ("" = every user)
	require string
	// passAll sends the data as is (the required permission already covers all of it)
	passAll bool
	// fields maps an allowed data key to the permissions needed to see it (none = everyone)
	fields map[string][]string
	// targeted events are personal: only sent when published to one user (EventBus.
	// PublishToUser), never as a broadcast
	targeted bool
}

var (
	sseTechnical      = []string{core.PermFilesViewTech}
	sseTechnicalPaths = []string{core.PermFilesViewTech, core.PermFilesViewPaths}
)

// sseEventRules lists every event type the server publishes (core.EventBus).
var sseEventRules = map[string]sseEventRule{
	// Processing of a scene
	"scene:metadata_complete":            {fields: map[string][]string{"duration": nil, "width": nil, "height": nil}},
	"scene:thumbnail_complete":           {fields: map[string][]string{"thumbnail_path": nil}},
	"scene:sprites_complete":             {fields: map[string][]string{"vtt_path": nil, "sprite_sheet_path": nil}},
	"scene:animated_thumbnails_complete": {},
	"scene:fingerprint_complete":         {fields: map[string][]string{"fingerprint_type": sseTechnical}},
	"scene:completed":                    {},
	"scene:failed":                       {fields: map[string][]string{"phase": nil, "error": sseTechnicalPaths}},
	"scene:cancelled":                    {fields: map[string][]string{"phase": nil}},
	"scene:timed_out":                    {fields: map[string][]string{"phase": nil}},
	// Library changes
	"scene:trashed":       {fields: map[string][]string{"title": nil, "trashed_at": nil}},
	"scene:restored":      {fields: map[string][]string{"title": nil}},
	"scene:deleted":       {fields: map[string][]string{"title": nil}},
	"scenes_bulk_updated": {},
	"scenes_bulk_trashed": {},
	"scenes_bulk_deleted": {},
	// Job queue
	"scene:retry_scheduled": {require: core.PermJobsViewStatus, fields: map[string][]string{"job_id": nil, "phase": nil, "retry_count": nil, "max_retries": nil, "next_retry_at": nil}},
	"scene:dlq_added":       {require: core.PermJobsViewStatus, fields: map[string][]string{"job_id": nil, "phase": nil, "failure_count": nil}},
	"scene:dlq_retry":       {require: core.PermJobsViewStatus, fields: map[string][]string{"job_id": nil, "phase": nil}},
	"scene:dlq_abandoned":   {require: core.PermJobsViewStatus, fields: map[string][]string{"job_id": nil, "phase": nil}},
	"jobs:bulk_cancelled":   {require: core.PermJobsViewStatus, fields: map[string][]string{"phases": nil, "pending_cancelled": nil, "queued_skipped": nil, "running_cancelled": nil}},
	// Goon2gether: number of public lobbies (nav badge), no personal data
	"together:public_count": {require: core.PermTogetherUse, fields: map[string][]string{"count": nil}},
}

// sseTargetedPrefixes are the event families that are always personal (targeted at one
// user): a broadcast of such an event is dropped. Presence and other per-user features add
// their prefix here.
var sseTargetedPrefixes = []string{"social:", "presence:"}

// sseRuleFor returns the rule of an event type. jobs:* need jobs:view_status, scan:* (absolute
// paths of the library files) need storage:manage, with their data as is; social:* and
// presence:* are targeted. An unknown type is sent without its data.
func sseRuleFor(eventType string) sseEventRule {
	if rule, ok := sseEventRules[eventType]; ok {
		return rule
	}
	for _, prefix := range sseTargetedPrefixes {
		if strings.HasPrefix(eventType, prefix) {
			return sseEventRule{targeted: true}
		}
	}
	switch {
	case strings.HasPrefix(eventType, "scan:"):
		return sseEventRule{require: core.PermStorageManage, passAll: true}
	case strings.HasPrefix(eventType, "jobs:"):
		return sseEventRule{require: core.PermJobsViewStatus}
	}
	return sseEventRule{}
}

// eventForUser returns the event as a user may see it, false when it must not be sent. An
// event targeted at a user (core.SceneEvent.UserID) goes to that user only, data as is; a
// targeted event type that was broadcast is dropped. Otherwise a superuser gets every event
// as is.
func eventForUser(event core.SceneEvent, access *core.UserAccess) (core.SceneEvent, bool) {
	if access == nil {
		return event, false
	}
	if event.UserID != 0 {
		return event, event.UserID == access.UserID
	}
	rule := sseRuleFor(event.Type)
	if rule.targeted {
		return event, false
	}
	if access.Superuser {
		return event, true
	}
	if rule.require != "" && !access.Has(rule.require) {
		return event, false
	}
	if rule.passAll || event.Data == nil {
		return event, true
	}
	data, ok := event.Data.(map[string]any)
	if !ok || len(rule.fields) == 0 {
		event.Data = nil
		return event, true
	}
	out := make(map[string]any, len(data))
	for k, v := range data {
		needs, allowed := rule.fields[k]
		if !allowed {
			continue
		}
		visible := true
		for _, p := range needs {
			if !access.Has(p) {
				visible = false
				break
			}
		}
		if visible {
			out[k] = v
		}
	}
	event.Data = out
	return event, true
}
