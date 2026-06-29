ALTER TABLE feeds ADD COLUMN title TEXT NOT NULL DEFAULT '';

CREATE VIRTUAL TABLE content_item_search_index USING fts5(
    title,
    feed_title,
    available_text,
    ai_summary_markdown,
    tokenize = 'trigram'
);

INSERT INTO content_item_search_index (rowid, title, feed_title, available_text, ai_summary_markdown)
SELECT content_items.id, content_items.title, feeds.title, content_items.available_text, ''
FROM content_items
JOIN feeds ON feeds.id = content_items.feed_id;


CREATE TRIGGER content_items_after_delete_search_index
AFTER DELETE ON content_items
BEGIN
    DELETE FROM content_item_search_index WHERE rowid = old.id;
END;
