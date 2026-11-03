package link

import (
	"context"
	"crypto/sha256"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func (l *Link) run(ctx context.Context) {
	history := &eventHistory{}
	outage := time.Now()

	for ctx.Err() == nil && !l.stopped.Load() {
		old := l.State()
		old.Attached = false
		old.Reason = "connecting"
		l.state.Store(&old)
		var resume *pb.ResumeSession
		if old.BotSessionID != "" {
			checkpoint, err := l.checkpoint(ctx)
			if err != nil {
				l.Invalidate("cannot synchronize command ledger")

				return
			}

			resume = &pb.ResumeSession{
				Identity:         l.identity,
				BotSessionId:     old.BotSessionID,
				Generation:       old.Generation,
				EventCheckpoint:  history.applied,
				CommandHighWater: checkpoint.HighWater,
				Receipts:         checkpoint.Receipts,
			}
		}

		stream, opened, cancel, err := l.connect(ctx, resume)
		if err != nil {
			code := status.Code(err)
			if code == codes.FailedPrecondition || code == codes.InvalidArgument || code == codes.Unauthenticated || code == codes.Unimplemented || time.Since(outage) > 120*time.Second {
				l.Invalidate("resynchronization required: " + err.Error())

				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				continue
			}
		}

		if err := history.acknowledge(opened.AppliedSequence); err != nil {
			cancel()
			l.Invalidate("invalid event checkpoint from bot")

			return
		}

		l.state.Store(&State{
			BotSessionID:     opened.BotSessionId,
			Generation:       opened.Generation,
			ScriptGeneration: opened.ScriptGeneration,
			Attached:         true,
		})
		outbox := make([]*pb.RelayMessage, 0, len(history.events)+1)
		if l.source != "" {
			hash := sha256.Sum256([]byte(l.source))
			outbox = append(outbox, &pb.RelayMessage{Payload: &pb.RelayMessage_Load{Load: &pb.LoadScript{RequestId: "relay-startup-script", Source: l.source, Sha256: hash[:]}}})
		}

		for _, event := range history.events {
			outbox = append(outbox, &pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: event}})
		}

		incoming := make(chan received, 4)
		writes := make(chan *pb.RelayMessage)
		writeError := make(chan error, 1)
		streamCtx := stream.Context()
		var workers sync.WaitGroup
		workers.Add(2)
		go func() {
			defer workers.Done()

			for {
				message, err := stream.Recv()

				select {
				case incoming <- received{message: message, err: err}:
				case <-streamCtx.Done():
					return
				}

				if err != nil {
					return
				}
			}
		}()
		go func() {
			defer workers.Done()

			for {
				select {
				case message := <-writes:
					if err := stream.Send(message); err != nil {
						writeError <- err

						return
					}
				case <-streamCtx.Done():
					return
				}
			}
		}()
		pending := make(map[string]*request)
		tick := time.NewTicker(10 * time.Second)
		connection := &connection{
			link:       l,
			opened:     opened,
			incoming:   incoming,
			writes:     writes,
			writeError: writeError,
			outbox:     outbox,
			pending:    pending,
			history:    history,
			tick:       tick,
		}
		err = connection.run(streamCtx)
		tick.Stop()
		cancel()
		workers.Wait()
		l.disconnects.Add(1)
		old = l.State()
		old.Attached = false
		old.Reason = "bot link disconnected"
		if err != nil {
			old.Reason += ": " + err.Error()
		}
		l.state.Store(&old)

		for _, request := range pending {
			request.respond(result{err: &Error{Code: "outcome_unknown", Reason: "bot link disconnected after request submission"}})
		}

		if err != nil && (status.Code(err) == codes.FailedPrecondition || status.Code(err) == codes.InvalidArgument || status.Code(err) == codes.Unauthenticated || status.Code(err) == codes.Unimplemented) {
			l.Invalidate("resynchronization required: " + err.Error())

			return
		}

		if l.closing.Load() {
			return
		}

		outage = time.Now()
	}
}

func requestID(message *pb.RelayMessage) (string, error) {
	switch v := message.Payload.(type) {
	case *pb.RelayMessage_Evaluate:
		return v.Evaluate.RequestId, nil
	case *pb.RelayMessage_Load:
		return v.Load.RequestId, nil
	case *pb.RelayMessage_Stop:
		return v.Stop.RequestId, nil
	default:
		return "", &Error{Code: "invalid_request", Reason: "script request required"}
	}
}
