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
func TestRunUpAppliesFeedsAndContentItems(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))

	requireTableExists(t, dbPath, "feeds")
	requireTableExists(t, dbPath, "content_items")
	requireIndexExists(t, dbPath, "idx_feeds_feed_url")
	requireIndexExists(t, dbPath, "idx_content_items_feed_dedupe_key")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	ctx := context.Background()
	result, err := db.ExecContext(ctx, `INSERT INTO feeds (feed_url, type, fetch_status) VALUES (?, ?, ?)`, "https://example.com/feed.xml", "rss", "idle")
	require.NoError(t, err)
	feedID, err := result.LastInsertId()
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO feeds (feed_url, type, fetch_status) VALUES (?, ?, ?)`, "https://example.com/feed.xml", "rss", "idle")
	require.Error(t, err)
	require.Contains(t, err.Error(), "UNIQUE")

	_, err = db.ExecContext(ctx, `INSERT INTO content_items (
		feed_id, dedupe_key, type, title, description, content, show_notes,
		description_safe, content_safe, show_notes_safe, available_text
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		feedID, "episode-1", "audio", "Episode 1", "Raw description", "Raw content", "Raw notes",
		"Safe description", "Safe content", "Safe notes", "Episode 1 Safe notes",
	)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO content_items (feed_id, dedupe_key, type, title, available_text) VALUES (?, ?, ?, ?, ?)`, feedID, "episode-1", "audio", "Duplicate", "Duplicate")
	require.Error(t, err)
	require.Contains(t, err.Error(), "UNIQUE")

	_, err = db.ExecContext(ctx, `INSERT INTO content_items (feed_id, dedupe_key, type, title, available_text) VALUES (?, ?, ?, ?, ?)`, feedID+1, "orphan", "text", "Orphan", "Orphan")
	require.Error(t, err)
}

func TestRunUpAppliesContentItemSearchIndex(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))

	requireTableExists(t, dbPath, "content_item_search_index")
	requireFeedColumnExists(t, dbPath, "title")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	ctx := context.Background()
	result, err := db.ExecContext(ctx, `INSERT INTO feeds (feed_url, title, type, fetch_status) VALUES (?, ?, ?, ?)`, "https://example.com/feed.xml", "工程日报", "rss", "idle")
	require.NoError(t, err)
	feedID, err := result.LastInsertId()
	require.NoError(t, err)
	result, err = db.ExecContext(ctx, `INSERT INTO content_items (feed_id, dedupe_key, type, title, available_text) VALUES (?, ?, ?, ?, ?)`, feedID, "fts-item", "text", "SQLite FTS5 入门", "开发者 中文字段 可以检索")
	require.NoError(t, err)
	itemID, err := result.LastInsertId()
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO content_item_search_index (rowid, title, feed_title, available_text, ai_summary_markdown) VALUES (?, ?, ?, ?, ?)`, itemID, "SQLite FTS5 入门", "工程日报", "开发者 中文字段 可以检索", "## 摘要\n自托管 搜索")
	require.NoError(t, err)

	for _, query := range []string{"SQLite", "工程日报", "中文字段", "自托管"} {
		var rowID int64
		err = db.QueryRowContext(ctx, `SELECT rowid FROM content_item_search_index WHERE content_item_search_index MATCH ?`, query).Scan(&rowID)
		require.NoError(t, err, query)
		require.Equal(t, itemID, rowID, query)
	}
}

