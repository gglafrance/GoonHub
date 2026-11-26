ALTER TABLE user_profile_lists DROP CONSTRAINT IF EXISTS chk_user_profile_lists_figures;
ALTER TABLE user_profile_lists DROP COLUMN IF EXISTS figures;
