package workerproto

import "time"

// ---------------------------------------------------------------------------------------
// Register: POST /api/v1/workers/register
// ---------------------------------------------------------------------------------------

// RegisterRequest is sent by the worker at start-up (and again after an unknown_session
// error). The worker identity comes from its token; the name is only a display label.
type RegisterRequest struct {
	APIVersion     int            `json:"api_version"`
	Name           string         `json:"name"`     // display name (defaults to the hostname)
	Version        string         `json:"version"`  // worker build version
	Hostname       string         `json:"hostname"` // machine hostname
	OS             string         `json:"os"`       // runtime.GOOS
	Arch           string         `json:"arch"`     // runtime.GOARCH
	ConnectionMode ConnectionMode `json:"connection_mode"`
	// PushURL is the base URL of the worker's push endpoint (push mode only), e.g.
	// "http://worker-1.tailnet.ts.net:9090".
	PushURL string `json:"push_url,omitempty"`
	// PushToken is a random secret the worker generated; the server sends it as
	// "Authorization: Bearer <push_token>" on every push call (push mode only).
	PushToken    string       `json:"push_token,omitempty"`
	Capabilities Capabilities `json:"capabilities"`
	// PathMappings lists the server path prefixes the worker can open directly (direct
	// input mode). Informational: the worker decides per job (see InputSpec).
	PathMappings []PathMapping `json:"path_mappings,omitempty"`
	// Resume lists the jobs whose state the worker reloaded from its outbox after a restart.
	// The server answers, per job, whether the lease is still valid (RegisterResponse.Resumed)
	// and releases every other lease still held by this worker.
	Resume []JobLeaseRef `json:"resume,omitempty"`
}

// Capabilities describes what a worker can run.
type Capabilities struct {
	Phases []Phase `json:"phases"`
	// Slots is the maximum number of concurrent jobs per phase.
	Slots      map[Phase]int `json:"slots"`
	InputModes []InputMode   `json:"input_modes"`
	// Accel is the hardware acceleration profile name in use ("none", "nvidia", "amd",
	// "intel", "videotoolbox", "vaapi"...).
	Accel string `json:"accel"`
	// HWAccels and Encoders are what the worker's ffmpeg reports (informational).
	HWAccels []string `json:"hwaccels,omitempty"`
	Encoders []string `json:"encoders,omitempty"`
	// AudioFingerprint is true when fpcalc (chromaprint) is available: without it the
	// server must not assign fingerprint jobs of scenes that have an audio track.
	AudioFingerprint bool   `json:"audio_fingerprint"`
	FFmpegVersion    string `json:"ffmpeg_version,omitempty"`
}

// PathMapping maps a server path prefix to the worker's local (read-only) mount of it.
type PathMapping struct {
	ServerPrefix string `json:"server_prefix"` // e.g. "/media/stash"
	LocalPrefix  string `json:"local_prefix"`  // e.g. "Z:\\stash" or "/Volumes/stash"
}

// JobLeaseRef identifies one lease: a job and the generation the worker holds.
type JobLeaseRef struct {
	JobID      string `json:"job_id"`
	Generation int64  `json:"generation"`
}

// RegisterResponse carries the session and the server timings the worker must follow.
type RegisterResponse struct {
	WorkerID   string    `json:"worker_id"`
	SessionID  string    `json:"session_id"`
	APIVersion int       `json:"api_version"`
	ServerTime time.Time `json:"server_time"`
	// HeartbeatIntervalSec: send a heartbeat at least this often while holding leases (and
	// at least every 2 intervals when idle, as a liveness signal).
	HeartbeatIntervalSec int `json:"heartbeat_interval_sec"`
	// LeaseDurationSec: a lease not renewed for this long expires.
	LeaseDurationSec int `json:"lease_duration_sec"`
	// ClaimMaxWaitSec: the longest long-poll the server accepts on claim.
	ClaimMaxWaitSec int `json:"claim_max_wait_sec"`
	// MaxArtifactBytes: the largest single artifact the server accepts.
	MaxArtifactBytes int64 `json:"max_artifact_bytes"`
	// UploadChunkBytes: recommended maximum body size of one artifact PUT.
	UploadChunkBytes int64 `json:"upload_chunk_bytes"`
	// Resumed answers RegisterRequest.Resume, one entry per job.
	Resumed []JobLeaseStatus `json:"resumed,omitempty"`
}

// ---------------------------------------------------------------------------------------
// Claim (pull): POST /api/v1/workers/claim
// ---------------------------------------------------------------------------------------

// ClaimRequest asks for jobs. The server returns at most Slots[phase] assignments per phase.
type ClaimRequest struct {
	SessionID string        `json:"session_id"`
	Slots     map[Phase]int `json:"slots"` // free slots per phase
	// WaitSec makes the call a long-poll: the server holds the request up to WaitSec seconds
	// (capped at ClaimMaxWaitSec) until a job is available. 0 = answer at once.
	WaitSec int `json:"wait_sec"`
}

// ClaimResponse lists the new assignments (empty when nothing is available).
type ClaimResponse struct {
	Assignments []Assignment `json:"assignments"`
}

