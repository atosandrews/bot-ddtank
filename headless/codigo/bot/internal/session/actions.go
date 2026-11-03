package session

import (
	"google.golang.org/protobuf/proto"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/actionwire"
	"ddbot/bot/internal/game/action"
)

func (s *Session) collect() ([]*pb.BotMessage, error) {
	requests := s.state.Pending()
	if len(requests) == 0 {
		return nil, nil
	}

	if len(s.pending)+len(requests) > contract.MaxCommands {
		for _, r := range requests {
			s.state.Report(action.Result{
				ID:     r.ID,
				Name:   r.Command.Name(),
				Status: "rejected",
				Reason: "waiting for command receipts",
			})
		}

		return nil, nil
	}

	s.batchID++
	batch := &pb.CommandBatch{
		Id:               s.batchID,
		BaseSequence:     s.sequence,
		Generation:       s.generation,
		ScriptGeneration: s.scriptGeneration,
	}

	for _, r := range requests {
		if err := s.state.Validate(r); err != nil {
			s.state.Report(action.Result{
				ID:     r.ID,
				Name:   r.Command.Name(),
				Status: "rejected",
				Reason: err.Error(),
			})
			continue
		}

		command, err := actionwire.Encode(r.ID, r.Command)
		if err != nil {
			s.state.Report(action.Result{
				ID:     r.ID,
				Name:   r.Command.Name(),
				Status: "rejected",
				Reason: err.Error(),
			})
			continue
		}

		batch.Commands = append(batch.Commands, command)
	}

	if proto.Size(batch) > contract.MaxMessageBytes/2 {
		for _, r := range requests {
			s.state.Report(action.Result{
				ID:     r.ID,
				Name:   r.Command.Name(),
				Status: "rejected",
				Reason: "command batch exceeds size limit",
			})
		}

		return nil, nil
	}

	for _, r := range requests {
		for _, c := range batch.Commands {
			if r.ID == c.Id {
				s.pending[r.ID] = pendingCommand{batch: s.batchID, name: r.Command.Name(), scriptGeneration: s.scriptGeneration}
				s.state.Report(action.Result{ID: r.ID, Name: r.Command.Name(), Status: "dispatched"})
			}
		}
	}

	if len(batch.Commands) == 0 {
		return nil, nil
	}

	return []*pb.BotMessage{{Payload: &pb.BotMessage_Batch{Batch: batch}}}, nil
}

func (s *Session) receipt(r *pb.CommandReceipt) error {
	receiptStatus, err := contract.ReceiptStatusName(r.Status)
	if err != nil {
		return err
	}

	pending, ok := s.pending[r.CommandId]
	if !ok {
		if r.CommandId <= s.state.LastActionID() {
			return nil
		}

		return &Error{Code: "invalid_receipt", Reason: "unknown command"}
	}

	if r.BatchId != pending.batch {
		return &Error{Code: "invalid_receipt", Reason: "batch mismatch"}
	}

	reason := ""
	if r.Error != nil {
		reason = r.Error.Message
	}

	s.state.Report(action.Result{
		ID:     r.CommandId,
		Name:   pending.name,
		Status: receiptStatus,
		Reason: reason,
	})
	delete(s.pending, r.CommandId)
	if r.Status == pb.ReceiptStatus_RECEIPT_STATUS_UNCERTAIN {
		s.state.Invalidate("uncertain command outcome")
	}

	return nil
}

func (s *Session) bind(generation uint64, resume *pb.ResumeSession) ([]*pb.BotMessage, error) {
	if generation <= s.generation || s.attached {
		return nil, &Error{Code: "duplicate_binding", Reason: "session already attached or stale generation"}
	}

	if resume != nil {
		if resume.EventCheckpoint > s.sequence {
			return nil, &Error{Code: "resync_required", Reason: "bot lost acknowledged events"}
		}

		for _, r := range resume.Receipts {
			if err := contract.ValidateRelay(&pb.RelayMessage{Payload: &pb.RelayMessage_Receipt{Receipt: r}}); err != nil {
				return nil, err
			}

			if err := s.receipt(r); err != nil {
				return nil, err
			}
		}

		for id, pending := range s.pending {
			status := "cancelled"
			reason := "unstarted command cancelled during reconnect"
			if id <= resume.CommandHighWater {
				status = "uncertain"
				reason = "command outcome missing after reconnect"
				s.state.Invalidate(reason)
			}

			s.state.Report(action.Result{
				ID:     id,
				Name:   pending.name,
				Status: status,
				Reason: reason,
			})
			delete(s.pending, id)
		}
	}

	s.generation = generation
	s.attached = true
	s.state.SetWritable(s.initialized)

	return []*pb.BotMessage{{Payload: &pb.BotMessage_Opened{Opened: &pb.SessionOpened{
		BotSessionId:     s.id,
		Generation:       generation,
		AppliedSequence:  s.sequence,
		ApiVersion:       contract.APIVersion,
		CodecRevision:    contract.CodecRevision,
		ScriptGeneration: s.scriptGeneration,
	}}}}, nil
}
