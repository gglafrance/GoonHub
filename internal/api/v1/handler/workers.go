package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"goonhub/internal/api/v1/response"
	"goonhub/internal/data"
	"goonhub/internal/workers"
	"goonhub/pkg/workerproto"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// WorkerHandler serves the remote worker protocol (/api/v1/workers/*, docs/REMOTE_WORKERS.md)
// and the admin API of the workers (/api/v1/admin/workers).
type WorkerHandler struct {
	svc    *workers.Service
	logger *zap.Logger
}

// NewWorkerHandler creates a WorkerHandler.
func NewWorkerHandler(svc *workers.Service, logger *zap.Logger) *WorkerHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &WorkerHandler{svc: svc, logger: logger}
}

const workerContextKey = "remote_worker"

// smallBodyLimit bounds the JSON bodies of the worker calls other than complete.
const smallBodyLimit = 4 << 20

func (h *WorkerHandler) fail(c *gin.Context, err error) {
	e := workers.AsError(err)
	if e.Status >= 500 && e.Status != http.StatusInsufficientStorage {
		h.logger.Error("Worker API error", zap.String("path", c.FullPath()), zap.Error(err))
	}
	c.AbortWithStatusJSON(e.Status, workerproto.ErrorResponse{Error: e.Message, Code: e.Code, Details: e.Details})
}

// gate refuses the worker API when it is disabled or from a source address outside
// workers.allowed_cidrs, and an unsupported API version header.
func (h *WorkerHandler) gate(c *gin.Context) bool {
	cfg := h.svc.Settings()
	if !cfg.Enabled {
		c.AbortWithStatusJSON(http.StatusNotFound, workerproto.ErrorResponse{Error: "the worker API is disabled", Code: workerproto.CodeWorkersDisabled})
		return false
	}
	if !cfg.AddressAllowed(c.ClientIP()) {
		c.AbortWithStatusJSON(http.StatusForbidden, workerproto.ErrorResponse{Error: "source address not allowed", Code: workerproto.CodeAddressNotAllowed})
		return false
	}
	if v := c.GetHeader(workerproto.HeaderAPIVersion); v != "" && v != strconv.Itoa(workerproto.APIVersion) {
		c.AbortWithStatusJSON(http.StatusUpgradeRequired, workerproto.ErrorResponse{
			Error: fmt.Sprintf("unsupported worker API version %s", v), Code: workerproto.CodeUnsupportedAPIVersion})
		return false
	}
	return true
}

// Gate is the middleware of the signed media URL (no bearer token).
func (h *WorkerHandler) Gate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.gate(c) {
			c.Next()
		}
	}
}

// Auth is the middleware of the worker calls: gate + worker bearer token.
func (h *WorkerHandler) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.gate(c) {
			return
		}
		token, ok := strings.CutPrefix(c.GetHeader(workerproto.HeaderAuthorization), "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, workerproto.ErrorResponse{Error: "missing worker token", Code: workerproto.CodeUnauthorized})
			return
		}
		w, err := h.svc.Authenticate(strings.TrimSpace(token))
		if err != nil {
			h.fail(c, err)
			return
		}
		c.Set(workerContextKey, w)
		c.Next()
	}
}

func currentWorker(c *gin.Context) *data.Worker {
	v, _ := c.Get(workerContextKey)
	w, _ := v.(*data.Worker)
	return w
}

func (h *WorkerHandler) bind(c *gin.Context, limit int64, dst any) bool {
	body := http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, workerproto.ErrorResponse{Error: "request body too large", Code: workerproto.CodeInvalidRequest})
			return false
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, workerproto.ErrorResponse{Error: "invalid request body: " + err.Error(), Code: workerproto.CodeInvalidRequest})
		return false
	}
	return true
}

// serverURL is the base URL the worker used to reach the server.
func serverURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// noReadDeadline lifts the server read timeout for a long request body (artifact uploads).
func noReadDeadline(c *gin.Context) {
	// Best effort: without support the server read timeout still applies.
	_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Time{})
}

