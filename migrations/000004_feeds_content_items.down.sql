DROP INDEX IF EXISTS idx_content_items_feed_id_published_at;
DROP INDEX IF EXISTS idx_content_items_feed_dedupe_key;
DROP TABLE IF EXISTS content_items;

DROP INDEX IF EXISTS idx_feeds_folder_id;
DROP INDEX IF EXISTS idx_feeds_feed_url;
DROP TABLE IF EXISTS feeds;
