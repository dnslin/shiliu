CREATE TABLE auto_summary_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    singleton_id INTEGER NOT NULL DEFAULT 1 CHECK (singleton_id = 1),
    enabled BOOLEAN NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    content_type_scope TEXT NOT NULL DEFAULT 'all' CHECK (content_type_scope IN ('text', 'audio', 'all')),
    enabled_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_auto_summary_configs_singleton ON auto_summary_configs (singleton_id);
