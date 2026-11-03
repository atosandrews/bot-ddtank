package grpcserver

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/session"
)

const maxObservationSessionIDBytes = 128

func (s *Server) GetSessionObservation(ctx context.Context, request *pb.GetSessionObservationRequest) (*pb.GetSessionObservationResponse, error) {
	tenant, err := s.authenticate(ctx)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}

	id := request.GetBotSessionId()
	if id == "" || len(id) > maxObservationSessionIDBytes || strings.TrimSpace(id) != id {
		return nil, status.Error(codes.InvalidArgument, "bot_session_id requires 1 to 128 bytes without surrounding whitespace")
	}

	result, err := s.registry.Observation(ctx, tenant, id)
	if err != nil {
		return nil, observationError(err)
	}

	return result, nil
}

func observationError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Err()
	}

	var domain *session.Error
	if !errors.As(err, &domain) {
		return status.Error(codes.Internal, "session observation unavailable")
	}

	switch domain.Code {
	case "session_not_found", "session_closed":
		return status.Error(codes.NotFound, "session not found")
	case "service_closed":
		return status.Error(codes.Unavailable, "bot is stopping")
	case "observation_busy", "observation_limit":
		return status.Error(codes.ResourceExhausted, domain.Code)
	default:
		return status.Error(codes.Internal, "session observation unavailable")
	}
}