func TestRunUpAppliesTagsAndContentItemTags(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))

	requireTableExists(t, dbPath, "tags")
	requireTableExists(t, dbPath, "content_item_tags")
	requireIndexExists(t, dbPath, "idx_tags_name")
	requireIndexExists(t, dbPath, "idx_content_item_tags_item_tag")
	requireIndexExists(t, dbPath, "idx_content_item_tags_tag_id")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	ctx := context.Background()
	feedResult, err := db.ExecContext(ctx, `INSERT INTO feeds (feed_url, title, type, fetch_status) VALUES (?, ?, ?, ?)`, "https://example.com/tag-feed.xml", "Tag Feed", "rss", "idle")
	require.NoError(t, err)
	feedID, err := feedResult.LastInsertId()
	require.NoError(t, err)
	itemResult, err := db.ExecContext(ctx, `INSERT INTO content_items (feed_id, dedupe_key, type, title, available_text) VALUES (?, ?, ?, ?, ?)`, feedID, "tagged-item", "text", "Tagged item", "Tagged item")
	require.NoError(t, err)
	itemID, err := itemResult.LastInsertId()
	require.NoError(t, err)
	tagResult, err := db.ExecContext(ctx, `INSERT INTO tags (name) VALUES (?)`, "sqlite")
	require.NoError(t, err)
	tagID, err := tagResult.LastInsertId()
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO tags (name) VALUES (?)`, "sqlite")
	require.Error(t, err)
	require.Contains(t, err.Error(), "UNIQUE")

	_, err = db.ExecContext(ctx, `INSERT INTO content_item_tags (content_item_id, tag_id) VALUES (?, ?)`, itemID, tagID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM tags WHERE id = ?`, tagID)
	require.NoError(t, err)
	var itemCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM content_items WHERE id = ?`, itemID).Scan(&itemCount))
	require.Equal(t, 1, itemCount)
	var relationCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM content_item_tags WHERE content_item_id = ?`, itemID).Scan(&relationCount))
	require.Zero(t, relationCount)
}

func TestRunUpAppliesFolders(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))

	requireTableExists(t, dbPath, "folders")
	requireIndexExists(t, dbPath, "idx_folders_name")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `INSERT INTO folders (name) VALUES (?)`, "Engineering")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO folders (name) VALUES (?)`, "Engineering")
	require.Error(t, err)
	require.Contains(t, err.Error(), "UNIQUE")
}
func TestRunUpAppliesAIServiceConfigs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))

	requireTableExists(t, dbPath, "ai_service_configs")
	requireIndexExists(t, dbPath, "idx_ai_service_configs_singleton")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `INSERT INTO ai_service_configs (singleton_id, api_base_url, model, api_key) VALUES (1, ?, ?, ?)`, "https://api.example.com/v1", "gpt-4.1-mini", "sk-secret")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO ai_service_configs (singleton_id, api_base_url, model, api_key) VALUES (2, ?, ?, ?)`, "https://api.other.test/v1", "other-model", "other-secret")
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK")
	_, err = db.ExecContext(ctx, `INSERT INTO ai_service_configs (singleton_id, api_base_url, model, api_key) VALUES (1, ?, ?, ?)`, "https://api.duplicate.test/v1", "duplicate-model", "duplicate-secret")
	require.Error(t, err)
	require.Contains(t, err.Error(), "UNIQUE")
}

func TestRunUpAppliesAutoSummaryConfigs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))

	requireTableExists(t, dbPath, "auto_summary_configs")
	requireIndexExists(t, dbPath, "idx_auto_summary_configs_singleton")

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()

	_, err = db.ExecContext(ctx, `INSERT INTO auto_summary_configs (singleton_id, enabled, content_type_scope, enabled_at) VALUES (1, 1, ?, CURRENT_TIMESTAMP)`, "all")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO auto_summary_configs (singleton_id, enabled, content_type_scope) VALUES (2, 0, ?)`, "text")
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK")
	_, err = db.ExecContext(ctx, `INSERT INTO auto_summary_configs (singleton_id, enabled, content_type_scope) VALUES (1, 0, ?)`, "text")
	require.Error(t, err)
	require.Contains(t, err.Error(), "UNIQUE")
	_, err = db.ExecContext(ctx, `UPDATE auto_summary_configs SET content_type_scope = ? WHERE singleton_id = 1`, "video")
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK")
	_, err = db.ExecContext(ctx, `UPDATE auto_summary_configs SET enabled = ? WHERE singleton_id = 1`, 2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK")
}

func TestRunUpAppliesContentItemAISummaryFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	for _, column := range []string{"ai_summary_markdown", "ai_summary_status", "ai_summary_generated_at", "ai_summary_error"} {
		requireContentItemsColumnExists(t, dbPath, column)
	}

	ctx := context.Background()
	feedResult, err := db.ExecContext(ctx, `INSERT INTO feeds (feed_url, title, type, fetch_status) VALUES (?, ?, ?, ?)`, "https://example.com/ai-summary.xml", "AI Summary Feed", "rss", "idle")
	require.NoError(t, err)
	feedID, err := feedResult.LastInsertId()
	require.NoError(t, err)
	itemResult, err := db.ExecContext(ctx, `INSERT INTO content_items (feed_id, dedupe_key, type, title, available_text) VALUES (?, ?, ?, ?, ?)`, feedID, "summary-item", "text", "Summary item", "Long enough available text for a future AI summary")
	require.NoError(t, err)
	itemID, err := itemResult.LastInsertId()
	require.NoError(t, err)

	var markdown string
	var status string
	var generatedAt any
	var summaryError string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT ai_summary_markdown, ai_summary_status, ai_summary_generated_at, ai_summary_error FROM content_items WHERE id = ?`, itemID).Scan(&markdown, &status, &generatedAt, &summaryError))
	require.Equal(t, "", markdown)
	require.Equal(t, "none", status)
	require.Nil(t, generatedAt)
	require.Equal(t, "", summaryError)

	_, err = db.ExecContext(ctx, `UPDATE content_items SET ai_summary_status = ? WHERE id = ?`, "queued", itemID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CHECK")
}

