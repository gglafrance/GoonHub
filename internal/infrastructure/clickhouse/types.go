package clickhouse

// AudioHit represents a match from the audio fingerprint index
type AudioHit struct {
	SceneID uint
	Offset  uint32
}

// VisualHit represents a match from the visual fingerprint index
type VisualHit struct {
	SceneID     uint
	FrameOffset uint32
	FullHash    uint64
}
