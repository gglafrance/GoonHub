package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"goonhub/internal/api/middleware"
	"goonhub/internal/config"
	"goonhub/internal/core"
	"goonhub/internal/together"
	"goonhub/internal/together/togethertest"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type togetherAccess map[uint][]string

func (a togetherAccess) ResolveUser(id uint) (*core.UserAccess, error) {
	return core.NewUserAccess(id, "user", false, a[id]...), nil
}

var togetherUsers = map[uint]string{1: "alice", 2: "bob", 3: "carol", 4: "mod"}

// togetherServer runs the Goon2gether routes behind a stand-in of the auth middleware: the
// X-Test-User header picks the user.
func togetherServer(t *testing.T) (*httptest.Server, *together.Hub) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := togethertest.NewStore()
	scenes := togethertest.Scenes{1: {ID: 1, Title: "One", Duration: 100}, 2: {ID: 2, Title: "Two", Duration: 50}}
	for id, n := range togetherUsers {
		store.Users[id] = n
	}
	for id, s := range scenes {
		store.Scenes[id] = s
	}
	base := []string{core.PermTogetherUse, core.PermTogetherHost, core.PermScenesView}
	access := togetherAccess{1: base, 2: base, 3: {core.PermTogetherUse}, 4: append([]string{core.PermTogetherModerate, core.PermEmotesManage}, base...)}
	cfg := config.Goon2getherConfig{
		Enabled: true, MaxMembers: 10, MaxQueueItems: 20, MaxSuggestions: 5, MaxConnectionsPerUser: 3,
		ChatHistory: 20, ChatMaxLength: 200, ChatRateLimit: 10, ReactionRateLimit: 10, RateWindow: 10 * time.Second,
		DriftThreshold: 0.75, HoldTimeout: 10 * time.Second, EndGrace: time.Second, HostGrace: time.Minute,
		UnloadAfter: time.Minute, IdleTimeout: time.Hour, CleanupInterval: time.Minute, SendBuffer: 64,
		PingInterval: time.Second, WriteTimeout: 5 * time.Second, MaxMessageBytes: 4096,
		EmoteDir: t.TempDir(), EmoteMaxBytes: 64 << 10, EmoteMaxDimension: 128, EmoteMaxCount: 10,
	}
	hub := together.NewHub(cfg, store, scenes, access, zap.NewNop())
	emotes := together.NewEmoteService(togethertest.NewEmotes(), cfg, zap.NewNop())
	hub.SetEmotes(emotes)
	emotes.OnChange(hub.BroadcastEmotesChanged)
	h := NewTogetherHandler(hub, emotes, OriginPatterns([]string{"http://localhost:3000"}), zap.NewNop())

	r := gin.New()
	auth := func(c *gin.Context) {
		id, err := strconv.ParseUint(c.GetHeader("X-Test-User"), 10, 32)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		uid := uint(id)
		c.Set("user", &core.UserPayload{UserID: uid, Username: togetherUsers[uid]})
		a, _ := access.ResolveUser(uid)
		middleware.SetAccess(c, a)
	}
	perm := middleware.RequirePermission
	tg := r.Group("/api/v1/together", auth, perm(core.PermTogetherUse))
	tg.GET("/lobbies", h.ListLobbies)
	tg.GET("/public-count", h.PublicCount)
	tg.POST("/lobbies", perm(core.PermTogetherHost), h.CreateLobby)
	tg.GET("/lobbies/:id", h.GetLobby)
	tg.DELETE("/lobbies/:id", h.CloseLobby)
	tg.POST("/lobbies/:id/join", h.JoinLobby)
	tg.POST("/lobbies/:id/leave", h.LeaveLobby)
	tg.POST("/lobbies/:id/queue", perm(core.PermScenesView), h.AddToQueue)
	tg.GET("/lobbies/:id/ws", h.Socket)
	tg.POST("/join", h.JoinByCode)
	tg.GET("/emotes", h.ListEmotes)
	tg.POST("/emotes", perm(core.PermEmotesManage), h.UploadEmote)
	tg.PUT("/emotes/:id", perm(core.PermEmotesManage), h.RenameEmote)
	tg.DELETE("/emotes/:id", perm(core.PermEmotesManage), h.DeleteEmote)

	srv := httptest.NewServer(r)
	t.Cleanup(func() {
		hub.Stop()
		srv.Close()
	})
	return srv, hub
}

