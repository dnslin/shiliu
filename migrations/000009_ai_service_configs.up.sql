CREATE TABLE ai_service_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    singleton_id INTEGER NOT NULL DEFAULT 1 CHECK (singleton_id = 1),
    api_base_url TEXT NOT NULL,
    model TEXT NOT NULL,
    api_key TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_ai_service_configs_singleton ON ai_service_configs (singleton_id);