// Register: POST /api/v1/workers/register
func (h *WorkerHandler) Register(c *gin.Context) {
	var req workerproto.RegisterRequest
	if !h.bind(c, smallBodyLimit, &req) {
		return
	}
	resp, err := h.svc.Register(currentWorker(c), req, serverURL(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Claim: POST /api/v1/workers/claim (long-poll)
func (h *WorkerHandler) Claim(c *gin.Context) {
	var req workerproto.ClaimRequest
	if !h.bind(c, smallBodyLimit, &req) {
		return
	}
	resp, err := h.svc.Claim(c.Request.Context(), currentWorker(c), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Heartbeat: POST /api/v1/workers/heartbeat
func (h *WorkerHandler) Heartbeat(c *gin.Context) {
	var req workerproto.HeartbeatRequest
	if !h.bind(c, smallBodyLimit, &req) {
		return
	}
	resp, err := h.svc.Heartbeat(currentWorker(c), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ReleaseAll: POST /api/v1/workers/release-all
func (h *WorkerHandler) ReleaseAll(c *gin.Context) {
	var req workerproto.ReleaseAllRequest
	if !h.bind(c, smallBodyLimit, &req) {
		return
	}
	resp, err := h.svc.ReleaseAll(currentWorker(c), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Release: POST /api/v1/workers/jobs/:job_id/release
func (h *WorkerHandler) Release(c *gin.Context) {
	var req workerproto.ReleaseRequest
	if !h.bind(c, smallBodyLimit, &req) {
		return
	}
	resp, err := h.svc.Release(currentWorker(c), c.Param("job_id"), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Fail: POST /api/v1/workers/jobs/:job_id/fail
func (h *WorkerHandler) Fail(c *gin.Context) {
	var req workerproto.FailRequest
	if !h.bind(c, smallBodyLimit, &req) {
		return
	}
	if err := h.svc.Fail(currentWorker(c), c.Param("job_id"), req); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

// MediaURL: POST /api/v1/workers/jobs/:job_id/media-url
func (h *WorkerHandler) MediaURL(c *gin.Context) {
	var req workerproto.MediaURLRequest
	if !h.bind(c, smallBodyLimit, &req) {
		return
	}
	resp, err := h.svc.MediaURL(currentWorker(c), c.Param("job_id"), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Complete: POST /api/v1/workers/jobs/:job_id/complete
func (h *WorkerHandler) Complete(c *gin.Context) {
	var req workerproto.CompleteRequest
	if !h.bind(c, h.svc.Settings().MaxCompleteBodyBytes, &req) {
		return
	}
	resp, err := h.svc.Complete(currentWorker(c), c.Param("job_id"), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *WorkerHandler) leaseCheck(c *gin.Context, jobID string, generation int64) bool {
	w := currentWorker(c)
	ok, err := h.svc.Leases.CheckLease(jobID, generation, w.ID)
	if err != nil {
		h.fail(c, err)
		return false
	}
	if !ok {
		h.fail(c, h.svc.LeaseError(jobID, generation, w.ID))
		return false
	}
	return true
}

// UploadArtifact: PUT /api/v1/workers/jobs/:job_id/artifacts/:name
func (h *WorkerHandler) UploadArtifact(c *gin.Context) {
	jobID, name := c.Param("job_id"), c.Param("name")
	gen, err1 := strconv.ParseInt(c.GetHeader(workerproto.HeaderLeaseGeneration), 10, 64)
	offset, err2 := strconv.ParseInt(c.GetHeader(workerproto.HeaderUploadOffset), 10, 64)
	size, err3 := strconv.ParseInt(c.GetHeader(workerproto.HeaderArtifactSize), 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		h.fail(c, &workers.Error{Status: http.StatusBadRequest, Code: workerproto.CodeInvalidRequest,
			Message: fmt.Sprintf("%s, %s and %s are required", workerproto.HeaderLeaseGeneration, workerproto.HeaderUploadOffset, workerproto.HeaderArtifactSize)})
		return
	}
	if !h.leaseCheck(c, jobID, gen) {
		return
	}
	noReadDeadline(c)
	st, err := h.svc.Staging.Put(jobID, gen, name, offset, size, c.GetHeader(workerproto.HeaderArtifactSHA256), c.Request.Body)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// ArtifactStatus: GET /api/v1/workers/jobs/:job_id/artifacts/:name?generation=N
func (h *WorkerHandler) ArtifactStatus(c *gin.Context) {
	jobID, name := c.Param("job_id"), c.Param("name")
	gen, err := strconv.ParseInt(c.Query("generation"), 10, 64)
	if err != nil {
		h.fail(c, &workers.Error{Status: http.StatusBadRequest, Code: workerproto.CodeInvalidRequest, Message: "generation is required"})
		return
	}
	if !h.leaseCheck(c, jobID, gen) {
		return
	}
	st, err := h.svc.Staging.Status(jobID, gen, name)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, st)
}

// Media: GET|HEAD /api/v1/workers/media/:job_id (signed URL, HTTP Range). The source is
// opened read-only; this path has no write method.
func (h *WorkerHandler) Media(c *gin.Context) {
	jobID := c.Param("job_id")
	path, err := h.svc.MediaFile(jobID, c.Request.URL.Query())
	if err != nil {
		h.fail(c, err)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		h.fail(c, &workers.Error{Status: http.StatusNotFound, Code: workerproto.CodeJobNotFound, Message: "source file not readable"})
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		h.fail(c, &workers.Error{Status: http.StatusNotFound, Code: workerproto.CodeJobNotFound, Message: "source file not readable"})
		return
	}
	c.Header("ETag", fmt.Sprintf(`"%x-%x"`, st.Size(), st.ModTime().UnixNano()))
	c.Header("Cache-Control", "no-store")
	http.ServeContent(c.Writer, c.Request, filepath.Base(path), st.ModTime(), io.ReadSeeker(f))
}

// ---------------------------------------------------------------------------------------
// Admin API
// ---------------------------------------------------------------------------------------

type workersListResponse struct {
	Enabled  bool                 `json:"enabled"`
	Workers  []workers.WorkerView `json:"workers"`
	Routing  workers.RoutingView  `json:"routing"`
	Settings workerSettingsView   `json:"settings"`
}

type workerSettingsView struct {
	HeartbeatIntervalSec int    `json:"heartbeat_interval_sec"`
	LeaseDurationSec     int    `json:"lease_duration_sec"`
	OfflineAfterSec      int    `json:"offline_after_sec"`
	PublicURL            string `json:"public_url"`
}

func (h *WorkerHandler) settingsView() workerSettingsView {
	s := h.svc.Settings()
	return workerSettingsView{
		HeartbeatIntervalSec: int(s.HeartbeatInterval / time.Second),
		LeaseDurationSec:     int(s.LeaseDuration / time.Second),
		OfflineAfterSec:      int(s.OfflineAfter / time.Second),
		PublicURL:            s.PublicURL,
	}
}

// AdminList: GET /api/v1/admin/workers
func (h *WorkerHandler) AdminList(c *gin.Context) {
	list, err := h.svc.ListWorkers()
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, workersListResponse{
		Enabled:  h.svc.Settings().Enabled,
		Workers:  list,
		Routing:  h.svc.Router.View(),
		Settings: h.settingsView(),
	})
}

// AdminCreate: POST /api/v1/admin/workers -> the worker and its token (shown once)
func (h *WorkerHandler) AdminCreate(c *gin.Context) {
	var in workers.WorkerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	created, err := h.svc.CreateWorker(in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, created)
}

func workerID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequest(c, "Invalid worker ID")
		return 0, false
	}
	return uint(id), true
}

// AdminUpdate: PATCH /api/v1/admin/workers/:id
func (h *WorkerHandler) AdminUpdate(c *gin.Context) {
	id, ok := workerID(c)
	if !ok {
		return
	}
	var in workers.WorkerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	v, err := h.svc.UpdateWorker(id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}

// AdminDelete: DELETE /api/v1/admin/workers/:id
func (h *WorkerHandler) AdminDelete(c *gin.Context) {
	id, ok := workerID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteWorker(id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// AdminRotateToken: POST /api/v1/admin/workers/:id/token -> the new token (shown once)
func (h *WorkerHandler) AdminRotateToken(c *gin.Context) {
	id, ok := workerID(c)
	if !ok {
		return
	}
	created, err := h.svc.RotateToken(id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, created)
}

// AdminGetRouting: GET /api/v1/admin/worker-routing
func (h *WorkerHandler) AdminGetRouting(c *gin.Context) {
	response.OK(c, h.svc.Router.View())
}

// AdminUpdateRouting: PUT /api/v1/admin/worker-routing {"routing": {"sprites": "remote", ...}}
func (h *WorkerHandler) AdminUpdateRouting(c *gin.Context) {
	var req struct {
		Routing map[string]string `json:"routing"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	v, err := h.svc.Router.Update(req.Routing)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, v)
}
