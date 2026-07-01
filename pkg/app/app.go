package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"shiliu/pkg/server"
	"syscall"
)

type App struct {
	name    string
	servers []server.Server
}

type Option func(a *App)

func NewApp(opts ...Option) *App {
	a := &App{}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func WithServer(servers ...server.Server) Option {
	return func(a *App) {
		a.servers = servers
	}
}

func WithName(name string) Option {
	return func(a *App) {
		a.name = name
	}
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	serverErrors := make(chan error, len(a.servers))
	for _, srv := range a.servers {
		go func(srv server.Server) {
			err := srv.Start(ctx)
			if err != nil {
				select {
				case serverErrors <- err:
				case <-ctx.Done():
				}
			}
		}(srv)
	}

	var runErr error
	select {
	case <-signals:
		// Received termination signal
		log.Println("Received termination signal")
	case <-ctx.Done():
		// Context canceled
		log.Println("Context canceled")
	case err := <-serverErrors:
		runErr = fmt.Errorf("server start err: %w", err)
		log.Printf("%v", runErr)
	}

	cancel()

	// Gracefully stop the servers
	shutdownCtx := context.WithoutCancel(ctx)
	for _, srv := range a.servers {
		err := srv.Stop(shutdownCtx)
		if err != nil {
			log.Printf("Server stop err: %v", err)
			runErr = errors.Join(runErr, err)
		}
	}

	return runErr
}