// ---------------------------------------------------------------------------------------
// Assignment (claim response and push assign body)
// ---------------------------------------------------------------------------------------

// Assignment is one leased job, everything the worker needs to compute it.
type Assignment struct {
	JobID   string `json:"job_id"`
	Phase   Phase  `json:"phase"`
	SceneID uint   `json:"scene_id"`
	// Attempt is the job retry count on the server (0 = first attempt).
	Attempt    int         `json:"attempt"`
	Lease      Lease       `json:"lease"`
	Input      InputSpec   `json:"input"`
	Params     PhaseParams `json:"params"`
	AssignedAt time.Time   `json:"assigned_at"`
}

// Lease is the fencing token of an assignment. Every lease-guarded call carries Generation;
// a call with another generation than the job's current one is refused (lease_lost).
type Lease struct {
	Generation  int64     `json:"generation"`
	ExpiresAt   time.Time `json:"expires_at"`
	DurationSec int       `json:"duration_sec"`
}

// InputSpec describes the source video. The worker opens ServerPath through one of its
// path mappings when it can (direct mode, after checking the size), otherwise it uses URL
// (indirect mode). URL is always set.
type InputSpec struct {
	ServerPath    string `json:"server_path"` // scene stored path on the server
	StoragePathID *uint  `json:"storage_path_id,omitempty"`
	// RelativePath is ServerPath relative to the storage path root (forward slashes).
	RelativePath string    `json:"relative_path,omitempty"`
	Size         int64     `json:"size"`     // bytes
	FileName     string    `json:"filename"` // base name, keeps the extension for local caches
	URL          string    `json:"url"`      // signed media URL, GET/HEAD with Range
	URLExpiresAt time.Time `json:"url_expires_at"`
	Video        VideoInfo `json:"video"`
}

// VideoInfo are the probed facts the server already knows about the video.
type VideoInfo struct {
	DurationSec int     `json:"duration_sec"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FrameRate   float64 `json:"frame_rate,omitempty"`
	VideoCodec  string  `json:"video_codec,omitempty"`
	AudioCodec  string  `json:"audio_codec,omitempty"` // "" = no audio track
}

// HasAudio reports whether the video has an audio track.
func (v VideoInfo) HasAudio() bool { return v.AudioCodec != "" }

// ---------------------------------------------------------------------------------------
// Heartbeat: POST /api/v1/workers/heartbeat
// ---------------------------------------------------------------------------------------

// HeartbeatRequest renews every lease the worker holds in one call. It is also sent when
// the worker holds no job (Jobs empty), as a liveness signal.
type HeartbeatRequest struct {
	SessionID     string         `json:"session_id"`
	Jobs          []JobHeartbeat `json:"jobs"`
	FreeDiskBytes int64          `json:"free_disk_bytes,omitempty"`
	Draining      bool           `json:"draining,omitempty"` // the worker is shutting down
}

// JobHeartbeat is the state of one job on the worker.
type JobHeartbeat struct {
	JobID      string    `json:"job_id"`
	Generation int64     `json:"generation"`
	Progress   int       `json:"progress"` // 0-100
	Stage      JobStage  `json:"stage"`
	InputMode  InputMode `json:"input_mode,omitempty"`
}

// JobStage is where a job is on the worker.
type JobStage string

const (
	StageDownloading JobStage = "downloading" // fetching the source to the local cache
	StageComputing   JobStage = "computing"
	StageUploading   JobStage = "uploading"
	StageCompleting  JobStage = "completing" // all artifacts uploaded, complete in flight
)

// HeartbeatResponse answers every job of the request.
type HeartbeatResponse struct {
	ServerTime time.Time        `json:"server_time"`
	Jobs       []JobLeaseStatus `json:"jobs"`
	// Drain asks the worker to stop claiming new jobs (admin action); running jobs finish.
	Drain bool `json:"drain,omitempty"`
}

// LeaseStatus is the server's view of one lease.
type LeaseStatus string

const (
	LeaseOK        LeaseStatus = "ok"         // renewed: keep working
	LeaseLost      LeaseStatus = "lease_lost" // stale generation: stop and discard
	LeaseCancelled LeaseStatus = "cancelled"  // cancelled on the server: stop and discard
	LeaseCommitted LeaseStatus = "committed"  // already completed by this generation: drop the outbox entry
)

// JobLeaseStatus is the answer for one job (heartbeat and register resume).
type JobLeaseStatus struct {
	JobID          string      `json:"job_id"`
	Generation     int64       `json:"generation"`
	Status         LeaseStatus `json:"status"`
	LeaseExpiresAt *time.Time  `json:"lease_expires_at,omitempty"` // set when Status is ok
}

// ---------------------------------------------------------------------------------------
// Artifacts: PUT/GET /api/v1/workers/jobs/{job_id}/artifacts/{name}
// ---------------------------------------------------------------------------------------

// ArtifactRef identifies one output file of a job, as declared in complete.
type ArtifactRef struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"` // lowercase hex
}

