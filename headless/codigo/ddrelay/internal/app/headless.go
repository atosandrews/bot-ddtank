package app

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"time"

	"go.uber.org/fx"

	"ddbot/relay/internal/headless"
)

func runHeadless(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	flags.SetOutput(output)
	path := flags.String("config", "headless.yaml", "headless character configuration")
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

	session, err := headless.LoadAccountSession(config.SessionFile)
	if err != nil {
		return err
	}

	application := fx.New(
		fx.NopLogger,
		fx.Supply(config, session, slog.New(slog.NewJSONHandler(output, nil))),
		fx.Provide(newHeadlessHTTPClient, newHeadlessDialer, headless.NewAuthenticator, headless.NewPeer, newHeadlessRuntime),
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

func newHeadlessHTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, &headless.Error{Operation: "initialize", Reason: "cannot initialize cookie jar", Cause: err}
	}

	return &http.Client{
		Timeout:       20 * time.Second,
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
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
