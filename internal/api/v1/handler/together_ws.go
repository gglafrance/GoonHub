package handler

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"goonhub/internal/apperrors"
	"goonhub/internal/together"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Socket: GET /together/lobbies/:id/ws?v=1 (WebSocket, docs/GOON2GETHER.md). The session is
// checked by the auth middleware before the upgrade. Refusals (unknown lobby, ban, full)
// are sent as a close code after the upgrade, since browsers cannot read an HTTP error of a
// WebSocket handshake.
func (h *TogetherHandler) Socket(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := lobbyID(c)
	if !ok {
		return
	}
	if v := c.Query("v"); v != "" && v != strconv.Itoa(together.ProtocolVersion) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported protocol version", "code": "BAD_VERSION"})
		return
	}

	cfg := h.hub.Config()
	// The long-lived connection must not be cut by the server read timeout
	_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Time{})
	conn, err := websocket.Accept(newUpgradeWriter(c.Writer), c.Request, &websocket.AcceptOptions{
		OriginPatterns: h.originPatterns,
	})
	if err != nil {
		// Accept already answered the request (bad handshake, origin refused)
		h.logger.Debug("WebSocket upgrade refused", zap.Error(err))
		return
	}
	defer conn.CloseNow()
	if cfg.MaxMessageBytes > 0 {
		conn.SetReadLimit(cfg.MaxMessageBytes)
	}

	client, err := h.hub.Connect(a, id)
	if err != nil {
		code, reason := closeFor(err)
		_ = conn.Close(code, reason)
		return
	}
	defer h.hub.Disconnect(client)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer cancel()
		h.writeLoop(ctx, conn, client, cfg.PingInterval, cfg.WriteTimeout)
	}()

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			break
		}
		if typ != websocket.MessageText {
			continue
		}
		h.hub.Receive(client, data)
	}
	cancel()
	<-writerDone
}

// upgradeWriter lets the WebSocket library upgrade through gin: gin refuses to hijack a
// response whose header was written, so the 101 goes straight to the net/http writer and the
// hijack through gin (which then knows the response is taken). Every other answer (refused
// handshake) goes through gin as usual.
type upgradeWriter struct {
	gin gin.ResponseWriter
	raw http.ResponseWriter
}

func newUpgradeWriter(w gin.ResponseWriter) http.ResponseWriter {
	u := &upgradeWriter{gin: w, raw: w}
	if uw, ok := w.(interface{ Unwrap() http.ResponseWriter }); ok {
		u.raw = uw.Unwrap()
	}
	return u
}

func (u *upgradeWriter) Header() http.Header         { return u.gin.Header() }
func (u *upgradeWriter) Write(b []byte) (int, error) { return u.gin.Write(b) }

func (u *upgradeWriter) WriteHeader(code int) {
	if code == http.StatusSwitchingProtocols {
		u.raw.WriteHeader(code)
		return
	}
	u.gin.WriteHeader(code)
}

func (u *upgradeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) { return u.gin.Hijack() }

// closeFor maps a refused connection to a close code.
func closeFor(err error) (websocket.StatusCode, string) {
	switch {
	case errors.Is(err, together.ErrLobbyFull):
		return together.CloseFull, "lobby full"
	case apperrors.IsForbidden(err):
		return together.CloseForbidden, "forbidden"
	case apperrors.IsNotFound(err):
		return together.CloseClosed, "lobby not found"
	default:
		return websocket.StatusInternalError, "internal error"
	}
}

// writeLoop writes the client's messages and pings the peer. When the hub closes the client,
// the messages still queued (a bye) are written before the close frame.
func (h *TogetherHandler) writeLoop(ctx context.Context, conn *websocket.Conn, client *together.Client, pingInterval, writeTimeout time.Duration) {
	if pingInterval <= 0 {
		pingInterval = 25 * time.Second
	}
	if writeTimeout <= 0 {
		writeTimeout = 10 * time.Second
	}
	write := func(b []byte) error {
		wctx, cancel := context.WithTimeout(ctx, writeTimeout)
		defer cancel()
		return conn.Write(wctx, websocket.MessageText, b)
	}
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.Done():
			code, reason := client.CloseStatus()
		drain:
			for code != together.ClosePolicy { // a slow client is not drained
				select {
				case b := <-client.Send():
					if write(b) != nil {
						return
					}
				default:
					break drain
				}
			}
			_ = conn.Close(websocket.StatusCode(code), reason)
			return
		case b := <-client.Send():
			if err := write(b); err != nil {
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
