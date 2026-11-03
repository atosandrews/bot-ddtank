package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"

	"ddbot/bot/internal/session"
)

const sessionExpiryInterval = time.Second

type Runtime struct {
	config     Config
	registry   *session.Registry
	server     *grpc.Server
	shutdown   fx.Shutdowner
	logger     *slog.Logger
	cancel     context.CancelFunc
	done       chan struct{}
	expired    chan struct{}
	serveError error
}

func newRuntime(config Config, registry *session.Registry, server *grpc.Server, shutdown fx.Shutdowner, logger *slog.Logger) *Runtime {
	return &Runtime{
		config:   config,
		registry: registry,
		server:   server,
		shutdown: shutdown,
		logger:   logger,
		done:     make(chan struct{}),
		expired:  make(chan struct{}),
	}
}

func (r *Runtime) Start(ctx context.Context) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", r.config.Listen)
	if err != nil {
		return &RuntimeError{Operation: "listen", Cause: err}
	}

	background, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.expire(background)
	go r.serve(listener)
	r.logger.InfoContext(ctx, "bot listening", "address", listener.Addr().String())

	return nil
}

func (r *Runtime) Stop(ctx context.Context) error {
	r.cancel()
	r.server.Stop()

	select {
	case <-r.done:
	case <-ctx.Done():
		return &RuntimeError{Operation: "stop server", Cause: ctx.Err()}
	}

	select {
	case <-r.expired:
	case <-ctx.Done():
		return &RuntimeError{Operation: "stop expiry worker", Cause: ctx.Err()}
	}

	if r.serveError != nil {
		return &RuntimeError{Operation: "serve", Cause: r.serveError}
	}

	return nil
}

func (r *Runtime) serve(listener net.Listener) {
	defer close(r.done)
	r.serveError = r.server.Serve(listener)
	if err := r.shutdown.Shutdown(); err != nil {
		r.serveError = errors.Join(r.serveError, err)
	}
}

func (r *Runtime) expire(ctx context.Context) {
	defer close(r.expired)
	ticker := time.NewTicker(sessionExpiryInterval)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			r.registry.Expire(now)
		case <-ctx.Done():
			return
		}
	}
}
