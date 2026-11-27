package core

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"goonhub/internal/config"
	"goonhub/internal/data"
	"goonhub/internal/mocks"

	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// --- fakes ------------------------------------------------------------------------------------

type presenceClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *presenceClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *presenceClock) Set(t time.Time) {
	c.mu.Lock()
	c.t = t
	c.mu.Unlock()
}

func (c *presenceClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// fakePresencePrivacy: per owner and item, the viewers allowed (or public).
type fakePresencePrivacy struct {
	mu      sync.Mutex
	allowed map[uint]map[string]map[uint]bool
	public  map[uint]map[string]bool
}

func newFakePresencePrivacy() *fakePresencePrivacy {
	return &fakePresencePrivacy{allowed: map[uint]map[string]map[uint]bool{}, public: map[uint]map[string]bool{}}
}

func (f *fakePresencePrivacy) allow(owner uint, item string, viewers ...uint) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.allowed[owner] == nil {
		f.allowed[owner] = map[string]map[uint]bool{}
	}
	f.allowed[owner][item] = map[uint]bool{}
	for _, v := range viewers {
		f.allowed[owner][item][v] = true
	}
}

// allowBoth lets the viewers see the online status and the activity of the owner.
func (f *fakePresencePrivacy) allowBoth(owner uint, viewers ...uint) {
	f.allow(owner, PrivacyOnlineStatus, viewers...)
	f.allow(owner, PrivacyNowWatching, viewers...)
}

func (f *fakePresencePrivacy) AudienceMembers(_ context.Context, owner uint, item string) (AudienceMembers, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.public[owner][item] {
		return AudienceMembers{Everyone: true}, nil
	}
	var ids []uint
	for v := range f.allowed[owner][item] {
		ids = append(ids, v)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return AudienceMembers{UserIDs: ids}, nil
}

func (f *fakePresencePrivacy) VisibleTo(_ context.Context, viewer uint, owners []uint, item string) (map[uint]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[uint]bool{}
	for _, o := range owners {
		out[o] = o == viewer || f.public[o][item] || f.allowed[o][item][viewer]
	}
	return out, nil
}

type fakePresenceScenes map[uint]data.Scene

func (f fakePresenceScenes) GetByIDs(ids []uint) ([]data.Scene, error) {
	var out []data.Scene
	for _, id := range ids {
		if sc, ok := f[id]; ok {
			out = append(out, sc)
		}
	}
	return out, nil
}

type fakePresenceAccess map[uint]*UserAccess

func (f fakePresenceAccess) ResolveUser(id uint) (*UserAccess, error) {
	if a, ok := f[id]; ok {
		return a, nil
	}
	return NewUserAccess(id, "user", false), nil
}

// --- env ----------------------------------------------------------------------------------------

const (
	pAlice = uint(1) // the owner in most tests
	pBob   = uint(2) // a friend who sees everything
	pCarol = uint(3) // sees the online status only
	pDave  = uint(4) // sees nothing
)

type presenceEnv struct {
	svc     *PresenceService
	clock   *presenceClock
	privacy *fakePresencePrivacy
	bus     *EventBus
	repo    *mocks.MockPresenceRepository

	mu       sync.Mutex
	lastSeen map[uint]time.Time
	stored   map[uint]data.PresenceContact
}

func newPresenceEnv(t *testing.T) *presenceEnv {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockPresenceRepository(ctrl)
	env := &presenceEnv{
		clock:    &presenceClock{t: time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)},
		privacy:  newFakePresencePrivacy(),
		bus:      NewEventBus(zap.NewNop()),
		repo:     repo,
		lastSeen: map[uint]time.Time{},
		stored: map[uint]data.PresenceContact{
			pAlice: {ID: pAlice, Username: "alice"},
			pBob:   {ID: pBob, Username: "bob"},
			pCarol: {ID: pCarol, Username: "carol"},
			pDave:  {ID: pDave, Username: "dave"},
		},
	}
	repo.EXPECT().GetUser(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, id uint) (*data.PresenceContact, error) {
		env.mu.Lock()
		defer env.mu.Unlock()
		c, ok := env.stored[id]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		return &c, nil
	}).AnyTimes()
	repo.EXPECT().SaveLastSeen(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, m map[uint]time.Time) error {
		env.mu.Lock()
		defer env.mu.Unlock()
		for id, at := range m {
			env.lastSeen[id] = at
			c := env.stored[id]
			t := at
			c.LastSeenAt = &t
			env.stored[id] = c
		}
		return nil
	}).AnyTimes()
	repo.EXPECT().SetInvisible(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, id uint, inv bool, at *time.Time) error {
		env.mu.Lock()
		defer env.mu.Unlock()
		c := env.stored[id]
		c.Invisible = inv
		if at != nil {
			t := *at
			c.LastSeenAt = &t
		}
		env.stored[id] = c
		return nil
	}).AnyTimes()

	scenes := fakePresenceScenes{
		10: {ID: 10, Title: "Scene ten", Duration: 600, ThumbnailPath: "10.jpg", StoredPath: "/media/secret/ten.mp4", OriginalFilename: "ten.mp4"},
		11: {ID: 11, Title: "Scene eleven", Duration: 300},
	}
	access := fakePresenceAccess{
		pBob:   NewUserAccess(pBob, "user", false, PermScenesView),
		pCarol: NewUserAccess(pCarol, "user", false, PermScenesView),
		pDave:  NewUserAccess(pDave, "user", false, PermScenesView),
	}
	env.svc = newPresenceService(repo, env.privacy, scenes, access, env.bus, config.PresenceConfig{
		DisconnectGrace:  30 * time.Second,
		IdleAfter:        5 * time.Minute,
		ActivityTTL:      60 * time.Second,
		PausedAfter:      20 * time.Second,
		SeekThreshold:    15 * time.Second,
		LastSeenInterval: 5 * time.Minute,
		MaxTabsPerUser:   2,
	}, zap.NewNop())
	env.svc.now = env.clock.Now
	return env
}