func tgJSON(t *testing.T, srv *httptest.Server, method, path string, user uint, body any) (int, map[string]any) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, srv.URL+path, rd)
	req.Header.Set("X-Test-User", strconv.Itoa(int(user)))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func tgDial(t *testing.T, srv *httptest.Server, lobby string, user uint, origin string, query ...string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	h := http.Header{"X-Test-User": {strconv.Itoa(int(user))}}
	if origin != "" {
		h.Set("Origin", origin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/together/lobbies/" + lobby + "/ws?v=" + strconv.Itoa(together.ProtocolVersion)
	for _, q := range query {
		url += "&" + q
	}
	return websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: h})
}

// readUntil reads messages until one of type typ.
func readUntil(t *testing.T, conn *websocket.Conn, typ string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, b, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %q: %v", typ, err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if m["t"] == typ {
			return m
		}
	}
}

func tgWrite(t *testing.T, conn *websocket.Conn, msg map[string]any) {
	t.Helper()
	b, _ := json.Marshal(msg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

// closeCode waits for the server to close the connection and gives its close code.
func closeCode(t *testing.T, conn *websocket.Conn) websocket.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func TestTogetherWebSocketRoundTrip(t *testing.T) {
	srv, _ := togetherServer(t)

	status, created := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 1, map[string]any{
		"name": "Movie night", "visibility": "public", "items": []map[string]any{{"scene_id": 1}, {"scene_id": 2}},
	})
	if status != http.StatusCreated || created["name"] != "Movie night" || created["queue_length"] != float64(2) {
		t.Fatalf("create: %d %v", status, created)
	}
	id := created["id"].(string)

	host, _, err := tgDial(t, srv, id, 1, "")
	if err != nil {
		t.Fatalf("dial host: %v", err)
	}
	defer host.CloseNow()
	welcome := readUntil(t, host, "welcome")
	if welcome["v"] != float64(together.ProtocolVersion) || len(welcome["queue"].([]any)) != 2 {
		t.Fatalf("welcome %v", welcome)
	}
	// No file information in the queue items
	item := welcome["queue"].([]any)[0].(map[string]any)
	for _, k := range []string{"stored_path", "size", "original_filename", "video_codec"} {
		if _, ok := item[k]; ok {
			t.Fatalf("queue item leaks %q: %v", k, item)
		}
	}

	// A member joins a public lobby by connecting, from an allowed dev origin
	member, _, err := tgDial(t, srv, id, 2, "http://localhost:3000")
	if err != nil {
		t.Fatalf("dial member: %v", err)
	}
	defer member.CloseNow()
	readUntil(t, member, "welcome")

	tgWrite(t, host, map[string]any{"t": "ping", "id": 1, "c": 1000})
	if pong := readUntil(t, host, "pong"); pong["id"] != float64(1) || pong["s"].(float64) <= 0 {
		t.Fatalf("pong %v", pong)
	}

	tgWrite(t, host, map[string]any{"t": "play", "position": 5})
	state := readUntil(t, member, "state")["state"].(map[string]any)
	if state["playing"] != true || state["position"] != float64(5) {
		t.Fatalf("member state %v", state)
	}

	// Refused command answers an error with the ref
	tgWrite(t, member, map[string]any{"t": "pause", "ref": "r1"})
	if e := readUntil(t, member, "error"); e["code"] != together.ErrCodeNotAllowed || e["ref"] != "r1" {
		t.Fatalf("error %v", e)
	}

	tgWrite(t, member, map[string]any{"t": "chat", "text": "hello :pog:"})
	if m := readUntil(t, host, "chat")["message"].(map[string]any); m["username"] != "bob" {
		t.Fatalf("chat %v", m)
	}

	// Kick: bye, then close code 4001
	tgWrite(t, host, map[string]any{"t": "kick", "user_id": 2})
	if bye := readUntil(t, member, "bye"); bye["reason"] != together.ByeKicked {
		t.Fatalf("bye %v", bye)
	}
	if code := closeCode(t, member); code != together.CloseKicked {
		t.Fatalf("kick close code %d", code)
	}

	// Closing the lobby closes the remaining connections with 4004
	if status, _ := tgJSON(t, srv, http.MethodDelete, "/api/v1/together/lobbies/"+id, 1, nil); status != http.StatusNoContent {
		t.Fatalf("close: %d", status)
	}
	if code := closeCode(t, host); code != together.CloseClosed {
		t.Fatalf("lobby closed code %d", code)
	}
}

func TestTogetherWebSocketRefusals(t *testing.T) {
	srv, _ := togetherServer(t)
	_, created := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 1, map[string]any{"name": "Private"})
	id := created["id"].(string)

	// Cross-origin upgrades are refused at the handshake
	if _, res, err := tgDial(t, srv, id, 1, "https://evil.example"); err == nil || res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatalf("a foreign origin was accepted: %v %v", res, err)
	}
	// A non-member of a private lobby: upgraded, then closed with 4004
	conn, _, err := tgDial(t, srv, id, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if code := closeCode(t, conn); code != together.CloseClosed {
		t.Fatalf("non-member close code %d", code)
	}
	// Unknown lobby, unsupported version, no session
	conn, _, err = tgDial(t, srv, "6f1c3e2a-0000-4000-8000-000000000000", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if code := closeCode(t, conn); code != together.CloseClosed {
		t.Fatalf("unknown lobby close code %d", code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, res, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/together/lobbies/"+id+"/ws", nil); err == nil || res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no session: %v %v", res, err)
	}

	// Oversized messages close the connection
	host, _, err := tgDial(t, srv, id, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	readUntil(t, host, "welcome")
	tgWrite(t, host, map[string]any{"t": "chat", "text": strings.Repeat("x", 5000)})
	if code := closeCode(t, host); code != websocket.StatusMessageTooBig {
		t.Fatalf("oversized message close code %d", code)
	}
}

func TestTogetherRESTFlow(t *testing.T) {
	srv, _ := togetherServer(t)
	// Creating needs goon2gether:host (carol only has use)
	if status, _ := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 3, map[string]any{}); status != http.StatusForbidden {
		t.Fatalf("create without host: %d", status)
	}
	_, created := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 1, map[string]any{"name": "Private"})
	id := created["id"].(string)
	code := created["invite_code"].(string)

	if status, _ := tgJSON(t, srv, http.MethodGet, "/api/v1/together/lobbies/"+id, 2, nil); status != http.StatusNotFound {
		t.Fatalf("private lobby visible: %d", status)
	}
	if status, _ := tgJSON(t, srv, http.MethodPost, "/api/v1/together/join", 2, map[string]any{"code": "NOPE1234"}); status != http.StatusNotFound {
		t.Fatalf("bad code: %d", status)
	}
	status, joined := tgJSON(t, srv, http.MethodPost, "/api/v1/together/join", 2, map[string]any{"code": strings.ToLower(code)})
	if status != http.StatusOK || joined["is_member"] != true {
		t.Fatalf("join by code: %d %v", status, joined)
	}
	// A member without queue rights suggests
	status, added := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies/"+id+"/queue", 2, map[string]any{"items": []map[string]any{{"scene_id": 1, "start": 10, "end": 20, "label": "Marker"}}})
	if status != http.StatusOK || added["suggested"] != float64(1) || added["queued"] != float64(0) {
		t.Fatalf("add as member: %d %v", status, added)
	}
	status, added = tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies/"+id+"/queue", 1, map[string]any{"items": []map[string]any{{"scene_id": 2}}})
	if status != http.StatusOK || added["queued"] != float64(1) {
		t.Fatalf("add as host: %d %v", status, added)
	}
	if status, _ := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies/"+id+"/queue", 1, map[string]any{"items": []map[string]any{{"scene_id": 99}}}); status != http.StatusNotFound {
		t.Fatalf("unknown scene: %d", status)
	}
	// Without scenes:view, adding is refused by the route
	if status, _ := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies/"+id+"/queue", 3, map[string]any{"items": []map[string]any{{"scene_id": 1}}}); status != http.StatusForbidden {
		t.Fatalf("add without scenes:view: %d", status)
	}
	_, list := tgJSON(t, srv, http.MethodGet, "/api/v1/together/lobbies", 2, nil)
	lobbies := list["lobbies"].([]any)
	if len(lobbies) != 1 || lobbies[0].(map[string]any)["now_playing"].(map[string]any)["title"] != "Two" {
		t.Fatalf("list: %v", list)
	}
	if status, _ := tgJSON(t, srv, http.MethodDelete, "/api/v1/together/lobbies/"+id, 2, nil); status != http.StatusForbidden {
		t.Fatalf("member close: %d", status)
	}
	if status, _ := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies/"+id+"/leave", 2, nil); status != http.StatusNoContent {
		t.Fatalf("leave: %d", status)
	}
	if status, _ := tgJSON(t, srv, http.MethodGet, "/api/v1/together/lobbies/not-a-uuid", 1, nil); status != http.StatusNotFound {
		t.Fatalf("bad id: %d", status)
	}
}

