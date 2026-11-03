package link

import (
	"context"
	"fmt"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
)

type connection struct {
	link       *Link
	opened     *pb.SessionOpened
	history    *eventHistory
	incoming   <-chan received
	writes     chan<- *pb.RelayMessage
	writeError <-chan error
	outbox     []*pb.RelayMessage
	pending    map[string]*request
	tick       *time.Ticker
}

func (c *connection) run(ctx context.Context) error {
	for !c.link.stopped.Load() {
		c.link.diagnostics.Store(&Diagnostics{
			HistoryEvents:   len(c.history.events),
			HistoryBytes:    c.history.bytes,
			PendingRequests: len(c.pending),
			AppliedSequence: c.history.applied,
		})
		var send chan<- *pb.RelayMessage
		var next *pb.RelayMessage
		if len(c.outbox) > 0 {
			send = c.writes
			next = c.outbox[0]
			next.BotSessionId = c.opened.BotSessionId
			next.Generation = c.opened.Generation
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.tick.C:
			c.outbox = append(c.outbox, &pb.RelayMessage{Payload: &pb.RelayMessage_Heartbeat{Heartbeat: &pb.Heartbeat{AppliedSequence: c.history.applied}}})
		case err := <-c.writeError:
			return err
		case send <- next:
			c.outbox[0] = nil
			c.outbox = c.outbox[1:]
		case item := <-c.link.input:
			if err := c.enqueue(item); err != nil {
				return err
			}
		case item := <-c.incoming:
			if err := c.receive(item); err != nil {
				return err
			}
		}

		if len(c.outbox) > contract.MaxEvents+128 {
			c.link.Invalidate("outbound queue is full")

			return &Error{Code: "queue_full"}
		}
	}

	return &Error{Code: "automation_stopped", Reason: "automation stopped"}
}

func (c *connection) receive(item received) error {
	if item.err != nil {
		return item.err
	}

	if err := contract.ValidateBot(item.message); err != nil {
		c.link.Invalidate("invalid bot message")

		return err
	}

	switch v := item.message.Payload.(type) {
	case *pb.BotMessage_Applied:
		if err := c.history.acknowledge(v.Applied.Sequence); err != nil {
			c.link.Invalidate("invalid event acknowledgement")

			return err
		}
	case *pb.BotMessage_Batch:
		if v.Batch.Generation != c.opened.Generation {
			c.link.Invalidate("invalid command binding")

			return &Error{Code: "invalid_batch"}
		}
		select {
		case c.link.commands <- v.Batch:
		default:
			c.link.Invalidate("command inbox full")
			return &Error{Code: "queue_full"}
		}
	case *pb.BotMessage_Result:
		if v.Result.Error == nil {
			state := c.link.State()
			if v.Result.ScriptGeneration > state.ScriptGeneration {
				state.ScriptGeneration = v.Result.ScriptGeneration
			}

			c.link.state.Store(&state)
		}
		if v.Result.RequestId == "relay-startup-script" {
			if v.Result.Error != nil {
				c.link.Invalidate("startup script failed")

				return &Error{Code: "script_failed", Reason: v.Result.Error.GetMessage()}
			}

			return nil
		}
		if request, ok := c.pending[v.Result.RequestId]; ok {
			request.respond(result{value: v.Result})
			delete(c.pending, v.Result.RequestId)
		}
	case *pb.BotMessage_Heartbeat:
	case *pb.BotMessage_Failure:
		c.link.Invalidate("bot requires resynchronization")
		return &Error{Code: "resync_required"}
	case *pb.BotMessage_Log:
		c.link.logger.Info("script log", "bot_session_id", c.opened.BotSessionId, "message", v.Log.Message)
	default:
		c.link.Invalidate("unsupported bot message")
		return &Error{Code: "invalid_message", Reason: fmt.Sprintf("unsupported payload %T", item.message.Payload)}
	}

	return nil
}

func (c *connection) enqueue(item input) error {
	c.link.queued.Add(-int64(item.size))
	if event := item.message.GetPacket(); event != nil {
		if err := c.history.append(event); err != nil {
			c.link.Invalidate(err.Error())

			return err
		}
	}

	if item.request != nil {
		if item.context.Err() != nil {
			item.request.respond(result{err: item.context.Err()})

			return nil
		}

		id, err := requestID(item.message)
		if err != nil {
			item.request.respond(result{err: err})

			return nil
		}

		if _, exists := c.pending[id]; exists || len(c.pending) >= 32 {
			item.request.respond(result{err: &Error{Code: "request_limit", Reason: "request is already pending or queue is full"}})

			return nil
		}

		c.pending[id] = item.request
	}

	c.outbox = append(c.outbox, item.message)

	return nil
}
