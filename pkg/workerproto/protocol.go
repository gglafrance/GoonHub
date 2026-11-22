// Package workerproto defines the wire protocol between a GoonHub server and its remote
// workers (goonhub-worker): the JSON types exchanged on every endpoint, the protocol
// constants (API version, phases, input and connection modes, HTTP headers, paths) and small
// helpers (artifact name validation, sha256 of a file, fingerprint encoding).
//
// The full design, the endpoint semantics, the status codes and the failure handling are in
// docs/REMOTE_WORKERS.md. This package has no dependency on the server (no internal/
// import) and builds on Windows, macOS and Linux.
package workerproto

import (
	"net/url"
	"strings"
)

// APIVersion is the version of the worker protocol. The server refuses a register whose
// api_version it does not support (426 Upgrade Required, code "unsupported_api_version").
const APIVersion = 1

// Phase is a processing phase a remote worker can run.
type Phase string

const (
	PhaseSprites            Phase = "sprites"
	PhaseAnimatedThumbnails Phase = "animated_thumbnails"
	PhaseFingerprint        Phase = "fingerprint"
)

// RemotePhases lists the phases that can be processed remotely. metadata and thumbnail
// always stay on the server.
var RemotePhases = []Phase{PhaseSprites, PhaseAnimatedThumbnails, PhaseFingerprint}

// Valid reports whether p is a phase that can be processed remotely.
func (p Phase) Valid() bool {
	switch p {
	case PhaseSprites, PhaseAnimatedThumbnails, PhaseFingerprint:
		return true
	}
	return false
}

// InputMode is how a worker reads the source video.
type InputMode string

const (
	// InputDirect: the worker sees the server's storage itself (read-only mount) and opens
	// the file through a path mapping.
	InputDirect InputMode = "direct"
	// InputIndirect: the worker reads the video from the server over HTTP (signed URL with
	// Range support), either streamed by ffmpeg or downloaded to a local cache first.
	InputIndirect InputMode = "indirect"
)

// Valid reports whether m is a known input mode.
func (m InputMode) Valid() bool { return m == InputDirect || m == InputIndirect }

// ConnectionMode is who initiates the job assignment.
type ConnectionMode string

const (
	// ConnPull: the worker calls the server to claim jobs (machines behind NAT).
	ConnPull ConnectionMode = "pull"
	// ConnPush: the worker runs an HTTP endpoint and the server assigns jobs to it.
	ConnPush ConnectionMode = "push"
)

// Valid reports whether m is a known connection mode.
func (m ConnectionMode) Valid() bool { return m == ConnPull || m == ConnPush }

// HTTP headers used by the protocol.
const (
	// HeaderAuthorization carries "Bearer <token>": the worker token on worker -> server
	// calls, the push token on server -> worker calls.
	HeaderAuthorization = "Authorization"
	// HeaderAPIVersion carries APIVersion on every request, both directions.
	HeaderAPIVersion = "X-Goonhub-Worker-Api"
	// HeaderLeaseGeneration carries the lease generation on artifact uploads.
	HeaderLeaseGeneration = "X-Lease-Generation"
	// HeaderUploadOffset is the byte offset of the body of an artifact PUT in the file.
	HeaderUploadOffset = "Upload-Offset"
	// HeaderArtifactSize is the total size in bytes of the artifact being uploaded.
	HeaderArtifactSize = "X-Artifact-Size"
	// HeaderArtifactSHA256 is the lowercase hex sha256 of the whole artifact.
	HeaderArtifactSHA256 = "X-Artifact-Sha256"
)

// Server endpoints (worker -> server), relative to the server base URL. {job_id} and
// {name} are replaced with JobPath / ArtifactPath.
const (
	PathRegister    = "/api/v1/workers/register"
	PathClaim       = "/api/v1/workers/claim"
	PathHeartbeat   = "/api/v1/workers/heartbeat"
	PathReleaseAll  = "/api/v1/workers/release-all"
	PathJobComplete = "/api/v1/workers/jobs/{job_id}/complete"
	PathJobFail     = "/api/v1/workers/jobs/{job_id}/fail"
	PathJobRelease  = "/api/v1/workers/jobs/{job_id}/release"
	PathJobMediaURL = "/api/v1/workers/jobs/{job_id}/media-url"
	PathJobArtifact = "/api/v1/workers/jobs/{job_id}/artifacts/{name}"
	// PathMedia is the signed media URL (no bearer token: the signature authorizes it).
	PathMedia = "/api/v1/workers/media/{job_id}"
)

// Worker endpoints (server -> worker) in push mode, relative to the worker push URL.
const (
	PathPushStatus = "/v1/status"
	PathPushAssign = "/v1/assign"
	PathPushCancel = "/v1/cancel"
)

// JobPath fills {job_id} in one of the Path* templates (the id is path-escaped).
func JobPath(template, jobID string) string {
	return strings.ReplaceAll(template, "{job_id}", url.PathEscape(jobID))
}

// ArtifactPath returns the upload path of an artifact of a job.
func ArtifactPath(jobID, name string) string {
	return strings.ReplaceAll(JobPath(PathJobArtifact, jobID), "{name}", url.PathEscape(name))
}

// Error codes returned in the "code" field of an error body ({"error": "...", "code": "..."}).
const (
	CodeUnauthorized          = "unauthorized"            // 401: missing/unknown/revoked token
	CodeWorkerDisabled        = "worker_disabled"         // 403: the admin disabled this worker
	CodeUnsupportedAPIVersion = "unsupported_api_version" // 426
	CodeInvalidRequest        = "invalid_request"         // 400
	CodeUnknownSession        = "unknown_session"         // 409: re-register
	CodeJobNotFound           = "job_not_found"           // 404
	CodeLeaseLost             = "lease_lost"              // 409: stale generation, job reassigned/expired/released
	CodeJobCancelled          = "job_cancelled"           // 409: the job was cancelled on the server
	CodeSceneGone             = "scene_gone"              // 410: the scene was deleted
	CodeOffsetMismatch        = "offset_mismatch"         // 409: Upload-Offset differs from the bytes received
	CodeSHA256Mismatch        = "sha256_mismatch"         // 422: the uploaded file does not match its sha256
	CodeArtifactTooLarge      = "artifact_too_large"      // 413
	CodeArtifactMissing       = "artifact_missing"        // 422: complete references artifacts not (fully) uploaded
	CodeInvalidArtifactName   = "invalid_artifact_name"   // 400
	CodeInvalidResult         = "invalid_result"          // 422: the result does not match the assignment
	CodeServerDiskFull        = "server_disk_full"        // 507: retry later with backoff
	CodeURLExpired            = "url_expired"             // 403 on the media URL: ask a new one
	CodeBusy                  = "busy"                    // 409 on push assign: no free slot
	CodeCommitFailed          = "commit_failed"           // 503: commit failed (DB down...), retry complete
)

// ErrorResponse is the body of every non-2xx response, the same shape as the rest of the
// GoonHub API.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}