func TestTogetherPublicCount(t *testing.T) {
	srv, _ := togetherServer(t)
	count := func(user uint) (int, any) {
		status, out := tgJSON(t, srv, http.MethodGet, "/api/v1/together/public-count", user, nil)
		return status, out["count"]
	}
	if status, n := count(3); status != http.StatusOK || n != float64(0) {
		t.Fatalf("empty count: %d %v", status, n)
	}
	tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 1, map[string]any{"name": "Private"})
	tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 1, map[string]any{"name": "Public", "visibility": "public"})
	tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 2, map[string]any{"name": "Public 2", "visibility": "public"})
	// Every user with goon2gether:use sees the same count (private lobbies never count)
	for _, user := range []uint{1, 3} {
		if status, n := count(user); status != http.StatusOK || n != float64(2) {
			t.Fatalf("count for %d: %d %v", user, status, n)
		}
	}
	// Without goon2gether:use
	if status, _ := count(5); status != http.StatusForbidden {
		t.Fatalf("count without goon2gether:use: %d", status)
	}
}

func TestTogetherEmotesRoutes(t *testing.T) {
	srv, _ := togetherServer(t)
	upload := func(user uint, name string, file []byte) (int, map[string]any) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("name", name)
		fw, _ := w.CreateFormFile("file", "e.png")
		_, _ = fw.Write(file)
		_ = w.Close()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/together/emotes", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("X-Test-User", strconv.Itoa(int(user)))
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		return res.StatusCode, out
	}
	var img bytes.Buffer
	_ = png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 16, 16)))

	if status, _ := upload(1, "pog", img.Bytes()); status != http.StatusForbidden {
		t.Fatalf("upload without emotes:manage: %d", status)
	}
	status, e := upload(4, "pog", img.Bytes())
	if status != http.StatusCreated || e["name"] != "pog" || !strings.HasPrefix(e["url"].(string), "/emotes/") {
		t.Fatalf("upload: %d %v", status, e)
	}
	if status, _ := upload(4, "POG", img.Bytes()); status != http.StatusConflict {
		t.Fatalf("duplicate name: %d", status)
	}
	if status, _ := upload(4, "svg", []byte("<svg/>")); status != http.StatusBadRequest {
		t.Fatalf("not an image: %d", status)
	}
	idStr := strconv.Itoa(int(e["id"].(float64)))
	if status, out := tgJSON(t, srv, http.MethodPut, "/api/v1/together/emotes/"+idStr, 4, map[string]any{"name": "Kappa"}); status != http.StatusOK || out["name"] != "Kappa" {
		t.Fatalf("rename: %d %v", status, out)
	}
	if _, list := tgJSON(t, srv, http.MethodGet, "/api/v1/together/emotes", 2, nil); len(list["emotes"].([]any)) != 1 {
		t.Fatalf("list %v", list)
	}
	if status, _ := tgJSON(t, srv, http.MethodDelete, "/api/v1/together/emotes/"+idStr, 4, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	if status, _ := tgJSON(t, srv, http.MethodDelete, "/api/v1/together/emotes/"+idStr, 4, nil); status != http.StatusNotFound {
		t.Fatalf("delete twice: %d", status)
	}
}

