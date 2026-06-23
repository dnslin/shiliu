package server

// Migration execution is owned by cmd/migration through internal/migration.
// Long-running server/task startup must not mutate database schema implicitly.