// stream opens a live stream of the viewer (SSE): presence connection + event subscription.
func (e *presenceEnv) stream(t *testing.T, userID uint) <-chan SceneEvent {
	t.Helper()
	id, ch := e.bus.SubscribeUser(userID)
	t.Cleanup(func() { e.bus.Unsubscribe(id) })
	e.svc.Connect(userID)
	return ch
}

// drain returns the presence views received so far (Publish is synchronous).
func drain(ch <-chan SceneEvent) []PresenceView {
	var out []PresenceView
	for {
		select {
		case ev := <-ch:
			if ev.Type == EventPresenceUpdate {
				out = append(out, ev.Data.(PresenceView))
			}
		default:
			return out
		}
	}
}

// only returns the single view received about the owner (fails otherwise).
func only(t *testing.T, ch <-chan SceneEvent, owner uint) PresenceView {
	t.Helper()
	views := drain(ch)
	if len(views) != 1 || views[0].User.ID != owner {
		t.Fatalf("want one view of user %d, got %+v", owner, views)
	}
	return views[0]
}

func none(t *testing.T, ch <-chan SceneEvent) {
	t.Helper()
	if views := drain(ch); len(views) != 0 {
		t.Fatalf("want no presence event, got %+v", views)
	}
}

func (e *presenceEnv) tick() { e.svc.tick(context.Background()) }

// --- tests --------------------------------------------------------------------------------------

func TestPresenceOnlineGracePeriodAndLastSeen(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.tick()
	drain(bob)

	env.svc.Connect(pAlice)
	env.tick()
	if v := only(t, bob, pAlice); !v.Visible || v.Status != PresenceOnline || v.User.Username != "alice" {
		t.Fatalf("online view: %+v", v)
	}

	// page reload: the stream closes and reopens within the grace period, nothing is pushed
	env.svc.Disconnect(pAlice)
	env.clock.Advance(10 * time.Second)
	env.tick()
	env.svc.Connect(pAlice)
	env.clock.Advance(40 * time.Second)
	env.tick()
	none(t, bob)

	// closed for good: offline once the grace period is over, last seen = the disconnection
	env.svc.Disconnect(pAlice)
	disconnectedAt := env.clock.Now()
	env.clock.Advance(29 * time.Second)
	env.tick()
	none(t, bob)
	env.clock.Advance(2 * time.Second)
	env.tick()
	v := only(t, bob, pAlice)
	if v.Status != PresenceOffline || v.LastSeenAt == nil || !v.LastSeenAt.Equal(disconnectedAt) {
		t.Fatalf("offline view: %+v", v)
	}
	env.mu.Lock()
	saved := env.lastSeen[pAlice]
	env.mu.Unlock()
	if !saved.Equal(disconnectedAt) {
		t.Fatalf("last seen saved %v, want %v", saved, disconnectedAt)
	}

	// the offline state is dropped at the next tick (bounded memory)
	env.tick()
	env.svc.mu.Lock()
	_, kept := env.svc.users[pAlice]
	env.svc.mu.Unlock()
	if kept {
		t.Fatal("offline state should be dropped once pushed")
	}
	none(t, bob)
}

