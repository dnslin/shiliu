ALTER TABLE content_items ADD COLUMN processing_status TEXT NOT NULL DEFAULT 'unprocessed' CHECK (processing_status IN ('unprocessed', 'completed'));
ALTER TABLE content_items ADD COLUMN marked_later INTEGER NOT NULL DEFAULT 0 CHECK (marked_later IN (0, 1));
ALTER TABLE content_items ADD COLUMN favorited INTEGER NOT NULL DEFAULT 0 CHECK (favorited IN (0, 1));

CREATE INDEX idx_content_items_processing_status ON content_items (processing_status);
CREATE INDEX idx_content_items_marked_later ON content_items (marked_later);
CREATE INDEX idx_content_items_favorited ON content_items (favorited);
