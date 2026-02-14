# Duplication Detection

This document provides a comprehensive reference for GoonHub's video duplication detection system. The feature identifies duplicate videos across the library -- even when duplicates differ in resolution, encoding, are subsets (clips), or have time offsets -- and lets administrators review, compare, and resolve duplicate groups.

**Feature flag:** `duplication.enabled` (default: `false`)
**External dependencies:** ClickHouse (inverted index), fpcalc/Chromaprint (audio fingerprinting), ffmpeg (visual fingerprinting)

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Configuration](#configuration)
4. [Database Schema](#database-schema)
5. [ClickHouse Schema](#clickhouse-schema)
6. [Fingerprint Extraction](#fingerprint-extraction)
7. [Matching Algorithms](#matching-algorithms)
8. [Duplicate Group Management](#duplicate-group-management)
9. [Job System Integration](#job-system-integration)
10. [API Endpoints](#api-endpoints)
11. [Frontend](#frontend)
12. [Dependency Injection Wiring](#dependency-injection-wiring)
13. [File Inventory](#file-inventory)

---

## Overview

The duplication detection pipeline works in three stages:

1. **Fingerprint extraction** -- Each video is fingerprinted using audio-based (Chromaprint) and/or visual-based (dHash) methods. The strategy depends on whether the video has an audio track and the configured `fingerprint_mode` (`audio_only` or `dual`).
2. **Matching** -- When a fingerprint completes, it is searched against an inverted index stored in ClickHouse. A diagonal alignment algorithm (Hough Transform-style) detects matches even across time offsets, subsets, and resolution differences.
3. **Group management** -- Matched scenes are organized into duplicate groups. Administrators can compare members, select a best variant, optionally merge metadata, and resolve the group (trashing non-best copies).

The entire feature is opt-in. When `duplication.enabled` is `false`:
- No ClickHouse client is created
- No fingerprint worker pool or feeder goroutine is started
- The fingerprint phase is hidden from the frontend
- Attempting to create fingerprint jobs returns an error

---

## Architecture

```
Video Upload / Import
         |
         v
  Metadata Extraction
         |
         v
  Determine extraction strategy
  (based on audio track + fingerprint_mode)
         |
    +-----------+------------------+
    |           |                  |
  audio_only  audio_only          dual
  + audio     + no audio          + audio
    |           |                  |
    v           v               Extract BOTH
  fpcalc      ffmpeg            fpcalc + ffmpeg
  (audio)     (visual)          (audio + visual)
    |           |                  |
    v           v                  v
  []int32     []uint64          []int32 + []uint64
    |           |                  |
    +-----+-----+--+--------------+
          |
          v
  Save to PostgreSQL
  (scenes.audio_fingerprint / visual_fingerprint)
          |
          v
  Per-type matching in ClickHouse
  (audio and/or visual, each independently)
          |
          v
  Deduplicate matches (keep highest confidence per scene)
          |
          v
  Index fingerprint(s) into ClickHouse
  (inserted AFTER matching to avoid self-match)
          |
          v
  Process matches
  (create / merge DuplicateGroup records)
          |
          v
  Admin reviews on /duplicates
  (compare, select best, resolve)
```

### Key Design Decisions

- **Two fingerprinting strategies with configurable dual mode**: Audio fingerprinting (Chromaprint/fpcalc) is used when an audio track is present because it is fast (~2-5s per file) and robust against resolution/codec changes. For silent videos, visual perceptual hashing (dHash at 1 frame/2s) provides a fallback. In `dual` mode, videos with audio produce **both** fingerprint types, enabling cross-type matching (e.g., detecting that a video with audio and the same video with audio stripped are duplicates).
- **ClickHouse as inverted index**: PostgreSQL stores the raw fingerprint bytes (for re-indexing). ClickHouse stores the inverted index for fast lookups. This separation keeps ClickHouse as a non-required dependency.
- **Find-then-index ordering**: When processing a new fingerprint, matches are searched first, then the fingerprint is added to the index. This prevents self-matching.
- **Transitive group merging**: If scene A matches scene B (group 1) and scene C (group 2), the groups are merged into one. This ensures transitive closure of duplicates.
- **Quality-based auto-scoring**: The best variant in each group is automatically determined by a heuristic: `duration * 1000 + (width * height) + codec_bonus + bitrate / 1000`, favoring longer, higher-resolution, better-encoded files.

---

## Configuration

### YAML Config

```yaml
duplication:
  enabled: false                   # Master feature flag
  fingerprint_workers: 1           # Number of concurrent fingerprint workers
  fingerprint_timeout: 5m          # Max duration per fingerprint job
  audio_density_threshold: 0.50    # Min density score for audio matches (0-1)
  audio_min_hashes: 80             # Min aligned hashes for audio match
  audio_max_hash_occurrences: 10   # Skip hashes appearing in more than N scenes
  audio_min_span: 160              # Min aligned span for audio match (~20s at 8 hashes/sec)
  visual_hamming_max: 5            # Max Hamming distance for visual dHash match (0-32)
  visual_min_frames: 20            # Min aligned frames for visual match
  visual_min_span: 30              # Min aligned span for visual match (~60s at 1 frame/2sec)
  delta_tolerance: 2               # Offset quantization bin width

clickhouse:
  host: localhost                  # ClickHouse host (empty = disabled)
  port: 9000                       # ClickHouse native protocol port
  database: goonhub                # Database name
  user: default                    # Username
  password: ""                     # Password
```

### Go Config Structs

**File:** `internal/config/config.go`

```go
type DuplicationConfig struct {
    Enabled                 bool          `mapstructure:"enabled"`
    FingerprintWorkers      int           `mapstructure:"fingerprint_workers"`
    FingerprintTimeout      time.Duration `mapstructure:"fingerprint_timeout"`
    AudioDensityThreshold   float64       `mapstructure:"audio_density_threshold"`
    AudioMinHashes          int           `mapstructure:"audio_min_hashes"`
    AudioMaxHashOccurrences int           `mapstructure:"audio_max_hash_occurrences"`
    AudioMinSpan            int           `mapstructure:"audio_min_span"`
    VisualHammingMax        int           `mapstructure:"visual_hamming_max"`
    VisualMinFrames         int           `mapstructure:"visual_min_frames"`
    VisualMinSpan           int           `mapstructure:"visual_min_span"`
    DeltaTolerance          int           `mapstructure:"delta_tolerance"`
}

type ClickHouseConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    Database string `mapstructure:"database"`
    User     string `mapstructure:"user"`
    Password string `mapstructure:"password"`
}
```

### Runtime-Configurable Thresholds

Matching thresholds can be adjusted at runtime via the admin API (`PUT /api/v1/admin/duplicates/config`) without restarting the server. These are stored in the `duplication_config` singleton table and loaded on every match operation.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `audio_density_threshold` | 0.50 | Minimum ratio of matched hashes to span length. Higher = stricter. |
| `audio_min_hashes` | 80 | Minimum number of aligned hash matches. At ~8 hashes/sec, 80 = ~10s of aligned audio. |
| `audio_max_hash_occurrences` | 10 | Skip sub-fingerprints appearing in more than N distinct scenes. These popular hashes (silence, noise, genre-typical patterns) have no discriminative power and only create false diagonal alignment. Set to 0 to disable. |
| `audio_min_span` | 160 | Minimum aligned span (in hash positions) required before accepting an audio match. At ~8 hashes/sec, 160 = ~20 seconds. Eliminates short coincidental matches. Set to 0 to disable. |
| `visual_hamming_max` | 5 | Maximum bit difference between two 64-bit dHashes (out of 64 bits). Higher = more tolerant of visual differences. |
| `visual_min_frames` | 20 | Minimum number of aligned frame matches. At 1 frame/2s, 20 = ~40s of aligned video. |
| `visual_min_span` | 30 | Minimum aligned span (in frame positions) required before accepting a visual match. At 1 frame/2sec, 30 = ~60 seconds. Eliminates short coincidental matches. Set to 0 to disable. |
| `delta_tolerance` | 2 | Bin width for offset quantization. A tolerance of 2 means offsets within +/-2 positions are grouped together. Uses floor division for uniform bin widths around zero. |
| `fingerprint_mode` | `audio_only` | Controls fingerprint extraction strategy. `audio_only`: videos with audio get only audio fingerprints, silent videos get visual fingerprints. `dual`: videos with audio get **both** audio and visual fingerprints, enabling cross-type matching between videos with and without audio tracks. Silent videos always get visual fingerprints regardless of mode. |

---

## Database Schema

### PostgreSQL

Eight migrations add the duplication tables and columns:

#### Migration 000056: Fingerprint Columns on `scenes`

```sql
ALTER TABLE scenes ADD COLUMN audio_fingerprint BYTEA;
ALTER TABLE scenes ADD COLUMN visual_fingerprint BYTEA;
ALTER TABLE scenes ADD COLUMN fingerprint_type VARCHAR(10);      -- 'audio' or 'visual'
ALTER TABLE scenes ADD COLUMN fingerprint_at TIMESTAMPTZ;
CREATE INDEX idx_scenes_fingerprint_type ON scenes(fingerprint_type)
    WHERE fingerprint_type IS NOT NULL;
```

- `audio_fingerprint`: Raw `[]int32` Chromaprint fingerprint stored as little-endian bytes.
- `visual_fingerprint`: Raw `[]uint64` dHash array stored as little-endian bytes.
- `fingerprint_type`: Determines which fingerprint column is populated and which matching algorithm to use.
- `fingerprint_at`: Timestamp of when the fingerprint was generated.

#### Migration 000057: Duplicate Groups

```sql
CREATE TABLE duplicate_groups (
    id              BIGSERIAL PRIMARY KEY,
    status          VARCHAR(20) NOT NULL DEFAULT 'unresolved',   -- unresolved | resolved | dismissed
    scene_count     INTEGER NOT NULL DEFAULT 0,
    best_scene_id   BIGINT REFERENCES scenes(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ
);

CREATE TABLE duplicate_group_members (
    id               BIGSERIAL PRIMARY KEY,
    group_id         BIGINT NOT NULL REFERENCES duplicate_groups(id) ON DELETE CASCADE,
    scene_id         BIGINT NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    is_best          BOOLEAN NOT NULL DEFAULT FALSE,
    confidence_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    match_type       VARCHAR(10) NOT NULL DEFAULT 'audio',       -- audio | visual
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(group_id, scene_id)
);
```

Key constraints:
- `UNIQUE(group_id, scene_id)` prevents a scene from being added to the same group twice.
- `ON DELETE CASCADE` on `group_id` ensures members are cleaned up when a group is deleted.
- `ON DELETE SET NULL` on `best_scene_id` prevents orphaned FK references.

#### Migration 000058: Duplication Config (Singleton)

```sql
CREATE TABLE duplication_config (
    id                      INTEGER PRIMARY KEY CHECK (id = 1),
    audio_density_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.50,
    audio_min_hashes        INTEGER NOT NULL DEFAULT 40,
    visual_hamming_max      INTEGER NOT NULL DEFAULT 5,
    visual_min_frames       INTEGER NOT NULL DEFAULT 20,
    delta_tolerance         INTEGER NOT NULL DEFAULT 2,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO duplication_config (id) VALUES (1);
```

The `CHECK (id = 1)` constraint enforces the singleton pattern.

#### Migration 000059: Fingerprint Workers in Pool Config

```sql
ALTER TABLE pool_config ADD COLUMN fingerprint_workers INTEGER NOT NULL DEFAULT 1;
```

#### Migration 000060: Fingerprint Trigger Config

Adds trigger config for fingerprint phase scheduling.

#### Migration 000061: Matching Enhancements

```sql
ALTER TABLE duplication_config
    ADD COLUMN IF NOT EXISTS audio_max_hash_occurrences INTEGER NOT NULL DEFAULT 10,
    ADD COLUMN IF NOT EXISTS audio_min_span INTEGER NOT NULL DEFAULT 160,
    ADD COLUMN IF NOT EXISTS visual_min_span INTEGER NOT NULL DEFAULT 30;

-- Update audio_min_hashes default from 40 to 80
UPDATE duplication_config SET audio_min_hashes = 80 WHERE audio_min_hashes = 40;
```

Adds three columns to reduce false positives:
- `audio_max_hash_occurrences`: Popular hash filter threshold. Hashes appearing in too many scenes are skipped.
- `audio_min_span`: Minimum aligned audio duration (in hash positions) to accept a match.
- `visual_min_span`: Minimum aligned visual duration (in frame positions) to accept a match.

#### Migration 000062: Unique Scene in Duplicate Group

Adds a unique constraint ensuring a scene can only appear in one duplicate group at a time.

#### Migration 000063: Fingerprint Mode

```sql
ALTER TABLE duplication_config ADD COLUMN IF NOT EXISTS fingerprint_mode VARCHAR(20) NOT NULL DEFAULT 'audio_only';
```

Adds the `fingerprint_mode` column to control extraction strategy:
- `audio_only` (default): Videos with audio get audio fingerprints only, silent videos get visual fingerprints.
- `dual`: Videos with audio get both audio and visual fingerprints, enabling cross-type duplicate detection.

### Go Models

**File:** `internal/data/duplication_models.go`

```go
type DuplicateGroup struct {
    ID          uint                   `gorm:"primaryKey"`
    Status      string                 `gorm:"size:20;not null;default:'unresolved'"`
    SceneCount  int                    `gorm:"not null;default:0"`
    BestSceneID *uint                  `gorm:"column:best_scene_id"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    ResolvedAt  *time.Time
    Members     []DuplicateGroupMember `gorm:"foreignKey:GroupID"`
}

type DuplicateGroupMember struct {
    ID              uint
    GroupID         uint
    SceneID         uint
    IsBest          bool
    ConfidenceScore float64
    MatchType       string   // "audio" or "visual"
    CreatedAt       time.Time
}

type DuplicationConfigRecord struct {
    ID                      int
    AudioDensityThreshold   float64
    AudioMinHashes          int
    AudioMaxHashOccurrences int
    AudioMinSpan            int
    VisualHammingMax        int
    VisualMinFrames         int
    VisualMinSpan           int
    DeltaTolerance          int
    FingerprintMode         string    // "audio_only" or "dual"
    UpdatedAt               time.Time
}
```

**File:** `internal/data/scene_models.go` (added fields)

```go
AudioFingerprint  []byte     `gorm:"column:audio_fingerprint"`
VisualFingerprint []byte     `gorm:"column:visual_fingerprint"`
FingerprintType   *string    `gorm:"column:fingerprint_type"`
FingerprintAt     *time.Time `gorm:"column:fingerprint_at"`
```

---

## ClickHouse Schema

ClickHouse serves as a high-performance inverted index. It is not a required dependency -- when duplication is disabled or ClickHouse is unavailable, fingerprints are still saved to PostgreSQL but matching does not occur.

### Docker Compose

**File:** `docker/docker-compose.yml`

```yaml
clickhouse:
  image: clickhouse/clickhouse-server:24.12-alpine
  container_name: goonhub-clickhouse
  ports:
    - "8123:8123"   # HTTP interface
    - "9000:9000"   # Native protocol
  volumes:
    - clickhousedata:/var/lib/clickhouse
    - ./clickhouse/init.sql:/docker-entrypoint-initdb.d/init.sql
  healthcheck:
    test: ["CMD", "clickhouse-client", "--query", "SELECT 1"]
    interval: 5s
    timeout: 3s
    retries: 5
```

### Tables

**File:** `docker/clickhouse/init.sql`

```sql
-- Audio: maps sub-fingerprint hash -> (scene_id, offset)
CREATE TABLE audio_fingerprint_index (
    sub_hash    Int32,
    scene_id    UInt64,
    offset      UInt32
) ENGINE = MergeTree()
ORDER BY (sub_hash, scene_id, offset);

-- Visual: bit-partition chunks for approximate dHash matching
CREATE TABLE visual_fingerprint_index (
    chunk_value  UInt16,
    chunk_index  UInt8,       -- 0-3 (which 16-bit partition)
    scene_id     UInt64,
    frame_offset UInt32,
    full_hash    UInt64       -- complete 64-bit dHash for verification
) ENGINE = MergeTree()
ORDER BY (chunk_index, chunk_value, scene_id, frame_offset);
```

**Audio index**: Each Chromaprint fingerprint is an array of `int32` sub-fingerprints. Each sub-fingerprint at position `i` is inserted as `(sub_hash=hash, scene_id=id, offset=i)`. The `ORDER BY (sub_hash, ...)` key enables efficient `WHERE sub_hash IN (...)` lookups.

**Visual index**: Each 64-bit dHash is split into 4 x 16-bit chunks (bit-partitioning). For each chunk, a row is inserted: `(chunk_value=chunk, chunk_index=0..3, scene_id=id, frame_offset=i, full_hash=hash)`. This yields 4 rows per dHash. The `ORDER BY (chunk_index, chunk_value, ...)` key enables efficient lookup by chunk. The `full_hash` column allows Hamming distance verification after the approximate lookup.

### Go Client

**File:** `internal/infrastructure/clickhouse/client.go`

| Method | Description |
|--------|-------------|
| `NewClient(cfg, logger)` | Connects via native protocol. Returns `nil, nil` if host is empty. |
| `InsertAudioFingerprints(ctx, sceneID, hashes)` | Batch inserts `(sub_hash, scene_id, offset)` rows. |
| `InsertVisualFingerprints(ctx, sceneID, hashes)` | Splits each 64-bit hash into 4 x 16-bit chunks, batch inserts. |
| `LookupAudioHashes(ctx, hashes)` | `WHERE sub_hash IN (?)`, returns `map[int32][]AudioHit`. |
| `LookupVisualChunks(ctx, chunks, chunkIndex)` | `WHERE chunk_index = ? AND chunk_value IN (?)`, returns `[]VisualHit`. |
| `DeleteSceneFingerprints(ctx, sceneID)` | `ALTER TABLE ... DELETE WHERE scene_id = ?` on both tables. |
| `Health(ctx)` | Connectivity check. |
| `Close()` | Closes the connection. |

**File:** `internal/infrastructure/clickhouse/types.go`

```go
type AudioHit struct {
    SceneID uint
    Offset  uint32
}

type VisualHit struct {
    SceneID     uint
    FrameOffset uint32
    FullHash    uint64
}
```

---

## Fingerprint Extraction

### Audio Path: Chromaprint

**Files:** `pkg/chromaprint/chromaprint.go`, `pkg/chromaprint/fingerprint.go`

Used when the scene has an audio track (`scene.AudioCodec != ""`).

```
fpcalc -raw -signed -length 0 -json <video_path>
```

- `-raw`: Returns the raw integer sub-fingerprint array (not compressed base64).
- `-signed`: Outputs signed `int32` values.
- `-length 0`: Processes the entire file (no duration limit).
- `-json`: JSON output format for easy parsing.

**Output:** `FingerprintResult{Duration float64, Fingerprint []int32}`

The fingerprint is an array of signed 32-bit integers, generated at approximately 8 hashes per second of audio. A 2-hour video produces ~57,600 hashes. Processing typically takes 2-5 seconds regardless of video length (audio decoding is fast).

`fpcalc` must be available on `$PATH` (provided by the `libchromaprint-tools` package on Debian/Ubuntu).

### Visual Path: dHash

**File:** `pkg/dhash/dhash.go`

Used when the scene has no audio track.

```
ffmpeg -i <video_path> -vf "fps=1/2,scale=9:8,format=gray" -f rawvideo pipe:1
```

- `fps=1/2`: Extract 1 frame every 2 seconds.
- `scale=9:8`: Scale to 9x8 pixels (72 bytes per frame in grayscale).
- `format=gray`: Convert to 8-bit grayscale.
- `-f rawvideo pipe:1`: Stream raw bytes to stdout.

A single streaming ffmpeg process reads all sampled frames sequentially. This avoids the overhead of per-timestamp seeking (~5-10s for a 2-hour video vs ~45s for seeking).

**dHash computation** (`ComputeDHash`):
1. Take a 9x8 pixel grayscale image (72 bytes).
2. For each of 8 rows, compare each pixel to the one to its right (8 comparisons per row).
3. If the left pixel is brighter, set the corresponding bit to 1.
4. Result: 64 bits (8 rows x 8 comparisons).

**Hamming distance** (`HammingDistance`):
```go
func HammingDistance(a, b uint64) int {
    return bits.OnesCount64(a ^ b)
}
```
Counts the number of differing bits between two dHashes. Lower = more similar. A distance of 0-5 typically indicates the same visual content.

### Fingerprint Job

**File:** `internal/jobs/fingerprint_job.go`

```go
type FingerprintResult struct {
    AudioFingerprint  []int32  // non-nil when audio fingerprint was extracted
    VisualFingerprint []uint64 // non-nil when visual fingerprint was extracted
}

// FingerprintTypeLabel returns "audio", "visual", or "dual" based on which arrays are populated.
func (r *FingerprintResult) FingerprintTypeLabel() string
```

The job receives the scene's `AudioCodec` and the configured `fingerprintMode` (`"audio_only"` or `"dual"`) at construction time. The extraction strategy is:

- **Audio extraction**: runs when `audioCodec != ""` (always, in both modes)
- **Visual extraction**: runs when `audioCodec == ""` (no audio) OR when `fingerprintMode == "dual"`

In `audio_only` mode (default), the behavior is the same as before: audio-only for videos with audio, visual-only for silent videos. In `dual` mode, videos with audio produce **both** fingerprint types. Silent videos always produce visual fingerprints regardless of mode.

The result's `FingerprintTypeLabel()` method returns `"audio"`, `"visual"`, or `"dual"` based on which arrays are populated, replacing the old `Type` string field.

Implements the standard `Job` interface with `GetPhase() = "fingerprint"`, cancellation support, and timeout detection.

---

## Matching Algorithms

Both algorithms share the same core idea: **diagonal alignment** (also known as the Hough Transform method, used by AcoustID). The key insight is that if two fingerprints share content at a time offset, the difference `delta = db_offset - query_offset` will be consistent across all matching hashes in that region.

### Audio Matching

**File:** `internal/core/matching/audio.go`

```go
func FindAudioMatches(
    querySceneID uint,
    queryHashes  []int32,
    hits         map[int32][]AudioHit,  // from ClickHouse lookup
    minHashes    int,                   // default: 80
    densityThreshold float64,           // default: 0.50
    deltaTolerance   int,               // default: 2
    maxSceneFreq     int,               // default: 10 (0 = disabled)
    minSpan          int,               // default: 160 (0 = disabled)
) []MatchResult
```

**Algorithm:**

1. **Popular hash filtering** (if `maxSceneFreq > 0`): Pre-compute the number of distinct scenes each sub-hash appears in. Mark any hash found in more than `maxSceneFreq` scenes as "popular" and skip it. Popular hashes (silence, noise, genre-typical audio patterns) have no discriminative power and only create accidental diagonal alignment that leads to false positives. This is standard practice in production fingerprinting systems (AcoustID, Shazam).

2. **Scatter phase**: For each query hash at position `queryOffset`, skip popular hashes, then iterate its ClickHouse hits `{sceneID, dbOffset}`. Skip hits from the query scene itself.

3. **Delta computation**: Calculate `delta = dbOffset - queryOffset`. This represents the temporal offset between the query and candidate scene.

4. **Binning**: Quantize delta: `binKey = floorDiv(delta, deltaTolerance)`. Uses floor division (rounds toward negative infinity) rather than Go's truncation-toward-zero, ensuring uniform bin widths. Without floor division, bin 0 covers 3 delta values ({-1,0,1}) while other bins cover only 2, concentrating random hits. Each bin tracks: unique query positions (deduplicated), min query offset, max query offset.

5. **Accumulation**: Group by `(sceneID, binKey)` to count how many unique query positions align at each temporal offset.

6. **Mode selection**: For each candidate scene, find the bin with the most unique query positions (the mode delta -- the most consistent alignment).

7. **Three-stage gate**:
   - **Gate 1 (minimum hashes)**: `bestBin.uniquePositions >= minHashes`. Ensures enough evidence. At ~8 hashes/sec, 80 hashes = ~10 seconds of aligned audio.
   - **Gate 2 (minimum span)**: `span >= minSpan` (if `minSpan > 0`). Requires the aligned region to cover a minimum duration. At ~8 hashes/sec, 160 positions = ~20 seconds. Eliminates short coincidental matches (~5 seconds) that can pass density thresholds.
   - **Gate 3 (density score)**: `score = uniquePositions / span`. Must be >= `densityThreshold`. This measures what fraction of the aligned region actually matched. A density of 0.50 means at least half the hash positions in the aligned span had matches.

8. **Output**: `MatchResult{SceneID, ConfidenceScore: density, MatchType: "audio"}`.

**Why this works for subsets/clips**: If video B is a 30-second clip from video A, the delta will be consistent for those 30 seconds. The minimum hashes threshold ensures enough aligned content exists, and the density score confirms the alignment is not coincidental.

### Visual Matching

**File:** `internal/core/matching/visual.go`

```go
func FindVisualMatches(
    querySceneID uint,
    queryHashes  []uint64,
    lookupFn     VisualLookupFn,        // queries ClickHouse
    hammingMax   int,                   // default: 5
    minFrames    int,                   // default: 20
    densityThreshold float64,           // default: 0.50
    deltaTolerance   int,               // default: 2
    minSpan          int,               // default: 30 (0 = disabled)
) ([]MatchResult, error)
```

**Algorithm (Bit-partition + Diagonal):**

1. **Bit-partition lookup**: For each of 4 chunk indices (0-3), corresponding to the four 16-bit partitions of each 64-bit dHash:
   - Extract the 16-bit chunk: `chunk = uint16((hash >> (chunkIdx * 16)) & 0xFFFF)`
   - Collect unique chunk values and batch-lookup candidates from ClickHouse.

2. **Hamming verification**: For each candidate `{sceneID, frameOffset, fullHash}`, compute the full Hamming distance: `HammingDistance(queryHash, fullHash)`. Only proceed if distance <= `hammingMax`. The strong Hamming verification already provides good discrimination for visual matching, so popular hash filtering is not needed here (unlike audio).

3. **Diagonal alignment**: Verified hits go through the same delta/binning/scoring pipeline as audio matching, using `minFrames` instead of `minHashes`, `minSpan` for minimum aligned duration, and `floorDiv` for uniform binning.

**Why bit-partitioning works**: This is a form of locality-sensitive hashing (LSH). If two 64-bit hashes differ by at most `K` bits, at least one of their four 16-bit chunks must be identical (by the pigeonhole principle, up to `K=3`). For `K=5`, there's a high probability that at least one chunk matches. Checking all 4 chunks provides recall guarantees while keeping the ClickHouse lookups efficient.

### Match Result

```go
type MatchResult struct {
    SceneID         uint
    ConfidenceScore float64   // density score (0-1)
    MatchType       string    // "audio" or "visual"
}
```

---

## Duplicate Group Management

### Matching Service

**File:** `internal/core/matching_service.go`

The `MatchingService` orchestrates the entire find-index-group pipeline:

```go
type MatchingService struct {
    clickhouse    *clickhouse.Client
    sceneRepo     data.SceneRepository
    dupGroupRepo  data.DuplicateGroupRepository
    dupConfigRepo data.DuplicationConfigRepository
    logger        *zap.Logger
}
```

| Method | Description |
|--------|-------------|
| `IndexFingerprint(sceneID, fpType, audioFP, visualFP)` | Writes fingerprint to ClickHouse inverted index. |
| `FindMatches(sceneID, fpType, audioFP, visualFP)` | Loads thresholds from DB, dispatches to audio or visual match algorithm. |
| `ProcessMatches(sceneID, matches)` | Creates or merges duplicate groups (see below). |
| `RemoveSceneFromIndex(sceneID)` | Deletes all ClickHouse data for a scene (called on permanent delete). |

**`ProcessMatches` logic:**

1. Collect all scene IDs involved: the query scene + all matched scenes.
2. Look up which of these scenes already belong to existing groups (`GetGroupsBySceneIDs`).
3. Also check if the query scene is already in a group (`GetGroupBySceneID`).
4. Branch:
   - **0 existing groups**: Create a new group. Add all scenes as members with their confidence scores.
   - **1 existing group**: Add the query scene to the existing group.
   - **Multiple existing groups**: Merge all groups into the lowest-ID group (transactionally moves all members, deletes source groups, updates count). Then add the query scene.
5. Call `autoScoreBest(groupID)` to recompute the best variant.

**`autoScoreBest` / `scoreScene` -- Quality heuristic:**

```
score = duration * 1000
      + (width * height)
      + codec_bonus
      + bit_rate / 1000
```

Codec bonuses: AV1 = 3,000,000 | HEVC/H265 = 2,000,000 | H264 = 1,000,000 | Other = 0

This heavily favors longer videos (to prefer full versions over clips), then resolution, then modern codecs, then bitrate.

### Duplicate Service

**File:** `internal/core/duplicate_service.go`

Handles admin-facing CRUD and resolution workflows:

```go
type DuplicateService struct {
    groupRepo  data.DuplicateGroupRepository
    sceneRepo  data.SceneRepository
    tagRepo    data.TagRepository
    actorRepo  data.ActorRepository
    eventBus   *EventBus
    logger     *zap.Logger
}
```

| Method | Description |
|--------|-------------|
| `ListGroups(page, limit, status, sortBy)` | Paginated groups enriched with full scene details per member. |
| `GetGroup(groupID)` | Single group with member scene metadata (resolution, codec, size, etc). |
| `GetStats()` | Counts by status: unresolved, resolved, dismissed, total. |
| `ScoreBestVariant(groupID)` | Recomputes and saves the best variant using `scoreScene()`. |
| `ResolveGroup(groupID, bestSceneID, mergeMetadata)` | Full resolution workflow (see below). |
| `DismissGroup(groupID)` | Sets status to "dismissed" -- no action taken on scenes. |
| `SetBest(groupID, sceneID)` | Manually changes the best variant designation. |

**`ResolveGroup` workflow:**

1. Validate that `bestSceneID` is a member of the group.
2. If `mergeMetadata` is true, call `mergeMetadata()`:
   - Collect all unique tags from all duplicate scenes.
   - Collect all unique actors from all duplicate scenes.
   - Add any missing tags/actors to the best scene.
3. Update `is_best` flags on all members.
4. Move all non-best scenes to trash via `sceneRepo.MoveToTrash()`.
5. Set group status to `"resolved"` with a `resolved_at` timestamp.

---

## Job System Integration

The fingerprint phase is fully integrated into the existing job processing pipeline.

### Phase Registration

**File:** `internal/api/v1/validators/phases.go`

`"fingerprint"` is included in both `AllPhases` and `ProcessingPhases` maps, making it valid for:
- Phase validation in API handlers
- Trigger configuration (can be used as an `after_job` target, e.g., triggered after metadata extraction)
- Bulk processing via the Manual jobs UI

### Worker Pool

**File:** `internal/core/processing/pool_manager.go`

The `PoolManager` conditionally creates a `fingerprintPool` only when `duplicationCfg.Enabled` is true:

```go
if duplicationCfg != nil && duplicationCfg.Enabled {
    fingerprintPool = jobs.NewWorkerPool(fpWorkers, queueBufferSize)
    fingerprintPool.SetTimeout(duplicationCfg.FingerprintTimeout)
}
```

Worker count defaults from `config.DuplicationConfig.FingerprintWorkers` but can be overridden at runtime via `pool_config.fingerprint_workers` in the database.

The fingerprint pool participates in:
- `Start()` / `Stop()` / `GracefulStop()` lifecycle
- `GetPoolConfig()` response (exposes `fingerprint_workers` and `duplication_enabled`)
- `GetQueueStatus()` response (exposes `fingerprint_queued` and `fingerprint_active`)
- `UpdatePoolConfig()` (validates 1-10 workers, resizes pool)
- `CancelJob()` / `GetJob()` searches

`SubmitToFingerprintPool(job)` returns an error if the pool is nil (feature disabled).

### Job Queue Feeder

**File:** `internal/core/job_queue_feeder.go`

The feeder conditionally includes fingerprint in its polling loop:

```go
phases := []string{"metadata", "thumbnail", "sprites", "animated_thumbnails"}
if f.duplicationEnabled {
    phases = append(phases, "fingerprint")
}
```

The feeder holds a `dupConfigRepo` (injected via Wire) to read the current `fingerprint_mode` when creating fingerprint jobs.

When polling, the feeder:
1. Checks `queueStatus.FingerprintQueued` and `poolConfig.FingerprintWorkers` to compute capacity.
2. Claims pending fingerprint jobs from the DB using `FOR UPDATE SKIP LOCKED`.
3. Reads the current `fingerprint_mode` from `dupConfigRepo.Get()` (defaults to `"audio_only"` if nil/error).
4. Creates a `FingerprintJobWithID` with the scene's `StoredPath`, `AudioCodec`, and the resolved `fingerprintMode`.
5. Submits to `poolManager.SubmitToFingerprintPool()`.

### Job Submission Guards

**File:** `internal/core/processing/job_submitter.go`

All submission methods (`SubmitPhase`, `SubmitPhaseWithPriority`, `SubmitPhaseWithForce`, `SubmitPhaseWithRetry`, `SubmitBulkPhase`) include guards:

- **Phase validation**: `"fingerprint"` is in the valid phases switch.
- **Feature guard**: `if phase == "fingerprint" && !poolManager.IsDuplicationEnabled()` returns an error.
- **Prerequisite check**: Fingerprint requires `scene.Duration > 0` (metadata must be extracted first).
- **Deduplication**: `ExistsPendingOrRunning(sceneID, phase)` prevents duplicate job creation.

### Result Handler

**File:** `internal/core/processing/result_handler.go`

The `ResultHandler` defines a `MatchingService` interface for loose coupling:

```go
type MatchingService interface {
    IndexFingerprint(sceneID uint, fpType string, audioFP []int32, visualFP []uint64) error
    FindMatches(sceneID uint, fpType string, audioFP []int32, visualFP []uint64) ([]MatchResult, error)
    ProcessMatches(sceneID uint, matches []MatchResult) error
}
```

`SetMatchingService()` injects it at server startup (late binding to avoid circular dependencies).

**`onFingerprintComplete` flow:**

1. Cast job result to `*jobs.FingerprintJob`, extract `FingerprintResult`.
2. Serialize fingerprint(s) to bytes (`int32SliceToBytes` / `uint64SliceToBytes`) based on which arrays are non-nil.
3. Save to PostgreSQL via `repo.UpdateFingerprint()` with `FingerprintTypeLabel()`.
4. If `matchingService` is available (ClickHouse is up), perform **per-type matching**:
   - For each populated fingerprint type (audio, visual), independently:
     - **Find matches** against the existing index for that type.
     - **Index the fingerprint** for that type (added after matching to avoid self-match).
   - **Deduplicate matches** across types: if the same scene matched via both audio and visual, keep the entry with the higher confidence score (`deduplicateMatches` helper).
   - **Process matches** once with the combined, deduplicated results.
5. Publish SSE event `"scene:fingerprint_complete"` with `fingerprint_type` from `FingerprintTypeLabel()`.
6. Trigger any `after_job` phases configured to follow `"fingerprint"`.
7. Mark fingerprint phase complete in `PhaseTracker`.

### Phase Tracker

**File:** `internal/core/processing/phase_tracker.go`

`PhaseState` includes `FingerprintDone bool`. When the fingerprint phase completes, `MarkPhaseComplete("fingerprint")` sets this flag. `CheckAllPhasesComplete()` includes fingerprint in its checks when it's part of the scene's processing pipeline.

---

## API Endpoints

All endpoints are admin-only (require RBAC admin role), registered under `/api/v1/admin/duplicates`.

**File:** `internal/api/v1/handler/duplicate_handler.go`

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/duplicates` | `ListGroups` | Paginated group list. Query params: `page`, `limit`, `status` (unresolved/resolved/dismissed), `sort_by` (newest/size). |
| GET | `/duplicates/stats` | `GetStats` | Returns counts: `{unresolved, resolved, dismissed, total}`. |
| GET | `/duplicates/config` | `GetConfig` | Returns current matching thresholds (defaults if no DB record). |
| PUT | `/duplicates/config` | `UpdateConfig` | Updates matching thresholds. Body: `DuplicationConfigResponse`. |
| GET | `/duplicates/:id` | `GetGroup` | Single group with full member scene details. |
| POST | `/duplicates/:id/resolve` | `ResolveGroup` | Body: `{best_scene_id: uint, merge_metadata: bool}`. Keeps best, trashes others. |
| POST | `/duplicates/:id/dismiss` | `DismissGroup` | Sets status to "dismissed". |
| PUT | `/duplicates/:id/best` | `SetBest` | Body: `{scene_id: uint}`. Changes best variant. |

### Response DTOs

**File:** `internal/api/v1/response/duplicate.go`

```go
type DuplicateGroupResponse struct {
    ID          uint                           `json:"id"`
    Status      string                         `json:"status"`
    SceneCount  int                            `json:"scene_count"`
    BestSceneID *uint                          `json:"best_scene_id"`
    Members     []DuplicateGroupMemberResponse `json:"members"`
    CreatedAt   string                         `json:"created_at"`
    UpdatedAt   string                         `json:"updated_at"`
    ResolvedAt  *string                        `json:"resolved_at,omitempty"`
}

type DuplicateGroupMemberResponse struct {
    SceneID         uint    `json:"scene_id"`
    Title           string  `json:"title"`
    Duration        int     `json:"duration"`
    Width           int     `json:"width"`
    Height          int     `json:"height"`
    VideoCodec      string  `json:"video_codec"`
    AudioCodec      string  `json:"audio_codec"`
    BitRate         int64   `json:"bit_rate"`
    Size            int64   `json:"size"`
    ThumbnailPath   string  `json:"thumbnail_path"`
    IsBest          bool    `json:"is_best"`
    ConfidenceScore float64 `json:"confidence_score"`
    MatchType       string  `json:"match_type"`
}

type DuplicateStatsResponse struct {
    Unresolved int64 `json:"unresolved"`
    Resolved   int64 `json:"resolved"`
    Dismissed  int64 `json:"dismissed"`
    Total      int64 `json:"total"`
}

type DuplicationConfigResponse struct {
    AudioDensityThreshold   float64 `json:"audio_density_threshold"`
    AudioMinHashes          int     `json:"audio_min_hashes"`
    AudioMaxHashOccurrences int     `json:"audio_max_hash_occurrences"`
    AudioMinSpan            int     `json:"audio_min_span"`
    VisualHammingMax        int     `json:"visual_hamming_max"`
    VisualMinFrames         int     `json:"visual_min_frames"`
    VisualMinSpan           int     `json:"visual_min_span"`
    DeltaTolerance          int     `json:"delta_tolerance"`
    FingerprintMode         string  `json:"fingerprint_mode"`
}
```

The `UpdateConfig` handler validates `fingerprint_mode`: must be `"audio_only"` or `"dual"`, defaults to `"audio_only"` if empty.

---

## Frontend

### Types

**File:** `web/app/types/duplicates.ts`

```typescript
interface DuplicateGroup {
    id: number
    status: 'unresolved' | 'resolved' | 'dismissed'
    scene_count: number
    best_scene_id: number | null
    members: DuplicateGroupMember[]
    created_at: string
    updated_at: string
    resolved_at?: string
}

interface DuplicateGroupMember {
    scene_id: number
    title: string
    duration: number
    width: number
    height: number
    video_codec: string
    audio_codec: string
    bit_rate: number
    size: number
    thumbnail_path: string
    is_best: boolean
    confidence_score: number
    match_type: 'audio' | 'visual'
}

interface DuplicateStats {
    unresolved: number
    resolved: number
    dismissed: number
    total: number
}

interface DuplicationConfig {
    audio_density_threshold: number
    audio_min_hashes: number
    audio_max_hash_occurrences: number
    audio_min_span: number
    visual_hamming_max: number
    visual_min_frames: number
    visual_min_span: number
    delta_tolerance: number
    fingerprint_mode: 'audio_only' | 'dual'
}
```

### API Composable

**File:** `web/app/composables/api/useApiDuplicates.ts`

| Function | HTTP | Endpoint |
|----------|------|----------|
| `listGroups(page, limit, status?, sortBy?)` | GET | `/api/v1/admin/duplicates` |
| `getGroup(id)` | GET | `/api/v1/admin/duplicates/:id` |
| `getStats()` | GET | `/api/v1/admin/duplicates/stats` |
| `resolveGroup(id, bestSceneId, mergeMetadata)` | POST | `/api/v1/admin/duplicates/:id/resolve` |
| `dismissGroup(id)` | POST | `/api/v1/admin/duplicates/:id/dismiss` |
| `setBest(id, sceneId)` | PUT | `/api/v1/admin/duplicates/:id/best` |
| `getConfig()` | GET | `/api/v1/admin/duplicates/config` |
| `updateConfig(config)` | PUT | `/api/v1/admin/duplicates/config` |

Also re-exported through `web/app/composables/useApi.ts` for backwards compatibility.

### Duplicates Page

**File:** `web/app/pages/duplicates.vue`

Admin-only page (redirects non-admins) with:

- **Stats bar**: Three cards showing Unresolved (lava accent), Resolved (emerald), Dismissed (dim gray) counts.
- **Config panel**: Collapsible `<DuplicatesConfigPanel>` component with threshold inputs.
- **Status filter tabs**: All, Unresolved (default), Resolved, Dismissed.
- **Sort dropdown**: Newest, Group Size.
- **Group list**: Expandable cards showing:
  - Stacked thumbnail previews (up to 3 members + overflow count).
  - Status badge, match type badge (audio=blue, visual=purple), scene count, creation date.
  - Expanded detail: Comparison table (resolution, duration, codec, bitrate, size, confidence %), best variant radio selector.
  - Action bar: Merge metadata checkbox, Dismiss button, Resolve button.
- **Pagination**: Ellipsis-based page navigation.

### Config Panel Component

**File:** `web/app/components/duplicates/ConfigPanel.vue`

Receives `DuplicationConfig` prop, emits `save` events.

**Fingerprint Mode** dropdown (full-width, above threshold grid):
- "Audio Only (default)" -- standard behavior: audio fingerprints for videos with audio, visual for silent videos
- "Dual (audio + visual)" -- videos with audio produce both fingerprint types for cross-type matching
- Includes a description text explaining the two modes

**Detection Thresholds** -- eight numeric inputs in a responsive grid:
- Audio Density Threshold (0-1, step 0.05)
- Audio Min Hashes (integer, min 1)
- Audio Max Hash Freq (integer, min 1) -- popular hash filter threshold
- Audio Min Span (integer, min 0) -- minimum aligned audio duration in hash positions
- Visual Hamming Max (0-32)
- Visual Min Frames (integer, min 1)
- Visual Min Span (integer, min 0) -- minimum aligned visual duration in frame positions
- Delta Tolerance (integer, min 1)

### Navigation

**File:** `web/app/components/AppHeader.vue`

A "Duplicates" link in the nav bar, visible only to admin users (`v-if="authStore.user?.role === 'admin'"`), with the `heroicons:document-duplicate` icon.

### Manual Jobs Integration

**File:** `web/app/components/settings/jobs/Manual.vue`

The fingerprint phase is conditionally shown in the bulk processing section. On mount, the component fetches pool config to read `duplication_enabled`. When disabled, "Fingerprint" is excluded from the available phases list and its info text is hidden.

### Job Types Integration

**File:** `web/app/types/jobs.ts`

`'fingerprint'` is included in all phase union types: `JobHistory.phase`, `TriggerConfig.phase`, `BulkJobRequest.phase`, `DLQEntry.phase`, `ActiveJobInfo.phase`. The `PoolConfig` interface includes `fingerprint_workers` and `duplication_enabled`. The `QueueStatus` interface includes `fingerprint_queued`, `fingerprint_running`, and `fingerprint_pending`.

---

## Dependency Injection Wiring

**File:** `internal/wire/wire.go`

The duplication feature uses conditional nil-propagation through the Wire DI chain:

```
Config.Duplication.Enabled = false
    -> provideClickHouseClient() returns nil
        -> provideMatchingService(nil) returns nil
            -> server.SetMatchingService(nil) is a no-op
                -> ResultHandler has no matching service (fingerprints saved but not matched)

Config.Duplication.Enabled = true
    -> provideClickHouseClient() connects to ClickHouse
        -> provideMatchingService(client) creates MatchingService
            -> server.SetMatchingService(ms) wires into ResultHandler
                -> Full pipeline: extract -> save -> match -> index -> group
```

| Provider | Dependencies | Conditional |
|----------|-------------|-------------|
| `provideDuplicateGroupRepository` | `*gorm.DB` | Always created |
| `provideDuplicationConfigRepository` | `*gorm.DB` | Always created |
| `provideClickHouseClient` | `Config, Logger` | Returns nil if `!Duplication.Enabled` |
| `provideMatchingService` | `ClickHouse, SceneRepo, DupGroupRepo, DupConfigRepo, Logger` | Returns nil if ClickHouse is nil |
| `provideDuplicateService` | `DupGroupRepo, SceneRepo, TagRepo, ActorRepo, EventBus, Logger` | Always created |
| `provideDuplicateHandler` | `DuplicateService, DupConfigRepo` | Always created |
| `provideJobQueueFeeder` | `JobHistoryRepo, SceneRepo, MarkerService, ProcessingService, DupConfigRepo, Config, Logger` | Always created (reads `fingerprint_mode` from DupConfigRepo at runtime) |

The `DuplicateService` and handler are always created so the API endpoints work even when fingerprinting is disabled (admins can still view/manage existing groups).

The `MatchingService` is late-bound to the `ResultHandler` via `SetMatchingService()` during server startup, avoiding circular dependency issues in the Wire graph.

---

## File Inventory

### New Files

| File | Purpose |
|------|---------|
| **Infrastructure** | |
| `docker/clickhouse/init.sql` | ClickHouse table definitions |
| `internal/infrastructure/clickhouse/client.go` | ClickHouse native client wrapper |
| `internal/infrastructure/clickhouse/types.go` | AudioHit, VisualHit types |
| **Migrations** | |
| `migrations/000056_add_fingerprint_columns.{up,down}.sql` | Scene fingerprint columns |
| `migrations/000057_create_duplicate_groups.{up,down}.sql` | Duplicate group tables |
| `migrations/000058_add_duplication_config.{up,down}.sql` | Config singleton |
| `migrations/000059_add_fingerprint_workers_to_pool_config.{up,down}.sql` | Pool config column |
| `migrations/000060_add_fingerprint_trigger_config.{up,down}.sql` | Fingerprint trigger config |
| `migrations/000061_add_matching_enhancements.{up,down}.sql` | Popular hash filter, min span columns |
| `migrations/000062_unique_scene_in_duplicate_group.{up,down}.sql` | Unique scene constraint per group |
| `migrations/000063_add_fingerprint_mode.{up,down}.sql` | Fingerprint mode column on config |
| **Data Layer** | |
| `internal/data/duplication_models.go` | DuplicateGroup, Member, Config models |
| `internal/data/duplication_repository.go` | Repository interfaces |
| `internal/data/duplication_repository_impl.go` | GORM implementations |
| **Fingerprint Extraction** | |
| `pkg/chromaprint/chromaprint.go` | fpcalc binary check |
| `pkg/chromaprint/fingerprint.go` | Audio fingerprint extraction |
| `pkg/dhash/dhash.go` | Visual dHash extraction |
| `internal/jobs/fingerprint_job.go` | FingerprintJob implementation |
| **Matching** | |
| `internal/core/matching/audio.go` | Audio diagonal alignment algorithm |
| `internal/core/matching/visual.go` | Visual bit-partition + alignment algorithm |
| `internal/core/matching_service.go` | Matching orchestrator (index, find, group) |
| **Business Logic** | |
| `internal/core/duplicate_service.go` | Group CRUD, resolution, metadata merge |
| **API** | |
| `internal/api/v1/handler/duplicate_handler.go` | HTTP handler |
| `internal/api/v1/response/duplicate.go` | Response DTOs |
| **Frontend** | |
| `web/app/types/duplicates.ts` | TypeScript interfaces |
| `web/app/composables/api/useApiDuplicates.ts` | API composable |
| `web/app/pages/duplicates.vue` | Duplicates management page |
| `web/app/components/duplicates/ConfigPanel.vue` | Fingerprint mode selector + threshold config panel |

### Modified Files

| File | Changes |
|------|---------|
| `internal/config/config.go` | Added `DuplicationConfig`, `ClickHouseConfig` structs and defaults |
| `internal/data/scene_models.go` | Added 4 fingerprint fields to `Scene` |
| `internal/data/repository.go` | Added `UpdateFingerprint`, `GetScenesNeedingFingerprint`, fingerprint case in `GetScenesNeedingPhase` |
| `internal/data/pool_config_repository.go` | Added `FingerprintWorkers` field |
| `internal/api/v1/validators/phases.go` | Added `"fingerprint"` to phase maps |
| `internal/core/processing/types.go` | Added fingerprint to `PoolConfig`, `QueueStatus`, `PhaseState` |
| `internal/core/processing/pool_manager.go` | Added `fingerprintPool`, `duplicationEnabled`, all pool lifecycle methods |
| `internal/core/processing/result_handler.go` | Added `MatchingService` interface, `onFingerprintComplete` with per-type dual dispatch and `deduplicateMatches` helper |
| `internal/core/processing/phase_tracker.go` | Added fingerprint to phase tracking |
| `internal/core/processing/job_submitter.go` | Added fingerprint to phase validation with duplication guard |
| `internal/core/job_queue_feeder.go` | Added fingerprint feeder goroutine (conditional), reads `fingerprint_mode` from `DuplicationConfigRepository` |
| `internal/core/scene_processing_service.go` | Added `matchingServiceAdapter`, `SetMatchingService`, `IsDuplicationEnabled` |
| `internal/wire/wire.go` | Added 6 provider functions, updated wiring; `provideJobQueueFeeder` now receives `DuplicationConfigRepository` |
| `internal/api/routes.go` | Added 8 duplicate routes |
| `internal/api/v1/handler/job_history.go` | Added fingerprint queue status fields, duplication guard on bulk trigger |
| `internal/api/v1/handler/pool_config_handler.go` | Added `FingerprintWorkers` to pool config record |
| `docker/docker-compose.yml` | Added ClickHouse service and volume |
| `web/app/types/jobs.ts` | Added `'fingerprint'` to phase unions, `duplication_enabled` to `PoolConfig` |
| `web/app/components/settings/jobs/Manual.vue` | Conditional fingerprint phase based on `duplication_enabled` |
| `web/app/components/AppHeader.vue` | Added Duplicates nav link (admin-only) |
| `web/app/composables/useApi.ts` | Re-exported `useApiDuplicates` functions |
