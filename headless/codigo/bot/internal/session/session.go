package session

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/game"
	"ddbot/bot/internal/metrics"
	"ddbot/bot/internal/projection"
	"ddbot/bot/internal/script"
)

const emptyScript = `return {api_version=1}`

type Session struct {
	logs                                            *logBuffer
	id                                              string
	state                                           *game.Session
	projector                                       *projection.Projector
	metrics                                         *metrics.Recorder
	runtime                                         *script.Runtime
	options                                         script.Options
	timeout                                         time.Duration
	inbox                                           chan work
	observations                                    chan observationRequest
	observationBusy                                 atomic.Bool
	scriptLoaded                                    bool
	context                                         context.Context
	cancel                                          context.CancelFunc
	done                                            chan struct{}
	closeOnce                                       sync.Once
	generation, sequence, scriptGeneration, batchID uint64
	initialized, attached                           bool
	pending                                         map[uint64]pendingCommand
	requests                                        map[string]*requestResult
	requestOrder                                    []string
	resultBytes                                     int
}

func New(id string, stream uint64, options script.Options, timeout time.Duration) (*Session, error) {
	state := game.NewSession(stream, false)
	logs := &logBuffer{}
	runtime, err := script.NewRuntimeWithOptions(state, []byte(emptyScript), logs, timeout, options)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		logs:         logs,
		id:           id,
		state:        state,
		projector:    projection.NewProjector(state),
		metrics:      metrics.New(time.Now()),
		runtime:      runtime,
		options:      options,
		timeout:      timeout,
		inbox:        make(chan work, 8),
		observations: make(chan observationRequest),
		context:      ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
		pending:      make(map[uint64]pendingCommand),
		requests:     make(map[string]*requestResult),
	}
	go s.run()

	return s, nil
}

func (s *Session) Close() { s.closeOnce.Do(func() { s.cancel(); <-s.done }) }

func (s *Session) Process(ctx context.Context, generation uint64, message *pb.RelayMessage) ([]*pb.BotMessage, error) {
	return s.call(work{context: ctx, generation: generation, message: message})
}

func (s *Session) Bind(ctx context.Context, generation uint64, resume *pb.ResumeSession) ([]*pb.BotMessage, error) {
	return s.call(work{
		context:    ctx,
		generation: generation,
		attach:     true,
		resume:     resume,
	})
}

func (s *Session) Detach(ctx context.Context, generation uint64) error {
	_, err := s.call(work{context: ctx, generation: generation})

	return err
}

func (s *Session) call(w work) ([]*pb.BotMessage, error) {
	w.reply = make(chan reply, 1)

	select {
	case s.inbox <- w:
	case <-w.context.Done():
		return nil, w.context.Err()
	case <-s.done:
		return nil, &Error{Code: "session_closed"}
	}

	select {
	case r := <-w.reply:
		return r.messages, r.err
	case <-w.context.Done():
		return nil, w.context.Err()
	case <-s.done:
		return nil, &Error{Code: "session_closed"}
	}
}

func (s *Session) run() {
	defer close(s.done)
	defer func() { s.runtime.Close() }()

	for {
		select {
		case <-s.context.Done():
			return
		case request := <-s.observations:
			s.respondObservation(request)
		case w := <-s.inbox:
			ctx, cancel := context.WithCancel(w.context)
			stop := context.AfterFunc(s.context, cancel)
			s.logs.take()
			messages, err := s.handle(ctx, w)
			if log := s.logs.take(); log != "" {
				messages = append(messages, &pb.BotMessage{Payload: &pb.BotMessage_Log{Log: &pb.Log{Message: log}}})
			}
			stop()
			cancel()
			w.reply <- reply{messages: messages, err: err}
		}
	}
}

func (s *Session) handle(ctx context.Context, w work) ([]*pb.BotMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if w.attach {
		return s.bind(w.generation, w.resume)
	}

	if w.generation != s.generation {
		return nil, &Error{Code: "stale_binding", Reason: "binding generation changed"}
	}

	if w.tick {
		return s.tick(ctx)
	}

	if w.message == nil {
		s.attached = false
		s.state.SetWritable(false)

		return nil, nil
	}

	if !s.attached {
		return nil, &Error{Code: "session_detached"}
	}

	if err := contract.ValidateRelay(w.message); err != nil {
		return nil, err
	}

	switch v := w.message.Payload.(type) {
	case *pb.RelayMessage_Packet:
		return s.packet(ctx, v.Packet)
	case *pb.RelayMessage_Evaluate, *pb.RelayMessage_Load, *pb.RelayMessage_Stop:
		return s.scriptRequest(ctx, w.message)
	case *pb.RelayMessage_Receipt:
		return s.commandReceipt(ctx, v.Receipt)
	case *pb.RelayMessage_Gap:
		s.state.Invalidate("relay reported a gap")
		return nil, &Error{Code: "resync_required", Reason: "relay reported a gap"}
	case *pb.RelayMessage_Close:
		s.state.Invalidate("game connection closed")
		return nil, s.runtime.Stop(ctx, "game connection closed")
	case *pb.RelayMessage_Heartbeat:
		return []*pb.BotMessage{{Payload: &pb.BotMessage_Heartbeat{Heartbeat: &pb.Heartbeat{AppliedSequence: s.sequence}}}}, nil
	default:
		return nil, &Error{Code: "invalid_operation", Reason: "unexpected message after opening"}
	}
}
