DROP TABLE IF EXISTS user_profile_lists;
ALTER TABLE user_profiles DROP CONSTRAINT IF EXISTS chk_user_profiles_tabs_array;
ALTER TABLE user_profiles DROP COLUMN IF EXISTS tabs;
