DROP INDEX IF EXISTS idx_content_items_favorited;
DROP INDEX IF EXISTS idx_content_items_marked_later;
DROP INDEX IF EXISTS idx_content_items_processing_status;

ALTER TABLE content_items DROP COLUMN favorited;
ALTER TABLE content_items DROP COLUMN marked_later;
ALTER TABLE content_items DROP COLUMN processing_status;
