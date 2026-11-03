package app

import (
	"context"
	"crypto/tls"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/transport"
)

func newBotClient(config ProxyConfig, lifecycle fx.Lifecycle) (pb.BotClient, error) {
	security := grpc.WithTransportCredentials(insecure.NewCredentials())
	if config.BotTLS {
		security = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	connection, err := grpc.NewClient(config.BotAddress, security, grpc.WithDisableRetry(), grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(contract.MaxMessageBytes),
		grpc.MaxCallSendMsgSize(contract.MaxMessageBytes),
	))
	if err != nil {
		return nil, &transport.Error{Operation: "bot connection", Cause: err}
	}

	lifecycle.Append(fx.StopHook(connection.Close))

	return pb.NewBotClient(connection), nil
}

func loadStartupScript(config ProxyConfig) ([]byte, error) {
	if config.ScriptPath == "" {
		return nil, nil
	}

	return readSource(config.ScriptPath, contract.MaxScriptBytes)
}

func newRecorder(config ProxyConfig, lifecycle fx.Lifecycle) Recorder {
	recorder := &lockedRecorder{recorder: discardRecorder{}}
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			opened, err := openRecorder(config, time.Now())
			if err != nil {
				return err
			}

			recorder.recorder = opened

			return nil
		},
		OnStop: func(context.Context) error { return recorder.Close() },
	})

	return recorder
}
