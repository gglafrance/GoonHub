UPDATE videos SET thumbnail_path = REPLACE(thumbnail_path, '_thumb.webp', '_thumb_sm.webp') WHERE thumbnail_path LIKE '%_thumb.webp';
