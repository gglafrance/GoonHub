package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// tick runs the time based transitions (grace period over, idle, paused, activities expired),
// saves the last seen times and pushes the presences that changed. Called by the loop every
// flush_interval (and directly by the tests, with a fake clock).
func (s *PresenceService) tick(ctx context.Context) {
	now := s.now()
	writes := make(map[uint]time.Time)

	s.mu.Lock()
	for id, u := range s.users {
		if u.online && u.conns == 0 && now.Sub(u.offlineAt) >= s.cfg.DisconnectGrace {
			u.online = false
			if !u.invisible {
				t := u.offlineAt
				u.lastSeen = &t
				writes[id] = t
			}
		}
		if u.online && u.conns > 0 && !u.invisible && now.Sub(u.lastSeenSaved) >= s.cfg.LastSeenInterval {
			u.lastSeenSaved = now
			writes[id] = now
		}
		for tab, a := range u.tabs {
			if now.Sub(a.UpdatedAt) >= s.cfg.ActivityTTL {
				delete(u.tabs, tab)
			}
		}
		key := ownerKey(s.ownerLocked(id, u, now))
		if !u.hasPublished || key != u.published {
			u.published, u.hasPublished = key, true
			s.dirty[id] = true
		} else if !u.online && len(u.tabs) == 0 && !s.dirty[id] {
			// offline and already pushed as such: the state is dropped (bounded memory)
			delete(s.users, id)
		}
	}
	// viewers gone (no live connection) forget what they were sent
	keep := s.cfg.DisconnectGrace + time.Minute
	for id, v := range s.viewers {
		if u, ok := s.users[id]; ok && u.online {
			continue
		}
		if now.Sub(v.touched) >= keep {
			delete(s.viewers, id)
		}
	}
	dirty := make([]uint, 0, len(s.dirty))
	for id := range s.dirty {
		dirty = append(dirty, id)
	}
	s.dirty = make(map[uint]bool)
	s.mu.Unlock()

	s.saveLastSeen(ctx, writes)
	for _, id := range dirty {
		if err := s.flushOwner(ctx, id); err != nil {
			s.logger.Warn("Failed to push a presence change, retried at the next flush", zap.Uint("user_id", id), zap.Error(err))
			s.markDirty(id)
		}
	}
}

// flushOwner pushes the presence of the owner to every live viewer whose view of it changed:
// the viewers allowed by the owner's privacy get the new view, those who were sent a view and
// are no longer allowed get {visible: false}.
func (s *PresenceService) flushOwner(ctx context.Context, ownerID uint) error {
	now := s.now()
	s.mu.Lock()
	u, known := s.users[ownerID]
	var owner presenceOwner
	if known {
		owner = s.ownerLocked(ownerID, u, now)
	}
	candidates := make([]uint, 0, len(s.viewers))
	for id := range s.viewers {
		if id != ownerID {
			candidates = append(candidates, id)
		}
	}
	s.mu.Unlock()
	if len(candidates) == 0 {
		return nil
	}

	if !known || owner.username == "" {
		c, err := s.repo.GetUser(ctx, ownerID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // deleted user
		}
		if err != nil {
			return err
		}
		if !known {
			owner = offlineOwner(*c)
		} else {
			owner.username = c.Username
			if owner.lastSeen == nil {
				owner.lastSeen = c.LastSeenAt
			}
		}
	}

	online, err := s.privacy.AudienceMembers(ctx, ownerID, PrivacyOnlineStatus)
	if err != nil {
		return fmt.Errorf("failed to read the online status audience: %w", err)
	}
	var watch AudienceMembers
	if owner.activity != nil && !owner.invisible && owner.status != PresenceOffline {
		if watch, err = s.privacy.AudienceMembers(ctx, ownerID, PrivacyNowWatching); err != nil {
			return fmt.Errorf("failed to read the now watching audience: %w", err)
		}
	}

	type computed struct {
		viewer uint
		view   PresenceView
		key    presenceKey
	}
	views := make([]computed, 0, len(candidates))
	for _, viewer := range candidates {
		canOnline := online.Contains(viewer)
		canWatch := canOnline && watch.Contains(viewer)
		canScene := canWatch && owner.activity != nil && owner.activity.Scene != nil && s.canSeeScenes(viewer)
		v, k := viewFor(owner, canOnline, canWatch, canScene)
		views = append(views, computed{viewer: viewer, view: v, key: k})
	}

	type push struct {
		viewer uint
		view   PresenceView
	}
	var pushes []push
	s.mu.Lock()
	for i := range views {
		c := &views[i]
		if _, live := s.viewers[c.viewer]; !live {
			continue
		}
		if s.recordLocked(c.viewer, &c.view, c.key, now) {
			pushes = append(pushes, push{viewer: c.viewer, view: c.view})
		}
	}
	s.mu.Unlock()

	if s.eventBus != nil {
		for _, p := range pushes {
			s.eventBus.PublishToUser(p.viewer, SceneEvent{Type: EventPresenceUpdate, Data: p.view})
		}
	}
	return nil
}

