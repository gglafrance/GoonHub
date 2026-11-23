-- The fingerprint job result dropped the aligned positions of each match, so every duplicate
-- group member was stored with the segment "0 to one fingerprint step" (0-2 s for a visual
-- match, 0-0.124 s for an audio one) on both sides, and its relation was derived from it.
-- A real match spans at least audio_min_span / visual_min_span positions (20 s / 60 s by
-- default), so this exact shape only comes from the bug. Clear those segments and relations:
-- the server recomputes them at startup from the stored fingerprints
-- (MatchingService.BackfillMemberSegments).
UPDATE duplicate_group_members
SET match_start = NULL,
    match_end = NULL,
    ref_scene_id = NULL,
    ref_start = NULL,
    ref_end = NULL,
    relation = 'unknown'
WHERE match_start = 0
  AND ref_start = 0
  AND match_end = ref_end
  AND (abs(match_end - 2.0) < 0.0001 OR abs(match_end - (4096.0 / 3.0 / 11025.0)) < 0.0001);