func TestPresenceMultiTab(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.svc.Connect(pAlice) // second device
	env.tick()
	drain(bob)

	env.svc.Disconnect(pAlice)
	env.clock.Advance(time.Minute)
	env.tick()
	none(t, bob)

	env.svc.Disconnect(pAlice)
	env.clock.Advance(time.Minute)
	env.tick()
	if v := only(t, bob, pAlice); v.Status != PresenceOffline {
		t.Fatalf("want offline after the last connection, got %+v", v)
	}
	// an extra disconnect is ignored
	env.svc.Disconnect(pAlice)
}

// watch reports the playback of a tab of alice (scene 10).
func (e *presenceEnv) watch(t *testing.T, tab string, position float64, paused bool) {
	t.Helper()
	if err := e.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: tab, Type: ActivityWatchingScene, SceneID: 10, Position: position, Paused: paused}); err != nil {
		t.Fatal(err)
	}
}

// statuses returns the statuses pushed about alice.
func statuses(ch <-chan SceneEvent) []string {
	var out []string
	for _, v := range drain(ch) {
		if v.User.ID == pAlice {
			out = append(out, v.Status)
		}
	}
	return out
}

func TestPresenceAwayAfterNoInput(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.svc.Heartbeat(pAlice, true, 0)
	env.tick()
	drain(bob)

	// a heartbeat that is not an input does not count
	env.clock.Advance(4 * time.Minute)
	env.svc.Heartbeat(pAlice, false, 0)
	env.clock.Advance(59 * time.Second)
	env.tick()
	none(t, bob)

	// away exactly idle_after after the last input, pushed once
	env.clock.Advance(time.Second)
	env.tick()
	if v := only(t, bob, pAlice); v.Status != PresenceAway || !v.Visible {
		t.Fatalf("want away, got %+v", v)
	}
	for i := 0; i < 5; i++ {
		env.clock.Advance(time.Minute)
		env.tick()
	}
	none(t, bob)

	// the next input brings the user back at the next flush, pushed once
	env.svc.Heartbeat(pAlice, true, 0)
	env.tick()
	if v := only(t, bob, pAlice); v.Status != PresenceOnline {
		t.Fatalf("want online after input, got %+v", v)
	}
	env.tick()
	none(t, bob)
}

func TestPresenceHeartbeatReportsTheInputTime(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.svc.Heartbeat(pAlice, true, 0)
	env.tick()
	drain(bob)

	// a trailing heartbeat: the input was 50 s ago, away 5 min after that input
	env.clock.Advance(time.Minute)
	env.svc.Heartbeat(pAlice, true, 50*time.Second)
	env.clock.Advance(4*time.Minute + 9*time.Second)
	env.tick()
	none(t, bob)
	env.clock.Advance(time.Second)
	env.tick()
	if got := statuses(bob); len(got) != 1 || got[0] != PresenceAway {
		t.Fatalf("want away 5 min after the reported input, got %v", got)
	}

	// an older input reported by another tab does not move the last input back
	env.svc.Heartbeat(pAlice, true, 0)
	env.svc.Heartbeat(pAlice, true, 20*time.Minute)
	env.svc.Heartbeat(pAlice, true, -time.Second) // clamped to now
	env.tick()
	if got := statuses(bob); len(got) != 1 || got[0] != PresenceOnline {
		t.Fatalf("want online, got %v", got)
	}
}

func TestPresenceFreshConnectionUntilTheFirstHeartbeat(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)

	// a connection never followed by a heartbeat (old client): active for idle_after
	env.svc.Connect(pAlice)
	env.tick()
	if v := only(t, bob, pAlice); v.Status != PresenceOnline {
		t.Fatalf("want online at connection, got %+v", v)
	}
	env.clock.Advance(5 * time.Minute)
	env.tick()
	if got := statuses(bob); len(got) != 1 || got[0] != PresenceAway {
		t.Fatalf("want away, got %v", got)
	}
	env.svc.Disconnect(pAlice)
	env.clock.Advance(31 * time.Second)
	env.tick()
	env.tick() // offline pushed, state dropped
	drain(bob)

	// server restart / long network loss: the tab reconnects and reports that its last
	// input was 20 minutes ago: away at the next flush instead of 5 more minutes online
	env.svc.Connect(pAlice)
	env.svc.Heartbeat(pAlice, true, 20*time.Minute)
	env.tick()
	if got := statuses(bob); len(got) != 1 || got[0] != PresenceAway {
		t.Fatalf("want away right after the reconnection, got %v", got)
	}
}

