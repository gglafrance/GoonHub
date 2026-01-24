UPDATE videos SET thumbnail_path = REPLACE(thumbnail_path, '_thumb_sm.webp', '_thumb.webp') WHERE thumbnail_path LIKE '%_thumb_sm.webp';
