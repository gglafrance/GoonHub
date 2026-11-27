package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sync"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/lifecycle"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// presencePrivacy is what the presence needs from the privacy framework (PrivacyService).
type presencePrivacy interface {
	AudienceMembers(ctx context.Context, ownerID uint, item string) (AudienceMembers, error)
	VisibleTo(ctx context.Context, viewerID uint, ownerIDs []uint, item string) (map[uint]bool, error)
}

// presenceScenes reads the scenes of the watching_scene activities (not trashed ones).
type presenceScenes interface {
	GetByIDs(ids []uint) ([]data.Scene, error)
}

// PresenceAccessResolver resolves the live permissions of a viewer (RBACService), to redact
// the scenes a viewer may not see.
type PresenceAccessResolver interface {
	ResolveUser(userID uint) (*UserAccess, error)
}

// presenceActivity is the activity of one tab of a user.
type presenceActivity struct {
	Type        string
	SceneID     uint
	Scene       *PresenceScene // nil when the scene is unknown or trashed
	Position    float64
	PositionAt  time.Time
	Paused      bool
	PausedSince time.Time
	StartedAt   time.Time
	UpdatedAt   time.Time
	// SeekEpoch counts the seeks: a seek is pushed, a normal progress is not
	SeekEpoch uint32
}

// presenceUser is the live presence of a user (in memory, never stored but last_seen_at).
type presenceUser struct {
	username string
	// conns: open live connections (SSE streams); offlineAt: when the last one closed
	conns     int
	offlineAt time.Time
	// online: connected, or disconnected for less than the grace period
	online bool
	// loaded: username, invisible mode and last seen were read from the database
	loaded bool
	// lastActive: the last real input (heartbeats carry the time since the last input) or
	// moment of playback (a playing report whose position moved)
	lastActive time.Time
	// heard: a heartbeat was received for this state. Until then a fresh connection counts as
	// activity (connectActive), so that a client that has not reported yet is not shown away;
	// the first heartbeat replaces it with the real input time.
	heard         bool
	connectActive time.Time
	invisible     bool
	lastSeen      *time.Time
	// lastSeenSaved: last write of last_seen_at while online (throttled)
	lastSeenSaved time.Time
	tabs          map[string]*presenceActivity
	// published: the key of the last flushed state (change detection)
	published    presenceKey
	hasPublished bool
}

// presenceKey is what a viewer sees of a presence, minus the normal progress of a playback:
// a push is sent when it changes.
type presenceKey struct {
	visible     bool
	status      string
	lastSeen    int64
	actType     string
	startedAt   int64
	sceneID     uint
	sceneHidden bool
	paused      bool
	seekEpoch   uint32
}

// presenceViewer is what was last sent to a viewer, per owner (to push only changes, and to
// revoke what the viewer may no longer see).
type presenceViewer struct {
	sent    map[uint]presenceKey
	touched time.Time
}

// presenceOwner is a copy of the state of an owner at one moment, read without the lock.
type presenceOwner struct {
	id        uint
	username  string
	status    string
	invisible bool
	lastSeen  *time.Time
	activity  *presenceActivity // displayed activity (copy), nil when none
	paused    bool              // the activity shows as paused
}

// PresenceService tracks who is online, away or offline and what they are doing, and pushes
// the changes to the users allowed to see them (docs/PRESENCE.md). Everything is in memory
// (one process) but last_seen_at and the invisible mode (user_presence). One goroutine
// (lifecycle) expires the grace periods, the activity (away) and the tab activities, and
// flushes the coalesced changes at flush_interval.
type PresenceService struct {
	repo     data.PresenceRepository
	privacy  presencePrivacy
	scenes   presenceScenes
	access   PresenceAccessResolver
	eventBus *EventBus
	cfg      config.PresenceConfig
	logger   *zap.Logger
	now      func() time.Time

	lifecycle *lifecycle.Manager
	stopOnce  sync.Once

	mu      sync.Mutex
	users   map[uint]*presenceUser
	viewers map[uint]*presenceViewer
	dirty   map[uint]bool
	seq     uint64
}

// NewPresenceService creates the presence service and subscribes it to the privacy and social
// changes (they apply to the presence at the next flush).
func NewPresenceService(repo data.PresenceRepository, privacy *PrivacyService, social *SocialService, scenes data.SceneRepository, access PresenceAccessResolver, eventBus *EventBus, cfg config.PresenceConfig, logger *zap.Logger) *PresenceService {
	var p presencePrivacy
	if privacy != nil {
		p = privacy
	}
	var sc presenceScenes
	if scenes != nil {
		sc = scenes
	}
	s := newPresenceService(repo, p, sc, access, eventBus, cfg, logger)
	if privacy != nil {
		privacy.OnChange(s.onPrivacyChange)
	}
	if social != nil {
		social.OnRelationChange(s.onRelationChange)
	}
	return s
}

