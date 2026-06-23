CREATE TABLE IF NOT EXISTS shiliu_migration_baseline (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO shiliu_migration_baseline (id) VALUES (1);