func TestPresenceHeartbeatBeforeTheStream(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.tick()
	drain(bob)
	env.mu.Lock()
	env.stored[pCarol] = data.PresenceContact{ID: pCarol, Username: "carol", Invisible: true}
	env.mu.Unlock()
	env.privacy.allowBoth(pCarol, pBob)

	// the heartbeat of the page load arrives before its stream opens: kept for it
	env.svc.Heartbeat(pAlice, true, 0)
	env.svc.Heartbeat(pCarol, true, 0)
	env.svc.Connect(pAlice)
	env.svc.Connect(pCarol)
	env.tick()
	views := drain(bob)
	got := map[uint]PresenceView{}
	for _, v := range views {
		got[v.User.ID] = v
	}
	if a := got[pAlice]; a.Status != PresenceOnline || a.User.Username != "alice" {
		t.Fatalf("want alice online, got %+v", views)
	}
	if c, ok := got[pCarol]; ok && c.Status != PresenceOffline {
		t.Fatalf("carol is invisible, got %+v", c)
	}
	// the heartbeat set the real input time: away 5 min after it
	env.clock.Advance(5 * time.Minute)
	env.tick()
	if got := statuses(bob); len(got) != 1 || got[0] != PresenceAway {
		t.Fatalf("want away, got %v", got)
	}
	// the invisible mode was read although the state existed before the stream
	if inv, _ := env.svc.Invisible(context.Background(), pCarol); !inv {
		t.Fatal("invisible mode of carol not loaded")
	}
}

func TestPresencePlayingKeepsOnline(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.svc.Heartbeat(pAlice, true, 0)
	env.tick()
	drain(bob)

	// a playing scene keeps the user online without input
	for i := 0; i < 40; i++ {
		env.clock.Advance(15 * time.Second)
		env.watch(t, "t1", float64(15*(i+1)), false)
		env.tick()
	}
	views := drain(bob)
	for _, v := range views {
		if v.Status != PresenceOnline {
			t.Fatalf("away while playing: %+v", v)
		}
	}
	if len(views) != 1 || views[0].Activity == nil {
		t.Fatalf("only the start of the activity is pushed, got %+v", views)
	}
}

func TestPresencePausedVideoGoesAway(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.svc.Heartbeat(pAlice, true, 0)
	env.watch(t, "t1", 0, false)
	env.tick()
	drain(bob)

	env.clock.Advance(time.Minute)
	env.watch(t, "t1", 60, false)
	pausedAt := env.clock.Now()
	env.watch(t, "t1", 60, true)
	// still reported every 15 s (to keep the activity), without input
	var views []PresenceView
	for i := 0; i < 19; i++ {
		env.clock.Advance(15 * time.Second)
		env.watch(t, "t1", 60, true)
		env.tick()
		views = append(views, drain(bob)...)
	}
	env.clock.Set(pausedAt.Add(5*time.Minute - time.Second))
	env.tick()
	views = append(views, drain(bob)...)
	// only the pause was pushed (after paused_after), the user stays online until idle_after
	if len(views) != 1 || views[0].Status != PresenceOnline || views[0].Activity == nil || !*views[0].Activity.Paused {
		t.Fatalf("want one paused view, got %+v", views)
	}
	env.clock.Set(pausedAt.Add(5 * time.Minute))
	env.watch(t, "t1", 60, true)
	env.tick()
	v := only(t, bob, pAlice)
	if v.Status != PresenceAway || v.Activity == nil || v.Activity.Scene == nil || !*v.Activity.Paused {
		t.Fatalf("want away, paused on the scene: %+v", v)
	}
	// resumed: online again, even before the position moves
	env.watch(t, "t1", 60, false)
	env.tick()
	v = only(t, bob, pAlice)
	if v.Status != PresenceOnline || *v.Activity.Paused {
		t.Fatalf("want online, playing: %+v", v)
	}
}

