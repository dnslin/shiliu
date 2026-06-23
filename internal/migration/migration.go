package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	sqlitemigrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"

	"shiliu/pkg/log"
)

type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
)

type Config struct {
	DatabaseDSN string
	SourceURL   string
	Direction   Direction
}

func Run(ctx context.Context, cfg Config, logger *log.Logger) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if cfg.Direction == "" {
		cfg.Direction = DirectionUp
	}
	if err := validateDirection(cfg.Direction); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.DatabaseDSN) == "" {
		return errors.New("sqlite migration dsn is required")
	}
	if cfg.SourceURL == "" {
		cfg.SourceURL = FileSourceURL("migrations")
	}

	db, err := sql.Open("sqlite", cfg.DatabaseDSN)
	if err != nil {
		return err
	}
	driver, err := sqlitemigrate.WithInstance(db, &sqlitemigrate.Config{})
	if err != nil {
		_ = db.Close()
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(cfg.SourceURL, "sqlite", driver)
	if err != nil {
		_ = db.Close()
		return err
	}
	defer func() {
		sourceErr, databaseErr := m.Close()
		err = mergeMigrationCloseError(err, sourceErr, databaseErr)
	}()

	stopForwarding := forwardContextCancel(ctx, m)
	defer stopForwarding()

	switch cfg.Direction {
	case DirectionUp:
		err = m.Up()
	case DirectionDown:
		err = m.Steps(-1)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		if logger != nil {
			logger.Info("database migrations already current", zap.String("direction", string(cfg.Direction)))
		}
		return nil
	}
	if err != nil {
		return err
	}
	if logger != nil {
		logger.Info("database migrations applied", zap.String("direction", string(cfg.Direction)))
	}
	return nil
}

func ValidateSQLiteDriver(driver string) error {
	if driver != "" && driver != "sqlite" {
		return fmt.Errorf("unsupported db driver %q: only sqlite is supported", driver)
	}
	return nil
}

func validateDirection(direction Direction) error {
	switch direction {
	case DirectionUp, DirectionDown:
		return nil
	default:
		return fmt.Errorf("unsupported migration direction %q: use up or down", direction)
	}
}

func FileSourceURL(path string) string {
	if strings.TrimSpace(path) == "" {
		path = "migrations"
	}
	if strings.HasPrefix(path, "file://") {
		return path
	}

	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		absPath = cleanPath
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(absPath)}).String()
}

func forwardContextCancel(ctx context.Context, m *migrate.Migrate) func() {
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			select {
			case m.GracefulStop <- true:
			default:
			}
		case <-stop:
		}
	}()
	return func() { close(stop) }
}

func mergeMigrationCloseError(primary error, sourceErr error, databaseErr error) error {
	if primary != nil {
		return primary
	}
	return errors.Join(sourceErr, databaseErr)
}