// UploadStatus is the answer of an artifact PUT and GET: how much of the artifact the
// server holds for the given generation.
type UploadStatus struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`     // declared total size
	Received int64  `json:"received"` // bytes stored so far; the next PUT starts here
	SHA256   string `json:"sha256"`   // declared sha256
	Complete bool   `json:"complete"` // Received == Size and the sha256 matched
}

// ---------------------------------------------------------------------------------------
// Complete / fail / release / media URL
// ---------------------------------------------------------------------------------------

// CompleteRequest finishes a job: the server verifies every artifact, then commits.
type CompleteRequest struct {
	Generation int64         `json:"generation"`
	Phase      Phase         `json:"phase"`
	Result     PhaseResult   `json:"result"`
	Artifacts  []ArtifactRef `json:"artifacts"`
	Stats      JobStats      `json:"stats"`
}

// JobStats are informational numbers recorded with the job.
type JobStats struct {
	InputMode       InputMode `json:"input_mode"`
	Accel           string    `json:"accel"`
	ComputeMillis   int64     `json:"compute_ms"`
	DownloadMillis  int64     `json:"download_ms,omitempty"`
	UploadMillis    int64     `json:"upload_ms,omitempty"`
	BytesDownloaded int64     `json:"bytes_downloaded,omitempty"`
}

// CompleteStatus is the outcome of a complete.
type CompleteStatus string

const (
	CompleteCommitted        CompleteStatus = "committed"         // committed by this call
	CompleteAlreadyCommitted CompleteStatus = "already_committed" // a previous call with this generation committed
)

// CompleteResponse answers a successful complete.
type CompleteResponse struct {
	Status CompleteStatus `json:"status"`
}

// FailureKind classifies a worker-side failure.
type FailureKind string

const (
	FailureInput       FailureKind = "input"       // the source cannot be read/decoded
	FailureCompute     FailureKind = "compute"     // ffmpeg/fpcalc failed on a readable source
	FailureTimeout     FailureKind = "timeout"     // the worker-side job timeout fired
	FailureUnsupported FailureKind = "unsupported" // the worker cannot run this assignment
	FailureInternal    FailureKind = "internal"    // a bug or environment problem on the worker
)

// FailRequest reports a failed job. The server applies its retry policy (attempt +1).
type FailRequest struct {
	Generation int64       `json:"generation"`
	Kind       FailureKind `json:"kind"`
	Error      string      `json:"error"`
	// Retryable=false sends the job straight to the dead letter queue.
	Retryable bool `json:"retryable"`
}

// ReleaseReason says why a worker gives a job back.
type ReleaseReason string

const (
	ReleaseShutdown      ReleaseReason = "shutdown"       // graceful stop: no attempt counted
	ReleaseDiskFull      ReleaseReason = "disk_full"      // worker disk full: no attempt counted
	ReleaseOverloaded    ReleaseReason = "overloaded"     // worker cannot take it now: no attempt counted
	ReleaseCrashRecovery ReleaseReason = "crash_recovery" // worker restarted mid-compute: attempt +1
)

// CountsAsAttempt reports whether the release increments the job attempt counter.
func (r ReleaseReason) CountsAsAttempt() bool { return r == ReleaseCrashRecovery }

// ReleaseRequest gives one job back: it goes back to pending at once.
type ReleaseRequest struct {
	Generation int64         `json:"generation"`
	Reason     ReleaseReason `json:"reason"`
}

// ReleaseAllRequest gives back every job of the worker (graceful shutdown).
type ReleaseAllRequest struct {
	SessionID string        `json:"session_id"`
	Reason    ReleaseReason `json:"reason"`
}

// ReleaseResponse answers release and release-all.
type ReleaseResponse struct {
	Released int `json:"released"`
}

// MediaURLRequest asks a fresh signed media URL for a leased job.
type MediaURLRequest struct {
	Generation int64 `json:"generation"`
}

// MediaURLResponse carries the new URL.
type MediaURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ---------------------------------------------------------------------------------------
// Push mode (server -> worker)
// ---------------------------------------------------------------------------------------

// PushAssignRequest is the body of POST /v1/assign on the worker.
type PushAssignRequest struct {
	Assignment Assignment `json:"assignment"`
}

// PushAssignResponse is returned with 202 Accepted when the worker took the job.
type PushAssignResponse struct {
	Accepted bool `json:"accepted"`
}

// PushCancelRequest is the body of POST /v1/cancel on the worker.
type PushCancelRequest struct {
	JobID      string `json:"job_id"`
	Generation int64  `json:"generation"`
}

// PushStatus is the answer of GET /v1/status on the worker.
type PushStatus struct {
	WorkerID   string          `json:"worker_id"`
	APIVersion int             `json:"api_version"`
	Accepting  bool            `json:"accepting"`  // false while draining/stopping or disk full
	FreeSlots  map[Phase]int   `json:"free_slots"` // per phase
	Jobs       []PushJobStatus `json:"jobs"`
	Accel      string          `json:"accel"`
}

// PushJobStatus is one job on a push worker.
type PushJobStatus struct {
	JobID      string   `json:"job_id"`
	Generation int64    `json:"generation"`
	Phase      Phase    `json:"phase"`
	Stage      JobStage `json:"stage"`
	Progress   int      `json:"progress"`
}
