DROP TRIGGER IF EXISTS content_items_after_delete_search_index;

DROP TABLE IF EXISTS content_item_search_index;

ALTER TABLE feeds DROP COLUMN title;
