ALTER TABLE feeds ADD COLUMN title TEXT NOT NULL DEFAULT '';

CREATE VIRTUAL TABLE content_item_search_index USING fts5(
    title,
    feed_title,
    available_text,
    ai_summary_markdown,
    tokenize = 'unicode61'
);

CREATE TRIGGER content_items_after_delete_search_index
AFTER DELETE ON content_items
BEGIN
    DELETE FROM content_item_search_index WHERE rowid = old.id;
END;