func newPresenceService(repo data.PresenceRepository, privacy presencePrivacy, scenes presenceScenes, access PresenceAccessResolver, eventBus *EventBus, cfg config.PresenceConfig, logger *zap.Logger) *PresenceService {
	logger = logger.With(zap.String("service", "presence"))
	return &PresenceService{
		repo:      repo,
		privacy:   privacy,
		scenes:    scenes,
		access:    access,
		eventBus:  eventBus,
		cfg:       normalizePresenceConfig(cfg),
		logger:    logger,
		now:       time.Now,
		lifecycle: lifecycle.NewManager(logger),
		users:     make(map[uint]*presenceUser),
		viewers:   make(map[uint]*presenceViewer),
		dirty:     make(map[uint]bool),
	}
}

// Config returns the effective timings.
func (s *PresenceService) Config() config.PresenceConfig { return s.cfg }

// Start runs the presence loop (expiry and flush) until Stop.
func (s *PresenceService) Start() {
	s.lifecycle.Go("presence-loop", func(done <-chan struct{}) {
		ticker := time.NewTicker(s.cfg.FlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				s.tick(ctx)
				cancel()
			}
		}
	})
}

// Stop ends the loop and saves the last seen time of the users still online.
func (s *PresenceService) Stop() {
	s.stopOnce.Do(func() {
		if err := s.lifecycle.Shutdown(5 * time.Second); err != nil {
			s.logger.Warn("Presence loop did not stop in time", zap.Error(err))
		}
		now := s.now()
		writes := make(map[uint]time.Time)
		s.mu.Lock()
		for id, u := range s.users {
			if u.online && !u.invisible {
				writes[id] = now
			}
		}
		s.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.saveLastSeen(ctx, writes)
	})
}

func (s *PresenceService) saveLastSeen(ctx context.Context, writes map[uint]time.Time) {
	if len(writes) == 0 || s.repo == nil {
		return
	}
	if err := s.repo.SaveLastSeen(ctx, writes); err != nil {
		s.logger.Warn("Failed to save the last seen times", zap.Int("users", len(writes)), zap.Error(err))
	}
}

// --- connections and activity -------------------------------------------------------------

func (s *PresenceService) touchViewerLocked(userID uint, now time.Time) {
	v, ok := s.viewers[userID]
	if !ok {
		v = &presenceViewer{sent: make(map[uint]presenceKey)}
		s.viewers[userID] = v
	}
	v.touched = now
}

func newPresenceUser() *presenceUser {
	return &presenceUser{tabs: make(map[string]*presenceActivity)}
}

// userLocked returns the state of the user, created when missing (a heartbeat or an activity
// may arrive before the stream opens).
func (s *PresenceService) userLocked(userID uint) *presenceUser {
	u, ok := s.users[userID]
	if !ok {
		u = newPresenceUser()
		s.users[userID] = u
	}
	return u
}

