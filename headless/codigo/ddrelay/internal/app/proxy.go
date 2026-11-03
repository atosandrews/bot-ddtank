package app

import (
	"context"
	"io"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/fx"

	"ddbot/relay/internal/control"
	"ddbot/relay/internal/packets"
)

func runProxy(config ProxyConfig, output io.Writer) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	application := fx.New(
		fx.NopLogger,
		fx.Supply(config, slog.New(slog.NewJSONHandler(output, nil))),
		fx.Provide(newBotClient, loadStartupScript, newRecorder, control.NewRegistry, packets.NewDecoder, newProxyRuntime),
		fx.Invoke(registerProxyLifecycle),
	)
	start, cancelStart := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelStart()
	if err := application.Start(start); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
	case <-application.Wait():
	}

	stop, cancelStop := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelStop()

	return application.Stop(stop)
}

func registerProxyLifecycle(lifecycle fx.Lifecycle, shutdown fx.Shutdowner, runtime *proxyRuntime) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return runtime.start(ctx, shutdown) },
		OnStop:  func(context.Context) error { return runtime.stop() },
	})
}
