package handler

import (
	"context"
	"errors"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/streaming"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Error codes of the transcode endpoints (the player shows a message for each).
const (
	codeTranscodeDisabled = "TRANSCODE_DISABLED"
	codeTranscodeLimit    = "TRANSCODE_LIMIT_REACHED"
	codeTranscodeFailed   = "TRANSCODE_FAILED"
)

// sceneDetailResponse is a scene with what the player needs to know to play it.
type sceneDetailResponse struct {
	response.SceneDetail
	Playback streaming.PlaybackInfo `json:"playback"`
}

// serveTranscode streams a scene converted on the fly to a fragmented MP4 (H.264 + AAC,
// or the copied streams when only the container is the problem), starting at ?start=
// seconds. ?mode=transcode forces re-encoding both streams (the player asks for it when
// the browser failed on a file considered playable). HEAD runs no conversion: it tells
// whether a slot is available (503 otherwise) and, in X-Transcode-Start, where the stream
// really starts (the keyframe before ?start= for a remux). The caller checks access.
func serveTranscode(c *gin.Context, tc *streaming.Transcoder, sm *streaming.Manager, sceneID uint) {
	if tc == nil || !tc.Enabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "On-the-fly transcoding is turned off",
			"code":  codeTranscodeDisabled,
		})
		return
	}

	// The stream can't be resumed in the middle: the player asks for a new start instead
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" && strings.TrimSpace(rangeHeader) != "bytes=0-" {
		c.Header("Accept-Ranges", "none")
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	start, ok := parseTranscodeStart(c.Query("start"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start time"})
		return
	}

	scene, err := sm.GetScene(sceneID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get scene"})
		return
	}
	fileInfo, err := os.Stat(scene.StoredPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Scene file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access scene file"})
		return
	}
	if scene.Duration > 0 && start >= float64(scene.Duration) {
		start = math.Max(0, float64(scene.Duration)-1)
	}

	plan := streaming.DecidePlayback(scene.StoredPath, scene.VideoCodec, scene.AudioCodec)
	if c.Query("mode") == string(streaming.PlaybackTranscode) || plan.Mode == streaming.PlaybackDirect {
		plan = streaming.FullTranscodePlan()
	}

	logger := tc.Logger()
	clientIP := c.ClientIP()
	sessionKey := transcodeSessionKey(clientIP, c.Query("sid"), sceneID)
	src := streaming.TranscodeSource{
		SceneID: sceneID,
		Path:    scene.StoredPath,
		Size:    fileInfo.Size(),
		ModTime: fileInfo.ModTime(),
		Plan:    plan,
	}

	if c.Request.Method == http.MethodHead {
		// Served from the cache, or from a conversion already running: no slot needed
		realStart, cached, shared := tc.CacheProbe(src, start)
		if !cached && !shared && !tc.Available(sessionKey) {
			c.Header("Retry-After", "10")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		switch {
		case cached:
			// The cached stream starts on the fragment holding the position
			start = realStart
		case plan.CopyVideo:
			// A stream copy starts at the keyframe before the requested position: tell
			// the player where, so that it asks for exactly that start and shows the
			// right time
			start = tc.KeyframeStart(c.Request.Context(), scene.StoredPath, start)
		default:
			start = tc.AlignStart(plan, start)
		}
		c.Header("Content-Type", "video/mp4")
		c.Header("Cache-Control", "no-store")
		c.Header("X-Transcode-Mode", string(plan.Mode))
		c.Header("X-Transcode-Start", strconv.FormatFloat(start, 'f', 3, 64))
		c.Status(http.StatusOK)
		return
	}

	// Counts as a stream like the original file (global and per-IP limits)
	if !sm.Limiter().Acquire(clientIP, sceneID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Too many concurrent streams",
			"code":  "STREAM_LIMIT_EXCEEDED",
		})
		return
	}
	defer sm.Limiter().Release(clientIP, sceneID)

	if tc.CacheEnabled() {
		serveCachedTranscode(c, tc, src, sessionKey, start)
		return
	}

	lease, err := tc.Acquire(c.Request.Context(), sessionKey)
	if err != nil {
		if errors.Is(err, streaming.ErrTranscodeLimit) || errors.Is(err, streaming.ErrTranscoderStopped) {
			transcodeLimitReached(c, tc)
			return
		}
		// Client gone while waiting for its previous stream to end
		return
	}
	defer lease.Release()

	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "no-store")
	c.Header("Accept-Ranges", "none")
	c.Header("X-Transcode-Mode", string(plan.Mode))
	c.Header("X-Transcode-Start", strconv.FormatFloat(start, 'f', 3, 64))

	logger.Info("On-the-fly transcode started",
		zap.Uint("scene_id", sceneID),
		zap.String("mode", string(plan.Mode)),
		zap.Bool("copy_video", plan.CopyVideo),
		zap.Bool("copy_audio", plan.CopyAudio),
		zap.Float64("start", start),
	)

	buf := sm.BufferPool().Get()
	defer sm.BufferPool().Put(buf)

	out := &streamWriter{w: c.Writer}
	err = tc.Stream(lease.Context(), out, tc.Args(scene.StoredPath, plan, start), buf)
	switch {
	case err == nil:
		logger.Info("On-the-fly transcode finished", zap.Uint("scene_id", sceneID), zap.Int64("bytes", out.written))
	case errors.Is(err, context.Canceled) || lease.Context().Err() != nil:
		logger.Debug("On-the-fly transcode stopped", zap.Uint("scene_id", sceneID), zap.Int64("bytes", out.written))
	default:
		logger.Warn("On-the-fly transcode failed", zap.Uint("scene_id", sceneID), zap.Int64("bytes", out.written), zap.Error(err))
		if out.written == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "The conversion of this scene failed",
				"code":  codeTranscodeFailed,
			})
		}
	}
}

