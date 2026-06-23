package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunUpAppliesBaseline(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")

	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))
	requireBaselineTableExists(t, dbPath)
}

func TestRunUpIsIdempotentWhenNoChangesRemain(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	sourceURL := testMigrationSourceURL(t)

	require.NoError(t, runUp(dbPath, sourceURL))
	require.NoError(t, runUp(dbPath, sourceURL))
	requireBaselineTableExists(t, dbPath)
}

func TestRunDownAfterUpRemovesBaseline(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	sourceURL := testMigrationSourceURL(t)

	require.NoError(t, runUp(dbPath, sourceURL))
	require.NoError(t, Run(context.Background(), Config{
		DatabaseDSN: dbPath,
		SourceURL:   sourceURL,
		Direction:   DirectionDown,
	}, nil))
	requireBaselineTableMissing(t, dbPath)
}

func TestRunReturnsErrorForInvalidSource(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")

	err := Run(context.Background(), Config{
		DatabaseDSN: dbPath,
		SourceURL:   "file://" + filepath.ToSlash(filepath.Join(t.TempDir(), "missing")),
		Direction:   DirectionUp,
	}, nil)
	require.Error(t, err)
}

func TestProductionMigrationCodeDoesNotCallAutoMigrate(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))

	paths := []string{
		filepath.Join(repoRoot, "internal", "server", "migration.go"),
		filepath.Join(repoRoot, "cmd", "migration", "main.go"),
		filepath.Join(repoRoot, "internal", "migration", "migration.go"),
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, strings.ReplaceAll(string(content), "TestProductionMigrationCodeDoesNotCallAutoMigrate", ""), "AutoMigrate", path)
	}
}

func runUp(dbPath string, sourceURL string) error {
	return Run(context.Background(), Config{
		DatabaseDSN: dbPath,
		SourceURL:   sourceURL,
		Direction:   DirectionUp,
	}, nil)
}

func requireBaselineTableExists(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var tableName string
	err = db.QueryRowContext(context.Background(), `SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'shiliu_migration_baseline'`).Scan(&tableName)
	require.NoError(t, err)
	require.Equal(t, "shiliu_migration_baseline", tableName)
}

func requireBaselineTableMissing(t *testing.T, dbPath string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'shiliu_migration_baseline'`).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
}

func testMigrationSourceURL(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	return "file://" + filepath.ToSlash(filepath.Join(repoRoot, "migrations"))
}