func TestRunUpBackfillsExistingContentItemSearchIndex(t *testing.T) {
	initialSourceDir := filepath.Join(t.TempDir(), "initial migrations")
	copyTestMigrations(t, initialSourceDir)
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000006_content_item_search_index.up.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000006_content_item_search_index.down.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000007_tags.up.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000007_tags.down.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000008_folders.up.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000008_folders.down.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000009_ai_service_configs.up.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000009_ai_service_configs.down.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000010_content_item_ai_summary.up.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000010_content_item_ai_summary.down.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000011_auto_summary_configs.up.sql")))
	require.NoError(t, os.Remove(filepath.Join(initialSourceDir, "000011_auto_summary_configs.down.sql")))
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	require.NoError(t, runUp(dbPath, FileSourceURL(initialSourceDir)))

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	ctx := context.Background()
	result, err := db.ExecContext(ctx, `INSERT INTO feeds (feed_url, type, fetch_status) VALUES (?, ?, ?)`, "https://example.com/existing.xml", "rss", "idle")
	require.NoError(t, err)
	feedID, err := result.LastInsertId()
	require.NoError(t, err)
	result, err = db.ExecContext(ctx, `INSERT INTO content_items (feed_id, dedupe_key, type, title, available_text) VALUES (?, ?, ?, ?, ?)`, feedID, "existing-item", "text", "SQLite FTS5 既有条目", "开发者 中文字段 可以检索")
	require.NoError(t, err)
	itemID, err := result.LastInsertId()
	require.NoError(t, err)

	require.NoError(t, runUp(dbPath, testMigrationSourceURL(t)))

	for _, query := range []string{"SQLite", "中文字段"} {
		var rowID int64
		err = db.QueryRowContext(ctx, `SELECT rowid FROM content_item_search_index WHERE content_item_search_index MATCH ?`, query).Scan(&rowID)
		require.NoError(t, err, query)
		require.Equal(t, itemID, rowID, query)
	}
}

