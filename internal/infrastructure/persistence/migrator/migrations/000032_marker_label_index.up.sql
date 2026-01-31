-- Add index for efficient GROUP BY label queries in marker aggregations
CREATE INDEX idx_user_video_markers_user_label ON user_video_markers(user_id, label);
