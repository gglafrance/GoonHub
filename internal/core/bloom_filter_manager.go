package core

import (
	"fmt"
	"goonhub/internal/data"
	"goonhub/pkg/fingerprint"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

// BloomFilterManager manages the persistent bloom filter for fingerprint pre-screening.
type BloomFilterManager struct {
	filter          *fingerprint.BloomFilter
	fingerprintRepo data.FingerprintRepository
	dataDir         string
	logger          *zap.Logger
	mu              sync.Mutex
}

func NewBloomFilterManager(
	fingerprintRepo data.FingerprintRepository,
	dataDir string,
	logger *zap.Logger,
) *BloomFilterManager {
	return &BloomFilterManager{
		fingerprintRepo: fingerprintRepo,
		dataDir:         dataDir,
		logger:          logger.With(zap.String("component", "bloom_filter_manager")),
	}
}

// Initialize loads the bloom filter from disk, or rebuilds from DB if missing.
func (m *BloomFilterManager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bloomPath := m.bloomFilePath()

	// Try loading from disk
	if _, err := os.Stat(bloomPath); err == nil {
		filter, loadErr := fingerprint.LoadFromFile(bloomPath)
		if loadErr == nil {
			m.filter = filter
			m.logger.Info("Loaded bloom filter from disk", zap.String("path", bloomPath))
			return nil
		}
		m.logger.Warn("Failed to load bloom filter from disk, rebuilding",
			zap.Error(loadErr),
		)
	}

	// Rebuild from database
	return m.rebuildLocked()
}

// AddHashes adds new hash values to the bloom filter and saves to disk.
func (m *BloomFilterManager) AddHashes(hashes []uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.filter == nil {
		m.filter = fingerprint.NewBloomFilter(100000, 0.01)
	}

	for _, h := range hashes {
		m.filter.Add(h)
	}

	if err := m.filter.SaveToFile(m.bloomFilePath()); err != nil {
		m.logger.Error("Failed to save bloom filter to disk", zap.Error(err))
	}
}

// MayContainAny returns true if any of the given hashes might exist in the filter.
func (m *BloomFilterManager) MayContainAny(hashes []uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.filter == nil {
		return false
	}

	for _, h := range hashes {
		if m.filter.MayContain(h) {
			return true
		}
	}
	return false
}

// Rebuild reconstructs the bloom filter from all fingerprints in the database.
func (m *BloomFilterManager) Rebuild() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rebuildLocked()
}

func (m *BloomFilterManager) rebuildLocked() error {
	hashes, err := m.fingerprintRepo.GetAllHashValues()
	if err != nil {
		return fmt.Errorf("failed to load hashes from database: %w", err)
	}

	m.filter = fingerprint.NewBloomFilter(max(len(hashes), 10000), 0.01)
	for _, h := range hashes {
		m.filter.Add(uint64(h))
	}

	if err := os.MkdirAll(filepath.Dir(m.bloomFilePath()), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if err := m.filter.SaveToFile(m.bloomFilePath()); err != nil {
		return fmt.Errorf("failed to save bloom filter: %w", err)
	}

	m.logger.Info("Rebuilt bloom filter from database",
		zap.Int("hash_count", len(hashes)),
	)
	return nil
}

func (m *BloomFilterManager) bloomFilePath() string {
	return filepath.Join(m.dataDir, "bloom_filter.dat")
}