func TestRunDownAfterUpRollsBackLatestCheckedInBoundary(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	sourceURL := testMigrationSourceURL(t)

	require.NoError(t, runUp(dbPath, sourceURL))
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	folderResult, err := db.ExecContext(ctx, `INSERT INTO folders (name) VALUES (?)`, "Rollback folder")
	require.NoError(t, err)
	folderID, err := folderResult.LastInsertId()
	require.NoError(t, err)
	feedResult, err := db.ExecContext(ctx, `INSERT INTO feeds (feed_url, title, type, fetch_status, folder_id) VALUES (?, ?, ?, ?, ?)`, "https://example.com/folder-rollback.xml", "Folder rollback", "rss", "idle", folderID)
	require.NoError(t, err)
	feedID, err := feedResult.LastInsertId()
	require.NoError(t, err)
	require.NoError(t, Run(context.Background(), Config{
		DatabaseDSN: dbPath,
		SourceURL:   sourceURL,
		Direction:   DirectionDown,
	}, nil))
	var rolledBackFolderID sql.NullInt64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT folder_id FROM feeds WHERE id = ?`, feedID).Scan(&rolledBackFolderID))
	require.True(t, rolledBackFolderID.Valid)
	require.Equal(t, folderID, rolledBackFolderID.Int64)
	requireBaselineTableExists(t, dbPath)
	requireTableExists(t, dbPath, "users")
	requireIndexExists(t, dbPath, "idx_users_singleton")
	requireTableExists(t, dbPath, "feeds")
	requireTableExists(t, dbPath, "content_items")
	requireIndexExists(t, dbPath, "idx_feeds_feed_url")
	requireIndexExists(t, dbPath, "idx_content_items_feed_dedupe_key")
	requireIndexExists(t, dbPath, "idx_content_items_processing_status")
	requireIndexExists(t, dbPath, "idx_content_items_marked_later")
	requireIndexExists(t, dbPath, "idx_content_items_favorited")
	requireContentItemsColumnExists(t, dbPath, "processing_status")
	requireContentItemsColumnExists(t, dbPath, "marked_later")
	requireContentItemsColumnExists(t, dbPath, "favorited")
	requireTableExists(t, dbPath, "content_item_search_index")
	requireFeedColumnExists(t, dbPath, "title")
	requireTableExists(t, dbPath, "content_item_tags")
	requireTableExists(t, dbPath, "tags")
	requireIndexExists(t, dbPath, "idx_content_item_tags_item_tag")
	requireIndexExists(t, dbPath, "idx_content_item_tags_tag_id")
	requireIndexExists(t, dbPath, "idx_tags_name")
	requireTableExists(t, dbPath, "folders")
	requireIndexExists(t, dbPath, "idx_folders_name")
	requireTableExists(t, dbPath, "ai_service_configs")
	requireIndexExists(t, dbPath, "idx_ai_service_configs_singleton")
	requireTableExists(t, dbPath, "content_item_search_index")
	for _, column := range []string{"ai_summary_markdown", "ai_summary_status", "ai_summary_generated_at", "ai_summary_error"} {
		requireContentItemsColumnExists(t, dbPath, column)
	}
	requireTableMissing(t, dbPath, "auto_summary_configs")
	requireIndexMissing(t, dbPath, "idx_auto_summary_configs_singleton")
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

func requireIndexExists(t *testing.T, dbPath string, indexName string) {
	t.Helper()
	require.Equal(t, 1, indexCount(t, dbPath, indexName))
}

func requireIndexMissing(t *testing.T, dbPath string, indexName string) {
	t.Helper()
	require.Zero(t, indexCount(t, dbPath, indexName))
}

func requireContentItemsColumnMissing(t *testing.T, dbPath string, columnName string) {
	t.Helper()
	require.Zero(t, contentItemsColumnCount(t, dbPath, columnName))
}
func requireContentItemsColumnExists(t *testing.T, dbPath string, columnName string) {
	t.Helper()
	require.Equal(t, 1, contentItemsColumnCount(t, dbPath, columnName))
}

func contentItemsColumnCount(t *testing.T, dbPath string, columnName string) int {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM pragma_table_info('content_items') WHERE name = ?`, columnName).Scan(&count)
	require.NoError(t, err)
	return count
}
func requireFeedColumnExists(t *testing.T, dbPath string, columnName string) {
	t.Helper()
	require.Equal(t, 1, feedColumnCount(t, dbPath, columnName))
}
func requireFeedColumnMissing(t *testing.T, dbPath string, columnName string) {
	t.Helper()
	require.Zero(t, feedColumnCount(t, dbPath, columnName))
}

func feedColumnCount(t *testing.T, dbPath string, columnName string) int {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM pragma_table_info('feeds') WHERE name = ?`, columnName).Scan(&count)
	require.NoError(t, err)
	return count
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

func indexCount(t *testing.T, dbPath string, indexName string) int {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName).Scan(&count)
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