// Snapshot returns the presence of the viewer's friends and followed users that the viewer
// may see, and records it as sent (later changes are pushed relative to it).
func (s *PresenceService) Snapshot(ctx context.Context, viewerID uint) (*PresenceSnapshot, error) {
	contacts, err := s.repo.Contacts(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, len(contacts))
	for i, c := range contacts {
		ids[i] = c.ID
	}
	onlineVis, err := s.privacy.VisibleTo(ctx, viewerID, ids, PrivacyOnlineStatus)
	if err != nil {
		return nil, err
	}
	watchVis, err := s.privacy.VisibleTo(ctx, viewerID, ids, PrivacyNowWatching)
	if err != nil {
		return nil, err
	}
	canScenes := s.canSeeScenes(viewerID)

	invisible, err := s.Invisible(ctx, viewerID)
	if err != nil {
		return nil, err
	}

	now := s.now()
	out := &PresenceSnapshot{
		Users:             make([]PresenceView, 0, len(contacts)),
		Invisible:         invisible,
		HeartbeatInterval: s.cfg.HeartbeatInterval,
		ActivityInterval:  s.cfg.ActivityInterval,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range contacts {
		owner := s.contactOwnerLocked(c, now)
		canOnline := onlineVis[c.ID]
		canWatch := canOnline && watchVis[c.ID]
		v, k := viewFor(owner, canOnline, canWatch, canWatch && canScenes)
		s.stampLocked(viewerID, &v, k, now)
		if v.Visible {
			out.Users = append(out.Users, v)
		}
	}
	return out, nil
}

// contactOwnerLocked is the state of a contact: live when known, else offline as stored.
func (s *PresenceService) contactOwnerLocked(c data.PresenceContact, now time.Time) presenceOwner {
	u, ok := s.users[c.ID]
	if !ok {
		return offlineOwner(c)
	}
	owner := s.ownerLocked(c.ID, u, now)
	if owner.username == "" {
		owner.username = c.Username
	}
	if owner.lastSeen == nil {
		owner.lastSeen = c.LastSeenAt
	}
	return owner
}

// stampLocked records a view answered to a viewer (snapshot, single view) and gives it a new
// sequence number, so that the client prefers it to any earlier push.
func (s *PresenceService) stampLocked(viewerID uint, view *PresenceView, key presenceKey, now time.Time) {
	if viewerID == view.User.ID {
		s.seq++
		view.Seq = s.seq
		return
	}
	if !s.recordLocked(viewerID, view, key, now) {
		s.seq++
		view.Seq = s.seq
	}
}

// ViewOf returns the presence of one user as the viewer may see it (Visible false when hidden),
// e.g. for a profile page.
func (s *PresenceService) ViewOf(ctx context.Context, viewerID, ownerID uint) (*PresenceView, error) {
	c, err := s.repo.GetUser(ctx, ownerID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperrors.ErrUserNotFound(ownerID)
	}
	if err != nil {
		return nil, err
	}
	ids := []uint{ownerID}
	onlineVis, err := s.privacy.VisibleTo(ctx, viewerID, ids, PrivacyOnlineStatus)
	if err != nil {
		return nil, err
	}
	watchVis, err := s.privacy.VisibleTo(ctx, viewerID, ids, PrivacyNowWatching)
	if err != nil {
		return nil, err
	}
	canScenes := s.canSeeScenes(viewerID)
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	owner := s.contactOwnerLocked(*c, now)
	canOnline := onlineVis[ownerID]
	canWatch := canOnline && watchVis[ownerID]
	v, k := viewFor(owner, canOnline, canWatch, canWatch && canScenes)
	s.stampLocked(viewerID, &v, k, now)
	return &v, nil
}
