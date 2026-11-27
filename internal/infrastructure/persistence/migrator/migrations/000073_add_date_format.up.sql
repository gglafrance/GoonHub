-- How dates are displayed: auto (browser locale) or a fixed pattern such as DD/MM/YYYY
ALTER TABLE user_settings
    ADD COLUMN date_format VARCHAR(20) NOT NULL DEFAULT 'auto';
