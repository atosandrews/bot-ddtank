package session

import (
	"context"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func (s *Session) Tick(ctx context.Context, generation uint64) ([]*pb.BotMessage, error) {
	return s.call(work{context: ctx, generation: generation, tick: true})
}

func (s *Session) tick(ctx context.Context) ([]*pb.BotMessage, error) {
	if !s.attached || !s.state.CanSend() {
		return nil, nil
	}

	if err := s.runtime.Tick(ctx); err != nil {
		s.state.Invalidate("timer callback failed")

		return nil, err
	}

	return s.collect()
}
