package app

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"time"

	"go.uber.org/fx"

	"ddbot/relay/internal/headless"
)

func runHeadless(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	flags.SetOutput(output)
	path := flags.String("config", "headless.yaml", "headless character configuration")
	checkAuth := flags.Bool("check-auth", false, "authenticate HTTP only, without connecting to the game or retrying")
	transport := flags.String("transport", "", "override authentication transport: standard or launcher_chromium87")
	statusFile := flags.String("status-file", "", "override private authentication status JSON path")
	coordinationDir := flags.String("coordination-dir", "", "override shared private authentication cooldown directory")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() != 0 {
		return &headless.Error{Operation: "config", Reason: "unexpected arguments"}
	}

	config, err := headless.LoadConfig(*path)
	if err != nil {
		return err
	}

	if *transport != "" {
		if *transport != "standard" && *transport != "launcher_chromium87" {
			return &headless.Error{Operation: "config", Reason: "unknown authentication transport"}
		}
		config.Auth.Transport = *transport
	}
	if *statusFile != "" {
		config.Auth.StatusFile, err = filepath.Abs(*statusFile)
		if err != nil {
			return &headless.Error{Operation: "config", Reason: "invalid status path"}
		}
	}
	if *coordinationDir != "" {
		config.Auth.CoordinationDir, err = filepath.Abs(*coordinationDir)
		if err != nil {
			return &headless.Error{Operation: "config", Reason: "invalid coordination path"}
		}
	}
	logger := slog.New(slog.NewJSONHandler(output, nil))
	if *checkAuth {
		return checkHeadlessAuth(config, logger)
	}

	application := fx.New(
		fx.NopLogger,
		fx.Supply(config, logger),
		fx.Provide(newHeadlessDialer, newHeadlessAuthenticator, headless.NewPeer, newHeadlessRuntime),
		fx.Invoke(registerHeadlessLifecycle),
	)
	start, cancelStart := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelStart()
	if err := application.Start(start); err != nil {
		return err
	}

	<-application.Wait()

	stop, cancelStop := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelStop()

	return application.Stop(stop)
}

func checkHeadlessAuth(config headless.Config, logger *slog.Logger) error {
	auth, err := newHeadlessAuthenticator(config, logger)
	if err != nil {
		return err
	}
	defer auth.CloseIdleConnections()
	gate := headless.NewCoordinator(config.Auth.CoordinationDir)
	for _, character := range config.Characters {
		ctx, cancel := context.WithTimeout(context.Background(), headlessAttemptTimeout)
		next, authErr := gate.Do(ctx, "s1.ddclassico.com", func(ctx context.Context) error {
			session, loadErr := headless.LoadAccountSession(config.SessionFile)
			if loadErr != nil {
				return loadErr
			}
			_, authErr := auth.Authenticate(ctx, session, character)
			return authErr
		})
		cancel()
		if authErr != nil {
			attributes := []any{"character_id", character.ID, "error", authErr}
			if !next.IsZero() {
				attributes = append(attributes, "next_attempt_at", next.UTC())
			}
			logger.Warn("headless HTTP authentication check failed", attributes...)
			return authErr
		}
		logger.Info("headless HTTP authentication check confirmed", "character_id", character.ID, "transport", config.Auth.Transport)
	}
	return nil
}

func newHeadlessAuthenticator(config headless.Config, logger *slog.Logger) (*headless.Authenticator, error) {
	auth, err := headless.NewConfiguredAuthenticator(config)
	if err != nil {
		return nil, err
	}
	auth.Observe = func(event headless.HTTPObservation) {
		logger.Info("headless HTTP request", "stage", event.Stage, "host", event.Host,
			"path", event.Path, "status", event.StatusCode, "protocol", event.Protocol,
			"duration_ms", event.Duration.Milliseconds(), "ray_id", event.RayID, "kind", event.Kind)
	}
	return auth, nil
}

func newHeadlessDialer() *net.Dialer {
	return &net.Dialer{Timeout: 10 * time.Second}
}

func registerHeadlessLifecycle(lifecycle fx.Lifecycle, shutdown fx.Shutdowner, runtime *headlessRuntime) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { return runtime.start(ctx, shutdown) },
		OnStop:  func(context.Context) error { return runtime.stop() },
	})
}