func TestPresenceStalledVideoDoesNotCount(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.svc.Heartbeat(pAlice, true, 0)
	env.watch(t, "t1", 10, false)
	env.tick()
	drain(bob)

	// buffering: not paused, but the position does not move
	lastProgress := env.clock.Now()
	for i := 0; i < 19; i++ {
		env.clock.Advance(15 * time.Second)
		env.watch(t, "t1", 10, false)
		env.tick()
	}
	none(t, bob)
	env.clock.Set(lastProgress.Add(5 * time.Minute))
	env.watch(t, "t1", 10, false)
	env.tick()
	if got := statuses(bob); len(got) != 1 || got[0] != PresenceAway {
		t.Fatalf("want away while stalled, got %v", got)
	}
	// playing again
	env.clock.Advance(15 * time.Second)
	env.watch(t, "t1", 12, false)
	env.tick()
	if got := statuses(bob); len(got) != 1 || got[0] != PresenceOnline {
		t.Fatalf("want online once it plays again, got %v", got)
	}
}

func TestPresenceAwayWithSeveralTabs(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.svc.Heartbeat(pAlice, true, 0)
	// tab 1: paused; tab 2: playing
	env.watch(t, "t1", 30, true)
	if err := env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t2", Type: ActivityWatchingScene, SceneID: 11, Position: 0}); err != nil {
		t.Fatal(err)
	}
	env.tick()
	drain(bob)

	var stoppedAt time.Time
	aways := 0
	for i := 1; i <= 40; i++ {
		env.clock.Advance(15 * time.Second)
		env.watch(t, "t1", 30, true)
		if i <= 20 {
			_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t2", Type: ActivityWatchingScene, SceneID: 11, Position: float64(15 * i)})
			stoppedAt = env.clock.Now()
		} else if i == 21 {
			_ = env.svc.ClearActivity(pAlice, "t2") // tab 2 closed
		}
		env.tick()
		for _, v := range drain(bob) {
			if v.Status == PresenceAway {
				aways++
				if env.clock.Now().Before(stoppedAt.Add(5 * time.Minute)) {
					t.Fatalf("away %v after tab 2 stopped playing: %+v", env.clock.Now().Sub(stoppedAt), v)
				}
			}
		}
	}
	if aways != 1 {
		t.Fatalf("want away pushed once, got %d", aways)
	}
	// 40*15 s = 10 min: tab 2 stopped at 5 min, away 5 min later with the paused tab shown
	v, err := env.svc.ViewOf(context.Background(), pBob, pAlice)
	if err != nil || v.Status != PresenceAway || v.Activity == nil || v.Activity.Scene.ID != 10 {
		t.Fatalf("want away on the paused tab: %+v %v", v, err)
	}
}

func TestPresenceWatchingScene(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.tick()
	drain(bob)

	if err := env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 30}); err != nil {
		t.Fatal(err)
	}
	env.tick()
	v := only(t, bob, pAlice)
	a := v.Activity
	if a == nil || a.Type != ActivityWatchingScene || a.Scene == nil || a.Scene.ID != 10 || a.Scene.Title != "Scene ten" ||
		a.Scene.Duration != 600 || !strings.HasPrefix(a.Scene.ThumbnailURL, "/thumbnails/10?v=") || a.Position == nil || *a.Position != 30 ||
		a.Paused == nil || *a.Paused {
		t.Fatalf("watching view: %+v %+v", v, a)
	}

	// normal progress: nothing; a seek: pushed
	env.clock.Advance(15 * time.Second)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 45})
	env.tick()
	none(t, bob)
	env.clock.Advance(15 * time.Second)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 300})
	env.tick()
	if v := only(t, bob, pAlice); *v.Activity.Position != 300 {
		t.Fatalf("seek not pushed: %+v", v.Activity)
	}

	// paused: shown as paused only after paused_after
	env.clock.Advance(5 * time.Second)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 305, Paused: true})
	env.tick()
	none(t, bob)
	env.clock.Advance(21 * time.Second)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 305, Paused: true})
	env.tick()
	if v := only(t, bob, pAlice); !*v.Activity.Paused {
		t.Fatalf("want paused: %+v", v.Activity)
	}

	// another tab plays another scene: the playing one is shown
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t2", Type: ActivityWatchingScene, SceneID: 11, Position: 0})
	env.tick()
	if v := only(t, bob, pAlice); v.Activity.Scene.ID != 11 {
		t.Fatalf("want the playing tab: %+v", v.Activity)
	}

	// the tab leaves: back to the paused one, then nothing
	_ = env.svc.ClearActivity(pAlice, "t2")
	env.tick()
	if v := only(t, bob, pAlice); v.Activity == nil || v.Activity.Scene.ID != 10 {
		t.Fatalf("want the remaining tab: %+v", v.Activity)
	}
	// not reported anymore (tab closed without a DELETE): expires after activity_ttl
	env.clock.Advance(61 * time.Second)
	env.tick()
	if v := only(t, bob, pAlice); v.Activity != nil || v.Status == PresenceOffline {
		t.Fatalf("activity should expire: %+v", v)
	}
}

