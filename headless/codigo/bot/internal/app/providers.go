package app

import (
	"context"
	"crypto/tls"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/grpcserver"
	"ddbot/bot/internal/script"
	"ddbot/bot/internal/session"
)

func loadLibrary(config Config) (script.Options, error) {
	return (ScriptResources{LibraryPath: config.LuaDirectory}).load()
}

func newRegistry(lifecycle fx.Lifecycle, config Config, options script.Options) (*session.Registry, error) {
	if config.Captcha.Enabled {
		solver, err := loadCaptcha(lifecycle, config.Captcha)
		if err != nil {
			return nil, err
		}
		options.CaptchaSolver = solver
	}

	registry, err := session.NewRegistry(options, session.Limits{
		Sessions:  config.MaxSessions,
		PerTenant: config.PerTenant,
		PerRelay:  config.PerRelay,
		Grace:     config.Grace,
		Timeout:   config.Timeout,
	})
	if err != nil {
		return nil, err
	}

	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		registry.Close()

		return nil
	}})

	return registry, nil
}

func newGRPCServer(config Config, handler *grpcserver.Server) (*grpc.Server, error) {
	options := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(contract.MaxMessageBytes),
		grpc.MaxSendMsgSize(contract.MaxMessageBytes),
		grpc.MaxConcurrentStreams(uint32(config.MaxSessions)),
		grpc.KeepaliveParams(keepalive.ServerParameters{Time: 20 * time.Second, Timeout: 10 * time.Second}),
	}
	if config.TLS.Cert != "" {
		certificate, err := tls.LoadX509KeyPair(config.TLS.Cert, config.TLS.Key)
		if err != nil {
			return nil, &RuntimeError{Operation: "load TLS certificate", Cause: err}
		}

		options = append(options, grpc.Creds(credentials.NewTLS(&tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{certificate},
		})))
	}

	server := grpc.NewServer(options...)
	pb.RegisterBotServer(server, handler)
	pb.RegisterSessionMetricsServer(server, handler)
	pb.RegisterSessionObservationServer(server, handler)

	return server, nil
}

func registerLifecycle(lifecycle fx.Lifecycle, runtime *Runtime) {
	lifecycle.Append(fx.Hook{OnStart: runtime.Start, OnStop: runtime.Stop})
}
