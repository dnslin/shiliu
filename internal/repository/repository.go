package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"shiliu/pkg/log"
	"shiliu/pkg/zapgorm2"
)

const ctxTxKey = "TxKey"

type Repository struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewRepository(
	logger *log.Logger,
	db *gorm.DB,
) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

type Transaction interface {
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}

func NewTransaction(r *Repository) Transaction {
	return r
}

// DB return tx
// If you need to create a Transaction, you must call DB(ctx) and Transaction(ctx,fn)
func (r *Repository) DB(ctx context.Context) *gorm.DB {
	v := ctx.Value(ctxTxKey)
	if v != nil {
		if tx, ok := v.(*gorm.DB); ok {
			return tx
		}
	}
	return r.db.WithContext(ctx)
}

func (r *Repository) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		ctx = context.WithValue(ctx, ctxTxKey, tx)
		return fn(ctx)
	})
}

func NewDB(conf *viper.Viper, l *log.Logger) *gorm.DB {
	driver := conf.GetString("data.db.user.driver")
	if driver != "" && driver != "sqlite" {
		panic(fmt.Sprintf("unsupported db driver %q: only sqlite is supported", driver))
	}

	logger := zapgorm2.New(l.Logger)
	dsn := conf.GetString("data.db.user.dsn")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger,
	})
	if err != nil {
		panic(err)
	}
	if conf.GetBool("data.db.user.debug") {
		db = db.Debug()
	}

	// Connection Pool config
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db
}