// serveCachedTranscode streams the conversion through the cache: from what earlier
// conversions left, from a conversion already running for another viewer, or from a new
// one (which holds the slot, not the request).
func serveCachedTranscode(c *gin.Context, tc *streaming.Transcoder, src streaming.TranscodeSource, sessionKey string, start float64) {
	logger := tc.Logger()
	session, err := tc.BeginSession(c.Request.Context(), sessionKey)
	if err != nil {
		if errors.Is(err, streaming.ErrTranscoderStopped) {
			transcodeLimitReached(c, tc)
		}
		// Otherwise the client went away while its previous stream was ending
		return
	}
	defer session.Release()

	stream, err := tc.OpenCached(session.Context(), src, start)
	if err != nil {
		switch {
		case errors.Is(err, streaming.ErrTranscodeLimit) || errors.Is(err, streaming.ErrTranscoderStopped):
			transcodeLimitReached(c, tc)
		case session.Context().Err() != nil:
			// client gone, or replaced by its own next stream
		default:
			logger.Warn("On-the-fly transcode failed", zap.Uint("scene_id", src.SceneID), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "The conversion of this scene failed",
				"code":  codeTranscodeFailed,
			})
		}
		return
	}
	defer stream.Close()

	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "no-store")
	c.Header("Accept-Ranges", "none")
	c.Header("X-Transcode-Mode", string(src.Plan.Mode))
	c.Header("X-Transcode-Start", strconv.FormatFloat(stream.Start(), 'f', 3, 64))

	out := &streamWriter{w: c.Writer}
	_, err = stream.WriteTo(out)
	hits, joins := stream.Stats()
	fields := []zap.Field{
		zap.Uint("scene_id", src.SceneID),
		zap.Float64("start", stream.Start()),
		zap.Int64("bytes", out.written),
		zap.Int("cached_fragments", hits),
		zap.Int("joins", joins),
	}
	switch {
	case err == nil:
		logger.Debug("On-the-fly transcode stream ended", fields...)
	case session.Context().Err() != nil:
		logger.Debug("On-the-fly transcode stream stopped", fields...)
	default:
		logger.Warn("On-the-fly transcode stream broke off", append(fields, zap.Error(err))...)
	}
}

// transcodeLimitReached answers 503 when all conversion slots are in use.
func transcodeLimitReached(c *gin.Context, tc *streaming.Transcoder) {
	stats := tc.Stats()
	c.Header("Retry-After", "10")
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": "All on-the-fly conversion slots are in use (" + strconv.Itoa(stats.Active) + "/" +
			strconv.Itoa(stats.MaxConcurrent) + "), try again in a moment",
		"code": codeTranscodeLimit,
	})
}

// setDownloadDisposition makes the original file an attachment with its file name when
// ?download=1 (offered by the player when the browser can't play the file).
func setDownloadDisposition(c *gin.Context, filePath string) {
	if c.Query("download") != "1" {
		return
	}
	if disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(filePath)}); disposition != "" {
		c.Header("Content-Disposition", disposition)
	}
}

// parseTranscodeStart parses the start offset in seconds ("" means 0).
func parseTranscodeStart(raw string) (float64, bool) {
	if raw == "" {
		return 0, true
	}
	start, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(start) || math.IsInf(start, 0) || start < 0 {
		return 0, false
	}
	return start, true
}

// transcodeSessionKey identifies one player (sid, random per player instance) of one
// scene from one client, so that a seek replaces the previous conversion of that player.
func transcodeSessionKey(clientIP, sid string, sceneID uint) string {
	if len(sid) > 64 {
		sid = sid[:64]
	}
	return clientIP + "|" + sid + "|" + strconv.FormatUint(uint64(sceneID), 10)
}

// streamWriter flushes every chunk so the browser gets the fragments as they are produced,
// and counts the bytes sent (no byte sent yet: an error can still be answered in JSON).
type streamWriter struct {
	w       gin.ResponseWriter
	written int64
}

func (s *streamWriter) Write(p []byte) (int, error) {
	n, err := s.w.Write(p)
	s.written += int64(n)
	if err != nil {
		return n, err
	}
	s.w.Flush()
	return n, nil
}

// transcodeCacheClearResponse tells what clearing the cache removed.
type transcodeCacheClearResponse struct {
	Removed      int   `json:"removed"`
	RemovedBytes int64 `json:"removed_bytes"`
	streaming.TranscodeCacheStats
}

// GetTranscodeCache returns the size of the on-the-fly conversion cache (admin).
// GET /admin/transcode-cache
func (h *SceneHandler) GetTranscodeCache(c *gin.Context) {
	if h.Transcoder == nil {
		response.OK(c, streaming.TranscodeCacheStats{})
		return
	}
	response.OK(c, h.Transcoder.CacheStats())
}

// ClearTranscodeCache removes every cached conversion (admin); conversions being watched
// are removed when their viewers leave. DELETE /admin/transcode-cache
func (h *SceneHandler) ClearTranscodeCache(c *gin.Context) {
	resp := transcodeCacheClearResponse{}
	if h.Transcoder != nil {
		resp.Removed, resp.RemovedBytes = h.Transcoder.ClearCache()
		resp.TranscodeCacheStats = h.Transcoder.CacheStats()
	}
	response.OK(c, resp)
}
