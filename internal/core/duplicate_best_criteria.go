package core

import (
	"fmt"
	"strings"

	"goonhub/internal/data"
)

// Criteria used to pick the best variant of a duplicate group, applied in order: the first
// criterion that differs between two scenes decides.
const (
	BestCriterionResolution = "resolution" // higher resolution (pixels) first
	BestCriterionDuration   = "duration"   // longer first
	BestCriterionBitrate    = "bitrate"    // higher bitrate first
	BestCriterionSizeSmall  = "size_small" // smaller file first
	BestCriterionSizeLarge  = "size_large" // larger file first
	BestCriterionFrameRate  = "frame_rate" // higher frame rate first
	BestCriterionCodec      = "codec"      // more efficient codec first (av1 > hevc > h264 > other)
	BestCriterionOldest     = "oldest"     // added to the library first
	BestCriterionNewest     = "newest"     // added to the library last
)

// DefaultBestCriteria is the default order: quality, then duration, bitrate, smallest file.
var DefaultBestCriteria = []string{
	BestCriterionResolution,
	BestCriterionDuration,
	BestCriterionBitrate,
	BestCriterionSizeSmall,
}

// AllBestCriteria lists the supported criteria (for validation and the settings UI).
var AllBestCriteria = []string{
	BestCriterionResolution,
	BestCriterionDuration,
	BestCriterionBitrate,
	BestCriterionSizeSmall,
	BestCriterionSizeLarge,
	BestCriterionFrameRate,
	BestCriterionCodec,
	BestCriterionOldest,
	BestCriterionNewest,
}

// ParseBestCriteria parses a comma-separated criteria list, falling back to the default when
// empty. Unknown or repeated criteria are rejected.
func ParseBestCriteria(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]string(nil), DefaultBestCriteria...), nil
	}
	valid := make(map[string]bool, len(AllBestCriteria))
	for _, c := range AllBestCriteria {
		valid[c] = true
	}
	seen := make(map[string]bool)
	var out []string
	for _, part := range strings.Split(raw, ",") {
		c := strings.TrimSpace(part)
		if c == "" {
			continue
		}
		if !valid[c] {
			return nil, fmt.Errorf("unknown best selection criterion %q", c)
		}
		if seen[c] {
			return nil, fmt.Errorf("best selection criterion %q is repeated", c)
		}
		if (c == BestCriterionSizeSmall && seen[BestCriterionSizeLarge]) || (c == BestCriterionSizeLarge && seen[BestCriterionSizeSmall]) ||
			(c == BestCriterionOldest && seen[BestCriterionNewest]) || (c == BestCriterionNewest && seen[BestCriterionOldest]) {
			return nil, fmt.Errorf("best selection criterion %q contradicts an earlier one", c)
		}
		seen[c] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return append([]string(nil), DefaultBestCriteria...), nil
	}
	return out, nil
}

// bestCriteriaFromConfig returns the configured criteria, or the default when unset/invalid.
func bestCriteriaFromConfig(cfg *data.DuplicationConfigRecord) []string {
	if cfg == nil {
		return DefaultBestCriteria
	}
	criteria, err := ParseBestCriteria(cfg.BestSelectionCriteria)
	if err != nil {
		return DefaultBestCriteria
	}
	return criteria
}

func codecRank(codec string) int {
	switch strings.ToLower(codec) {
	case "av1":
		return 3
	case "hevc", "h265":
		return 2
	case "h264":
		return 1
	default:
		return 0
	}
}

// compareByCriterion returns >0 when a is better than b for the criterion, <0 when worse.
func compareByCriterion(a, b *data.Scene, criterion string) int {
	cmp := func(x, y int64) int {
		switch {
		case x > y:
			return 1
		case x < y:
			return -1
		default:
			return 0
		}
	}
	switch criterion {
	case BestCriterionResolution:
		return cmp(int64(a.Width)*int64(a.Height), int64(b.Width)*int64(b.Height))
	case BestCriterionDuration:
		return cmp(int64(a.Duration), int64(b.Duration))
	case BestCriterionBitrate:
		return cmp(a.BitRate, b.BitRate)
	case BestCriterionSizeSmall:
		return cmp(b.Size, a.Size)
	case BestCriterionSizeLarge:
		return cmp(a.Size, b.Size)
	case BestCriterionFrameRate:
		return cmp(int64(a.FrameRate*1000), int64(b.FrameRate*1000))
	case BestCriterionCodec:
		return cmp(int64(codecRank(a.VideoCodec)), int64(codecRank(b.VideoCodec)))
	case BestCriterionOldest:
		return cmp(b.CreatedAt.UnixNano(), a.CreatedAt.UnixNano())
	case BestCriterionNewest:
		return cmp(a.CreatedAt.UnixNano(), b.CreatedAt.UnixNano())
	default:
		return 0
	}
}

// betterSceneBy reports whether a is a better variant to keep than b with the given ordered
// criteria. Full ties keep the lowest scene ID, so the choice is deterministic.
func betterSceneBy(a, b *data.Scene, criteria []string) bool {
	for _, c := range criteria {
		if r := compareByCriterion(a, b, c); r != 0 {
			return r > 0
		}
	}
	return a.ID < b.ID
}

// betterScene compares with the default criteria.
func betterScene(a, b *data.Scene) bool {
	return betterSceneBy(a, b, DefaultBestCriteria)
}
