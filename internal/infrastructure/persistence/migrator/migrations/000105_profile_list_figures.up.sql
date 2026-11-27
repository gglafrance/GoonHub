-- Figures of the custom lists (docs/PROFILES.md, "Figures"): which of the owner's figures the
-- cards of a list show (O count, rating, like date, views). 'auto' = from the sort and the
-- filters of the list, 'none' = no figure. A figure is only sent to viewers who may see the
-- privacy item it reveals (checked by the server).
ALTER TABLE user_profile_lists ADD COLUMN figures VARCHAR(16) NOT NULL DEFAULT 'auto';
ALTER TABLE user_profile_lists ADD CONSTRAINT chk_user_profile_lists_figures
    CHECK (figures IN ('auto', 'none', 'jizz', 'rating', 'liked', 'views'));