// Connect records a live connection of the user (an SSE stream opened).
func (s *PresenceService) Connect(userID uint) {
	if userID == 0 {
		return
	}
	s.mu.Lock()
	if u, ok := s.users[userID]; ok && u.loaded {
		s.connectLocked(userID, u)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	// first connection: read the username and the invisible mode (outside the lock). A state
	// created by a heartbeat or an activity received before the stream is completed too.
	var info *data.PresenceContact
	if s.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		c, err := s.repo.GetUser(ctx, userID)
		cancel()
		if err != nil {
			s.logger.Warn("Failed to read the presence of a connecting user", zap.Uint("user_id", userID), zap.Error(err))
		} else {
			info = c
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.userLocked(userID)
	if !u.loaded && info != nil {
		u.username = info.Username
		u.invisible = info.Invisible
		u.lastSeen = info.LastSeenAt
		u.loaded = true
	}
	s.connectLocked(userID, u)
}

// connectLocked adds a connection to the user. A user coming online who never sent a
// heartbeat counts as active until their first one (it carries the real input time).
func (s *PresenceService) connectLocked(userID uint, u *presenceUser) {
	now := s.now()
	u.conns++
	u.offlineAt = time.Time{}
	if !u.online {
		u.online = true
		if !u.heard {
			u.connectActive = now
		}
	}
	s.touchViewerLocked(userID, now)
}

// Disconnect records that a live connection of the user closed. The user stays online for
// the grace period (a page reload reconnects within it).
func (s *PresenceService) Disconnect(userID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[userID]
	if !ok || u.conns == 0 {
		return
	}
	u.conns--
	if u.conns == 0 {
		u.offlineAt = s.now()
	}
}

// presenceMaxIdleReport bounds the time since the last input a heartbeat may report.
const presenceMaxIdleReport = 7 * 24 * time.Hour

// Heartbeat records the last real input in a tab of the user: idle is the time since that
// input (0: now). The latest input of all the tabs counts (a heartbeat never moves it back).
// The first heartbeat also ends the "just connected" activity (connectActive): a tab that
// reconnects after a long time without input (server restart, network loss) reports its real
// idle time and the user is away at once. Accepted before the stream opens (kept for it).
func (s *PresenceService) Heartbeat(userID uint, active bool, idle time.Duration) {
	if !active || userID == 0 {
		return
	}
	if idle < 0 {
		idle = 0
	}
	if idle > presenceMaxIdleReport {
		idle = presenceMaxIdleReport
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.userLocked(userID)
	if at := s.now().Add(-idle); at.After(u.lastActive) {
		u.lastActive = at
	}
	u.heard = true
	u.connectActive = time.Time{}
}

var presenceTabIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const presenceDefaultTab = "default"

func normalizeTabID(tabID string) (string, error) {
	if tabID == "" {
		return presenceDefaultTab, nil
	}
	if !presenceTabIDPattern.MatchString(tabID) {
		return "", apperrors.NewValidationErrorWithField("tab_id", "must be 1 to 64 letters, digits, - or _")
	}
	return tabID, nil
}

// validateActivity checks an activity reported by a client.
func validateActivity(in PresenceActivityInput) error {
	switch in.Type {
	case ActivityWatchingScene:
		if in.SceneID == 0 {
			return apperrors.NewValidationErrorWithField("scene_id", "is required")
		}
		if in.Position < 0 || math.IsNaN(in.Position) || math.IsInf(in.Position, 0) {
			return apperrors.NewValidationErrorWithField("position", "must be a positive number of seconds")
		}
		return nil
	default:
		return apperrors.NewValidationErrorWithField("type", fmt.Sprintf("unknown activity type %q", in.Type))
	}
}

// sceneCard reads the scene of a watching_scene activity: nil when it does not exist or is
// trashed (the activity then shows as "watching something" to everyone).
func (s *PresenceService) sceneCard(sceneID uint) (*PresenceScene, error) {
	if s.scenes == nil {
		return nil, nil
	}
	scenes, err := s.scenes.GetByIDs([]uint{sceneID})
	if err != nil {
		return nil, fmt.Errorf("failed to read the scene: %w", err)
	}
	for _, sc := range scenes {
		if sc.ID != sceneID {
			continue
		}
		card := &PresenceScene{ID: sc.ID, Title: sc.Title, Duration: sc.Duration}
		if sc.ThumbnailPath != "" {
			card.ThumbnailURL = fmt.Sprintf("/thumbnails/%d?v=%d", sc.ID, sc.UpdatedAt.UnixMilli())
		}
		return card, nil
	}
	return nil, nil
}

// UpdateActivity records the activity of a tab of the user (start, progress, pause). The
// caller checks that the user may do the activity (scenes:view for watching_scene). A playback
// that really plays counts as activity (not away while watching): not paused, and started,
// resumed or moved since the previous report (a stalled playback reports the same position).
func (s *PresenceService) UpdateActivity(userID uint, in PresenceActivityInput) error {
	tabID, err := normalizeTabID(in.TabID)
	if err != nil {
		return err
	}
	if err := validateActivity(in); err != nil {
		return err
	}

	s.mu.Lock()
	var known *PresenceScene
	sameScene := false
	if u, ok := s.users[userID]; ok {
		if a, ok := u.tabs[tabID]; ok && a.Type == in.Type && a.SceneID == in.SceneID {
			sameScene, known = true, a.Scene
		}
	}
	s.mu.Unlock()

	card := known
	if !sameScene {
		if card, err = s.sceneCard(in.SceneID); err != nil {
			return err
		}
		if card == nil {
			return apperrors.ErrSceneNotFound(in.SceneID)
		}
	}

	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	// no live connection yet (the stream opens meanwhile): kept, shown once online
	u := s.userLocked(userID)
	playing := false
	a, ok := u.tabs[tabID]
	if !ok || a.Type != in.Type || a.SceneID != in.SceneID {
		playing = !in.Paused
		a = &presenceActivity{Type: in.Type, SceneID: in.SceneID, Scene: card, StartedAt: now}
		if in.Paused {
			a.PausedSince = now
		}
		if !ok {
			s.evictOldestTabLocked(u)
		}
		u.tabs[tabID] = a
	} else {
		playing = !in.Paused && (a.Paused || math.Abs(in.Position-a.Position) >= presenceMinProgress)
		expected := a.Position
		if !a.Paused {
			expected += now.Sub(a.PositionAt).Seconds()
		}
		if math.Abs(in.Position-expected) > s.cfg.SeekThreshold.Seconds() {
			a.SeekEpoch++
		}
		if in.Paused && !a.Paused {
			a.PausedSince = now
		}
	}
	a.Position = in.Position
	a.PositionAt = now
	a.Paused = in.Paused
	a.UpdatedAt = now
	if playing {
		u.lastActive = now
	}
	return nil
}

// presenceMinProgress (seconds): a playing report whose position moved less than this since
// the previous one is a stalled playback (the client rounds positions to 0.1 s).
const presenceMinProgress = 0.05

// evictOldestTabLocked makes room for a new tab activity (max_tabs_per_user).
func (s *PresenceService) evictOldestTabLocked(u *presenceUser) {
	for len(u.tabs) >= s.cfg.MaxTabsPerUser {
		var oldestID string
		var oldest time.Time
		for id, a := range u.tabs {
			if oldestID == "" || a.UpdatedAt.Before(oldest) {
				oldestID, oldest = id, a.UpdatedAt
			}
		}
		delete(u.tabs, oldestID)
	}
}

// ClearActivity ends the activity of a tab of the user (the page was left).
func (s *PresenceService) ClearActivity(userID uint, tabID string) error {
	tabID, err := normalizeTabID(tabID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[userID]; ok {
		delete(u.tabs, tabID)
	}
	return nil
}

// Invisible returns the invisible mode of the user.
func (s *PresenceService) Invisible(ctx context.Context, userID uint) (bool, error) {
	s.mu.Lock()
	if u, ok := s.users[userID]; ok {
		inv := u.invisible
		s.mu.Unlock()
		return inv, nil
	}
	s.mu.Unlock()
	info, err := s.repo.GetUser(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, apperrors.ErrUserNotFound(userID)
	}
	if err != nil {
		return false, err
	}
	return info.Invisible, nil
}

// SetInvisible sets the invisible mode of the user: invisible, they appear offline to
// everyone (last seen frozen at the moment they turned it on). Applied at the next flush.
func (s *PresenceService) SetInvisible(ctx context.Context, userID uint, invisible bool) error {
	now := s.now()
	s.mu.Lock()
	u, ok := s.users[userID]
	var lastSeen *time.Time
	if ok && invisible && !u.invisible && u.online {
		// the others saw the user online until now
		t := now
		lastSeen = &t
	}
	s.mu.Unlock()

	if err := s.repo.SetInvisible(ctx, userID, invisible, lastSeen); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[userID]; ok {
		if lastSeen != nil {
			u.lastSeen = lastSeen
		}
		if u.invisible && !invisible {
			u.lastSeenSaved = time.Time{} // saved again at the next tick
		}
		u.invisible = invisible
	}
	s.dirty[userID] = true
	return nil
}

// --- privacy and social changes -------------------------------------------------------------

func (s *PresenceService) markDirty(ids ...uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if id != 0 {
			s.dirty[id] = true
		}
	}
}

// onPrivacyChange re-evaluates who sees the presence of the user when a presence item changed.
func (s *PresenceService) onPrivacyChange(userID uint, items []string) {
	for _, item := range items {
		if item == PrivacyOnlineStatus || item == PrivacyNowWatching {
			s.markDirty(userID)
			return
		}
	}
}

// onRelationChange re-evaluates the presence of both users toward each other (follow,
// unfollow, friendship made or removed).
func (s *PresenceService) onRelationChange(a, b uint) {
	s.markDirty(a, b)
}

// --- state ----------------------------------------------------------------------------------

// statusLocked is the real status of a user: away when connected without input nor playback
// for idle_after.
func (s *PresenceService) statusLocked(u *presenceUser, now time.Time) string {
	if !u.online {
		return PresenceOffline
	}
	last := u.lastActive
	if u.connectActive.After(last) {
		last = u.connectActive
	}
	if now.Sub(last) >= s.cfg.IdleAfter {
		return PresenceAway
	}
	return PresenceOnline
}

// displayedActivityLocked is the activity shown for the user: a playing one before a paused
// one, then the most recently reported. None while offline.
func (s *PresenceService) displayedActivityLocked(u *presenceUser, now time.Time) *presenceActivity {
	if !u.online {
		return nil
	}
	var best *presenceActivity
	for _, a := range u.tabs {
		if now.Sub(a.UpdatedAt) >= s.cfg.ActivityTTL {
			continue
		}
		if best == nil || (best.Paused && !a.Paused) || (best.Paused == a.Paused && a.UpdatedAt.After(best.UpdatedAt)) {
			best = a
		}
	}
	return best
}

// ownerLocked copies the state of a known user.
func (s *PresenceService) ownerLocked(id uint, u *presenceUser, now time.Time) presenceOwner {
	o := presenceOwner{id: id, username: u.username, status: s.statusLocked(u, now), invisible: u.invisible, lastSeen: u.lastSeen}
	if a := s.displayedActivityLocked(u, now); a != nil {
		cp := *a
		if a.Scene != nil {
			sc := *a.Scene
			cp.Scene = &sc
		}
		o.activity = &cp
		o.paused = a.Paused && now.Sub(a.PausedSince) >= s.cfg.PausedAfter
	}
	return o
}

func offlineOwner(c data.PresenceContact) presenceOwner {
	return presenceOwner{id: c.ID, username: c.Username, status: PresenceOffline, invisible: c.Invisible, lastSeen: c.LastSeenAt}
}

// viewFor builds the presence of the owner as a viewer sees it: nothing without online_status,
// offline when invisible, the activity only with now_watching, its scene only when the viewer
// may see scenes.
func viewFor(o presenceOwner, canOnline, canWatch, canScene bool) (PresenceView, presenceKey) {
	v := PresenceView{User: PresenceUserRef{ID: o.id, Username: o.username}}
	if !canOnline {
		return v, presenceKey{}
	}
	v.Visible = true
	v.Status = o.status
	if o.invisible {
		v.Status = PresenceOffline
	}
	key := presenceKey{visible: true, status: v.Status}
	if v.Status == PresenceOffline {
		if o.lastSeen != nil {
			t := *o.lastSeen
			v.LastSeenAt = &t
			key.lastSeen = t.UnixNano()
		}
		return v, key
	}
	if canWatch && o.activity != nil {
		v.Activity = activityView(o, canScene)
		key.actType = o.activity.Type
		key.startedAt = o.activity.StartedAt.UnixNano()
		if v.Activity.Scene != nil {
			key.sceneID = o.activity.SceneID
			key.paused = o.paused
			key.seekEpoch = o.activity.SeekEpoch
		} else {
			key.sceneHidden = true
		}
	}
	return v, key
}

// activityView is the displayed activity of the owner as a viewer sees it.
func activityView(o presenceOwner, canScene bool) *PresenceActivityView {
	a := o.activity
	av := &PresenceActivityView{Type: a.Type, StartedAt: a.StartedAt}
	if a.Type == ActivityWatchingScene && canScene && a.Scene != nil {
		sc := *a.Scene
		pos := a.Position
		at := a.PositionAt
		paused := o.paused
		av.Scene, av.Position, av.PositionAt, av.Paused = &sc, &pos, &at, &paused
	}
	return av
}

// ownerKey is the key of the full presence of a user (change detection).
func ownerKey(o presenceOwner) presenceKey {
	_, k := viewFor(o, true, true, true)
	return k
}

// canSeeScenes reports whether the viewer may see scene titles and thumbnails (scenes:view).
func (s *PresenceService) canSeeScenes(viewerID uint) bool {
	if s.access == nil {
		return false
	}
	access, err := s.access.ResolveUser(viewerID)
	if err != nil {
		return false
	}
	return access.Has(PermScenesView)
}

// recordLocked stores what was sent to a viewer for an owner and stamps the view; false when
// it did not change (nothing to push).
func (s *PresenceService) recordLocked(viewerID uint, view *PresenceView, key presenceKey, now time.Time) bool {
	s.touchViewerLocked(viewerID, now)
	vw := s.viewers[viewerID]
	prev, had := vw.sent[view.User.ID]
	if !key.visible {
		if !had {
			return false
		}
		delete(vw.sent, view.User.ID)
	} else {
		if had && prev == key {
			return false
		}
		vw.sent[view.User.ID] = key
	}
	s.seq++
	view.Seq = s.seq
	return true
}
