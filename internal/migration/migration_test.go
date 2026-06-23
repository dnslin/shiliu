package migration

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
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

func TestRunUpSupportsReservedCharactersInSQLiteFilename(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration # file.db")

	require.NoError(t, runUp(dbPath+"?_busy_timeout=5000", testMigrationSourceURL(t)))
	require.FileExists(t, dbPath)
	requireBaselineTableExists(t, dbPath)
	require.NoFileExists(t, strings.Split(dbPath, "#")[0])
}

func TestRunUpSupportsReservedCharactersInFileSourcePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "migration # source")
	copyTestMigrations(t, dir)
	dbPath := filepath.Join(t.TempDir(), "migration.db")

	require.NoError(t, runUp(dbPath, FileSourceURL(dir)))
	requireBaselineTableExists(t, dbPath)
}

func TestFileSourceURLResolvesRelativePathFromWorkingDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	require.Equal(t, (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(cwd, "migrations"))}).String(), FileSourceURL("migrations"))
}

func TestFileSourceURLNormalizesBackslashes(t *testing.T) {
	sourceURL := FileSourceURL(`nested\migration # source`)

	require.True(t, strings.HasPrefix(sourceURL, "file://"))
	require.NotContains(t, sourceURL, `\`)
	require.Contains(t, sourceURL, "migration%20%23%20source")
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

func TestRunDownRollsBackOneMigrationBoundary(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "multi migrations")
	writeMigrationFile(t, sourceDir, "000001_first.up.sql", `CREATE TABLE first_boundary (id INTEGER PRIMARY KEY);`)
	writeMigrationFile(t, sourceDir, "000001_first.down.sql", `DROP TABLE first_boundary;`)
	writeMigrationFile(t, sourceDir, "000002_second.up.sql", `CREATE TABLE second_boundary (id INTEGER PRIMARY KEY);`)
	writeMigrationFile(t, sourceDir, "000002_second.down.sql", `DROP TABLE second_boundary;`)
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	sourceURL := FileSourceURL(sourceDir)

	require.NoError(t, runUp(dbPath, sourceURL))
	requireTableExists(t, dbPath, "first_boundary")
	requireTableExists(t, dbPath, "second_boundary")

	require.NoError(t, Run(context.Background(), Config{
		DatabaseDSN: dbPath,
		SourceURL:   sourceURL,
		Direction:   DirectionDown,
	}, nil))
	requireTableExists(t, dbPath, "first_boundary")
	requireTableMissing(t, dbPath, "second_boundary")
}

func TestRunReturnsErrorForInvalidSource(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")

	err := Run(context.Background(), Config{
		DatabaseDSN: dbPath,
		SourceURL:   FileSourceURL(filepath.Join(t.TempDir(), "missing")),
		Direction:   DirectionUp,
	}, nil)
	require.Error(t, err)
}

func TestRunValidatesUnsupportedDirectionBeforeOpeningSource(t *testing.T) {
	err := Run(context.Background(), Config{
		DatabaseDSN: filepath.Join(t.TempDir(), "migration.db"),
		SourceURL:   FileSourceURL(filepath.Join(t.TempDir(), "missing")),
		Direction:   Direction("sideways"),
	}, nil)

	require.ErrorContains(t, err, "unsupported migration direction")
	require.NotContains(t, err.Error(), "no such file")
}

func TestValidateSQLiteDriverRejectsUnsupportedDriver(t *testing.T) {
	require.NoError(t, ValidateSQLiteDriver(""))
	require.NoError(t, ValidateSQLiteDriver("sqlite"))
	require.ErrorContains(t, ValidateSQLiteDriver("mysql"), "only sqlite is supported")
}

func TestMergeMigrationCloseError(t *testing.T) {
	primary := errors.New("primary")
	sourceClose := errors.New("source close")
	databaseClose := errors.New("database close")

	require.Same(t, primary, mergeMigrationCloseError(primary, sourceClose, databaseClose))
	merged := mergeMigrationCloseError(nil, sourceClose, databaseClose)
	require.ErrorIs(t, merged, sourceClose)
	require.ErrorIs(t, merged, databaseClose)
	require.NoError(t, mergeMigrationCloseError(nil, nil, nil))
}

func TestProductionEntrypointsDoNotRunMigrationsImplicitly(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))

	paths := []string{
		filepath.Join(repoRoot, "internal", "server", "migration.go"),
		filepath.Join(repoRoot, "cmd", "migration", "main.go"),
		filepath.Join(repoRoot, "internal", "migration", "migration.go"),
		filepath.Join(repoRoot, "cmd", "server", "main.go"),
		filepath.Join(repoRoot, "cmd", "server", "wire", "wire.go"),
		filepath.Join(repoRoot, "cmd", "server", "wire", "wire_gen.go"),
		filepath.Join(repoRoot, "cmd", "task", "main.go"),
		filepath.Join(repoRoot, "cmd", "task", "wire", "wire.go"),
		filepath.Join(repoRoot, "cmd", "task", "wire", "wire_gen.go"),
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		text := strings.ReplaceAll(string(content), "TestProductionEntrypointsDoNotRunMigrationsImplicitly", "")
		require.NotContains(t, text, "AutoMigrate", path)
		if strings.Contains(path, filepath.Join("cmd", "server")) || strings.Contains(path, filepath.Join("cmd", "task")) {
			require.NotContains(t, text, `"shiliu/internal/migration"`, path)
			require.NotContains(t, text, "migration.Run", path)
		}
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
	requireTableExists(t, dbPath, "shiliu_migration_baseline")
}

func requireBaselineTableMissing(t *testing.T, dbPath string) {
	t.Helper()
	requireTableMissing(t, dbPath, "shiliu_migration_baseline")
}

func requireTableExists(t *testing.T, dbPath string, tableName string) {
	t.Helper()
	require.Equal(t, 1, tableCount(t, dbPath, tableName))
}

func requireTableMissing(t *testing.T, dbPath string, tableName string) {
	t.Helper()
	require.Zero(t, tableCount(t, dbPath, tableName))
}

func tableCount(t *testing.T, dbPath string, tableName string) int {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName).Scan(&count)
	require.NoError(t, err)
	return count
}

func testMigrationSourceURL(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	return FileSourceURL(filepath.Join(repoRoot, "migrations"))
}

func copyTestMigrations(t *testing.T, destDir string) {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	sourceDir := filepath.Join(repoRoot, "migrations")
	entries, err := os.ReadDir(sourceDir)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(destDir, 0o755))
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(destDir, entry.Name()), content, 0o644))
	}
}

func writeMigrationFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}
