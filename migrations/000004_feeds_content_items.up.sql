CREATE TABLE feeds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_url TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('rss', 'podcast')),
    fetch_status TEXT NOT NULL DEFAULT 'idle' CHECK (fetch_status IN ('idle', 'fetching', 'success', 'failed')),
    fetch_started_at DATETIME NULL,
    last_fetched_at DATETIME NULL,
    last_fetch_error TEXT NULL,
    folder_id INTEGER NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_feeds_feed_url ON feeds (feed_url);
CREATE INDEX idx_feeds_folder_id ON feeds (folder_id);

CREATE TABLE content_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id INTEGER NOT NULL,
    dedupe_key TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('text', 'audio')),
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    show_notes TEXT NOT NULL DEFAULT '',
    description_safe TEXT NOT NULL DEFAULT '',
    content_safe TEXT NOT NULL DEFAULT '',
    show_notes_safe TEXT NOT NULL DEFAULT '',
    available_text TEXT NOT NULL DEFAULT '',
    published_at DATETIME NULL,
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    audio_progress_seconds INTEGER NOT NULL DEFAULT 0 CHECK (audio_progress_seconds >= 0),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (feed_id) REFERENCES feeds (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_content_items_feed_dedupe_key ON content_items (feed_id, dedupe_key);
CREATE INDEX idx_content_items_feed_id_published_at ON content_items (feed_id, published_at DESC);