func TestPresenceActivityValidation(t *testing.T) {
	env := newPresenceEnv(t)
	cases := []PresenceActivityInput{
		{Type: "dancing", SceneID: 10},
		{Type: ActivityWatchingScene},
		{Type: ActivityWatchingScene, SceneID: 10, Position: -1},
		{Type: ActivityWatchingScene, SceneID: 10, TabID: "bad tab id!"},
		{Type: ActivityWatchingScene, SceneID: 10, TabID: strings.Repeat("a", 65)},
	}
	for _, in := range cases {
		if err := env.svc.UpdateActivity(pAlice, in); err == nil {
			t.Fatalf("accepted %+v", in)
		}
	}
	if err := env.svc.UpdateActivity(pAlice, PresenceActivityInput{Type: ActivityWatchingScene, SceneID: 99}); err == nil {
		t.Fatal("unknown scene accepted")
	}
	// tab limit: the oldest tab is dropped
	for i, tab := range []string{"a", "b", "c"} {
		env.clock.Advance(time.Second)
		if err := env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: tab, Type: ActivityWatchingScene, SceneID: 10, Position: float64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	env.svc.mu.Lock()
	_, hasA := env.svc.users[pAlice].tabs["a"]
	n := len(env.svc.users[pAlice].tabs)
	env.svc.mu.Unlock()
	if hasA || n != 2 {
		t.Fatalf("tabs not bounded: %d, oldest kept %v", n, hasA)
	}
}

func TestPresencePrivacyFilteringOfPushes(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	env.privacy.allow(pAlice, PrivacyOnlineStatus, pBob, pCarol)
	env.privacy.allow(pAlice, PrivacyNowWatching, pBob)
	bob, carol, dave := env.stream(t, pBob), env.stream(t, pCarol), env.stream(t, pDave)
	env.svc.Connect(pAlice)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 5})
	env.tick()

	if v := only(t, bob, pAlice); v.Activity == nil || v.Activity.Scene == nil {
		t.Fatalf("bob sees the scene: %+v", v)
	}
	if v := only(t, carol, pAlice); !v.Visible || v.Status != PresenceOnline || v.Activity != nil {
		t.Fatalf("carol sees the status only: %+v", v)
	}
	none(t, dave)

	// carol loses the online status: revoked at the next flush, dave still gets nothing
	env.privacy.allow(pAlice, PrivacyOnlineStatus, pBob)
	env.svc.onPrivacyChange(pAlice, []string{PrivacyFavorites})
	env.tick()
	none(t, carol) // another item: not re-evaluated
	env.svc.onPrivacyChange(pAlice, []string{PrivacyOnlineStatus})
	env.tick()
	if v := only(t, carol, pAlice); v.Visible || v.Status != "" || v.Activity != nil {
		t.Fatalf("carol must be told she can no longer see alice: %+v", v)
	}
	none(t, bob)
	none(t, dave)

	// bob unfriends alice: revoked at once (relation change), both ways
	env.privacy.allow(pAlice, PrivacyOnlineStatus)
	env.privacy.allow(pAlice, PrivacyNowWatching)
	env.svc.onRelationChange(pAlice, pBob)
	env.tick()
	if v := only(t, bob, pAlice); v.Visible {
		t.Fatalf("bob must lose alice: %+v", v)
	}

	// public: every live viewer gets it
	env.privacy.mu.Lock()
	env.privacy.public[pAlice] = map[string]bool{PrivacyOnlineStatus: true}
	env.privacy.mu.Unlock()
	env.svc.onPrivacyChange(pAlice, []string{PrivacyOnlineStatus})
	env.tick()
	for name, ch := range map[string]<-chan SceneEvent{"bob": bob, "carol": carol, "dave": dave} {
		if v := only(t, ch, pAlice); !v.Visible || v.Activity != nil {
			t.Fatalf("%s: public status without activity: %+v", name, v)
		}
	}
}

