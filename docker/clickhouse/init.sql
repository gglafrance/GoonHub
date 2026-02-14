CREATE DATABASE IF NOT EXISTS goonhub;

CREATE TABLE IF NOT EXISTS goonhub.audio_fingerprint_index (
    sub_hash    Int32,
    scene_id    UInt64,
    offset      UInt32
) ENGINE = MergeTree()
ORDER BY (sub_hash, scene_id, offset);

CREATE TABLE IF NOT EXISTS goonhub.visual_fingerprint_index (
    chunk_value  UInt16,
    chunk_index  UInt8,
    scene_id     UInt64,
    frame_offset UInt32,
    full_hash    UInt64
) ENGINE = MergeTree()
ORDER BY (chunk_index, chunk_value, scene_id, frame_offset);
