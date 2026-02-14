package clickhouse

import (
	"context"
	"fmt"
	"goonhub/internal/config"
	"time"

	"go.uber.org/zap"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Client wraps ClickHouse connection for fingerprint index operations
type Client struct {
	conn   driver.Conn
	logger *zap.Logger
}

// NewClient creates a new ClickHouse client. Returns nil if host is empty (feature disabled).
func NewClient(cfg config.ClickHouseConfig, logger *zap.Logger) (*Client, error) {
	if cfg.Host == "" {
		return nil, nil
	}

	conn, err := ch.Open(&ch.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: ch.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
		Settings: ch.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 5 * time.Second,
		Compression: &ch.Compression{
			Method: ch.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	logger.Info("Connected to ClickHouse",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
	)

	return &Client{conn: conn, logger: logger}, nil
}

// InsertAudioFingerprints batch-inserts audio sub-fingerprint hashes
func (c *Client) InsertAudioFingerprints(ctx context.Context, sceneID uint, hashes []int32) error {
	if len(hashes) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO audio_fingerprint_index (sub_hash, scene_id, offset)")
	if err != nil {
		return fmt.Errorf("failed to prepare audio batch: %w", err)
	}

	for i, hash := range hashes {
		if err := batch.Append(hash, uint64(sceneID), uint32(i)); err != nil {
			return fmt.Errorf("failed to append audio hash: %w", err)
		}
	}

	return batch.Send()
}

// InsertVisualFingerprints batch-inserts visual dHash fingerprints split into 4x16-bit chunks
func (c *Client) InsertVisualFingerprints(ctx context.Context, sceneID uint, hashes []uint64) error {
	if len(hashes) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO visual_fingerprint_index (chunk_value, chunk_index, scene_id, frame_offset, full_hash)")
	if err != nil {
		return fmt.Errorf("failed to prepare visual batch: %w", err)
	}

	for frameIdx, hash := range hashes {
		for chunkIdx := uint8(0); chunkIdx < 4; chunkIdx++ {
			chunkValue := uint16((hash >> (chunkIdx * 16)) & 0xFFFF)
			if err := batch.Append(chunkValue, chunkIdx, uint64(sceneID), uint32(frameIdx), hash); err != nil {
				return fmt.Errorf("failed to append visual chunk: %w", err)
			}
		}
	}

	return batch.Send()
}

// LookupAudioHashes looks up sub-fingerprint hashes and returns scene matches.
// Batches queries to stay within ClickHouse max_query_size limits.
func (c *Client) LookupAudioHashes(ctx context.Context, hashes []int32) (map[int32][]AudioHit, error) {
	if len(hashes) == 0 {
		return make(map[int32][]AudioHit), nil
	}

	const batchSize = 10000
	result := make(map[int32][]AudioHit)

	for i := 0; i < len(hashes); i += batchSize {
		end := i + batchSize
		if end > len(hashes) {
			end = len(hashes)
		}
		batch := hashes[i:end]

		rows, err := c.conn.Query(ctx,
			"SELECT sub_hash, scene_id, offset FROM audio_fingerprint_index WHERE sub_hash IN (?)",
			batch,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup audio hashes: %w", err)
		}

		for rows.Next() {
			var subHash int32
			var sceneID uint64
			var offset uint32
			if err := rows.Scan(&subHash, &sceneID, &offset); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan audio hit: %w", err)
			}
			result[subHash] = append(result[subHash], AudioHit{
				SceneID: uint(sceneID),
				Offset:  offset,
			})
		}
		rows.Close()
	}

	return result, nil
}

// LookupAudioHashesFiltered looks up sub-fingerprint hashes after excluding popular
// ones server-side in a single query. A hash is "popular" if it appears in more than
// maxSceneFreq distinct scenes. When maxSceneFreq <= 0, delegates directly to
// LookupAudioHashes (no filtering).
func (c *Client) LookupAudioHashesFiltered(ctx context.Context, hashes []int32, maxSceneFreq int) (map[int32][]AudioHit, error) {
	if maxSceneFreq <= 0 {
		return c.LookupAudioHashes(ctx, hashes)
	}
	if len(hashes) == 0 {
		return make(map[int32][]AudioHit), nil
	}

	const batchSize = 10000
	result := make(map[int32][]AudioHit)

	for i := 0; i < len(hashes); i += batchSize {
		end := i + batchSize
		if end > len(hashes) {
			end = len(hashes)
		}
		batch := hashes[i:end]

		// Single query: lookup hashes and exclude popular ones via NOT IN subquery
		rows, err := c.conn.Query(ctx, `
			SELECT sub_hash, scene_id, offset
			FROM audio_fingerprint_index
			WHERE sub_hash IN (?)
			AND sub_hash NOT IN (
				SELECT sub_hash FROM audio_fingerprint_index
				WHERE sub_hash IN (?)
				GROUP BY sub_hash
				HAVING uniqExact(scene_id) > ?
			)`,
			batch, batch, uint64(maxSceneFreq),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup filtered audio hashes: %w", err)
		}

		for rows.Next() {
			var subHash int32
			var sceneID uint64
			var offset uint32
			if err := rows.Scan(&subHash, &sceneID, &offset); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan filtered audio hit: %w", err)
			}
			result[subHash] = append(result[subHash], AudioHit{
				SceneID: uint(sceneID),
				Offset:  offset,
			})
		}
		rows.Close()
	}

	return result, nil
}

// LookupVisualChunks looks up bit-partition chunks for approximate dHash matching
func (c *Client) LookupVisualChunks(ctx context.Context, chunks []uint16, chunkIndex uint8) ([]VisualHit, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	rows, err := c.conn.Query(ctx,
		"SELECT scene_id, frame_offset, full_hash FROM visual_fingerprint_index WHERE chunk_index = ? AND chunk_value IN (?)",
		chunkIndex, chunks,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup visual chunks: %w", err)
	}
	defer rows.Close()

	var hits []VisualHit
	for rows.Next() {
		var sceneID uint64
		var frameOffset uint32
		var fullHash uint64
		if err := rows.Scan(&sceneID, &frameOffset, &fullHash); err != nil {
			return nil, fmt.Errorf("failed to scan visual hit: %w", err)
		}
		hits = append(hits, VisualHit{
			SceneID:     uint(sceneID),
			FrameOffset: frameOffset,
			FullHash:    fullHash,
		})
	}

	return hits, nil
}

// DeleteSceneFingerprints removes all fingerprint data for a scene
func (c *Client) DeleteSceneFingerprints(ctx context.Context, sceneID uint) error {
	if err := c.conn.Exec(ctx, "ALTER TABLE audio_fingerprint_index DELETE WHERE scene_id = ?", uint64(sceneID)); err != nil {
		return fmt.Errorf("failed to delete audio fingerprints: %w", err)
	}
	if err := c.conn.Exec(ctx, "ALTER TABLE visual_fingerprint_index DELETE WHERE scene_id = ?", uint64(sceneID)); err != nil {
		return fmt.Errorf("failed to delete visual fingerprints: %w", err)
	}
	return nil
}

// Health checks if ClickHouse is reachable
func (c *Client) Health(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

// Close closes the ClickHouse connection
func (c *Client) Close() error {
	return c.conn.Close()
}