func TestPresenceSceneRedaction(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob, pCarol)
	// carol may not see scenes: only "watching something"
	env.svc.access = fakePresenceAccess{
		pBob:   NewUserAccess(pBob, "user", false, PermScenesView, PermFilesViewPaths, PermFilesViewTech),
		pCarol: NewUserAccess(pCarol, "user", false),
	}
	bob, carol := env.stream(t, pBob), env.stream(t, pCarol)
	env.svc.Connect(pAlice)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 5})
	env.tick()

	vb := only(t, bob, pAlice)
	raw, _ := json.Marshal(vb)
	for _, leak := range []string{"secret", "ten.mp4", "stored_path", "original_filename", "size"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("file info leaked in %s", raw)
		}
	}
	vc := only(t, carol, pAlice)
	if vc.Activity == nil || vc.Activity.Type != ActivityWatchingScene || vc.Activity.Scene != nil || vc.Activity.Position != nil || vc.Activity.Paused != nil {
		t.Fatalf("carol must see 'watching something' only: %+v", vc.Activity)
	}
	raw, _ = json.Marshal(vc)
	if strings.Contains(string(raw), "Scene ten") || strings.Contains(string(raw), "thumbnail") {
		t.Fatalf("scene leaked to carol: %s", raw)
	}
	// carol's view does not change on a seek she cannot see
	env.clock.Advance(15 * time.Second)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 400})
	env.tick()
	only(t, bob, pAlice)
	none(t, carol)
}

func TestPresenceInvisible(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	bob := env.stream(t, pBob)
	env.svc.Connect(pAlice)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 5})
	env.tick()
	drain(bob)

	env.clock.Advance(time.Minute)
	if err := env.svc.SetInvisible(context.Background(), pAlice, true); err != nil {
		t.Fatal(err)
	}
	wentInvisible := env.clock.Now()
	env.mu.Lock()
	env.lastSeen = map[uint]time.Time{}
	env.mu.Unlock()
	env.tick()
	v := only(t, bob, pAlice)
	if v.Status != PresenceOffline || v.Activity != nil || v.LastSeenAt == nil || !v.LastSeenAt.Equal(wentInvisible) {
		t.Fatalf("invisible must look offline: %+v", v)
	}
	// while invisible: activity and idle changes are not pushed, last seen is not saved
	env.clock.Advance(10 * time.Minute)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 11, Position: 0})
	env.tick()
	none(t, bob)
	env.mu.Lock()
	_, saved := env.lastSeen[pAlice]
	env.mu.Unlock()
	if saved {
		t.Fatal("last seen saved while invisible")
	}
	if inv, _ := env.svc.Invisible(context.Background(), pAlice); !inv {
		t.Fatal("invisible not kept")
	}

	if err := env.svc.SetInvisible(context.Background(), pAlice, false); err != nil {
		t.Fatal(err)
	}
	env.tick()
	if v := only(t, bob, pAlice); v.Status != PresenceOnline || v.Activity == nil || v.Activity.Scene.ID != 11 {
		t.Fatalf("visible again: %+v", v)
	}
}

