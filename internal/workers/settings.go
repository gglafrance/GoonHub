package workers

import (
	"fmt"
	"net"
	"strings"
	"time"

	"goonhub/internal/config"
)

// Settings are the effective worker settings (config.WorkersConfig with defaults applied).
type Settings struct {
	Enabled              bool
	HeartbeatInterval    time.Duration
	LeaseDuration        time.Duration
	ExpirySweepInterval  time.Duration
	ClaimMaxWait         time.Duration
	ClaimPollInterval    time.Duration
	MediaURLTTL          time.Duration
	MaxArtifactBytes     int64
	UploadChunkBytes     int64
	MaxCompleteBodyBytes int64
	StagingRetention     time.Duration
	JanitorInterval      time.Duration
	MaxAssignmentAge     time.Duration
	StartupLeaseGrace    time.Duration
	PushPollInterval     time.Duration
	PushTimeout          time.Duration
	OfflineAfter         time.Duration
	LocalPhases          []string
	AllowedCIDRs         []*net.IPNet
	PublicURL            string
	StagingRoot          string
}

func orDefault(d, def time.Duration) time.Duration {
	if d <= 0 {
		return def
	}
	return d
}

// NewSettings applies the defaults of docs/REMOTE_WORKERS.md (section 3) to the config.
func NewSettings(cfg config.WorkersConfig, stagingRoot string) (Settings, error) {
	s := Settings{
		Enabled:              cfg.Enabled,
		HeartbeatInterval:    orDefault(cfg.HeartbeatInterval, 10*time.Second),
		LeaseDuration:        orDefault(cfg.LeaseDuration, 60*time.Second),
		ExpirySweepInterval:  orDefault(cfg.ExpirySweepInterval, 5*time.Second),
		ClaimMaxWait:         orDefault(cfg.ClaimMaxWait, 25*time.Second),
		ClaimPollInterval:    time.Second,
		MediaURLTTL:          orDefault(cfg.MediaURLTTL, 6*time.Hour),
		MaxArtifactBytes:     cfg.MaxArtifactBytes,
		UploadChunkBytes:     cfg.UploadChunkBytes,
		MaxCompleteBodyBytes: cfg.MaxCompleteBodyBytes,
		StagingRetention:     orDefault(cfg.StagingRetention, 24*time.Hour),
		MaxAssignmentAge:     orDefault(cfg.MaxAssignmentAge, 24*time.Hour),
		PushPollInterval:     orDefault(cfg.PushPollInterval, 5*time.Second),
		PushTimeout:          orDefault(cfg.PushTimeout, 10*time.Second),
		LocalPhases:          cfg.LocalPhases,
		PublicURL:            strings.TrimRight(cfg.PublicURL, "/"),
		StagingRoot:          stagingRoot,
	}
	if s.MaxArtifactBytes <= 0 {
		s.MaxArtifactBytes = 2 << 30
	}
	if s.UploadChunkBytes <= 0 {
		s.UploadChunkBytes = 16 << 20
	}
	if s.MaxCompleteBodyBytes <= 0 {
		s.MaxCompleteBodyBytes = 64 << 20
	}
	if s.LeaseDuration < 3*s.HeartbeatInterval {
		return s, fmt.Errorf("workers.lease_duration (%s) must be at least 3 x workers.heartbeat_interval (%s)", s.LeaseDuration, s.HeartbeatInterval)
	}
	s.StartupLeaseGrace = orDefault(cfg.StartupLeaseGrace, s.LeaseDuration)
	s.OfflineAfter = orDefault(cfg.OfflineAfter, 3*s.HeartbeatInterval)
	s.JanitorInterval = min(10*time.Minute, max(time.Minute, s.StagingRetention/4))
	for _, c := range cfg.AllowedCIDRs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			return s, fmt.Errorf("workers.allowed_cidrs: invalid CIDR %q: %w", c, err)
		}
		s.AllowedCIDRs = append(s.AllowedCIDRs, n)
	}
	return s, nil
}

// AddressAllowed reports whether a client IP may use the worker API.
func (s Settings) AddressAllowed(ip string) bool {
	if len(s.AllowedCIDRs) == 0 {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range s.AllowedCIDRs {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}
