package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/link"
	"ddbot/relay/internal/packets"
	"ddbot/relay/internal/protocol"
)

const writeTimeout = 10 * time.Second

type Recorder interface{ Write(capture.Record) error }

type View func(capture.Record, []framing.Segment, []packets.Result) error

type Connection struct {
	stream             uint64
	client, server     net.Conn
	recorder           Recorder
	view               View
	bodies             *packets.Decoder
	relay              *framing.Relay
	incoming, outgoing *framing.EncryptedFramer
	automation         Automation
	staleRejections    uint64
	uncertainCommands  uint64
	sequence           uint64
	highWater          uint64
	receipts           []*pb.CommandReceipt
	closeOnce          sync.Once
	closeErr           error
	writeMu            sync.Mutex
	calls              chan controlCall
	done               chan struct{}
	gameReady          bool
	closing            bool
	clientEOF          bool
	serverEOF          bool
}

func NewConnection(stream uint64, client, server net.Conn, recorder Recorder, view View, bodies *packets.Decoder, factory func(func(context.Context) (link.Checkpoint, error)) Automation) *Connection {
	c := &Connection{
		calls:    make(chan controlCall),
		done:     make(chan struct{}),
		stream:   stream,
		client:   client,
		server:   server,
		recorder: recorder,
		view:     view,
		bodies:   bodies,
		relay:    framing.NewRelay(),
		incoming: framing.NewEncryptedFramer(),
		outgoing: framing.NewEncryptedFramer(),
	}
	c.automation = factory(c.Checkpoint)

	return c
}

func (c *Connection) Run(ctx context.Context) (result error) {
	defer close(c.done)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, c.close)
	defer stop()
	go c.automation.Run(ctx)
	var readers sync.WaitGroup
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
		result = errors.Join(result, c.automation.Finish(closeCtx))
		closeCancel()
		cancel()
		c.close()
		readers.Wait()
		<-c.automation.Done()
		result = errors.Join(result, c.closeErr, c.relay.Finish(), c.drain())
	}()
	input := make(chan input)
	readers.Add(2)
	go func() { defer readers.Done(); readStream(ctx, c.client, capture.ClientToServer, input) }()
	go func() { defer readers.Done(); readStream(ctx, c.server, capture.ServerToClient, input) }()

	for {
		select {
		case <-ctx.Done():
			return nil
		case batch := <-c.automation.Commands():
			if err := c.execute(ctx, batch); err != nil {
				return err
			}
		case request := <-c.calls:
			if err := c.handleControl(request); err != nil {
				return err
			}
		case item := <-input:
			if item.err != nil {
				if ctx.Err() != nil {
					return nil
				}

				if errors.Is(item.err, io.EOF) {
					finished, err := c.finishInput(item.direction)
					if err != nil || finished {
						return err
					}

					continue
				}

				return &Error{Operation: "read", Cause: item.err}
			}
			if err := c.forward(ctx, item); err != nil {
				return err
			}
		}
	}
}

func (c *Connection) forward(ctx context.Context, item input) error {
	wire := item.data
	destination := c.client

	switch item.direction {
	case capture.ClientToServer:
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		destination = c.server
		translated, err := c.relay.Forward(wire)
		if err != nil {
			return err
		}
		wire = translated
	case capture.ServerToClient:
	default:
		return &Error{Operation: "direction", Cause: &framing.RelayError{Reason: "unknown direction"}}
	}

	if len(wire) == 0 {
		return nil
	}

	n, writeErr := write(ctx, destination, wire)
	if n > 0 {
		record := c.record(item.direction, wire[:n], item.at)
		if err := c.recorder.Write(record); err != nil {
			return errors.Join(writeErr, &Error{Operation: "record", Cause: err})
		}

		if writeErr == nil {
			if err := c.observe(ctx, record); err != nil {
				return err
			}
		}
	}

	return writeErr
}

func (c *Connection) send(ctx context.Context, frame framing.Frame) (int, capture.Record, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	wire, err := c.relay.Inject(frame)
	if err != nil {
		return 0, capture.Record{}, err
	}

	n, writeErr := write(ctx, c.server, wire)
	var record capture.Record
	if n > 0 {
		record = c.record(capture.ClientToServer, wire[:n], time.Now())
		record.Origin = "script"
		if err := c.recorder.Write(record); err != nil {
			return n, record, errors.Join(writeErr, &Error{Operation: "record command", Cause: err})
		}
	}

	return n, record, writeErr
}

func (c *Connection) observe(ctx context.Context, record capture.Record) error {
	return c.observeCommand(ctx, record, 0)
}

func (c *Connection) observeCommand(ctx context.Context, record capture.Record, commandID uint64) error {
	framer := c.incoming
	if record.Direction == capture.ClientToServer {
		framer = c.outgoing
	}

	segments := framer.Push(record.Data)
	results := make([]packets.Result, len(segments))

	for i, segment := range segments {
		if segment.Kind != framing.FrameSegment {
			if segment.Note != "" {
				c.automation.Invalidate("packet decoding lost synchronization: " + string(record.Direction) + ": " + segment.Note)
			}

			continue
		}

		results[i] = c.bodies.Decode(record.Direction, segment.Frame)
		if !segment.Decrypted {
			continue
		}

		if record.Direction == capture.ServerToClient && segment.Frame.Header.Code == 1 && results[i].Status == "decoded" {
			c.gameReady = results[i].Fields["status"] == int8(0)
		}

		c.sequence++
		event, err := protocol.Event(c.sequence, record, segment.Frame.Header, results[i])
		if err != nil {
			c.automation.Invalidate("packet event conversion failed")
			continue
		}

		event.CommandId = commandID
		c.automation.Observe(event)
	}

	if c.view != nil {
		return c.view(record, segments, results)
	}

	return nil
}

func (c *Connection) record(direction capture.Direction, data []byte, at time.Time) capture.Record {
	source, destination := c.client.RemoteAddr().String(), c.server.RemoteAddr().String()
	if direction == capture.ServerToClient {
		source, destination = destination, source
	}

	return capture.Record{
		Time:        at,
		Stream:      c.stream,
		Direction:   direction,
		Source:      source,
		Destination: destination,
		Data:        data,
	}
}

func write(ctx context.Context, connection net.Conn, data []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, &Error{Operation: "write", Cause: err}
	}

	deadline := time.Now().Add(writeTimeout)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}

	if err := connection.SetWriteDeadline(deadline); err != nil {
		return 0, &Error{Operation: "write deadline", Cause: err}
	}

	total := 0

	for total < len(data) {
		n, err := connection.Write(data[total:])
		total += n
		if err != nil {
			return total, &Error{Operation: "write", Cause: err}
		}

		if n == 0 {
			return total, &Error{Operation: "write", Cause: io.ErrShortWrite}
		}
	}

	return total, nil
}

func closeError(err error) error {
	if errors.Is(err, net.ErrClosed) {
		return nil
	}

	return err
}

func (c *Connection) close() {
	c.closeOnce.Do(func() { c.closeErr = errors.Join(closeError(c.client.Close()), closeError(c.server.Close())) })
}

func (c *Connection) drain() error {
	var result error

	for _, direction := range []capture.Direction{capture.ClientToServer, capture.ServerToClient} {
		framer := c.incoming
		if direction == capture.ClientToServer {
			framer = c.outgoing
		}

		segments := framer.Drain()
		if c.view != nil && len(segments) > 0 {
			result = errors.Join(result, c.view(c.record(direction, nil, time.Now()), segments, make([]packets.Result, len(segments))))
		}
	}

	return result
}
