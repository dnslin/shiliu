package migration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
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

func Run(ctx context.Context, cfg Config, logger *log.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	databaseURL, err := sqliteURLFromDSN(cfg.DatabaseDSN)
	if err != nil {
		return err
	}
	if cfg.SourceURL == "" {
		cfg.SourceURL = "file://migrations"
	}
	if cfg.Direction == "" {
		cfg.Direction = DirectionUp
	}

	m, err := migrate.New(cfg.SourceURL, databaseURL)
	if err != nil {
		return err
	}
	defer func() {
		sourceErr, databaseErr := m.Close()
		if logger == nil {
			return
		}
		if sourceErr != nil {
			logger.Error("migration source close error", zap.Error(sourceErr))
		}
		if databaseErr != nil {
			logger.Error("migration database close error", zap.Error(databaseErr))
		}
	}()

	switch cfg.Direction {
	case DirectionUp:
		err = m.Up()
	case DirectionDown:
		err = m.Down()
	default:
		return fmt.Errorf("unsupported migration direction %q: use up or down", cfg.Direction)
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

func sqliteURLFromDSN(dsn string) (string, error) {
	if strings.TrimSpace(dsn) == "" {
		return "", errors.New("sqlite migration dsn is required")
	}
	if strings.HasPrefix(dsn, "sqlite://") {
		return dsn, nil
	}
	return "sqlite://" + strings.ReplaceAll(dsn, "\\", "/"), nil
}

func FileSourceURL(path string) string {
	if strings.TrimSpace(path) == "" {
		path = "migrations"
	}
	if strings.HasPrefix(path, "file://") {
		return path
	}
	return "file://" + strings.ReplaceAll(path, "\\", "/")
}
