package transport

import (
	"context"
	"fmt"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/link"
)

func (c *Connection) Inspect(ctx context.Context) (Info, error) {
	value, err := c.call(ctx, inspectOperation, framing.Frame{})
	if err != nil {
		return Info{}, err
	}

	return value.info, nil
}

func (c *Connection) Checkpoint(ctx context.Context) (link.Checkpoint, error) {
	value, err := c.call(ctx, checkpointOperation, framing.Frame{})
	if err != nil {
		return link.Checkpoint{}, err
	}

	return value.checkpoint, nil
}

func (c *Connection) Request(ctx context.Context, id, operation, source string) (*pb.ScriptResult, error) {
	return c.automation.Request(ctx, id, operation, source)
}

func (c *Connection) Inject(ctx context.Context, frame framing.Frame) (int, error) {
	value, err := c.call(ctx, injectOperation, frame)
	if err != nil {
		return 0, err
	}

	return value.written, nil
}

func (c *Connection) inject(ctx context.Context, frame framing.Frame) (int, error) {
	written, record, err := c.send(ctx, frame)
	if written <= 0 || err != nil {
		return written, err
	}

	if observeErr := c.observeCommand(ctx, record, 0); observeErr != nil {
		return written, observeErr
	}

	return written, nil
}

func (c *Connection) call(ctx context.Context, operation controlOperation, frame framing.Frame) (controlReply, error) {
	request := controlCall{context: ctx, operation: operation, frame: frame, response: make(chan controlReply, 1)}

	select {
	case c.calls <- request:
	case <-ctx.Done():
		return controlReply{}, ctx.Err()
	case <-c.done:
		return controlReply{}, &ControlError{Code: "session_unavailable"}
	}

	select {
	case reply := <-request.response:
		return reply, reply.err
	case <-ctx.Done():
		return controlReply{}, ctx.Err()
	case <-c.done:
		return controlReply{}, &ControlError{Code: "session_unavailable"}
	}
}

func (c *Connection) handleControl(request controlCall) error {
	if err := request.context.Err(); err != nil {
		request.response <- controlReply{err: err}

		return nil
	}

	switch request.operation {
	case inspectOperation:
		state := c.automation.State()
		characterID := c.characterID
		if !c.gameReady || c.closing {
			characterID = 0
		}
		request.response <- controlReply{info: Info{
			Diagnostics:       state.Diagnostics,
			EventSequence:     c.sequence,
			StaleRejections:   c.staleRejections,
			UncertainCommands: c.uncertainCommands,
			Session:           c.stream,
			Client:            c.client.RemoteAddr().String(),
			Upstream:          c.server.RemoteAddr().String(),
			CipherReady:       !c.closing && c.relay.Ready(),
			GameReady:         c.gameReady && !c.closing,
			CharacterID:       characterID,
			BotAttached:       state.Attached,
			BotSessionID:      state.BotSessionID,
			Generation:        state.Generation,
			Reason:            state.Reason,
		}}
	case checkpointOperation:
		request.response <- controlReply{checkpoint: link.Checkpoint{HighWater: c.highWater, Receipts: append([]*pb.CommandReceipt(nil), c.receipts...)}}
	case injectOperation:
		written, err := c.inject(request.context, request.frame)
		request.response <- controlReply{written: written, err: err}
	default:
		request.response <- controlReply{err: &ControlError{Code: "invalid_operation", Reason: fmt.Sprintf("unknown operation %d", request.operation)}}
	}

	return nil
}
