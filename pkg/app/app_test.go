package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunReturnsServerStartError(t *testing.T) {
	startErr := errors.New("invalid schedule")
	srv := &failingStartServer{err: startErr}
	app := NewApp(WithServer(srv))

	err := app.Run(context.Background())

	require.ErrorIs(t, err, startErr)
	require.True(t, srv.stopped.Load())
	require.NoError(t, srv.stopContextErr)
}

type failingStartServer struct {
	err            error
	stopped        atomic.Bool
	stopContextErr error
}

func (s *failingStartServer) Start(context.Context) error {
	return s.err
}

func (s *failingStartServer) Stop(ctx context.Context) error {
	s.stopped.Store(true)
	if err := ctx.Err(); err != nil {
		s.stopContextErr = err
	}
	return nil
}
