package app

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"time"

	"go.uber.org/fx"

	"ddbot/bot/internal/grpcserver"
)

const (
	startupTimeout  = 30 * time.Second
	shutdownTimeout = 10 * time.Second
)

func Run(args []string, output io.Writer) error {
	if len(args) == 0 || args[0] != "serve" {
		return &ConfigurationError{Field: "command", Reason: "ddbot serve -config bot.yaml"}
	}

	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(output)
	path := flags.String("config", "bot.yaml", "bot YAML configuration")
	if err := flags.Parse(args[1:]); err != nil {
		return &ConfigurationError{Field: "arguments", Cause: err}
	}

	if flags.NArg() != 0 {
		return &ConfigurationError{Field: "arguments", Reason: "unexpected arguments"}
	}

	config, keys, err := LoadConfig(*path)
	if err != nil {
		return err
	}

	application := fx.New(
		fx.NopLogger,
		fx.Supply(config, keys, slog.New(slog.NewJSONHandler(output, nil))),
		fx.Provide(loadLibrary, newRegistry, grpcserver.New, newGRPCServer, newRuntime),
		fx.Invoke(registerLifecycle),
	)
	start, cancelStart := context.WithTimeout(context.Background(), startupTimeout)
	defer cancelStart()
	if err := application.Start(start); err != nil {
		return err
	}

	<-application.Wait()

	stop, cancelStop := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelStop()

	return application.Stop(stop)
}