// readRaw reads the frames until one of type typ and gives them all, raw.
func readRaw(t *testing.T, conn *websocket.Conn, typ string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out []string
	for {
		_, b, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %q: %v", typ, err)
		}
		out = append(out, string(b))
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if m["t"] == typ {
			return out
		}
	}
}

// bobLeaks gives the frames that name bob (user 2).
func bobLeaks(frames []string) []string {
	var out []string
	for _, f := range frames {
		if strings.Contains(f, "bob") || strings.Contains(f, `"user_id":2,`) || strings.Contains(f, `"added_by":{"id":2`) {
			out = append(out, f)
		}
	}
	return out
}

func TestTogetherAnonymousRedaction(t *testing.T) {
	srv, _ := togetherServer(t)
	_, created := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 1, map[string]any{"name": "Open", "visibility": "public", "items": []map[string]any{{"scene_id": 1}}})
	id := created["id"].(string)

	// Private lobbies have no anonymous mode
	_, priv := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies", 1, map[string]any{"name": "Closed"})
	if status, _ := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies/"+priv["id"].(string)+"/join", 4, map[string]any{"anonymous": true}); status != http.StatusBadRequest {
		t.Fatalf("anonymous join of a private lobby: %d", status)
	}

	host, _, err := tgDial(t, srv, id, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	defer host.CloseNow()
	readUntil(t, host, "welcome")
	moderator, _, err := tgDial(t, srv, id, 4, "")
	if err != nil {
		t.Fatal(err)
	}
	defer moderator.CloseNow()
	readUntil(t, moderator, "welcome")

	// bob joins anonymously by connecting
	bob, _, err := tgDial(t, srv, id, 2, "", "anonymous=1")
	if err != nil {
		t.Fatal(err)
	}
	defer bob.CloseNow()
	you := readUntil(t, bob, "welcome")["you"].(map[string]any)
	if you["anonymous"] != true || you["anon_number"] != float64(1) {
		t.Fatalf("you: %v", you)
	}
	tgWrite(t, bob, map[string]any{"t": "chat", "text": "psst"})
	tgWrite(t, bob, map[string]any{"t": "reaction", "emote": "🔥"})
	tgWrite(t, bob, map[string]any{"t": "queue_add", "items": []map[string]any{{"scene_id": 2}}})

	frames := readRaw(t, host, "queue") // after the joined line, the chat and the reaction
	if l := bobLeaks(frames); len(l) > 0 {
		t.Fatalf("the host got bob's identity: %v", l)
	}
	joined := strings.Join(frames, "\n")
	for _, want := range []string{"Anonymous #1 joined", `"username":"Anonymous #1"`, `"t":"reaction","user_id":null`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the host did not get %q: %s", want, joined)
		}
	}
	modFrames := strings.Join(readRaw(t, moderator, "queue"), "\n")
	if !strings.Contains(modFrames, "Anonymous #1 (bob) joined") || !strings.Contains(modFrames, `"username":"bob","anonymous":true`) {
		t.Fatalf("the moderator does not see who it is: %s", modFrames)
	}

	// A late joiner's welcome is redacted too
	carol, _, err := tgDial(t, srv, id, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	defer carol.CloseNow()
	if l := bobLeaks(readRaw(t, carol, "welcome")); len(l) > 0 {
		t.Fatalf("the welcome leaks bob: %v", l)
	}

	// REST: bob's summary says anonymous, the others' list never names him
	if status, s := tgJSON(t, srv, http.MethodGet, "/api/v1/together/lobbies/"+id, 2, nil); status != http.StatusOK || s["anonymous"] != true {
		t.Fatalf("bob's summary: %d %v", status, s)
	}
	for _, user := range []uint{1, 3} {
		status, list := tgJSON(t, srv, http.MethodGet, "/api/v1/together/lobbies", user, nil)
		b, _ := json.Marshal(list)
		if status != http.StatusOK || strings.Contains(string(b), "bob") {
			t.Fatalf("lobby list of %d: %d %s", user, status, b)
		}
	}
	// Named members cannot turn anonymous
	if status, _ := tgJSON(t, srv, http.MethodPost, "/api/v1/together/lobbies/"+id+"/join", 3, map[string]any{"anonymous": true}); status != http.StatusConflict {
		t.Fatalf("a named member turned anonymous: %d", status)
	}
}

func TestOriginPatterns(t *testing.T) {

	got := OriginPatterns([]string{"http://localhost:3000", "https://sms.example.dev", "*", " ", "example.org"})
	want := []string{"localhost:3000", "sms.example.dev", "example.org"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", got, want)
	}
}
