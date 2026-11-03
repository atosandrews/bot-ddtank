package grpcserver

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/session"
)

const scriptTickInterval = 100 * time.Millisecond

type Server struct {
	pb.UnimplementedBotServer
	pb.UnimplementedSessionMetricsServer
	pb.UnimplementedSessionObservationServer
	logger      *slog.Logger
	idleTimeout time.Duration
	registry    *session.Registry
	keys        map[[32]byte]string
}

func New(registry *session.Registry, keys map[string]string, logger *slog.Logger) *Server {
	hashed := make(map[[32]byte]string, len(keys))

	for key, tenant := range keys {
		hashed[sha256.Sum256([]byte(key))] = tenant
	}

	return &Server{
		registry:    registry,
		keys:        hashed,
		idleTimeout: 30 * time.Second,
		logger:      logger,
	}
}

func (s *Server) GameSession(stream grpc.BidiStreamingServer[pb.RelayMessage, pb.BotMessage]) error {
	tenant, err := s.authenticate(stream.Context())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()
	incoming := make(chan received, 1)
	outgoing := make(chan *pb.BotMessage, 2)
	sendError := make(chan error, 1)
	go receive(ctx, stream, incoming)
	go transmit(ctx, stream, outgoing, sendError)
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	var first received

	select {
	case first = <-incoming:
	case <-timer.C:
		return status.Error(codes.DeadlineExceeded, "opening timeout")
	case <-ctx.Done():
		return ctx.Err()
	}

	if first.err != nil {
		return first.err
	}

	if err := contract.ValidateRelay(first.message); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	var open *pb.OpenSession
	var resume *pb.ResumeSession

	switch v := first.message.Payload.(type) {
	case *pb.RelayMessage_Open:
		open = v.Open
	case *pb.RelayMessage_Resume:
		resume = v.Resume
		open = resume.Identity
	default:
		return status.Errorf(codes.InvalidArgument, "open or resume required, got %T", first.message.Payload)
	}

	owner, messages, generation, err := s.registry.Open(ctx, tenant, open, resume)
	if err != nil {
		var domain *session.Error
		if errors.As(err, &domain) && domain.Code == "session_attached" {
			return status.Error(codes.AlreadyExists, domain.Error())
		}

		return status.Error(codes.FailedPrecondition, err.Error())
	}

	defer s.registry.Release(tenant, open, generation)
	botID := messages[0].GetOpened().BotSessionId
	s.logger.InfoContext(ctx, "session attached", "tenant", tenant, "bot_session_id", botID, "generation", generation)
	defer s.logger.Info("session detached", "tenant", tenant, "bot_session_id", botID, "generation", generation)
	if err := send(ctx, outgoing, messages); err != nil {
		return err
	}

	timer.Reset(s.idleTimeout)
	ticks := time.NewTicker(scriptTickInterval)
	defer ticks.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return status.Error(codes.DeadlineExceeded, "heartbeat timeout")
		case err := <-sendError:
			return err
		case <-ticks.C:
			messages, err := owner.Tick(ctx, generation)
			if err != nil {
				s.logger.ErrorContext(ctx, "session tick failed", "bot_session_id", botID, "error", err)

				return status.Error(codes.FailedPrecondition, err.Error())
			}

			if err := send(ctx, outgoing, messages); err != nil {
				return err
			}
		case item := <-incoming:
			if errors.Is(item.err, io.EOF) {
				return nil
			}
			if item.err != nil {
				return item.err
			}
			timer.Reset(s.idleTimeout)
			if item.message.GetBotSessionId() != botID || item.message.GetGeneration() != generation {
				return status.Error(codes.FailedPrecondition, "foreign or stale session binding")
			}
			messages, err := owner.Process(ctx, generation, item.message)
			if err != nil {
				s.logger.ErrorContext(ctx, "session processing failed", "bot_session_id", botID, "error", err)

				return status.Error(codes.FailedPrecondition, err.Error())
			}
			if err := send(ctx, outgoing, messages); err != nil {
				return err
			}
			if item.message.GetClose() != nil {
				s.registry.Remove(tenant, open, generation)

				return nil
			}
		}
	}
}

func (s *Server) authenticate(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "API key required")
	}

	keys := md.Get("x-api-key")
	if len(keys) != 1 {
		return "", status.Error(codes.Unauthenticated, "one API key required")
	}

	tenant, ok := s.keys[sha256.Sum256([]byte(keys[0]))]
	if !ok {
		return "", status.Error(codes.Unauthenticated, "unknown API key")
	}

	return tenant, nil
}

func send(ctx context.Context, queue chan<- *pb.BotMessage, messages []*pb.BotMessage) error {
	for _, message := range messages {
		select {
		case queue <- message:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return status.Error(codes.ResourceExhausted, "bot response queue is full")
		}
	}

	return nil
}