func TestPresenceSnapshot(t *testing.T) {
	env := newPresenceEnv(t)
	seen := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	env.mu.Lock()
	env.stored[pCarol] = data.PresenceContact{ID: pCarol, Username: "carol", LastSeenAt: &seen}
	env.mu.Unlock()
	env.repo.EXPECT().Contacts(gomock.Any(), pBob).DoAndReturn(func(context.Context, uint) ([]data.PresenceContact, error) {
		env.mu.Lock()
		defer env.mu.Unlock()
		return []data.PresenceContact{env.stored[pAlice], env.stored[pCarol], env.stored[pDave]}, nil
	}).AnyTimes()
	env.privacy.allowBoth(pAlice, pBob)
	env.privacy.allow(pCarol, PrivacyOnlineStatus, pBob)
	// dave hides everything from bob

	env.svc.Connect(pAlice)
	_ = env.svc.UpdateActivity(pAlice, PresenceActivityInput{TabID: "t1", Type: ActivityWatchingScene, SceneID: 10, Position: 5})
	env.tick()

	snap, err := env.svc.Snapshot(context.Background(), pBob)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Users) != 2 {
		t.Fatalf("want alice and carol, got %+v", snap.Users)
	}
	alice, carol := snap.Users[0], snap.Users[1]
	if alice.User.ID != pAlice || alice.Status != PresenceOnline || alice.Activity == nil || alice.Activity.Scene.ID != 10 || alice.Seq == 0 {
		t.Fatalf("alice: %+v", alice)
	}
	if carol.User.ID != pCarol || carol.Status != PresenceOffline || carol.LastSeenAt == nil || !carol.LastSeenAt.Equal(seen) {
		t.Fatalf("carol: %+v", carol)
	}
	if snap.Invisible || snap.HeartbeatInterval != time.Minute || snap.ActivityInterval != 15*time.Second ||
		snap.IdleAfter != 5*time.Minute || snap.StalledAfter != 30*time.Second {
		t.Fatalf("snapshot settings: %+v", snap)
	}

	// what the snapshot gave is revoked by a push when it is no longer allowed (bob is not
	// even connected: the snapshot is enough)
	id, bobCh := env.bus.SubscribeUser(pBob)
	defer env.bus.Unsubscribe(id)
	env.privacy.allow(pCarol, PrivacyOnlineStatus)
	env.svc.onPrivacyChange(pCarol, []string{PrivacyOnlineStatus})
	env.tick()
	if v := only(t, bobCh, pCarol); v.Visible || v.Seq <= carol.Seq {
		t.Fatalf("carol must be revoked after the snapshot: %+v", v)
	}

	// one user
	v, err := env.svc.ViewOf(context.Background(), pBob, pAlice)
	if err != nil || !v.Visible || v.Activity == nil {
		t.Fatalf("view of alice: %+v %v", v, err)
	}
	v, err = env.svc.ViewOf(context.Background(), pBob, pDave)
	if err != nil || v.Visible || v.Status != "" {
		t.Fatalf("view of dave must be hidden: %+v %v", v, err)
	}
	if _, err := env.svc.ViewOf(context.Background(), pBob, 99); err == nil {
		t.Fatal("unknown user")
	}
}

func TestPresenceViewersForgottenAfterDisconnect(t *testing.T) {
	env := newPresenceEnv(t)
	env.privacy.allowBoth(pAlice, pBob)
	env.stream(t, pBob)
	env.svc.Connect(pAlice)
	env.tick()
	env.svc.Disconnect(pBob)
	env.clock.Advance(31 * time.Second)
	env.tick()
	env.clock.Advance(2 * time.Minute)
	env.tick()
	env.svc.mu.Lock()
	_, bobUser := env.svc.users[pBob]
	_, bobViewer := env.svc.viewers[pBob]
	env.svc.mu.Unlock()
	if bobUser || bobViewer {
		t.Fatalf("bob's state should be dropped (user %v, viewer %v)", bobUser, bobViewer)
	}
}

func TestPresenceLoopStartStop(t *testing.T) {
	env := newPresenceEnv(t)
	env.svc.cfg.FlushInterval = 5 * time.Millisecond
	env.svc.Connect(pAlice)
	env.svc.Start()
	time.Sleep(30 * time.Millisecond)
	env.svc.Stop()
	env.svc.Stop() // idempotent
	env.mu.Lock()
	_, saved := env.lastSeen[pAlice]
	env.mu.Unlock()
	if !saved {
		t.Fatal("last seen of the online users must be saved at shutdown")
	}
}

func TestPresenceListenersWired(t *testing.T) {
	ctrl := gomock.NewController(t)
	privacyRepo := mocks.NewMockPrivacyRepository(ctrl)
	socialRepo := mocks.NewMockSocialRepository(ctrl)
	privacy := NewPrivacyService(privacyRepo, socialRepo, zap.NewNop())
	social := NewSocialService(socialRepo, privacy, nil, config.SocialConfig{}, zap.NewNop())
	svc := NewPresenceService(mocks.NewMockPresenceRepository(ctrl), privacy, social, nil, nil, nil, config.PresenceConfig{}, zap.NewNop())

	privacyRepo.EXPECT().SaveSettings(gomock.Any(), pAlice, gomock.Any(), gomock.Any()).Return(nil)
	privacyRepo.EXPECT().ListSettings(gomock.Any(), pAlice).Return(nil, nil)
	socialRepo.EXPECT().GetFriendRequestPolicy(gomock.Any(), pAlice).Return(data.FriendRequestPolicyEveryone, nil)
	if _, err := privacy.UpdateSettings(context.Background(), pAlice, UpdatePrivacyInput{Items: map[string]string{PrivacyNowWatching: "private"}}); err != nil {
		t.Fatal(err)
	}
	social.relationChanged(pBob, pCarol)
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if !svc.dirty[pAlice] || !svc.dirty[pBob] || !svc.dirty[pCarol] {
		t.Fatalf("privacy and relation changes must mark the users: %v", svc.dirty)
	}
}
