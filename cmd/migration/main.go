package main

import (
	"context"
	"flag"

	"shiliu/internal/migration"
	"shiliu/pkg/config"
	"shiliu/pkg/log"
)

func main() {
	envConf := flag.String("conf", "config/local.yml", "config path, eg: -conf ./config/local.yml")
	direction := flag.String("direction", string(migration.DirectionUp), "migration direction: up or down")
	migrationPath := flag.String("path", "migrations", "migration directory or file:// source URL; relative paths resolve from the current working directory")
	flag.Parse()

	conf := config.NewConfig(*envConf)
	logger := log.NewLog(conf)

	if err := migration.ValidateSQLiteDriver(conf.GetString("data.db.user.driver")); err != nil {
		panic(err)
	}

	err := migration.Run(context.Background(), migration.Config{
		DatabaseDSN: conf.GetString("data.db.user.dsn"),
		SourceURL:   migration.FileSourceURL(*migrationPath),
		Direction:   migration.Direction(*direction),
	}, logger)
	if err != nil {
		panic(err)
	}
}
