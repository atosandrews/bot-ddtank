package grpcserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
	"ddbot/bot/internal/session"
)

func TestAuthenticatedStreamsKeepTenantStateSeparate(t *testing.T) {
	registry, err := session.NewRegistry(script.Options{}, session.Limits{
		Sessions:  8,
		PerTenant: 4,
		PerRelay:  2,
		Grace:     time.Second,
		Timeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer registry.Close()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	pb.RegisterBotServer(server, New(registry, map[string]string{"one": "tenant-a", "two": "tenant-b", "alias": "tenant-a"}, slog.New(slog.NewJSONHandler(io.Discard, nil))))
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() {
		server.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	cc, err := grpc.NewClient("passthrough:///memory", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}

	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := pb.NewBotClient(cc)
	var bindings []string

	for _, key := range []string{"one", "two"} {
		stream, err := client.GameSession(metadata.AppendToOutgoingContext(ctx, "x-api-key", key))
		if err != nil {
			t.Fatal(err)
		}

		open := &pb.RelayMessage{Payload: &pb.RelayMessage_Open{Open: &pb.OpenSession{
			ApiVersion:      contract.APIVersion,
			CodecRevision:   contract.CodecRevision,
			RelayInstanceId: "relay",
			GameSessionId:   "game",
			LocalStream:     1,
		}}}
		if err := stream.Send(open); err != nil {
			t.Fatal(err)
		}

		answer, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}

		binding := answer.GetOpened()
		bindings = append(bindings, binding.BotSessionId)
		request := &pb.RelayMessage{BotSessionId: binding.BotSessionId, Generation: binding.Generation, Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "eval", Source: `count=(count or 0)+1;return count`}}}
		if err := stream.Send(request); err != nil {
			t.Fatal(err)
		}

		answer, err = stream.Recv()
		if err != nil {
			t.Fatal(err)
		}

		value, err := contract.DecodeValue(answer.GetResult().Value)
		if err != nil || value != float64(1) {
			t.Fatalf("tenant state %v %v", value, err)
		}

		request.BotSessionId = "foreign"
		if err := stream.Send(request); err != nil {
			t.Fatal(err)
		}

		if _, err := stream.Recv(); status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("foreign binding %v", err)
		}
	}

	if bindings[0] == bindings[1] {
		t.Fatal("tenant session IDs collided")
	}

	unauthorized, err := client.GameSession(metadata.AppendToOutgoingContext(ctx, "x-api-key", "missing"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := unauthorized.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("authentication %v", err)
	}
}

func TestLostCloseExpiresIdleStreamAndDetachedOwner(t *testing.T) {
	registry, err := session.NewRegistry(script.Options{}, session.Limits{
		Sessions:  1,
		PerTenant: 1,
		PerRelay:  1,
		Grace:     10 * time.Millisecond,
		Timeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer registry.Close()
	service := New(registry, map[string]string{"key": "tenant"}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	service.idleTimeout = 20 * time.Millisecond
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	pb.RegisterBotServer(server, service)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() {
		server.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	cc, err := grpc.NewClient("passthrough:///memory", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}

	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stream, err := pb.NewBotClient(cc).GameSession(metadata.AppendToOutgoingContext(ctx, "x-api-key", "key"))
	if err != nil {
		t.Fatal(err)
	}

	identity := &pb.OpenSession{
		ApiVersion:      contract.APIVersion,
		CodecRevision:   contract.CodecRevision,
		RelayInstanceId: "relay",
		GameSessionId:   "game",
		LocalStream:     1,
	}
	if err := stream.Send(&pb.RelayMessage{Payload: &pb.RelayMessage_Open{Open: identity}}); err != nil {
		t.Fatal(err)
	}

	answer, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}

	binding := answer.GetOpened()
	if _, err := stream.Recv(); status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("idle stream=%v", err)
	}

	registry.Expire(time.Now().Add(time.Second))
	if _, _, _, err := registry.Open(ctx, "tenant", identity, &pb.ResumeSession{Identity: identity, BotSessionId: binding.BotSessionId, Generation: binding.Generation}); err == nil {
		t.Fatal("expired owner resumed")
	}

	if _, _, _, err := registry.Open(ctx, "tenant", identity, nil); err != nil {
		t.Fatalf("expired owner retained its slot: %v", err)
	}
}
