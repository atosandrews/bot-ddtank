package session

import (
	"context"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func (s *Session) commandReceipt(ctx context.Context, receipt *pb.CommandReceipt) ([]*pb.BotMessage, error) {
	pending, exists := s.pending[receipt.CommandId]
	if err := s.receipt(receipt); err != nil {
		return nil, err
	}

	if !exists || pending.scriptGeneration != s.scriptGeneration || !s.state.CanSend() {
		return nil, nil
	}

	result, err := s.state.Action(receipt.CommandId)
	if err != nil {
		return nil, err
	}

	if err := s.runtime.Action(ctx, result); err != nil {
		s.state.Invalidate("action callback failed")

		return nil, err
	}

	return s.collect()
}
